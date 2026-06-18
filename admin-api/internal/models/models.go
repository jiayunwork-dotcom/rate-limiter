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
	CombineMode string      `json:"combineMode"`
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
	DimensionsJSON    json.RawMessage   `json:"-" gorm:"column:dimensions;type:jsonb"`
	TokenBucketJSON   *json.RawMessage  `json:"-" gorm:"column:token_bucket_config;type:jsonb"`
	LeakyBucketJSON   *json.RawMessage  `json:"-" gorm:"column:leaky_bucket_config;type:jsonb"`
	ShapingJSON       *json.RawMessage  `json:"-" gorm:"column:shaping_config;type:jsonb"`
	GrayReleaseJSON   *json.RawMessage  `json:"-" gorm:"column:gray_release_config;type:jsonb"`
	ConfigJSON        json.RawMessage   `json:"-" gorm:"column:config_json;type:jsonb"`
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
	ConfigJSON json.RawMessage `json:"config_json" gorm:"column:config_json;type:jsonb"`
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
	TokenBucketJSON   *json.RawMessage  `json:"-" gorm:"column:token_bucket_config;type:jsonb"`
	LeakyBucketJSON   *json.RawMessage  `json:"-" gorm:"column:leaky_bucket_config;type:jsonb"`
	ShapingJSON       *json.RawMessage  `json:"-" gorm:"column:shaping_config;type:jsonb"`
	TokenBucketConfig *TokenBucketConfig `json:"tokenBucketConfig,omitempty" gorm:"-"`
	LeakyBucketConfig *LeakyBucketConfig `json:"leakyBucketConfig,omitempty" gorm:"-"`
	ShapingConfig     *ShapingConfig     `json:"shapingConfig,omitempty" gorm:"-"`
	CreatedAt         time.Time         `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt         time.Time         `json:"updatedAt" gorm:"column:updated_at"`
}

func (RuleTemplate) TableName() string {
	return "rule_templates"
}

type AlertSeverity string
type AlertStatus string
type AlertTriggerType string
type AlertScopeType string

const (
	SeverityCritical AlertSeverity = "critical"
	SeverityWarning  AlertSeverity = "warning"
	SeverityInfo     AlertSeverity = "info"

	StatusFiring       AlertStatus = "firing"
	StatusAcknowledged AlertStatus = "acknowledged"
	StatusResolved     AlertStatus = "resolved"
	StatusExpired      AlertStatus = "expired"

	TriggerTypeThreshold   AlertTriggerType = "threshold"
	TriggerTypeRate        AlertTriggerType = "rate"
	TriggerTypeDuration    AlertTriggerType = "duration"

	ScopeGlobal  AlertScopeType = "global"
	ScopeAPI     AlertScopeType = "api"
	ScopeTenant  AlertScopeType = "tenant"
)

type ThresholdTriggerConfig struct {
	WindowSeconds int64 `json:"windowSeconds"`
	Threshold     int64 `json:"threshold"`
	Metric        string `json:"metric"`
}

type RateTriggerConfig struct {
	WindowSeconds     int     `json:"windowSeconds"`
	ThresholdPercent  float64 `json:"thresholdPercent"`
	Metric            string  `json:"metric"`
}

type DurationTriggerConfig struct {
	DurationSeconds int64  `json:"durationSeconds"`
	Metric          string `json:"metric"`
}

type AlertRule struct {
	ID                     string            `json:"id" gorm:"column:id;primaryKey;type:varchar(64)"`
	Name                   string            `json:"name" gorm:"column:name;type:varchar(255)"`
	Description            string            `json:"description" gorm:"column:description;type:text"`
	Severity               AlertSeverity     `json:"severity" gorm:"column:severity;type:varchar(16)"`
	Enabled                bool              `json:"enabled" gorm:"column:enabled"`
	TriggerType            AlertTriggerType  `json:"triggerType" gorm:"column:trigger_type;type:varchar(32)"`
	TriggerConfigJSON      json.RawMessage   `json:"-" gorm:"column:trigger_config;type:jsonb"`
	ScopeType              AlertScopeType    `json:"scopeType" gorm:"column:scope_type;type:varchar(16)"`
	ScopeValue             string            `json:"scopeValue,omitempty" gorm:"column:scope_value;type:varchar(512)"`
	NotificationChannels   []string          `json:"notificationChannels" gorm:"-"`
	NotificationJSON       json.RawMessage   `json:"-" gorm:"column:notification_channels;type:jsonb"`
	SilentPeriodSeconds    int64             `json:"silentPeriodSeconds" gorm:"column:silent_period_seconds"`
	RetentionHours         int64             `json:"retentionHours" gorm:"column:retention_hours"`
	ThresholdTriggerConfig *ThresholdTriggerConfig `json:"thresholdConfig,omitempty" gorm:"-"`
	RateTriggerConfig      *RateTriggerConfig      `json:"rateConfig,omitempty" gorm:"-"`
	DurationTriggerConfig  *DurationTriggerConfig  `json:"durationConfig,omitempty" gorm:"-"`
	CreatedAt              time.Time         `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt              time.Time         `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`
}

func (AlertRule) TableName() string {
	return "alert_rules"
}

type AlertEvent struct {
	ID                  int64           `json:"id" gorm:"column:id;primaryKey"`
	AlertRuleID         string          `json:"alertRuleId" gorm:"column:alert_rule_id;type:varchar(64);index"`
	RuleName            string          `json:"ruleName" gorm:"column:rule_name;type:varchar(255)"`
	Severity            AlertSeverity   `json:"severity" gorm:"column:severity;type:varchar(16);index"`
	Status              AlertStatus     `json:"status" gorm:"column:status;type:varchar(16);index"`
	DimensionType       string          `json:"dimensionType" gorm:"column:dimension_type;type:varchar(32);index"`
	DimensionValue      string          `json:"dimensionValue" gorm:"column:dimension_value;type:varchar(512);index"`
	CurrentValue        float64         `json:"currentValue" gorm:"column:current_value"`
	ThresholdValue      float64         `json:"thresholdValue" gorm:"column:threshold_value"`
	TriggerSnapshot     json.RawMessage `json:"triggerSnapshot,omitempty" gorm:"column:trigger_snapshot;type:jsonb"`
	AcknowledgedBy      *string         `json:"acknowledgedBy,omitempty" gorm:"column:acknowledged_by;type:varchar(128)"`
	AcknowledgedAt      *time.Time      `json:"acknowledgedAt,omitempty" gorm:"column:acknowledged_at"`
	ResolvedAt          *time.Time      `json:"resolvedAt,omitempty" gorm:"column:resolved_at"`
	ExpiredAt           *time.Time      `json:"expiredAt,omitempty" gorm:"column:expired_at"`
	FiringStartedAt     time.Time       `json:"firingStartedAt" gorm:"column:firing_started_at;default:now()"`
	LastFiringAt        time.Time       `json:"lastFiringAt" gorm:"column:last_firing_at;default:now()"`
	Suppressed          bool            `json:"suppressed" gorm:"column:suppressed;default:false;index"`
	SuppressedByRuleID  string          `json:"suppressedByRuleId,omitempty" gorm:"column:suppressed_by_rule_id;type:varchar(64)"`
	CreatedAt           time.Time       `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt           time.Time       `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`
}

func (AlertEvent) TableName() string {
	return "alert_events"
}

type AlertStats struct {
	FiringCount     int64 `json:"firingCount"`
	TodayNewCount   int64 `json:"todayNewCount"`
	WeekTotalCount  int64 `json:"weekTotalCount"`
}

type WebSocketMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type AlertPushMessage struct {
	ID             int64         `json:"id"`
	Severity       AlertSeverity `json:"severity"`
	RuleName       string        `json:"ruleName"`
	DimensionType  string        `json:"dimensionType"`
	DimensionValue string        `json:"dimensionValue"`
	TriggerTime    time.Time     `json:"triggerTime"`
	CurrentValue   float64       `json:"currentValue"`
	ThresholdValue float64       `json:"thresholdValue"`
	Status         AlertStatus   `json:"status"`
	Suppressed     bool          `json:"suppressed,omitempty"`
	SuppressedBy   string        `json:"suppressedBy,omitempty"`
}

type AggregationDimensionType string

const (
	AggregateByAPI      AggregationDimensionType = "api_path"
	AggregateByTenant   AggregationDimensionType = "tenant_id"
	AggregateByRule     AggregationDimensionType = "rule_id"
)

type AlertAggregationRule struct {
	ID              string                  `json:"id" gorm:"column:id;primaryKey;type:varchar(64)"`
	Name            string                  `json:"name" gorm:"column:name;type:varchar(255)"`
	DimensionType   AggregationDimensionType `json:"dimensionType" gorm:"column:dimension_type;type:varchar(32)"`
	WindowSeconds   int64                   `json:"windowSeconds" gorm:"column:window_seconds"`
	Enabled         bool                    `json:"enabled" gorm:"column:enabled"`
	CreatedAt       time.Time               `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time               `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`
}

func (AlertAggregationRule) TableName() string {
	return "alert_aggregation_rules"
}

type AlertSuppressionRule struct {
	ID                    string          `json:"id" gorm:"column:id;primaryKey;type:varchar(64)"`
	Name                  string          `json:"name" gorm:"column:name;type:varchar(255)"`
	SourceSeverity        AlertSeverity   `json:"sourceSeverity" gorm:"column:source_severity;type:varchar(16)"`
	SourceStatus          AlertStatus     `json:"sourceStatus" gorm:"column:source_status;type:varchar(16)"`
	SourceRuleID          string          `json:"sourceRuleId,omitempty" gorm:"column:source_rule_id;type:varchar(64)"`
	TargetSeverity        AlertSeverity   `json:"targetSeverity" gorm:"column:target_severity;type:varchar(16)"`
	TargetDimensionType   string          `json:"targetDimensionType,omitempty" gorm:"column:target_dimension_type;type:varchar(32)"`
	MatchDimensionFields  string          `json:"matchDimensionFields" gorm:"column:match_dimension_fields;type:varchar(512)"`
	Enabled               bool            `json:"enabled" gorm:"column:enabled"`
	CreatedAt             time.Time       `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt             time.Time       `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`
}

func (AlertSuppressionRule) TableName() string {
	return "alert_suppression_rules"
}

type AlertAggregationGroup struct {
	ID                int64                   `json:"id" gorm:"column:id;primaryKey"`
	AggregationRuleID string                  `json:"aggregationRuleId" gorm:"column:aggregation_rule_id;type:varchar(64);index"`
	DimensionType     AggregationDimensionType `json:"dimensionType" gorm:"column:dimension_type;type:varchar(32)"`
	DimensionValue    string                  `json:"dimensionValue" gorm:"column:dimension_value;type:varchar(512)"`
	TriggerCount      int64                   `json:"triggerCount" gorm:"column:trigger_count"`
	FirstTriggeredAt  time.Time               `json:"firstTriggeredAt" gorm:"column:first_triggered_at"`
	LastTriggeredAt   time.Time               `json:"lastTriggeredAt" gorm:"column:last_triggered_at"`
	WindowEndsAt      time.Time               `json:"windowEndsAt" gorm:"column:window_ends_at;index"`
	Severity          AlertSeverity           `json:"severity" gorm:"column:severity;type:varchar(16)"`
	Status            AlertStatus             `json:"status" gorm:"column:status;type:varchar(16)"`
	UniqueValuesJSON  json.RawMessage         `json:"-" gorm:"column:unique_values;type:jsonb"`
	UniqueValues      []string                `json:"uniqueValues" gorm:"-"`
	CreatedAt         time.Time               `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt         time.Time               `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`
}

func (AlertAggregationGroup) TableName() string {
	return "alert_aggregation_groups"
}

type AlertAggregationEvent struct {
	ID                int64 `json:"id" gorm:"column:id;primaryKey"`
	AggregationGroupID int64 `json:"aggregationGroupId" gorm:"column:aggregation_group_id;index"`
	AlertEventID      int64 `json:"alertEventId" gorm:"column:alert_event_id;index"`
	CreatedAt         time.Time `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
}

func (AlertAggregationEvent) TableName() string {
	return "alert_aggregation_events"
}

type AlertAggregationGroupWithEvents struct {
	AlertAggregationGroup
	Events []AlertEvent `json:"events,omitempty"`
}

type AggregationPushMessage struct {
	GroupID          int64                   `json:"groupId"`
	AggregationRuleID string                 `json:"aggregationRuleId"`
	DimensionType    AggregationDimensionType `json:"dimensionType"`
	DimensionValue   string                  `json:"dimensionValue"`
	TriggerCount     int64                   `json:"triggerCount"`
	FirstTriggeredAt time.Time               `json:"firstTriggeredAt"`
	LastTriggeredAt  time.Time               `json:"lastTriggeredAt"`
	Severity         AlertSeverity           `json:"severity"`
	Status           AlertStatus             `json:"status"`
	LatestDimension  string                  `json:"latestDimension"`
	UniqueValues     []string                `json:"uniqueValues"`
}

type AuditOperationType string
type AuditResourceType string

const (
	AuditOpCreate   AuditOperationType = "create"
	AuditOpUpdate   AuditOperationType = "update"
	AuditOpDelete   AuditOperationType = "delete"
	AuditOpToggle   AuditOperationType = "toggle"
	AuditOpRollback AuditOperationType = "rollback"
)

const (
	AuditResRule               AuditResourceType = "rule"
	AuditResQuota              AuditResourceType = "quota"
	AuditResAlertRule          AuditResourceType = "alert_rule"
	AuditResAggregationRule    AuditResourceType = "aggregation_rule"
	AuditResSuppressionRule    AuditResourceType = "suppression_rule"
)

type DiffField struct {
	OldValue interface{} `json:"oldValue"`
	NewValue interface{} `json:"newValue"`
}

type AuditLog struct {
	ID              int64               `json:"id" gorm:"column:id;primaryKey"`
	Operator        string              `json:"operator" gorm:"column:operator;type:varchar(128);index"`
	OperationType   AuditOperationType  `json:"operationType" gorm:"column:operation_type;type:varchar(16);index"`
	ResourceType    AuditResourceType   `json:"resourceType" gorm:"column:resource_type;type:varchar(32);index"`
	ResourceID      string              `json:"resourceId" gorm:"column:resource_id;type:varchar(128);index"`
	BeforeSnapshot  json.RawMessage     `json:"beforeSnapshot" gorm:"column:before_snapshot;type:jsonb"`
	AfterSnapshot   json.RawMessage     `json:"afterSnapshot" gorm:"column:after_snapshot;type:jsonb"`
	DiffSummary     json.RawMessage     `json:"diffSummary" gorm:"column:diff_summary;type:jsonb"`
	RequestIP       string              `json:"requestIp" gorm:"column:request_ip;type:varchar(64)"`
	CreatedAt       time.Time           `json:"createdAt" gorm:"column:created_at;index"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}

type AuditLogQuery struct {
	Operator      string              `form:"operator"`
	ResourceType  AuditResourceType   `form:"resource_type"`
	ResourceID    string              `form:"resource_id"`
	OperationType AuditOperationType  `form:"operation_type"`
	StartTime     *time.Time          `form:"-"`
	EndTime       *time.Time          `form:"-"`
	Pagination
}

type AuditStats struct {
	TodayTotalCount   int64  `json:"todayTotalCount"`
	WeekTopOperator   string `json:"weekTopOperator"`
	WeekTopCount      int64  `json:"weekTopCount"`
	LastOperationTime time.Time `json:"lastOperationTime"`
	LastOperationType AuditOperationType `json:"lastOperationType"`
	LastResourceType  AuditResourceType  `json:"lastResourceType"`
	LastResourceID    string `json:"lastResourceId"`
}

type TimelineNode struct {
	ID            int64              `json:"id"`
	OperationType AuditOperationType `json:"operationType"`
	Operator      string             `json:"operator"`
	DiffSummary   json.RawMessage    `json:"diffSummary"`
	CreatedAt     time.Time          `json:"createdAt"`
	CanRollback   bool               `json:"canRollback"`
}
