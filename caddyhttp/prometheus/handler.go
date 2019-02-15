package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/journeymidnight/yig/circuitbreak"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/journeymidnight/yig-front-caddy/caddyhttp/httpserver"
)

func (m *Metrics) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	next := m.next

	hostname := m.hostname

	if hostname == "" {
		originalHostname, err := host(r)
		if err != nil {
			hostname = "-"
		} else {
			hostname = originalHostname
		}
	}
	start := time.Now()

	// Record response to get status code and size of the reply.
	rw := httpserver.NewResponseRecorder(w)
	// Get time to first write.
	tw := &timedResponseWriter{ResponseWriter: rw}

	status, err := next.ServeHTTP(tw, r)

	// If nothing was explicitly written, consider the request written to
	// now that it has completed.
	tw.didWrite()

	// Transparently capture the status code so as to not side effect other plugins
	stat := status
	if err != nil && status == 0 {
		// Some middlewares set the status to 0, but return an non nil error: map these to status 500
		stat = 500
	} else if status == 0 {
		// 'proxy' returns a status code of 0, but the actual status is available on rw.
		// Note that if 'proxy' encounters an error, it returns the appropriate status code (such as 502)
		// from ServeHTTP and is captured above with 'stat := status'.
		stat = rw.Status()
	}

	fam := "1"
	if isIPv6(r.RemoteAddr) {
		fam = "2"
	}

	proto := strconv.Itoa(r.ProtoMajor)
	proto = proto + "." + strconv.Itoa(r.ProtoMinor)

	statusStr := strconv.Itoa(stat)

	requestCount.WithLabelValues(hostname, fam, proto).Inc()
	requestDuration.WithLabelValues(hostname, fam, proto).Observe(time.Since(start).Seconds())
	responseSize.WithLabelValues(hostname, fam, proto, statusStr).Observe(float64(rw.Size()))
	responseStatus.WithLabelValues(hostname, fam, proto, statusStr).Inc()
	responseLatency.WithLabelValues(hostname, fam, proto, statusStr).Observe(tw.firstWrite.Sub(start).Seconds())

	// prometheus exporter
	// current is bucket_name, method, status
	var labelValues []string
	var isInternal = "n"
	bucketName, _ := getBucketAndObjectInfoFromRequest(m.s3Endpoint, r)

	if strings.TrimSpace(bucketName) == "" {
		bucketName = "-"
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if isPrivateSubnet(net.ParseIP(ip)) {
		isInternal = "y"
	}

	bucketOwner := getBucketOwnerFromRequest(bucketName, m.yigUrl)
	if strings.TrimSpace(bucketOwner) == "" {
		bucketOwner = "-"
	}

	labelValues = append(labelValues, bucketName, r.Method, statusStr, isInternal, bucketOwner)
	countTotal.WithLabelValues(labelValues...).Inc()
	bytesTotal.WithLabelValues(labelValues...).Add(float64(rw.Size()))

	upstream_response_time := time.Since(start).Seconds()
	upstreamSeconds.WithLabelValues(labelValues...).Observe(upstream_response_time)
	upstreamSecondsHist.WithLabelValues(labelValues...).Observe(upstream_response_time)

	request_time := tw.firstWrite.Sub(start).Seconds() + upstream_response_time
	responseSecondsHist.WithLabelValues(labelValues...).Observe(request_time)

	return status, err
}

func host(r *http.Request) (string, error) {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		if !strings.Contains(r.Host, ":") {
			return strings.ToLower(r.Host), nil
		}
		return "", err
	}
	return strings.ToLower(host), nil
}

func isIPv6(addr string) bool {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		// Strip away the port.
		addr = host
	}
	ip := net.ParseIP(addr)
	return ip != nil && ip.To4() == nil
}

func getBucketAndObjectInfoFromRequest(s3Endpoint string, r *http.Request) (bucketName string, objectName string) {
	splits := strings.SplitN(r.URL.Path[1:], "/", 2)
	v := strings.Split(r.Host, ":")
	hostWithOutPort := v[0]
	if strings.HasSuffix(hostWithOutPort, "."+s3Endpoint) {
		bucketName = strings.TrimSuffix(hostWithOutPort, "."+s3Endpoint)
		if len(splits) == 1 {
			objectName = splits[0]
		}
	} else {
		if len(splits) == 1 {
			bucketName = splits[0]
		}
		if len(splits) == 2 {
			bucketName = splits[0]
			objectName = splits[1]
		}
	}
	return
}

var client = &http.Client{}

func getBucketOwnerFromRequest(bucketName string, yigUrl string) (bucketOwner string) {

	if bucketName == "-" {
		bucketOwner = "-"
		return bucketOwner
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"bucket": bucketName,
	})

	//0
	tokenString, err := token.SignedString([]byte("secret"))
	if err == nil {
		//go use token
		fmt.Printf("\nHS256 = %v\n", tokenString)
	} else {
		fmt.Println("internal error", err)
		return
	}

	//1
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("panic recover!!!")
			fmt.Println("request err: \n", err)
		}
	}()
	request, err := http.NewRequest("GET", yigUrl, nil)
	request.Header.Set("Authorization", "Bearer "+tokenString)

	//2
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("panic recover!!!")
			fmt.Println("response err: \n", err)
		}
	}()

	CacheCircuit := circuitbreak.NewCacheCircuit()
	response := new(http.Response)

	circuitErr := CacheCircuit.Execute(
		context.Background(),
		func(ctx context.Context) error {
			response, err = client.Do(request)
			if err != nil {
				fmt.Println("err:", err)
				fmt.Println("admin circuit is open now!")
			}
			return nil
		},
		nil,
	)
	if circuitErr != nil {
		fmt.Println("circuit is error")
	}

	if response.StatusCode != 200 {
		fmt.Println("getBucketInfo failed as status != 200", response.StatusCode)
		return
	}
	defer response.Body.Close()

	//3
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("panic recover!!!")
			fmt.Println("io err: \n", err)
		}
	}()
	body, err := ioutil.ReadAll(response.Body)

	var respBody RespBody
	json.Unmarshal([]byte(string(body)), &respBody)
	bucketOwner = respBody.Bucket.OwnerId
	return bucketOwner
}

type RespBody struct {
	Bucket Bucket `json:"Bucket"`
}
type Bucket struct {
	Name       string `json:"Name"`
	CreateTime string `json:"CreateTime"`
	OwnerId    string `json:"OwnerId"`
	CORS       CORS   `json:"CORS"`
	ACL        ACL    `json:"ACL"`
	LC         LC     `json:"LC"`
	Policy     Policy `json:"Policy"`
	Versioning string `json:"Versioning"`
	Usage      string `json:"Usage"`
}
type CORS struct {
	CorsRules string `json:"CorsRules"`
}
type ACL struct {
	CannedAcl string `json:"CannedAcl"`
}
type LC struct {
	XMLName XMLName `json:"XMLName"`
	Rule    string  `json:"Rule"`
}

type XMLName struct {
	Space string `json:"Space"`
	Local string `json:"Local"`
}

type Policy struct {
	Version   string `json:"Version"`
	Statement string `json:"Statement"`
}

// A timedResponseWriter tracks the time when the first response write
// happened.
type timedResponseWriter struct {
	firstWrite time.Time
	http.ResponseWriter
}

func (w *timedResponseWriter) didWrite() {
	if w.firstWrite.IsZero() {
		w.firstWrite = time.Now()
	}
}

func (w *timedResponseWriter) Write(data []byte) (int, error) {
	w.didWrite()
	return w.ResponseWriter.Write(data)
}

func (w *timedResponseWriter) WriteHeader(statuscode int) {
	// We consider this a write as it's valid to respond to a request by
	// just setting a status code and returning.
	w.didWrite()
	w.ResponseWriter.WriteHeader(statuscode)
}
