package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Collector struct {
	RequestTotal       *prometheus.CounterVec
	RequestAllowed     *prometheus.CounterVec
	RequestRejected    *prometheus.CounterVec
	RequestLatency     *prometheus.HistogramVec
	QueueLatency       *prometheus.HistogramVec
	QuotaUsage         *prometheus.GaugeVec
	TokenBucketFill    *prometheus.GaugeVec
	AdaptiveCoeff      *prometheus.GaugeVec
	RedisLatency       *prometheus.HistogramVec
	ModeState          *prometheus.GaugeVec
	RuleMatchTotal     *prometheus.CounterVec
}

func NewCollector(namespace string) *Collector {
	factory := promauto.With(prometheus.DefaultRegisterer)

	return &Collector{
		RequestTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests processed",
		}, []string{"tenant", "user", "api_path", "method", "status"}),

		RequestAllowed: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "requests_allowed_total",
			Help:      "Total number of requests allowed by rate limiter",
		}, []string{"tenant", "user", "api_path", "rule_id", "algorithm"}),

		RequestRejected: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "requests_rejected_total",
			Help:      "Total number of requests rejected by rate limiter",
		}, []string{"tenant", "user", "api_path", "rule_id", "algorithm", "triggered_level"}),

		RequestLatency: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency in seconds",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}, []string{"api_path", "method"}),

		QueueLatency: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "queue_wait_duration_seconds",
			Help:      "Time spent in shaping queue in seconds",
			Buckets:   []float64{.001, .005, .01, .05, .1, .5, 1, 5},
		}, []string{"rule_id", "priority"}),

		QuotaUsage: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "quota_usage_ratio",
			Help:      "Current quota usage ratio (0-1)",
		}, []string{"level", "identifier"}),

		TokenBucketFill: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "token_bucket_fill_ratio",
			Help:      "Token bucket current fill ratio",
		}, []string{"rule_id", "bucket_key"}),

		AdaptiveCoeff: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "adaptive_coefficient",
			Help:      "Current adaptive rate limiting coefficient",
		}, []string{"component"}),

		RedisLatency: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "redis_operation_duration_seconds",
			Help:      "Redis operation latency",
			Buckets:   prometheus.DefBuckets,
		}, []string{"operation"}),

		ModeState: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "mode_state",
			Help:      "Current rate limiter mode (1=distributed, 0=local)",
		}, []string{"node"}),

		RuleMatchTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "rule_matches_total",
			Help:      "Total number of rule matches",
		}, []string{"rule_id", "enabled"}),
	}
}

func (c *Collector) RecordRequest(tenant, user, apiPath, method, status string) {
	c.RequestTotal.WithLabelValues(tenant, user, apiPath, method, status).Inc()
}

func (c *Collector) RecordAllowed(tenant, user, apiPath, ruleID, algorithm string) {
	c.RequestAllowed.WithLabelValues(tenant, user, apiPath, ruleID, algorithm).Inc()
}

func (c *Collector) RecordRejected(tenant, user, apiPath, ruleID, algorithm, triggeredLevel string) {
	c.RequestRejected.WithLabelValues(tenant, user, apiPath, ruleID, algorithm, triggeredLevel).Inc()
}

func (c *Collector) RecordLatency(apiPath, method string, d time.Duration) {
	c.RequestLatency.WithLabelValues(apiPath, method).Observe(d.Seconds())
}

func (c *Collector) RecordQueueLatency(ruleID string, priority int, d time.Duration) {
	c.QueueLatency.WithLabelValues(ruleID, strconv.Itoa(priority)).Observe(d.Seconds())
}

func (c *Collector) SetQuotaUsage(level, identifier string, ratio float64) {
	c.QuotaUsage.WithLabelValues(level, identifier).Set(ratio)
}

func (c *Collector) SetTokenBucketFill(ruleID, bucketKey string, ratio float64) {
	c.TokenBucketFill.WithLabelValues(ruleID, bucketKey).Set(ratio)
}

func (c *Collector) SetAdaptiveCoeff(component string, coeff float64) {
	c.AdaptiveCoeff.WithLabelValues(component).Set(coeff)
}

func (c *Collector) RecordRedisOp(operation string, d time.Duration) {
	c.RedisLatency.WithLabelValues(operation).Observe(d.Seconds())
}

func (c *Collector) SetMode(node string, isDistributed bool) {
	val := 0.0
	if isDistributed {
		val = 1.0
	}
	c.ModeState.WithLabelValues(node).Set(val)
}

func (c *Collector) RecordRuleMatch(ruleID string, enabled bool) {
	enabledStr := "false"
	if enabled {
		enabledStr = "true"
	}
	c.RuleMatchTotal.WithLabelValues(ruleID, enabledStr).Inc()
}

func (c *Collector) Initialize(nodeID string) {
	c.SetMode(nodeID, true)
	c.SetAdaptiveCoeff("global", 1.0)
	c.SetQuotaUsage("global", "global", 0)
	c.RequestTotal.WithLabelValues("", "", "", "", "200")
}
