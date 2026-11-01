package httpapi

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/asamigentoku/PinguCoin/services/bff/internal/orcanclient"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/paymentclient"
	"github.com/prometheus/client_golang/prometheus"
)

func metricValue(t *testing.T, name string, labels map[string]string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
	metricLoop:
		for _, metric := range family.GetMetric() {
			have := map[string]string{}
			for _, label := range metric.GetLabel() {
				have[label.GetName()] = label.GetValue()
			}
			for key, want := range labels {
				if have[key] != want {
					continue metricLoop
				}
			}
			return metric.GetCounter().GetValue()
		}
	}
	return 0
}

func newMetricsRouter() http.Handler {
	return NewRouter(slog.New(slog.DiscardHandler), &orcanclient.Client{}, &paymentclient.Client{}, nil, nil)
}

func TestRouterCountsRequestsByRoutePattern(t *testing.T) {
	router := newMetricsRouter()
	labels := map[string]string{"method": "GET", "route": "/healthz", "code": "200"}
	before := metricValue(t, "pingucoin_http_requests_total", labels)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if got := metricValue(t, "pingucoin_http_requests_total", labels) - before; got != 1 {
		t.Errorf("requests increased by %v, want 1", got)
	}
}

func TestRouterDoesNotServeMetricsOrGraphQL(t *testing.T) {
	for _, path := range []string{"/metrics", "/api/v1/graphql", "/"} {
		recorder := httptest.NewRecorder()
		newMetricsRouter().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotFound {
			t.Errorf("%s status=%d, want 404", path, recorder.Code)
		}
		if strings.Contains(recorder.Body.String(), "pingucoin_http_requests_total") {
			t.Errorf("%s exposed metrics", path)
		}
	}
}
