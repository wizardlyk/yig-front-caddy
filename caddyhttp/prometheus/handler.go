package prometheus

import (
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
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

	bucketOwner := getBucketOwnerFromRequest("happy")
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

func getBucketOwnerFromRequest(bucket string) (bucketOwner string) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"bucket": bucket,
	})

	tokenString, err := token.SignedString([]byte("secret"))

	if err == nil {
		//go use token
		fmt.Printf("\nHS256 = %v\n", tokenString)
	} else {
		fmt.Println("internal error", err)
		return
	}

	url := "http://s3.test.com:9000/admin/bucket"
	request, _ := http.NewRequest("GET", url, nil)
	request.Header.Set("Authorization", "Bearer "+tokenString)
	response, _ := client.Do(request)
	if response.StatusCode != 200 {
		fmt.Println("getBucketInfo failed as status != 200", response.StatusCode)
		return
	}

	defer response.Body.Close()
	body, _ := ioutil.ReadAll(response.Body)

	var respBody RespBody
	json.Unmarshal([]byte(string(body)), &respBody)
	fmt.Println("respBody:", respBody)
	fmt.Println("Bucket:", respBody.Bucket)
	bucketOwner = respBody.Bucket.OwnerId
	fmt.Println("BucketOwner:", bucketOwner)
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
	xMLName XMLName `json:"XMLName"`
	rule    string  `json:"Rule"`
}

type XMLName struct {
	space string `json:"Space"`
	local string `json:"Local"`
}

type Policy struct {
	version   string `json:"Version"`
	statement string `json:"Statement"`
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
