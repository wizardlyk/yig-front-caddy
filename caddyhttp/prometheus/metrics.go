package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "caddy"

var (
	requestCount    *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	responseSize    *prometheus.HistogramVec
	responseStatus  *prometheus.CounterVec
	responseLatency *prometheus.HistogramVec

	countTotal          *prometheus.CounterVec
	bytesTotal          *prometheus.CounterVec
	upstreamSeconds     *prometheus.SummaryVec
	upstreamSecondsHist *prometheus.HistogramVec
	responseSeconds     *prometheus.SummaryVec
	responseSecondsHist *prometheus.HistogramVec
)

func define(subsystem string) {
	if subsystem == "" {
		subsystem = "http"
	}
	requestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "request_count_total",
		Help:      "Counter of HTTP(S) requests made.",
	}, []string{"host", "family", "proto"})

	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "request_duration_seconds",
		Help:      "Histogram of the time (in seconds) each request took.",
		Buckets:   append(prometheus.DefBuckets, 15, 20, 30, 60, 120, 180, 240, 480, 960),
	}, []string{"host", "family", "proto"})

	responseSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "response_size_bytes",
		Help:      "Size of the returns response in bytes.",
		Buckets:   []float64{0, 500, 1000, 2000, 3000, 4000, 5000, 10000, 20000, 30000, 50000, 1e5, 5e5, 1e6, 2e6, 3e6, 4e6, 5e6, 10e6},
	}, []string{"host", "family", "proto", "status"})

	responseStatus = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "response_status_count_total",
		Help:      "Counter of response status codes.",
	}, []string{"host", "family", "proto", "status"})

	responseLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "response_latency_seconds",
		Help:      "Histogram of the time (in seconds) until the first write for each request.",
		Buckets:   append(prometheus.DefBuckets, 15, 20, 30, 60, 120, 180, 240, 480, 960),
	}, []string{"host", "family", "proto", "status"})

	// prometheus exporter
	var labels []string
	labels = append(labels, "bucket_name", "method", "status", "internal","bucket_owner")

	countTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nginx",
		Name:      "http_response_count_total",
		Help:      "Amount of processed HTTP requests",
	}, labels)

	bytesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nginx",
		Name:      "http_response_size_bytes",
		Help:      "Total amount of transferred bytes",
	}, labels)

	upstreamSeconds = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "nginx",
		Name:      "http_upstream_time_seconds",
		Help:      "Time needed by upstream servers to handle requests",
	}, labels)

	upstreamSecondsHist = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "nginx",
		Name:      "http_upstream_time_seconds_hist",
		Help:      "Time needed by upstream servers to handle requests",
	}, labels)

	responseSeconds = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "nginx",
		Name:      "http_response_time_seconds",
		Help:      "Time needed by NGINX to handle requests",
	}, labels)

	responseSecondsHist = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "nginx",
		Name:      "http_response_time_seconds_hist",
		Help:      "Time needed by NGINX to handle requests",
	}, labels)
}
