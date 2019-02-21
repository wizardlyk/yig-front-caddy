package prometheus

import (
	"context"
	"encoding/json"
	"github.com/dgrijalva/jwt-go"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/journeymidnight/yig-front-caddy/caddyhttp/circuitbreak"
	"github.com/journeymidnight/yig-front-caddy/caddyhttp/httpserver"
)

type Cache interface {
	Put(key interface{}, value interface{}, lifeTime time.Duration) error
	Get(key interface{}) interface{}
}

type Item struct {
	Value      interface{}
	createTime time.Time
	lifeTime   time.Duration
}

type MemoryCache struct {
	syncMap sync.Map
	sync.RWMutex
	duration time.Duration
}

func (mc *MemoryCache) Put(key interface{}, value interface{}, lifeTime time.Duration) error {
	mc.syncMap.Store(key, Item{
		Value:      value,
		createTime: time.Now(),
		lifeTime:   lifeTime,
	})
	return nil
}

func (mc *MemoryCache) Get(key interface{}) string {
	if e, ok := mc.syncMap.Load(key); ok {
		return e.(Item).Value.(string)
	}
	return ""
}

func (e *Item) isExpire() bool {
	if e.lifeTime == 0 {
		return false
	}
	return time.Now().Sub(e.createTime) > e.lifeTime
}

func (mc *MemoryCache) StartTimerGC() error {
	go mc.checkAndClearExpire()
	return nil
}

//Detects and clears expired elements
func (mc *MemoryCache) checkAndClearExpire() {
	for {
		<-time.After(mc.duration)
		if keys := mc.expireKeys(); len(keys) != 0 {
			mc.clearItems(keys)
		}
	}
}

//Use expired elements to clean up the cache
func (mc *MemoryCache) clearItems(keys []interface{}) {
	mc.Lock()
	defer mc.Unlock()
	for _, key := range keys {
		mc.syncMap.Delete(key)
	}
}

//Gets the expired key
func (mc *MemoryCache) expireKeys() (keys []interface{}) {
	mc.RLock()
	defer mc.RUnlock()
	mc.syncMap.Range(func(key, value interface{}) bool {
		item := value.(Item)
		if item.isExpire() {
			keys = append(keys, key)
		}
		return true
	})
	return
}

var mc = MemoryCache{
	sync.Map{},
	sync.RWMutex{},
	0,
}

func (m *Metrics) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	checkTimeCount, err := strconv.Atoi(m.checkTime)
	checkTime := time.Duration(checkTimeCount) * time.Hour
	mc.duration = checkTime

	lifeTimeCount, err := strconv.Atoi(m.lifeTime)
	lifeTime := time.Duration(lifeTimeCount) * time.Second

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

	bucketOwner := mc.Get(bucketName)
	if bucketOwner == "" {
		bucketOwner, err = getBucketOwnerFromRequest(bucketName, m.yigUrl, time.Duration(lifeTime))
		if strings.TrimSpace(bucketOwner) == "" {
			bucketOwner = "-"
		}
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

func getBucketOwnerFromRequest(bucketName string, yigUrl string, lifeTime time.Duration) (bucketOwner string, err error) {
	client := &http.Client{}
	if bucketName == "-" {
		return "", nil
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"bucket": bucketName,
	})

	tokenString, err := token.SignedString([]byte("secret"))
	if err != nil {
		return "", err
	}

	request, err := http.NewRequest("GET", yigUrl, nil)
	request.Header.Set("Authorization", "Bearer "+tokenString)

	AdminServiceCircuit := circuitbreak.NewAdminServiceCircuit()
	response := new(http.Response)
	circuitErr := AdminServiceCircuit.Execute(
		context.Background(),
		func(ctx context.Context) error {
			response, err = client.Do(request)
			if err != nil {
				return err
			}
			return nil
		},
		nil,
	)
	if circuitErr != nil {
		return "", err
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	var respBody RespBody
	json.Unmarshal(body, &respBody)
	bucketOwner = respBody.Bucket.OwnerId

	//mc.Put(bucketName, bucketOwner, time.Duration(lifeTime))
	mc.Put(bucketName, bucketOwner, lifeTime)
	return bucketOwner, nil
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
