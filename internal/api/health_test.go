package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vermasarthak/cairn/internal/observability"
)

func TestOperationalEndpointsExposeFailureAndMetrics(t *testing.T) {
	metrics := observability.NewMetrics()
	metrics.Inc("cairn_test_events_total")
	server := Server{Ready: func(context.Context) error { return errors.New("database down") }, Metrics: metrics}
	ready := httptest.NewRecorder()
	server.Handler().ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if ready.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status=%d", ready.Code)
	}
	exporter := httptest.NewRecorder()
	server.Handler().ServeHTTP(exporter, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if exporter.Code != http.StatusOK || !strings.Contains(exporter.Body.String(), "cairn_test_events_total 1") {
		t.Fatalf("metrics=%d %q", exporter.Code, exporter.Body.String())
	}
}
