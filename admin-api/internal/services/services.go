package services

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ratelimiter/admin-api/internal/models"
	"github.com/ratelimiter/admin-api/internal/repository"
)

type ctxKey string

type RuleService struct {
	ruleRepo *repository.RuleRepo
	rdb      *redis.Client
}

func NewRuleService(ruleRepo *repository.RuleRepo, rdb *redis.Client) *RuleService {
	return &RuleService{ruleRepo: ruleRepo, rdb: rdb}
}

func (s *RuleService) List(page models.Pagination, search string, enabled *bool) (*models.PaginatedResult, error) {
	return s.ruleRepo.List(page, search, enabled)
}

func (s *RuleService) Get(id string) (*models.RateLimitRule, error) {
	return s.ruleRepo.Get(id)
}

func (s *RuleService) Create(rule *models.RateLimitRule) error {
	if rule.ID == "" {
		return errors.New("rule id required")
	}
	if err := s.ruleRepo.Create(rule); err != nil {
		return err
	}
	return s.broadcast(rule)
}

func (s *RuleService) Update(rule *models.RateLimitRule) error {
	if err := s.ruleRepo.Update(rule); err != nil {
		return err
	}
	return s.broadcast(rule)
}

func (s *RuleService) Delete(id string) error {
	if err := s.ruleRepo.Delete(id); err != nil {
		return err
	}
	return s.broadcastDelete(id)
}

func (s *RuleService) Toggle(id string, enabled bool) error {
	if err := s.ruleRepo.Toggle(id, enabled); err != nil {
		return err
	}
	return s.broadcastToggle(id, enabled)
}

func (s *RuleService) GetVersions(ruleID string) ([]models.RuleVersion, error) {
	return s.ruleRepo.GetVersions(ruleID)
}

func (s *RuleService) Rollback(ruleID string, version int64) error {
	if err := s.ruleRepo.Rollback(ruleID, version); err != nil {
		return err
	}
	rule, err := s.ruleRepo.Get(ruleID)
	if err != nil {
		return err
	}
	return s.broadcast(rule)
}

func (s *RuleService) BulkToggle(ids []string, enabled bool) error {
	for _, id := range ids {
		if err := s.ruleRepo.Toggle(id, enabled); err != nil {
			return err
		}
		s.broadcastToggle(id, enabled)
	}
	return nil
}

type changeMessage struct {
	Type      string               `json:"type"`
	Rule      *models.RateLimitRule `json:"rule,omitempty"`
	RuleID    string               `json:"rule_id,omitempty"`
	Enabled   bool                 `json:"enabled,omitempty"`
	Timestamp time.Time            `json:"timestamp"`
}

func (s *RuleService) broadcast(rule *models.RateLimitRule) error {
	if s.rdb == nil {
		return nil
	}
	msg := changeMessage{
		Type:      "upsert",
		Rule:      rule,
		Timestamp: time.Now(),
	}
	data, _ := json.Marshal(msg)
	return s.rdb.Publish(context.Background(), "rl:rule:updates", data).Err()
}

func (s *RuleService) broadcastDelete(id string) error {
	if s.rdb == nil {
		return nil
	}
	msg := changeMessage{
		Type:      "delete",
		RuleID:    id,
		Timestamp: time.Now(),
	}
	data, _ := json.Marshal(msg)
	return s.rdb.Publish(context.Background(), "rl:rule:updates", data).Err()
}

func (s *RuleService) broadcastToggle(id string, enabled bool) error {
	if s.rdb == nil {
		return nil
	}
	msg := changeMessage{
		Type:      "toggle",
		RuleID:    id,
		Enabled:   enabled,
		Timestamp: time.Now(),
	}
	data, _ := json.Marshal(msg)
	return s.rdb.Publish(context.Background(), "rl:rule:updates", data).Err()
}

type EventService struct {
	eventRepo *repository.EventRepo
}

func NewEventService(eventRepo *repository.EventRepo) *EventService {
	return &EventService{eventRepo: eventRepo}
}

func (s *EventService) List(
	page models.Pagination,
	startTime, endTime *time.Time,
	ruleID, tenantID, userID, apiPath string,
	allowed *bool,
) (*models.PaginatedResult, error) {
	return s.eventRepo.List(page, startTime, endTime, ruleID, tenantID, userID, apiPath, allowed)
}

func (s *EventService) TrafficSeries(lastHours int, intervalSec int, apiPath, tenantID string) ([]models.TrafficPoint, error) {
	endTime := time.Now()
	startTime := endTime.Add(-time.Duration(lastHours) * time.Hour)
	return s.eventRepo.TrafficSeries(startTime, endTime, intervalSec, apiPath, tenantID)
}

func (s *EventService) TenantTrafficShare(lastHours int) ([]models.TenantTrafficShare, error) {
	endTime := time.Now()
	startTime := endTime.Add(-time.Duration(lastHours) * time.Hour)
	return s.eventRepo.TenantTrafficShare(startTime, endTime)
}

func (s *EventService) Heatmap(lastDays int) ([]models.HeatmapPoint, error) {
	endTime := time.Now()
	startTime := endTime.Add(-time.Duration(lastDays) * 24 * time.Hour)
	return s.eventRepo.Heatmap(startTime, endTime)
}

type QuotaService struct {
	quotaRepo  *repository.QuotaRepo
	tenantRepo *repository.TenantRepo
	eventRepo  *repository.EventRepo
	rdb        *redis.Client
}

func NewQuotaService(
	quotaRepo *repository.QuotaRepo,
	tenantRepo *repository.TenantRepo,
	eventRepo *repository.EventRepo,
	rdb *redis.Client,
) *QuotaService {
	return &QuotaService{
		quotaRepo:  quotaRepo,
		tenantRepo: tenantRepo,
		eventRepo:  eventRepo,
		rdb:        rdb,
	}
}

func (s *QuotaService) List() ([]models.QuotaConfig, error) {
	return s.quotaRepo.List()
}

func (s *QuotaService) Upsert(quota *models.QuotaConfig) error {
	if err := s.quotaRepo.Upsert(quota); err != nil {
		return err
	}
	return s.broadcastQuota(quota)
}

func (s *QuotaService) Delete(id int64) error {
	return s.quotaRepo.Delete(id)
}

func (s *QuotaService) GetTree() (*models.QuotaTreeNode, error) {
	quotas, err := s.quotaRepo.List()
	if err != nil {
		return nil, err
	}
	tenants, err := s.tenantRepo.List()
	if err != nil {
		return nil, err
	}
	users, err := s.tenantRepo.ListUsers("")
	if err != nil {
		return nil, err
	}

	quotaMap := make(map[string]*models.QuotaConfig)
	for _, q := range quotas {
		key := string(q.Level) + ":" + q.Identifier
		quotaMap[key] = &q
	}

	endTime := time.Now()
	startTime := endTime.Add(-60 * time.Second)
	series, err := s.eventRepo.TrafficSeries(startTime, endTime, 60, "", "")
	if err != nil {
		return nil, err
	}
	globalUsed := int64(0)
	for _, p := range series {
		globalUsed += p.Total
	}

	root := &models.QuotaTreeNode{
		Level:      "global",
		Identifier: "global",
		Name:       "平台全局配额",
		Limit:      100000,
		Used:       globalUsed,
		InheritFrom: false,
	}
	if gq := quotaMap["global:global"]; gq != nil {
		root.Limit = gq.Limit
		root.InheritFrom = gq.InheritFrom
	}
	root.Remaining = root.Limit - root.Used
	if root.Remaining < 0 {
		root.Remaining = 0
	}
	if root.Limit > 0 {
		root.UsageRatio = float64(root.Used) / float64(root.Limit)
	}
	root.OverQuota = root.UsageRatio >= 0.9

	tenantMap := make(map[string]*models.Tenant)
	for i := range tenants {
		tenantMap[tenants[i].ID] = &tenants[i]
	}
	tenantUserMap := make(map[string][]*models.APIUser)
	for i := range users {
		u := &users[i]
		tenantUserMap[u.TenantID] = append(tenantUserMap[u.TenantID], u)
	}

	for _, t := range tenants {
		if !t.Active {
			continue
		}
		tenantSeries, _ := s.eventRepo.TrafficSeries(startTime, endTime, 60, "", t.ID)
		tenantUsed := int64(0)
		for _, p := range tenantSeries {
			tenantUsed += p.Total
		}
		tenantLimit := int64(10000)
		inherit := true
		key := "tenant:" + t.ID
		if tq := quotaMap[key]; tq != nil {
			tenantLimit = tq.Limit
			inherit = tq.InheritFrom
			if tq.OverrideValue > 0 {
				tenantLimit = tq.OverrideValue
			}
		}
		tnode := &models.QuotaTreeNode{
			Level:       "tenant",
			Identifier:  t.ID,
			Name:        t.Name,
			Limit:       min64(tenantLimit, root.Limit),
			Used:        tenantUsed,
			InheritFrom: inherit,
		}
		tnode.Remaining = tnode.Limit - tnode.Used
		if tnode.Remaining < 0 {
			tnode.Remaining = 0
		}
		if tnode.Limit > 0 {
			tnode.UsageRatio = float64(tnode.Used) / float64(tnode.Limit)
		}
		tnode.OverQuota = tnode.UsageRatio >= 0.9

		for _, u := range tenantUserMap[t.ID] {
			if !u.Active {
				continue
			}
			userUsed := int64(0)
			userLimit := int64(1000)
			inheritU := true
			userKey := "user:" + t.ID + ":" + u.ID
			if uq := quotaMap[userKey]; uq != nil {
				userLimit = uq.Limit
				inheritU = uq.InheritFrom
				if uq.OverrideValue > 0 {
					userLimit = uq.OverrideValue
				}
			}
			uname := u.Name
			if uname == "" {
				uname = u.ID
			}
			unode := &models.QuotaTreeNode{
				Level:       "user",
				Identifier:  u.ID,
				Name:        uname,
				Limit:       min64(userLimit, tnode.Limit),
				Used:        userUsed,
				InheritFrom: inheritU,
			}
			unode.Remaining = unode.Limit - unode.Used
			if unode.Remaining < 0 {
				unode.Remaining = 0
			}
			if unode.Limit > 0 {
				unode.UsageRatio = float64(unode.Used) / float64(unode.Limit)
			}
			unode.OverQuota = unode.UsageRatio >= 0.9
			tnode.Children = append(tnode.Children, unode)
		}
		root.Children = append(root.Children, tnode)
	}

	return root, nil
}

func (s *QuotaService) broadcastQuota(quota *models.QuotaConfig) error {
	if s.rdb == nil {
		return nil
	}
	data, _ := json.Marshal(quota)
	return s.rdb.Publish(context.Background(), "rl:quota:updates", data).Err()
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

type AdaptiveService struct {
	adaptiveRepo *repository.AdaptiveRepo
	rdb          *redis.Client
}

func NewAdaptiveService(adaptiveRepo *repository.AdaptiveRepo, rdb *redis.Client) *AdaptiveService {
	return &AdaptiveService{adaptiveRepo: adaptiveRepo, rdb: rdb}
}

func (s *AdaptiveService) GetStatus(component string) (*models.AdaptiveStatus, error) {
	cfg, err := s.adaptiveRepo.Get(component)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return &models.AdaptiveStatus{
			Enabled:            true,
			CurrentCoefficient: 1.0,
			TargetP99LatencyMs: 200,
			ErrorRateThreshold: 0.05,
			LastUpdated:        time.Now(),
			PIDKp:              0.5,
			PIDKi:              0.1,
			PIDKd:              0.2,
		}, nil
	}

	currentCoeff := 1.0
	if cfg.ManualOverrideCoeff != nil {
		currentCoeff = *cfg.ManualOverrideCoeff
	}

	return &models.AdaptiveStatus{
		Enabled:               cfg.Enabled,
		CurrentCoefficient:    currentCoeff,
		P99LatencyMs:          0,
		ErrorRate:             0,
		TargetP99LatencyMs:    cfg.TargetP99LatencyMs,
		ErrorRateThreshold:    cfg.ErrorRateThreshold,
		StableSince:           nil,
		LastUpdated:           cfg.UpdatedAt,
		PIDKp:                 cfg.PIDKp,
		PIDKi:                 cfg.PIDKi,
		PIDKd:                 cfg.PIDKd,
		ManualOverrideCoeff:   cfg.ManualOverrideCoeff,
	}, nil
}

func (s *AdaptiveService) UpdateConfig(component string, cfg *models.AdaptiveConfigDB) error {
	cfg.Component = component
	return s.adaptiveRepo.Update(cfg)
}

func (s *AdaptiveService) OverrideCoefficient(component string, coeff float64) error {
	cfg, err := s.adaptiveRepo.Get(component)
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = &models.AdaptiveConfigDB{Component: component}
	}
	cfg.ManualOverrideCoeff = &coeff
	return s.adaptiveRepo.Update(cfg)
}

func (s *AdaptiveService) ClearOverride(component string) error {
	cfg, err := s.adaptiveRepo.Get(component)
	if err != nil {
		return err
	}
	if cfg == nil {
		return nil
	}
	cfg.ManualOverrideCoeff = nil
	return s.adaptiveRepo.Update(cfg)
}

type TemplateService struct {
	templateRepo *repository.TemplateRepo
}

func NewTemplateService(templateRepo *repository.TemplateRepo) *TemplateService {
	return &TemplateService{templateRepo: templateRepo}
}

func (s *TemplateService) List(page models.Pagination, search string) (*models.PaginatedResult, error) {
	return s.templateRepo.List(page, search)
}

func (s *TemplateService) ListAll() ([]models.RuleTemplate, error) {
	return s.templateRepo.ListAll()
}

func (s *TemplateService) Get(id string) (*models.RuleTemplate, error) {
	return s.templateRepo.Get(id)
}

func (s *TemplateService) Create(template *models.RuleTemplate) error {
	return s.templateRepo.Create(template)
}

func (s *TemplateService) Update(template *models.RuleTemplate) error {
	return s.templateRepo.Update(template)
}

func (s *TemplateService) Delete(id string) error {
	return s.templateRepo.Delete(id)
}
