package exchange

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

type Record struct {
	Version     string
	CacheStatus string
	StatusCode  string
	Duration    float64
}

type Metrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func NewMetrics(registerer prometheus.Registerer) *Metrics {
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pingucoin_nginx_exchange_rate_requests_total",
		Help: "Exchange-rate requests observed at nginx, partitioned by API version and cache result.",
	}, []string{"version", "cache_status", "code"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "pingucoin_nginx_exchange_rate_request_duration_seconds",
		Help:    "End-to-end exchange-rate request duration observed at nginx.",
		Buckets: []float64{.001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"version", "cache_status"})
	registerer.MustRegister(requests, duration)
	return &Metrics{requests: requests, duration: duration}
}

func (m *Metrics) Observe(record Record) {
	m.requests.WithLabelValues(record.Version, record.CacheStatus, record.StatusCode).Inc()
	m.duration.WithLabelValues(record.Version, record.CacheStatus).Observe(record.Duration)
}

// ParseLine parses the tab-separated dedicated nginx access log:
// msec, request_time, status, cache_status, uri.
func ParseLine(line string) (Record, error) {
	fields := strings.Split(strings.TrimSpace(line), "\t")
	if len(fields) != 5 {
		return Record{}, fmt.Errorf("expected 5 fields, got %d", len(fields))
	}
	duration, err := strconv.ParseFloat(fields[1], 64)
	if err != nil || duration < 0 {
		return Record{}, fmt.Errorf("invalid request duration %q", fields[1])
	}
	if _, err := strconv.Atoi(fields[2]); err != nil {
		return Record{}, fmt.Errorf("invalid status code %q", fields[2])
	}

	version := ""
	switch {
	case fields[4] == "/api/v1/exchange-rates":
		version = "v1"
	case strings.HasPrefix(fields[4], "/api/v2/exchange-rates/"):
		version = "v2"
	default:
		return Record{}, fmt.Errorf("unsupported exchange-rate URI %q", fields[4])
	}

	cacheStatus := strings.ToUpper(fields[3])
	if cacheStatus == "" || cacheStatus == "-" {
		cacheStatus = "UNKNOWN"
	}
	return Record{Version: version, CacheStatus: cacheStatus, StatusCode: fields[2], Duration: duration}, nil
}
