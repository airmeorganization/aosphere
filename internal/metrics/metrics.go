package metrics

import (
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// MetricRegistry tracks system and business telemetry for Prometheus scraping
type MetricRegistry struct {
	mu sync.RWMutex

	// Counters
	httpRequestsTotal       map[string]*int64 // key: method|path|status
	agentToolExecTotal      map[string]*int64 // key: tool|status
	outboxDispatchedTotal   int64
	outboxDeadLetterTotal   int64

	// Histograms / Durations (sum in ms, count)
	httpRequestDurationSum   map[string]*int64 // key: method|path (in milliseconds)
	httpRequestDurationCount map[string]*int64 // key: method|path

	// Dynamic Gauges provider functions
	gaugeProviders map[string]func() float64
}

var (
	defaultRegistry *MetricRegistry
	once            sync.Once
)

// Default returns the singleton metrics registry
func Default() *MetricRegistry {
	once.Do(func() {
		defaultRegistry = &MetricRegistry{
			httpRequestsTotal:        make(map[string]*int64),
			agentToolExecTotal:       make(map[string]*int64),
			httpRequestDurationSum:   make(map[string]*int64),
			httpRequestDurationCount: make(map[string]*int64),
			gaugeProviders:           make(map[string]func() float64),
		}
	})
	return defaultRegistry
}

// RecordHTTPRequest records status code and duration
func (m *MetricRegistry) RecordHTTPRequest(method, path string, statusCode int, duration time.Duration) {
	// Normalize path to avoid high cardinality (group IDs / UUIDs)
	normPath := normalizePath(path)
	key := fmt.Sprintf("%s|%s|%d", method, normPath, statusCode)

	m.mu.Lock()
	counter, exists := m.httpRequestsTotal[key]
	if !exists {
		var val int64
		counter = &val
		m.httpRequestsTotal[key] = counter
	}

	durKey := fmt.Sprintf("%s|%s", method, normPath)
	durSum, sumExists := m.httpRequestDurationSum[durKey]
	if !sumExists {
		var sVal int64
		durSum = &sVal
		m.httpRequestDurationSum[durKey] = durSum
	}
	durCount, countExists := m.httpRequestDurationCount[durKey]
	if !countExists {
		var cVal int64
		durCount = &cVal
		m.httpRequestDurationCount[durKey] = durCount
	}
	m.mu.Unlock()

	atomic.AddInt64(counter, 1)
	atomic.AddInt64(durSum, duration.Milliseconds())
	atomic.AddInt64(durCount, 1)
}

// RecordAgentToolExecution records an MCP agent tool invocation
func (m *MetricRegistry) RecordAgentToolExecution(toolName, status string) {
	key := fmt.Sprintf("%s|%s", toolName, status)
	m.mu.Lock()
	counter, exists := m.agentToolExecTotal[key]
	if !exists {
		var val int64
		counter = &val
		m.agentToolExecTotal[key] = counter
	}
	m.mu.Unlock()

	atomic.AddInt64(counter, 1)
}

// RecordOutboxDispatch records outbox processing outcomes
func (m *MetricRegistry) RecordOutboxDispatch(success bool) {
	if success {
		atomic.AddInt64(&m.outboxDispatchedTotal, 1)
	} else {
		atomic.AddInt64(&m.outboxDeadLetterTotal, 1)
	}
}

// RegisterGauge registers a dynamic gauge provider function
func (m *MetricRegistry) RegisterGauge(name string, provider func() float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gaugeProviders[name] = provider
}

// RegisterDBMetrics registers SQLite connection pool stats
func (m *MetricRegistry) RegisterDBMetrics(db *sql.DB) {
	if db == nil {
		return
	}
	m.RegisterGauge("sqlite_open_connections", func() float64 {
		return float64(db.Stats().OpenConnections)
	})
	m.RegisterGauge("sqlite_in_use_connections", func() float64 {
		return float64(db.Stats().InUse)
	})
	m.RegisterGauge("sqlite_idle_connections", func() float64 {
		return float64(db.Stats().Idle)
	})
	m.RegisterGauge("sqlite_wait_count_total", func() float64 {
		return float64(db.Stats().WaitCount)
	})
}

// PrometheusHandler outputs metrics in standard Prometheus exposition format
func (m *MetricRegistry) PrometheusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		var b strings.Builder

		// 1. HTTP Requests Total
		b.WriteString("# HELP http_requests_total Total number of HTTP requests processed\n")
		b.WriteString("# TYPE http_requests_total counter\n")
		m.mu.RLock()
		reqKeys := make([]string, 0, len(m.httpRequestsTotal))
		for k := range m.httpRequestsTotal {
			reqKeys = append(reqKeys, k)
		}
		sort.Strings(reqKeys)
		for _, k := range reqKeys {
			parts := strings.Split(k, "|")
			if len(parts) == 3 {
				val := atomic.LoadInt64(m.httpRequestsTotal[k])
				b.WriteString(fmt.Sprintf("http_requests_total{method=\"%s\",path=\"%s\",status=\"%s\"} %d\n",
					parts[0], parts[1], parts[2], val))
			}
		}

		// 2. HTTP Request Duration (Sum & Count)
		b.WriteString("\n# HELP http_request_duration_seconds_sum Total request latency sum in seconds\n")
		b.WriteString("# TYPE http_request_duration_seconds_sum counter\n")
		for durKey, durSumPtr := range m.httpRequestDurationSum {
			parts := strings.Split(durKey, "|")
			if len(parts) == 2 {
				sumMs := atomic.LoadInt64(durSumPtr)
				b.WriteString(fmt.Sprintf("http_request_duration_seconds_sum{method=\"%s\",path=\"%s\"} %.4f\n",
					parts[0], parts[1], float64(sumMs)/1000.0))
			}
		}

		b.WriteString("\n# HELP http_request_duration_seconds_count Total request count for duration\n")
		b.WriteString("# TYPE http_request_duration_seconds_count counter\n")
		for durKey, durCountPtr := range m.httpRequestDurationCount {
			parts := strings.Split(durKey, "|")
			if len(parts) == 2 {
				count := atomic.LoadInt64(durCountPtr)
				b.WriteString(fmt.Sprintf("http_request_duration_seconds_count{method=\"%s\",path=\"%s\"} %d\n",
					parts[0], parts[1], count))
			}
		}

		// 3. Agent Tool Executions
		b.WriteString("\n# HELP aosphere_agent_tool_executions_total Count of MCP tool executions by agent\n")
		b.WriteString("# TYPE aosphere_agent_tool_executions_total counter\n")
		toolKeys := make([]string, 0, len(m.agentToolExecTotal))
		for k := range m.agentToolExecTotal {
			toolKeys = append(toolKeys, k)
		}
		sort.Strings(toolKeys)
		for _, k := range toolKeys {
			parts := strings.Split(k, "|")
			if len(parts) == 2 {
				val := atomic.LoadInt64(m.agentToolExecTotal[k])
				b.WriteString(fmt.Sprintf("aosphere_agent_tool_executions_total{tool=\"%s\",status=\"%s\"} %d\n",
					parts[0], parts[1], val))
			}
		}

		// 4. Outbox Metrics
		b.WriteString("\n# HELP aosphere_outbox_dispatched_total Total events dispatched from transactional outbox\n")
		b.WriteString("# TYPE aosphere_outbox_dispatched_total counter\n")
		b.WriteString(fmt.Sprintf("aosphere_outbox_dispatched_total %d\n", atomic.LoadInt64(&m.outboxDispatchedTotal)))

		b.WriteString("\n# HELP aosphere_outbox_dead_letter_total Total events routed to dead letter status\n")
		b.WriteString("# TYPE aosphere_outbox_dead_letter_total counter\n")
		b.WriteString(fmt.Sprintf("aosphere_outbox_dead_letter_total %d\n", atomic.LoadInt64(&m.outboxDeadLetterTotal)))

		// 5. Dynamic Gauges (DB stats, pending approvals, GL balance)
		gaugeNames := make([]string, 0, len(m.gaugeProviders))
		for name := range m.gaugeProviders {
			gaugeNames = append(gaugeNames, name)
		}
		sort.Strings(gaugeNames)
		for _, name := range gaugeNames {
			fn := m.gaugeProviders[name]
			if fn != nil {
				val := fn()
				b.WriteString(fmt.Sprintf("\n# TYPE %s gauge\n", name))
				b.WriteString(fmt.Sprintf("%s %.2f\n", name, val))
			}
		}
		m.mu.RUnlock()

		_, _ = w.Write([]byte(b.String()))
	}
}

// normalizePath replaces dynamic IDs with :id to preserve metric cardinality
func normalizePath(path string) string {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if strings.HasPrefix(seg, "sku_") || strings.HasPrefix(seg, "po_") ||
			strings.HasPrefix(seg, "inv_") || strings.HasPrefix(seg, "usr_") ||
			strings.HasPrefix(seg, "appr_") || strings.HasPrefix(seg, "cust_") ||
			strings.HasPrefix(seg, "doc_") || strings.HasPrefix(seg, "emp_") ||
			strings.HasPrefix(seg, "ts_") || strings.HasPrefix(seg, "quote_") {
			segments[i] = ":id"
		}
	}
	return strings.Join(segments, "/")
}
