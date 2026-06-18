package repository

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/ratelimiter/admin-api/internal/models"
)

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[0:4]) + "-" +
		hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" +
		hex.EncodeToString(b[8:10]) + "-" +
		hex.EncodeToString(b[10:16])
}

var snakeToCamel = map[string]string{
	`"refill_rate":`:      `"refillRate":`,
	`"tokens_per_req":`:   `"tokensPerReq":`,
	`"out_rate":`:         `"outflowRate":`,
	`"max_queue_depth":`:  `"maxQueueDepth":`,
	`"max_wait_ms":`:      `"maxWaitMs":`,
	`"priority_enabled":`: `"priorityEnabled":`,
	`"traffic_ratio":`:    `"trafficPercent":`,
	`"rollback_version":`: `"rollbackVersion":`,
	`"combine_mode":`:     `"combineMode":`,
	`"header_name":`:      `"headerName":`,
}

func normalizeJSONKeys(data []byte) []byte {
	s := string(data)
	for snake, camel := range snakeToCamel {
		s = strings.ReplaceAll(s, snake, camel)
	}
	return []byte(s)
}

type RuleRepo struct {
	db *gorm.DB
}

func NewRuleRepo(db *gorm.DB) *RuleRepo {
	return &RuleRepo{db: db}
}

func (r *RuleRepo) List(page models.Pagination, search string, enabled *bool) (*models.PaginatedResult, error) {
	query := r.db.Model(&models.RateLimitRule{})
	if search != "" {
		q := "%" + search + "%"
		query = query.Where("name ILIKE ? OR api_path ILIKE ? OR id ILIKE ?", q, q, q)
	}
	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var rules []models.RateLimitRule
	err := query.Order("updated_at DESC").
		Offset(page.GetOffset()).
		Limit(page.GetPageSize()).
		Find(&rules).Error
	if err != nil {
		return nil, err
	}

	for i := range rules {
		unmarshalRuleFields(&rules[i])
	}

	return &models.PaginatedResult{
		Total:    total,
		Page:     page.Page,
		PageSize: page.GetPageSize(),
		Data:     rules,
	}, nil
}

func (r *RuleRepo) Get(id string) (*models.RateLimitRule, error) {
	var rule models.RateLimitRule
	err := r.db.Where("id = ?", id).First(&rule).Error
	if err != nil {
		return nil, err
	}
	unmarshalRuleFields(&rule)
	return &rule, nil
}

func (r *RuleRepo) Create(rule *models.RateLimitRule) error {
	if rule.ID == "" {
		return errors.New("rule id is required")
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	if rule.Version == 0 {
		rule.Version = 1
	}
	marshalRuleFields(rule)
	err := r.db.Create(rule).Error
	if err != nil {
		return err
	}
	return r.saveVersion(rule)
}

func (r *RuleRepo) Update(rule *models.RateLimitRule) error {
	existing, err := r.Get(rule.ID)
	if err != nil {
		return err
	}
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()
	rule.Version = existing.Version + 1
	marshalRuleFields(rule)

	err = r.db.Save(rule).Error
	if err != nil {
		return err
	}
	return r.saveVersion(rule)
}

func (r *RuleRepo) saveVersion(rule *models.RateLimitRule) error {
	configCopy := *rule
	marshalRuleFields(&configCopy)
	ver := &models.RuleVersion{
		RuleID:     rule.ID,
		Version:    rule.Version,
		ConfigJSON: configCopy.ConfigJSON,
		CreatedAt:  time.Now(),
	}
	return r.db.Create(ver).Error
}

func (r *RuleRepo) Delete(id string) error {
	return r.db.Delete(&models.RateLimitRule{}, "id = ?", id).Error
}

func (r *RuleRepo) Toggle(id string, enabled bool) error {
	res := r.db.Model(&models.RateLimitRule{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"enabled":    enabled,
			"version":    gorm.Expr("version + 1"),
			"updated_at": time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *RuleRepo) GetVersions(ruleID string) ([]models.RuleVersion, error) {
	var versions []models.RuleVersion
	err := r.db.Where("rule_id = ?", ruleID).
		Order("version DESC").
		Limit(50).
		Find(&versions).Error
	return versions, err
}

func (r *RuleRepo) Rollback(ruleID string, version int64) error {
	var versionRec models.RuleVersion
	err := r.db.Where("rule_id = ? AND version = ?", ruleID, version).
		First(&versionRec).Error
	if err != nil {
		return fmt.Errorf("version not found: %w", err)
	}

	var rule models.RateLimitRule
	if err := json.Unmarshal(versionRec.ConfigJSON, &rule); err != nil {
		return err
	}
	unmarshalRuleFields(&rule)
	return r.Update(&rule)
}

func marshalRuleFields(rule *models.RateLimitRule) {
	if rule.Dimensions != nil {
		data, _ := json.Marshal(rule.Dimensions)
		rule.DimensionsJSON = data
	}
	if rule.TokenBucketConfig != nil {
		data, _ := json.Marshal(rule.TokenBucketConfig)
		raw := json.RawMessage(data)
		rule.TokenBucketJSON = &raw
	}
	if rule.LeakyBucketConfig != nil {
		data, _ := json.Marshal(rule.LeakyBucketConfig)
		raw := json.RawMessage(data)
		rule.LeakyBucketJSON = &raw
	}
	if rule.ShapingConfig != nil {
		data, _ := json.Marshal(rule.ShapingConfig)
		raw := json.RawMessage(data)
		rule.ShapingJSON = &raw
	}
	if rule.GrayReleaseConfig != nil {
		data, _ := json.Marshal(rule.GrayReleaseConfig)
		raw := json.RawMessage(data)
		rule.GrayReleaseJSON = &raw
	}
	data, _ := json.Marshal(rule)
	rule.ConfigJSON = data
}

func unmarshalRuleFields(rule *models.RateLimitRule) {
	if len(rule.DimensionsJSON) > 0 {
		normalized := normalizeJSONKeys(rule.DimensionsJSON)
		var dims models.RuleDimensions
		if json.Unmarshal(normalized, &dims) == nil {
			rule.Dimensions = &dims
		}
	}
	if rule.TokenBucketJSON != nil && len(*rule.TokenBucketJSON) > 0 {
		normalized := normalizeJSONKeys(*rule.TokenBucketJSON)
		var cfg models.TokenBucketConfig
		if json.Unmarshal(normalized, &cfg) == nil {
			rule.TokenBucketConfig = &cfg
		}
	}
	if rule.LeakyBucketJSON != nil && len(*rule.LeakyBucketJSON) > 0 {
		normalized := normalizeJSONKeys(*rule.LeakyBucketJSON)
		var cfg models.LeakyBucketConfig
		if json.Unmarshal(normalized, &cfg) == nil {
			rule.LeakyBucketConfig = &cfg
		}
	}
	if rule.ShapingJSON != nil && len(*rule.ShapingJSON) > 0 {
		normalized := normalizeJSONKeys(*rule.ShapingJSON)
		var cfg models.ShapingConfig
		if json.Unmarshal(normalized, &cfg) == nil {
			rule.ShapingConfig = &cfg
		}
	}
	if rule.GrayReleaseJSON != nil && len(*rule.GrayReleaseJSON) > 0 {
		normalized := normalizeJSONKeys(*rule.GrayReleaseJSON)
		var cfg models.GrayReleaseConfig
		if json.Unmarshal(normalized, &cfg) == nil {
			rule.GrayReleaseConfig = &cfg
		}
	}
}

type EventRepo struct {
	db *gorm.DB
}

func NewEventRepo(db *gorm.DB) *EventRepo {
	return &EventRepo{db: db}
}

func (e *EventRepo) List(
	page models.Pagination,
	startTime, endTime *time.Time,
	ruleID, tenantID, userID, apiPath string,
	allowed *bool,
) (*models.PaginatedResult, error) {
	query := e.db.Model(&models.RateLimitEvent{})
	if startTime != nil {
		query = query.Where("timestamp >= ?", *startTime)
	}
	if endTime != nil {
		query = query.Where("timestamp <= ?", *endTime)
	}
	if ruleID != "" {
		query = query.Where("rule_id = ?", ruleID)
	}
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if apiPath != "" {
		query = query.Where("api_path ILIKE ?", "%"+apiPath+"%")
	}
	if allowed != nil {
		query = query.Where("allowed = ?", *allowed)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var events []models.RateLimitEvent
	err := query.Order("timestamp DESC").
		Offset(page.GetOffset()).
		Limit(page.GetPageSize()).
		Find(&events).Error
	if err != nil {
		return nil, err
	}

	return &models.PaginatedResult{
		Total:    total,
		Page:     page.Page,
		PageSize: page.GetPageSize(),
		Data:     events,
	}, nil
}

func (e *EventRepo) TrafficSeries(startTime, endTime time.Time, intervalSec int, apiPath, tenantID string) ([]models.TrafficPoint, error) {
	if intervalSec <= 0 {
		intervalSec = 60
	}

	whereClauses := []string{"timestamp >= ?", "timestamp <= ?"}
	args := []interface{}{startTime, endTime}
	if apiPath != "" {
		whereClauses = append(whereClauses, "api_path = ?")
		args = append(args, apiPath)
	}
	if tenantID != "" {
		whereClauses = append(whereClauses, "tenant_id = ?")
		args = append(args, tenantID)
	}
	whereSQL := strings.Join(whereClauses, " AND ")

	sql := fmt.Sprintf(`
		SELECT
			to_timestamp(floor(extract(epoch FROM timestamp) / %d) * %d) AS bucket_ts,
			COALESCE(api_path, 'unknown') AS api_path,
			COUNT(*) FILTER (WHERE allowed = true) AS allowed,
			COUNT(*) FILTER (WHERE allowed = false) AS rejected
		FROM rate_limit_events
		WHERE %s
		GROUP BY bucket_ts, api_path
		ORDER BY bucket_ts ASC
	`, intervalSec, intervalSec, whereSQL)

	rows, err := e.db.Raw(sql, args...).Rows()
	if err != nil {
		return nil, fmt.Errorf("traffic series query failed: %w", err)
	}
	defer rows.Close()

	points := make([]models.TrafficPoint, 0)
	for rows.Next() {
		var (
			ts       time.Time
			apiPath  string
			allowed  int64
			rejected int64
		)
		if err := rows.Scan(&ts, &apiPath, &allowed, &rejected); err != nil {
			return nil, fmt.Errorf("traffic series scan failed: %w", err)
		}
		total := allowed + rejected
		ratio := 0.0
		if total > 0 {
			ratio = float64(rejected) / float64(total)
		}
		points = append(points, models.TrafficPoint{
			Timestamp:   ts,
			APIPath:     apiPath,
			Allowed:     allowed,
			Rejected:    rejected,
			Total:       total,
			RejectRatio: ratio,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return points, nil
}

func (e *EventRepo) TenantTrafficShare(startTime, endTime time.Time) ([]models.TenantTrafficShare, error) {
	sql := `
		SELECT
			COALESCE(tenant_id, 'unknown') AS tenant_id,
			COUNT(*) AS cnt
		FROM rate_limit_events
		WHERE timestamp >= ? AND timestamp <= ?
		GROUP BY COALESCE(tenant_id, 'unknown')
		ORDER BY cnt DESC
		LIMIT 20
	`
	rows, err := e.db.Raw(sql, startTime, endTime).Rows()
	if err != nil {
		return nil, fmt.Errorf("tenant traffic share query failed: %w", err)
	}
	defer rows.Close()

	type raw struct {
		TenantID string
		Count    int64
	}
	rawData := make([]raw, 0)
	var total int64
	for rows.Next() {
		var r raw
		if err := rows.Scan(&r.TenantID, &r.Count); err != nil {
			return nil, fmt.Errorf("tenant share scan failed: %w", err)
		}
		rawData = append(rawData, r)
		total += r.Count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]models.TenantTrafficShare, 0, len(rawData))
	for _, r := range rawData {
		pct := 0.0
		if total > 0 {
			pct = float64(r.Count) / float64(total) * 100
		}
		name := r.TenantID
		if r.TenantID == "unknown" {
			name = "未分类"
		}
		result = append(result, models.TenantTrafficShare{
			TenantID:     r.TenantID,
			TenantName:   name,
			RequestCount: r.Count,
			Percentage:   pct,
		})
	}
	return result, nil
}

func (e *EventRepo) Heatmap(startTime, endTime time.Time) ([]models.HeatmapPoint, error) {
	sql := `
		SELECT
			CAST(EXTRACT(HOUR FROM timestamp) AS INTEGER) AS h,
			CAST(EXTRACT(DOW FROM timestamp) AS INTEGER) AS dow,
			COUNT(*) AS cnt
		FROM rate_limit_events
		WHERE timestamp >= ? AND timestamp <= ?
		GROUP BY 1, 2
	`
	rows, err := e.db.Raw(sql, startTime, endTime).Rows()
	if err != nil {
		return nil, fmt.Errorf("heatmap query failed: %w", err)
	}
	defer rows.Close()

	points := make([]models.HeatmapPoint, 0)
	for rows.Next() {
		var (
			hour    int
			weekday int
			count   int64
		)
		if err := rows.Scan(&hour, &weekday, &count); err != nil {
			return nil, fmt.Errorf("heatmap scan failed: %w", err)
		}
		points = append(points, models.HeatmapPoint{
			Hour:    hour,
			Weekday: weekday,
			Count:   count,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return points, nil
}

type QuotaRepo struct {
	db *gorm.DB
}

func NewQuotaRepo(db *gorm.DB) *QuotaRepo {
	return &QuotaRepo{db: db}
}

func (q *QuotaRepo) List() ([]models.QuotaConfig, error) {
	var quotas []models.QuotaConfig
	err := q.db.Order("quota_level, identifier").Find(&quotas).Error
	return quotas, err
}

func (q *QuotaRepo) Get(level models.QuotaLevel, identifier string) (*models.QuotaConfig, error) {
	var quota models.QuotaConfig
	err := q.db.Where("quota_level = ? AND identifier = ?", level, identifier).
		First(&quota).Error
	if err != nil {
		return nil, err
	}
	return &quota, nil
}

func (q *QuotaRepo) Upsert(quota *models.QuotaConfig) error {
	quota.UpdatedAt = time.Now()
	return q.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "quota_level"}, {Name: "identifier"}},
		DoUpdates: clause.AssignmentColumns([]string{"limit_count", "window_seconds", "inherit_from", "override_value", "updated_at"}),
	}).Create(quota).Error
}

func (q *QuotaRepo) Delete(id int64) error {
	return q.db.Delete(&models.QuotaConfig{}, id).Error
}

type TenantRepo struct {
	db *gorm.DB
}

func NewTenantRepo(db *gorm.DB) *TenantRepo {
	return &TenantRepo{db: db}
}

func (t *TenantRepo) List() ([]models.Tenant, error) {
	var tenants []models.Tenant
	err := t.db.Order("name").Find(&tenants).Error
	return tenants, err
}

func (t *TenantRepo) ListUsers(tenantID string) ([]models.APIUser, error) {
	var users []models.APIUser
	q := t.db.Order("name")
	if tenantID != "" {
		q = q.Where("tenant_id = ?", tenantID)
	}
	err := q.Find(&users).Error
	return users, err
}

type AdaptiveRepo struct {
	db *gorm.DB
}

func NewAdaptiveRepo(db *gorm.DB) *AdaptiveRepo {
	return &AdaptiveRepo{db: db}
}

func (a *AdaptiveRepo) Get(component string) (*models.AdaptiveConfigDB, error) {
	var cfg models.AdaptiveConfigDB
	err := a.db.Where("component = ?", component).First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &cfg, err
}

func (a *AdaptiveRepo) Update(cfg *models.AdaptiveConfigDB) error {
	cfg.UpdatedAt = time.Now()
	return a.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "component"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"enabled", "target_p99_latency_ms", "error_rate_threshold",
			"min_coefficient", "max_coefficient", "tightening_ratio",
			"recovery_interval_sec", "recovery_step_percent", "stable_period_sec",
			"pid_kp", "pid_ki", "pid_kd", "pid_setpoint",
			"pid_output_min", "pid_output_max", "manual_override_coeff",
			"updated_at",
		}),
	}).Create(cfg).Error
}

type TemplateRepo struct {
	db *gorm.DB
}

func NewTemplateRepo(db *gorm.DB) *TemplateRepo {
	return &TemplateRepo{db: db}
}

func (r *TemplateRepo) List(page models.Pagination, search string) (*models.PaginatedResult, error) {
	query := r.db.Model(&models.RuleTemplate{})
	if search != "" {
		q := "%" + search + "%"
		query = query.Where("name ILIKE ? OR description ILIKE ?", q, q)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var templates []models.RuleTemplate
	err := query.Order("updated_at DESC").
		Offset(page.GetOffset()).
		Limit(page.GetPageSize()).
		Find(&templates).Error
	if err != nil {
		return nil, err
	}

	for i := range templates {
		unmarshalTemplateFields(&templates[i])
	}

	return &models.PaginatedResult{
		Total:    total,
		Page:     page.Page,
		PageSize: page.GetPageSize(),
		Data:     templates,
	}, nil
}

func (r *TemplateRepo) ListAll() ([]models.RuleTemplate, error) {
	var templates []models.RuleTemplate
	err := r.db.Model(&models.RuleTemplate{}).
		Order("name ASC").
		Find(&templates).Error
	if err != nil {
		return nil, err
	}
	for i := range templates {
		unmarshalTemplateFields(&templates[i])
	}
	return templates, nil
}

func (r *TemplateRepo) Get(id string) (*models.RuleTemplate, error) {
	var template models.RuleTemplate
	err := r.db.Where("id = ?", id).First(&template).Error
	if err != nil {
		return nil, err
	}
	unmarshalTemplateFields(&template)
	return &template, nil
}

func (r *TemplateRepo) Create(template *models.RuleTemplate) error {
	if template.ID == "" {
		template.ID = generateUUID()
	}
	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()
	marshalTemplateFields(template)
	return r.db.Create(template).Error
}

func (r *TemplateRepo) Update(template *models.RuleTemplate) error {
	existing, err := r.Get(template.ID)
	if err != nil {
		return err
	}
	template.CreatedAt = existing.CreatedAt
	template.UpdatedAt = time.Now()
	marshalTemplateFields(template)
	return r.db.Save(template).Error
}

func (r *TemplateRepo) Delete(id string) error {
	return r.db.Delete(&models.RuleTemplate{}, "id = ?", id).Error
}

func marshalTemplateFields(template *models.RuleTemplate) {
	if template.TokenBucketConfig != nil {
		data, _ := json.Marshal(template.TokenBucketConfig)
		raw := json.RawMessage(data)
		template.TokenBucketJSON = &raw
	}
	if template.LeakyBucketConfig != nil {
		data, _ := json.Marshal(template.LeakyBucketConfig)
		raw := json.RawMessage(data)
		template.LeakyBucketJSON = &raw
	}
	if template.ShapingConfig != nil {
		data, _ := json.Marshal(template.ShapingConfig)
		raw := json.RawMessage(data)
		template.ShapingJSON = &raw
	}
}

func unmarshalTemplateFields(template *models.RuleTemplate) {
	if template.TokenBucketJSON != nil && len(*template.TokenBucketJSON) > 0 {
		normalized := normalizeJSONKeys(*template.TokenBucketJSON)
		var cfg models.TokenBucketConfig
		if json.Unmarshal(normalized, &cfg) == nil {
			template.TokenBucketConfig = &cfg
		}
	}
	if template.LeakyBucketJSON != nil && len(*template.LeakyBucketJSON) > 0 {
		normalized := normalizeJSONKeys(*template.LeakyBucketJSON)
		var cfg models.LeakyBucketConfig
		if json.Unmarshal(normalized, &cfg) == nil {
			template.LeakyBucketConfig = &cfg
		}
	}
	if template.ShapingJSON != nil && len(*template.ShapingJSON) > 0 {
		normalized := normalizeJSONKeys(*template.ShapingJSON)
		var cfg models.ShapingConfig
		if json.Unmarshal(normalized, &cfg) == nil {
			template.ShapingConfig = &cfg
		}
	}
}

type AlertRuleRepo struct {
	db *gorm.DB
}

func NewAlertRuleRepo(db *gorm.DB) *AlertRuleRepo {
	return &AlertRuleRepo{db: db}
}

func (r *AlertRuleRepo) List(page models.Pagination, search string, enabled *bool) (*models.PaginatedResult, error) {
	query := r.db.Model(&models.AlertRule{})
	if search != "" {
		q := "%" + search + "%"
		query = query.Where("name ILIKE ? OR description ILIKE ?", q, q)
	}
	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var rules []models.AlertRule
	err := query.Order("updated_at DESC").
		Offset(page.GetOffset()).
		Limit(page.GetPageSize()).
		Find(&rules).Error
	if err != nil {
		return nil, err
	}

	for i := range rules {
		unmarshalAlertRuleFields(&rules[i])
	}

	return &models.PaginatedResult{
		Total:    total,
		Page:     page.Page,
		PageSize: page.GetPageSize(),
		Data:     rules,
	}, nil
}

func (r *AlertRuleRepo) ListAllEnabled() ([]models.AlertRule, error) {
	var rules []models.AlertRule
	err := r.db.Where("enabled = ?", true).Find(&rules).Error
	if err != nil {
		return nil, err
	}
	for i := range rules {
		unmarshalAlertRuleFields(&rules[i])
	}
	return rules, nil
}

func (r *AlertRuleRepo) Get(id string) (*models.AlertRule, error) {
	var rule models.AlertRule
	err := r.db.Where("id = ?", id).First(&rule).Error
	if err != nil {
		return nil, err
	}
	unmarshalAlertRuleFields(&rule)
	return &rule, nil
}

func (r *AlertRuleRepo) Create(rule *models.AlertRule) error {
	if rule.ID == "" {
		rule.ID = generateUUID()
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	marshalAlertRuleFields(rule)
	return r.db.Create(rule).Error
}

func (r *AlertRuleRepo) Update(rule *models.AlertRule) error {
	existing, err := r.Get(rule.ID)
	if err != nil {
		return err
	}
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()
	marshalAlertRuleFields(rule)
	return r.db.Save(rule).Error
}

func (r *AlertRuleRepo) Delete(id string) error {
	return r.db.Delete(&models.AlertRule{}, "id = ?", id).Error
}

func (r *AlertRuleRepo) Toggle(id string, enabled bool) error {
	res := r.db.Model(&models.AlertRule{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"enabled":    enabled,
			"updated_at": time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func marshalAlertRuleFields(rule *models.AlertRule) {
	switch rule.TriggerType {
	case models.TriggerTypeThreshold:
		if rule.ThresholdTriggerConfig != nil {
			data, _ := json.Marshal(rule.ThresholdTriggerConfig)
			rule.TriggerConfigJSON = data
		}
	case models.TriggerTypeRate:
		if rule.RateTriggerConfig != nil {
			data, _ := json.Marshal(rule.RateTriggerConfig)
			rule.TriggerConfigJSON = data
		}
	case models.TriggerTypeDuration:
		if rule.DurationTriggerConfig != nil {
			data, _ := json.Marshal(rule.DurationTriggerConfig)
			rule.TriggerConfigJSON = data
		}
	}
	if rule.NotificationChannels != nil {
		data, _ := json.Marshal(rule.NotificationChannels)
		rule.NotificationJSON = data
	}
}

func unmarshalAlertRuleFields(rule *models.AlertRule) {
	if len(rule.TriggerConfigJSON) > 0 {
		normalized := normalizeJSONKeys(rule.TriggerConfigJSON)
		switch rule.TriggerType {
		case models.TriggerTypeThreshold:
			var cfg models.ThresholdTriggerConfig
			if json.Unmarshal(normalized, &cfg) == nil {
				rule.ThresholdTriggerConfig = &cfg
			}
		case models.TriggerTypeRate:
			var cfg models.RateTriggerConfig
			if json.Unmarshal(normalized, &cfg) == nil {
				rule.RateTriggerConfig = &cfg
			}
		case models.TriggerTypeDuration:
			var cfg models.DurationTriggerConfig
			if json.Unmarshal(normalized, &cfg) == nil {
				rule.DurationTriggerConfig = &cfg
			}
		}
	}
	if len(rule.NotificationJSON) > 0 {
		var channels []string
		if json.Unmarshal(rule.NotificationJSON, &channels) == nil {
			rule.NotificationChannels = channels
		}
	}
}

type AlertEventRepo struct {
	db *gorm.DB
}

func NewAlertEventRepo(db *gorm.DB) *AlertEventRepo {
	return &AlertEventRepo{db: db}
}

func (r *AlertEventRepo) List(
	page models.Pagination,
	status *models.AlertStatus,
	severity *models.AlertSeverity,
	ruleID string,
	dimensionType string,
	dimensionValue string,
) (*models.PaginatedResult, error) {
	query := r.db.Model(&models.AlertEvent{})
	if status != nil {
		query = query.Where("status = ?", *status)
	}
	if severity != nil {
		query = query.Where("severity = ?", *severity)
	}
	if ruleID != "" {
		query = query.Where("alert_rule_id = ?", ruleID)
	}
	if dimensionType != "" {
		query = query.Where("dimension_type = ?", dimensionType)
	}
	if dimensionValue != "" {
		query = query.Where("dimension_value ILIKE ?", "%"+dimensionValue+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var events []models.AlertEvent
	err := query.Order("created_at DESC").
		Offset(page.GetOffset()).
		Limit(page.GetPageSize()).
		Find(&events).Error
	if err != nil {
		return nil, err
	}

	return &models.PaginatedResult{
		Total:    total,
		Page:     page.Page,
		PageSize: page.GetPageSize(),
		Data:     events,
	}, nil
}

func (r *AlertEventRepo) Get(id int64) (*models.AlertEvent, error) {
	var event models.AlertEvent
	err := r.db.Where("id = ?", id).First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *AlertEventRepo) Create(event *models.AlertEvent) error {
	now := time.Now()
	event.CreatedAt = now
	event.UpdatedAt = now
	if event.FiringStartedAt.IsZero() {
		event.FiringStartedAt = now
	}
	if event.LastFiringAt.IsZero() {
		event.LastFiringAt = now
	}
	return r.db.Create(event).Error
}

func (r *AlertEventRepo) Update(event *models.AlertEvent) error {
	event.UpdatedAt = time.Now()
	return r.db.Save(event).Error
}

func (r *AlertEventRepo) Acknowledge(id int64, acknowledgedBy string) error {
	now := time.Now()
	res := r.db.Model(&models.AlertEvent{}).
		Where("id = ? AND status = ?", id, models.StatusFiring).
		Updates(map[string]interface{}{
			"status":          models.StatusAcknowledged,
			"acknowledged_by": acknowledgedBy,
			"acknowledged_at": now,
			"updated_at":      now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var count int64
		r.db.Model(&models.AlertEvent{}).Where("id = ?", id).Count(&count)
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
		return errors.New("alert is not in firing state")
	}
	return nil
}

func (r *AlertEventRepo) Resolve(id int64) error {
	now := time.Now()
	res := r.db.Model(&models.AlertEvent{}).
		Where("id = ? AND status IN ?", id, []models.AlertStatus{models.StatusFiring, models.StatusAcknowledged}).
		Updates(map[string]interface{}{
			"status":     models.StatusResolved,
			"resolved_at": now,
			"updated_at": now,
		})
	return res.Error
}

func (r *AlertEventRepo) FindActiveByRuleAndDimension(ruleID, dimensionType, dimensionValue string) (*models.AlertEvent, error) {
	var event models.AlertEvent
	err := r.db.Where(
		"alert_rule_id = ? AND dimension_type = ? AND dimension_value = ? AND status IN ?",
		ruleID, dimensionType, dimensionValue,
		[]models.AlertStatus{models.StatusFiring, models.StatusAcknowledged},
	).Order("created_at DESC").First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *AlertEventRepo) ListActiveEvents() ([]models.AlertEvent, error) {
	var events []models.AlertEvent
	err := r.db.Where(
		"status IN ?",
		[]models.AlertStatus{models.StatusFiring, models.StatusAcknowledged},
	).Find(&events).Error
	return events, err
}

func (r *AlertEventRepo) GetStats() (*models.AlertStats, error) {
	stats := &models.AlertStats{}

	var firingCount int64
	if err := r.db.Model(&models.AlertEvent{}).
		Where("status = ?", models.StatusFiring).
		Count(&firingCount).Error; err != nil {
		return nil, err
	}
	stats.FiringCount = firingCount

	todayStart := time.Now().Truncate(24 * time.Hour)
	var todayCount int64
	if err := r.db.Model(&models.AlertEvent{}).
		Where("created_at >= ?", todayStart).
		Count(&todayCount).Error; err != nil {
		return nil, err
	}
	stats.TodayNewCount = todayCount

	weekStart := time.Now().AddDate(0, 0, -7)
	var weekCount int64
	if err := r.db.Model(&models.AlertEvent{}).
		Where("created_at >= ?", weekStart).
		Count(&weekCount).Error; err != nil {
		return nil, err
	}
	stats.WeekTotalCount = weekCount

	return stats, nil
}
