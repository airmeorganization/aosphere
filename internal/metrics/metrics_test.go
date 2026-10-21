package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetricRegistry(t *testing.T) {
	reg := Default()

	reg.RecordHTTPRequest("GET", "/api/skus/sku_01", 200, 15*time.Millisecond)
	reg.RecordHTTPRequest("POST", "/api/purchase-orders", 201, 45*time.Millisecond)
	reg.RecordAgentToolExecution("inventory_adjust_stock", "success")
	reg.RecordOutboxDispatch(true)
	reg.RecordOutboxDispatch(false)

	reg.RegisterGauge("test_custom_gauge", func() float64 {
		return 42.5
	})

	handler := reg.PrometheusHandler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `http_requests_total{method="GET",path="/api/skus/:id",status="200"} 1`) {
		t.Errorf("missing http_requests_total for GET /api/skus/:id in body: %s", body)
	}
	if !strings.Contains(body, `aosphere_agent_tool_executions_total{tool="inventory_adjust_stock",status="success"} 1`) {
		t.Errorf("missing agent tool metric in body: %s", body)
	}
	if !strings.Contains(body, "test_custom_gauge 42.50") {
		t.Errorf("missing custom gauge in body: %s", body)
	}
	if !strings.Contains(body, "aosphere_outbox_dispatched_total 1") {
		t.Errorf("missing outbox dispatched metric in body: %s", body)
	}
}
