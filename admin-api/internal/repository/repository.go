package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/ratelimiter/admin-api/internal/models"
)

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
	err := r.db.First(&rule, "id = ?", id).Error
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
		var dims models.RuleDimensions
		if json.Unmarshal(rule.DimensionsJSON, &dims) == nil {
			rule.Dimensions = &dims
		}
	}
	if rule.TokenBucketJSON != nil && len(*rule.TokenBucketJSON) > 0 {
		var cfg models.TokenBucketConfig
		if json.Unmarshal(*rule.TokenBucketJSON, &cfg) == nil {
			rule.TokenBucketConfig = &cfg
		}
	}
	if rule.LeakyBucketJSON != nil && len(*rule.LeakyBucketJSON) > 0 {
		var cfg models.LeakyBucketConfig
		if json.Unmarshal(*rule.LeakyBucketJSON, &cfg) == nil {
			rule.LeakyBucketConfig = &cfg
		}
	}
	if rule.ShapingJSON != nil && len(*rule.ShapingJSON) > 0 {
		var cfg models.ShapingConfig
		if json.Unmarshal(*rule.ShapingJSON, &cfg) == nil {
			rule.ShapingConfig = &cfg
		}
	}
	if rule.GrayReleaseJSON != nil && len(*rule.GrayReleaseJSON) > 0 {
		var cfg models.GrayReleaseConfig
		if json.Unmarshal(*rule.GrayReleaseJSON, &cfg) == nil {
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
	dbType := e.db.Dialector.Name()
	var timeTruncExpr string
	switch dbType {
	case "postgres":
		timeTruncExpr = fmt.Sprintf("date_trunc('second', timestamp) - (MOD(EXTRACT(SECOND FROM timestamp)::int, %d) || ' seconds')::interval", intervalSec)
	default:
		timeTruncExpr = "timestamp"
	}

	rows, err := e.db.Model(&models.RateLimitEvent{}).
		Select(fmt.Sprintf(`%s as bucket_ts,
			COUNT(*) FILTER (WHERE allowed = true) as allowed,
			COUNT(*) FILTER (WHERE allowed = false) as rejected`, timeTruncExpr)).
		Where("timestamp >= ? AND timestamp <= ?", startTime, endTime).
		Where(func(db *gorm.DB) *gorm.DB {
			q := db
			if apiPath != "" {
				q = q.Where("api_path = ?", apiPath)
			}
			if tenantID != "" {
				q = q.Where("tenant_id = ?", tenantID)
			}
			return q
		}).
		Group("bucket_ts").
		Order("bucket_ts ASC").
		Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]models.TrafficPoint, 0)
	for rows.Next() {
		var ts time.Time
		var allowed, rejected int64
		rows.Scan(&ts, &allowed, &rejected)
		total := allowed + rejected
		ratio := 0.0
		if total > 0 {
			ratio = float64(rejected) / float64(total)
		}
		points = append(points, models.TrafficPoint{
			Timestamp:   ts,
			Allowed:     allowed,
			Rejected:    rejected,
			Total:       total,
			RejectRatio: ratio,
		})
	}
	return points, nil
}

func (e *EventRepo) TenantTrafficShare(startTime, endTime time.Time) ([]models.TenantTrafficShare, error) {
	rows, err := e.db.Model(&models.RateLimitEvent{}).
		Select(`COALESCE(tenant_id, 'unknown') as tenant_id,
			COUNT(*) as cnt`).
		Where("timestamp >= ? AND timestamp <= ?", startTime, endTime).
		Group("tenant_id").
		Order("cnt DESC").
		Limit(20).
		Rows()
	if err != nil {
		return nil, err
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
		rows.Scan(&r.TenantID, &r.Count)
		rawData = append(rawData, r)
		total += r.Count
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
	rows, err := e.db.Model(&models.RateLimitEvent{}).
		Select(`EXTRACT(HOUR FROM timestamp)::int as h,
			EXTRACT(MINUTE FROM timestamp)::int / 5 * 5 as m,
			EXTRACT(ISODOW FROM timestamp)::int as dow,
			COUNT(*) as cnt`).
		Where("timestamp >= ? AND timestamp <= ?", startTime, endTime).
		Group("h, m, dow").
		Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]models.HeatmapPoint, 0)
	for rows.Next() {
		var p models.HeatmapPoint
		rows.Scan(&p.Hour, &p.Minute, &p.DayOfWeek, &p.Count)
		points = append(points, p)
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
