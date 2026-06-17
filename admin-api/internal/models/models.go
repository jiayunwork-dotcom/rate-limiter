package models

import (
	"encoding/json"
	"time"
)

type AlgorithmType string
type DimensionType string
type QuotaLevel string

type Dimension struct {
	Type       DimensionType `json:"type"`
	Value      string        `json:"value"`
	HeaderName string        `json:"header_name,omitempty"`
}

type RuleDimensions struct {
	Dimensions  []Dimension `json:"dimensions"`
	CombineMode string      `json:"combine_mode"`
}

type TokenBucketConfig struct {
	RefillRate    int64 `json:"refillRate"`
	Capacity      int64 `json:"capacity"`
	TokensPerReq  int64 `json:"tokensPerReq"`
}

type LeakyBucketConfig struct {
	OutRate  int64 `json:"outflowRate"`
	Capacity int64 `json:"capacity"`
}

type ShapingConfig struct {
	Enabled         bool  `json:"enabled"`
	MaxQueueDepth   int   `json:"maxQueueDepth"`
	MaxWaitMs       int64 `json:"maxWaitMs"`
	PriorityEnabled bool  `json:"priorityEnabled"`
}

type GrayReleaseConfig struct {
	Enabled        bool    `json:"enabled"`
	TrafficRatio   float64 `json:"trafficPercent"`
	RollbackVersion int64  `json:"rollbackVersion"`
}

type RateLimitRule struct {
	ID                string            `json:"id" gorm:"column:id;primaryKey"`
	Name              string            `json:"name" gorm:"column:name"`
	APIPath           string            `json:"apiPath" gorm:"column:api_path"`
	Method            string            `json:"method" gorm:"column:method"`
	Algorithm         AlgorithmType     `json:"algorithm" gorm:"column:algorithm"`
	Enabled           bool              `json:"enabled" gorm:"column:enabled"`
	Version           int64             `json:"version" gorm:"column:version"`
	Limit             int64             `json:"limit" gorm:"column:limit_count"`
	WindowSeconds     int64             `json:"windowSeconds" gorm:"column:window_seconds"`
	DimensionsJSON    json.RawMessage   `json:"-" gorm:"column:dimensions"`
	TokenBucketJSON   *json.RawMessage  `json:"-" gorm:"column:token_bucket_config"`
	LeakyBucketJSON   *json.RawMessage  `json:"-" gorm:"column:leaky_bucket_config"`
	ShapingJSON       *json.RawMessage  `json:"-" gorm:"column:shaping_config"`
	GrayReleaseJSON   *json.RawMessage  `json:"-" gorm:"column:gray_release_config"`
	ConfigJSON        json.RawMessage   `json:"-" gorm:"column:config_json"`
	Dimensions        *RuleDimensions     `json:"dimensions" gorm:"-"`
	TokenBucketConfig *TokenBucketConfig  `json:"tokenBucketConfig,omitempty" gorm:"-"`
	LeakyBucketConfig *LeakyBucketConfig  `json:"leakyBucketConfig,omitempty" gorm:"-"`
	ShapingConfig     *ShapingConfig      `json:"shapingConfig,omitempty" gorm:"-"`
	GrayReleaseConfig *GrayReleaseConfig  `json:"grayRelease,omitempty" gorm:"-"`
	CreatedAt         time.Time         `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt         time.Time         `json:"updatedAt" gorm:"column:updated_at"`
}

func (RateLimitRule) TableName() string {
	return "rate_limit_rules"
}

type RuleVersion struct {
	ID         int64           `json:"id" gorm:"column:id;primaryKey"`
	RuleID     string          `json:"rule_id" gorm:"column:rule_id"`
	Version    int64           `json:"version" gorm:"column:version"`
	ConfigJSON json.RawMessage `json:"config_json" gorm:"column:config_json"`
	CreatedAt  time.Time       `json:"created_at" gorm:"column:created_at"`
}

func (RuleVersion) TableName() string {
	return "rule_versions"
}

type QuotaConfig struct {
	ID            int64      `json:"id" gorm:"column:id;primaryKey"`
	Level         QuotaLevel `json:"level" gorm:"column:quota_level"`
	Identifier    string     `json:"identifier" gorm:"column:identifier"`
	Limit         int64      `json:"limit" gorm:"column:limit_count"`
	WindowSeconds int64      `json:"window_seconds" gorm:"column:window_seconds"`
	InheritFrom   bool       `json:"inherit_from" gorm:"column:inherit_from"`
	OverrideValue int64      `json:"override_value" gorm:"column:override_value"`
	CreatedAt     time.Time  `json:"created_at" gorm:"column:created_at"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"column:updated_at"`
}

func (QuotaConfig) TableName() string {
	return "quota_configs"
}

type RateLimitEvent struct {
	ID             int64     `json:"id" gorm:"column:id;primaryKey"`
	Timestamp      time.Time `json:"timestamp" gorm:"column:timestamp"`
	RequestID      string    `json:"request_id" gorm:"column:request_id"`
	Allowed        bool      `json:"allowed" gorm:"column:allowed"`
	RuleID         string    `json:"rule_id" gorm:"column:rule_id"`
	RuleName       string    `json:"rule_name" gorm:"column:rule_name"`
	DimensionsJSON *string   `json:"-" gorm:"column:dimensions"`
	Limit          int64     `json:"limit" gorm:"column:limit_count"`
	Remaining      int64     `json:"remaining" gorm:"column:remaining"`
	APIPath        string    `json:"api_path" gorm:"column:api_path"`
	Method         string    `json:"method" gorm:"column:method"`
	UserID         string    `json:"user_id" gorm:"column:user_id"`
	TenantID       string    `json:"tenant_id" gorm:"column:tenant_id"`
	ClientIP       string    `json:"client_ip" gorm:"column:client_ip"`
	TriggeredLevel string    `json:"triggered_level" gorm:"column:triggered_level"`
	Mode           string    `json:"mode" gorm:"column:mode"`
}

func (RateLimitEvent) TableName() string {
	return "rate_limit_events"
}

type Tenant struct {
	ID          string    `json:"id" gorm:"column:id;primaryKey"`
	Name        string    `json:"name" gorm:"column:name"`
	Description string    `json:"description" gorm:"column:description"`
	Active      bool      `json:"active" gorm:"column:active"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (Tenant) TableName() string {
	return "tenants"
}

type APIUser struct {
	ID        string    `json:"id" gorm:"column:id;primaryKey"`
	TenantID  string    `json:"tenant_id" gorm:"column:tenant_id"`
	Name      string    `json:"name" gorm:"column:name"`
	Email     string    `json:"email" gorm:"column:email"`
	Active    bool      `json:"active" gorm:"column:active"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (APIUser) TableName() string {
	return "api_users"
}

type AdaptiveConfigDB struct {
	ID                    int64     `json:"-" gorm:"column:id;primaryKey"`
	Component             string    `json:"component" gorm:"column:component"`
	Enabled               bool      `json:"enabled" gorm:"column:enabled"`
	TargetP99LatencyMs    int64     `json:"target_p99_latency_ms" gorm:"column:target_p99_latency_ms"`
	ErrorRateThreshold    float64   `json:"error_rate_threshold" gorm:"column:error_rate_threshold"`
	MinCoefficient        float64   `json:"min_coefficient" gorm:"column:min_coefficient"`
	MaxCoefficient        float64   `json:"max_coefficient" gorm:"column:max_coefficient"`
	TighteningRatio       float64   `json:"tightening_ratio" gorm:"column:tightening_ratio"`
	RecoveryIntervalSec   int64     `json:"recovery_interval_sec" gorm:"column:recovery_interval_sec"`
	RecoveryStepPercent   float64   `json:"recovery_step_percent" gorm:"column:recovery_step_percent"`
	StablePeriodSec       int64     `json:"stable_period_sec" gorm:"column:stable_period_sec"`
	PIDKp                 float64   `json:"pid_kp" gorm:"column:pid_kp"`
	PIDKi                 float64   `json:"pid_ki" gorm:"column:pid_ki"`
	PIDKd                 float64   `json:"pid_kd" gorm:"column:pid_kd"`
	PIDSetpoint           float64   `json:"pid_setpoint" gorm:"column:pid_setpoint"`
	PIDOutputMin          float64   `json:"pid_output_min" gorm:"column:pid_output_min"`
	PIDOutputMax          float64   `json:"pid_output_max" gorm:"column:pid_output_max"`
	ManualOverrideCoeff   *float64  `json:"manual_override_coeff" gorm:"column:manual_override_coeff"`
	UpdatedAt             time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (AdaptiveConfigDB) TableName() string {
	return "adaptive_configs"
}

type QuotaTreeNode struct {
	Level       QuotaLevel      `json:"level"`
	Identifier  string          `json:"identifier"`
	Name        string          `json:"name"`
	Limit       int64           `json:"limit"`
	Used        int64           `json:"used"`
	Remaining   int64           `json:"remaining"`
	UsageRatio  float64         `json:"usage_ratio"`
	InheritFrom bool            `json:"inherit_from"`
	Children    []*QuotaTreeNode `json:"children,omitempty"`
	OverQuota   bool            `json:"over_quota"`
}

type TrafficPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	APIPath     string    `json:"apiPath"`
	Allowed     int64     `json:"allowed"`
	Rejected    int64     `json:"rejected"`
	Total       int64     `json:"total"`
	RejectRatio float64   `json:"rejectRatio"`
}

type TenantTrafficShare struct {
	TenantID     string  `json:"tenantId"`
	TenantName   string  `json:"tenantName"`
	RequestCount int64   `json:"requestCount"`
	Percentage   float64 `json:"percentage"`
}

type HeatmapPoint struct {
	Hour    int   `json:"hour"`
	Weekday int   `json:"weekday"`
	Count   int64 `json:"count"`
}

type Pagination struct {
	Page     int `json:"page" form:"page" binding:"omitempty,min=1"`
	PageSize int `json:"page_size" form:"page_size" binding:"omitempty,min=1,max=500"`
}

func (p Pagination) GetOffset() int {
	if p.Page <= 1 {
		return 0
	}
	return (p.Page - 1) * p.GetPageSize()
}

func (p Pagination) GetPageSize() int {
	if p.PageSize <= 0 {
		return 20
	}
	if p.PageSize > 500 {
		return 500
	}
	return p.PageSize
}

type PaginatedResult struct {
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Data     interface{} `json:"data"`
}

type AdaptiveStatus struct {
	Enabled               bool      `json:"enabled"`
	CurrentCoefficient    float64   `json:"current_coefficient"`
	P99LatencyMs          int64     `json:"p99_latency_ms"`
	ErrorRate             float64   `json:"error_rate"`
	TargetP99LatencyMs    int64     `json:"target_p99_latency_ms"`
	ErrorRateThreshold    float64   `json:"error_rate_threshold"`
	StableSince           *time.Time `json:"stable_since,omitempty"`
	LastUpdated           time.Time `json:"last_updated"`
	PIDKp                 float64   `json:"pid_kp"`
	PIDKi                 float64   `json:"pid_ki"`
	PIDKd                 float64   `json:"pid_kd"`
	ManualOverrideCoeff   *float64  `json:"manual_override_coeff,omitempty"`
}

type RuleTemplate struct {
	ID                string            `json:"id" gorm:"column:id;primaryKey"`
	Name              string            `json:"name" gorm:"column:name"`
	Description       string            `json:"description" gorm:"column:description"`
	Algorithm         AlgorithmType     `json:"algorithm" gorm:"column:algorithm"`
	Limit             int64             `json:"limit" gorm:"column:limit_count"`
	WindowSeconds     int64             `json:"windowSeconds" gorm:"column:window_seconds"`
	TokenBucketJSON   *json.RawMessage  `json:"-" gorm:"column:token_bucket_config"`
	LeakyBucketJSON   *json.RawMessage  `json:"-" gorm:"column:leaky_bucket_config"`
	ShapingJSON       *json.RawMessage  `json:"-" gorm:"column:shaping_config"`
	TokenBucketConfig *TokenBucketConfig `json:"tokenBucketConfig,omitempty" gorm:"-"`
	LeakyBucketConfig *LeakyBucketConfig `json:"leakyBucketConfig,omitempty" gorm:"-"`
	ShapingConfig     *ShapingConfig     `json:"shapingConfig,omitempty" gorm:"-"`
	CreatedAt         time.Time         `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt         time.Time         `json:"updatedAt" gorm:"column:updated_at"`
}

func (RuleTemplate) TableName() string {
	return "rule_templates"
}
