package metrics

import (
	"context"
	"os"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	commonMetrics "github.com/go-tangra/go-tangra-common/metrics"
)

const namespace = "tangra"
const subsystem = "executor"

// Collector holds all Prometheus metrics for the executor module.
type Collector struct {
	log    *log.Helper
	server *commonMetrics.MetricsServer

	// Script metrics
	ScriptsTotal    prometheus.Gauge
	ScriptsByEnabled *prometheus.GaugeVec

	// Assignment metrics
	AssignmentsTotal prometheus.Gauge

	// Execution metrics
	ExecutionsTotal    prometheus.Gauge
	ExecutionsByStatus *prometheus.GaugeVec

	// gRPC request metrics
	RequestDuration *prometheus.HistogramVec
	RequestsTotal  *prometheus.CounterVec
}

// NewCollector creates and registers all executor Prometheus metrics.
func NewCollector(ctx *bootstrap.Context) *Collector {
	c := &Collector{
		log: ctx.NewLoggerHelper("executor/metrics"),

		ScriptsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "scripts_total",
			Help:      "Total number of scripts.",
		}),

		ScriptsByEnabled: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "scripts_by_enabled",
			Help:      "Number of scripts by enabled status.",
		}, []string{"enabled"}),

		AssignmentsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "assignments_total",
			Help:      "Total number of script assignments.",
		}),

		ExecutionsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "executions_total",
			Help:      "Total number of executions.",
		}),

		ExecutionsByStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "executions_by_status",
			Help:      "Number of executions by status.",
		}, []string{"status"}),

		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "grpc_request_duration_seconds",
			Help:      "Histogram of gRPC request durations in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method"}),

		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "grpc_requests_total",
			Help:      "Total number of gRPC requests by method and status.",
		}, []string{"method", "status"}),
	}

	prometheus.MustRegister(
		c.ScriptsTotal,
		c.ScriptsByEnabled,
		c.AssignmentsTotal,
		c.ExecutionsTotal,
		c.ExecutionsByStatus,
		c.RequestDuration,
		c.RequestsTotal,
	)

	addr := os.Getenv("METRICS_ADDR")
	if addr == "" {
		addr = ":9810"
	}
	c.server = commonMetrics.NewMetricsServer(addr, nil, ctx.GetLogger())

	go func() {
		if err := c.server.Start(); err != nil {
			c.log.Errorf("Metrics server failed: %v", err)
		}
	}()

	return c
}

// Stop shuts down the metrics HTTP server.
func (c *Collector) Stop(ctx context.Context) {
	if c.server != nil {
		c.server.Stop(ctx)
	}
}

// --- Script helpers ---

// ScriptCreated increments the script counters.
func (c *Collector) ScriptCreated(enabled bool) {
	c.ScriptsTotal.Inc()
	c.ScriptsByEnabled.WithLabelValues(enabledLabel(enabled)).Inc()
}

// ScriptDeleted decrements the script counters.
func (c *Collector) ScriptDeleted(enabled bool) {
	c.ScriptsTotal.Dec()
	c.ScriptsByEnabled.WithLabelValues(enabledLabel(enabled)).Dec()
}

// ScriptEnabledChanged adjusts the enabled gauge when a script's enabled status changes.
func (c *Collector) ScriptEnabledChanged(oldEnabled, newEnabled bool) {
	c.ScriptsByEnabled.WithLabelValues(enabledLabel(oldEnabled)).Dec()
	c.ScriptsByEnabled.WithLabelValues(enabledLabel(newEnabled)).Inc()
}

// --- Assignment helpers ---

// AssignmentCreated increments the assignment counter.
func (c *Collector) AssignmentCreated() {
	c.AssignmentsTotal.Inc()
}

// AssignmentDeleted decrements the assignment counter.
func (c *Collector) AssignmentDeleted() {
	c.AssignmentsTotal.Dec()
}

// --- Execution helpers ---

// ExecutionCreated increments the execution counters.
func (c *Collector) ExecutionCreated(status string) {
	c.ExecutionsTotal.Inc()
	c.ExecutionsByStatus.WithLabelValues(status).Inc()
}

// ExecutionStatusChanged adjusts the status gauge when an execution's status changes.
func (c *Collector) ExecutionStatusChanged(oldStatus, newStatus string) {
	c.ExecutionsByStatus.WithLabelValues(oldStatus).Dec()
	c.ExecutionsByStatus.WithLabelValues(newStatus).Inc()
}

// Middleware returns a Kratos middleware that records gRPC request metrics.
func (c *Collector) Middleware() middleware.Middleware {
	return commonMetrics.NewServerMiddleware(c.RequestDuration, c.RequestsTotal)
}

func enabledLabel(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}
