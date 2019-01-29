package prometheus

import (
	"encoding/json"
	"fmt"
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

	bucketOwner := getBucketOwnerFromRequest("MunVJU9Em4pszZYX")
	//bucketOwner := getBucketOwnerFromRequest(m.ak)
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

func getBucketOwnerFromRequest(ak string) (bucketOwner string) {
	resp, err := http.Get("https://unicloud.com:12011/iam/v1/access/" + ak)
	body, err := ioutil.ReadAll(resp.Body)
	fmt.Println("body:", string(body))
	fmt.Println("error:", err)

	var respBody respBody
	json.Unmarshal([]byte(string(body)), &respBody)
	userId := respBody.UserId

	fmt.Println("jsonBody:", respBody)
	fmt.Println("user_id:", respBody.UserId)
	resp.Body.Close()
	return userId
}

type respBody struct {
	AccessKey    string `json:"access_key"`
	AccessSecret string `json:"access_secret"`
	UserId       string `json:"user_id"`
	ProjectId    string `json:"project_id"`
	ProjectName  string `json:"project_name"`
	CreateAt     string `json:"create_at"`
	ExpiredAt    string `json:"expired_at"`
	Enabled      string `json:"enabled"`
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
