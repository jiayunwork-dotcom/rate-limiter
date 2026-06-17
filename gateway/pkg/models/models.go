package models

import (
	"time"
)

type AlgorithmType string

const (
	AlgorithmTokenBucket     AlgorithmType = "token_bucket"
	AlgorithmLeakyBucket     AlgorithmType = "leaky_bucket"
	AlgorithmFixedWindow     AlgorithmType = "fixed_window"
	AlgorithmSlidingWindow   AlgorithmType = "sliding_window"
	AlgorithmSlidingLog      AlgorithmType = "sliding_log"
)

type DimensionType string

const (
	DimensionAPI         DimensionType = "api"
	DimensionMethod      DimensionType = "method"
	DimensionUserID      DimensionType = "user_id"
	DimensionTenantID    DimensionType = "tenant_id"
	DimensionIP          DimensionType = "ip"
	DimensionHeader      DimensionType = "header"
)

type QuotaLevel string

const (
	QuotaLevelGlobal QuotaLevel = "global"
	QuotaLevelTenant QuotaLevel = "tenant"
	QuotaLevelUser   QuotaLevel = "user"
	QuotaLevelAPI    QuotaLevel = "api"
)

type Mode string

const (
	ModeDistributed Mode = "distributed"
	ModeLocal       Mode = "local"
)

type Dimension struct {
	Type       DimensionType `json:"type"`
	Value      string        `json:"value"`
	HeaderName string        `json:"header_name,omitempty"`
}

type RuleDimensions struct {
	Dimensions []Dimension   `json:"dimensions"`
	CombineMode string       `json:"combine_mode"`
}

type RuleConfig struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	APIPath        string         `json:"api_path"`
	Method         string         `json:"method"`
	Algorithm      AlgorithmType  `json:"algorithm"`
	Enabled        bool           `json:"enabled"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	Version        int64          `json:"version"`

	Dimensions     RuleDimensions `json:"dimensions"`

	Limit          int64          `json:"limit"`
	WindowSeconds  int64          `json:"window_seconds,omitempty"`

	TokenBucketConfig  *TokenBucketConfig  `json:"token_bucket_config,omitempty"`
	LeakyBucketConfig  *LeakyBucketConfig  `json:"leaky_bucket_config,omitempty"`
	ShapingConfig      *ShapingConfig      `json:"shaping_config,omitempty"`
	GrayReleaseConfig  *GrayReleaseConfig  `json:"gray_release_config,omitempty"`
}

type TokenBucketConfig struct {
	RefillRate   int64 `json:"refill_rate"`
	Capacity     int64 `json:"capacity"`
	TokensPerReq int64 `json:"tokens_per_req"`
}

type LeakyBucketConfig struct {
	OutRate    int64 `json:"out_rate"`
	Capacity   int64 `json:"capacity"`
}

type ShapingConfig struct {
	Enabled          bool  `json:"enabled"`
	MaxQueueDepth    int   `json:"max_queue_depth"`
	MaxWaitMs        int64 `json:"max_wait_ms"`
	PriorityEnabled  bool  `json:"priority_enabled"`
}

type GrayReleaseConfig struct {
	Enabled       bool    `json:"enabled"`
	TrafficRatio  float64 `json:"traffic_ratio"`
	RollbackVersion int64 `json:"rollback_version"`
}

type QuotaConfig struct {
	Level         QuotaLevel `json:"level"`
	Identifier    string     `json:"identifier"`
	Limit         int64      `json:"limit"`
	WindowSeconds int64      `json:"window_seconds"`
	InheritFrom   bool       `json:"inherit_from"`
	OverrideValue int64      `json:"override_value"`
}

type RateLimitResult struct {
	Allowed     bool      `json:"allowed"`
	Limit       int64     `json:"limit"`
	Remaining   int64     `json:"remaining"`
	ResetTime   time.Time `json:"reset_time"`
	RetryAfter  int64     `json:"retry_after,omitempty"`
	RuleID      string    `json:"rule_id"`
	Algorithm   AlgorithmType `json:"algorithm"`
	Mode        Mode      `json:"mode"`
	Queued      bool      `json:"queued"`
	QueueDelayMs int64    `json:"queue_delay_ms,omitempty"`
	TriggeredLevel QuotaLevel `json:"triggered_level,omitempty"`
}

type RequestContext struct {
	RequestID    string
	APIPath      string
	Method       string
	UserID       string
	TenantID     string
	ClientIP     string
	Headers      map[string]string
	Priority     int
	ReceivedAt   time.Time
}

type RateLimitEvent struct {
	Timestamp     time.Time   `json:"timestamp"`
	RequestID     string      `json:"request_id"`
	Allowed       bool        `json:"allowed"`
	RuleID        string      `json:"rule_id"`
	RuleName      string      `json:"rule_name"`
	Dimensions    []Dimension `json:"dimensions"`
	Limit         int64       `json:"limit"`
	Remaining     int64       `json:"remaining"`
	APIPath       string      `json:"api_path"`
	Method        string      `json:"method"`
	UserID        string      `json:"user_id"`
	TenantID      string      `json:"tenant_id"`
	ClientIP      string      `json:"client_ip"`
	TriggeredLevel QuotaLevel `json:"triggered_level,omitempty"`
	Mode          Mode        `json:"mode"`
}

type AdaptiveConfig struct {
	Enabled              bool    `json:"enabled"`
	TargetP99LatencyMs   int64   `json:"target_p99_latency_ms"`
	ErrorRateThreshold   float64 `json:"error_rate_threshold"`
	MinCoefficient       float64 `json:"min_coefficient"`
	MaxCoefficient       float64 `json:"max_coefficient"`
	TighteningRatio      float64 `json:"tightening_ratio"`
	RecoveryIntervalSec  int64   `json:"recovery_interval_sec"`
	RecoveryStepPercent  float64 `json:"recovery_step_percent"`
	StablePeriodSec      int64   `json:"stable_period_sec"`
	PIDConfig            PIDConfig `json:"pid_config"`
}

type PIDConfig struct {
	Kp          float64 `json:"kp"`
	Ki          float64 `json:"ki"`
	Kd          float64 `json:"kd"`
	Setpoint    float64 `json:"setpoint"`
	OutputMin   float64 `json:"output_min"`
	OutputMax   float64 `json:"output_max"`
}

type BackendHealth struct {
	P99LatencyMs  int64   `json:"p99_latency_ms"`
	ErrorRate     float64 `json:"error_rate"`
	CurrentCoeff  float64 `json:"current_coeff"`
	LastUpdated   time.Time `json:"last_updated"`
	StableSince   *time.Time `json:"stable_since,omitempty"`
}
