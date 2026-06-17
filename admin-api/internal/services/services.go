package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

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

type WebSocketHub struct {
	clients    map[*wsClient]bool
	broadcast  chan []byte
	register   chan *wsClient
	unregister chan *wsClient
}

type wsClient struct {
	hub  *WebSocketHub
	conn *websocket.Conn
	send chan []byte
}

func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		broadcast:  make(chan []byte, 256),
		register:   make(chan *wsClient),
		unregister: make(chan *wsClient),
		clients:    make(map[*wsClient]bool),
	}
}

func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

func (h *WebSocketHub) Broadcast(msgType string, payload interface{}) {
	msg := models.WebSocketMessage{
		Type:    msgType,
		Payload: payload,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.broadcast <- data
}

func (c *wsClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *wsClient) writePump() {
	defer func() {
		c.conn.Close()
	}()
	for message := range c.send {
		err := c.conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			break
		}
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *WebSocketHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &wsClient{hub: h, conn: conn, send: make(chan []byte, 256)}
	h.register <- client

	go client.writePump()
	go client.readPump()
}

type AlertRuleService struct {
	ruleRepo *repository.AlertRuleRepo
}

func NewAlertRuleService(ruleRepo *repository.AlertRuleRepo) *AlertRuleService {
	return &AlertRuleService{ruleRepo: ruleRepo}
}

func (s *AlertRuleService) List(page models.Pagination, search string, enabled *bool) (*models.PaginatedResult, error) {
	return s.ruleRepo.List(page, search, enabled)
}

func (s *AlertRuleService) ListAllEnabled() ([]models.AlertRule, error) {
	return s.ruleRepo.ListAllEnabled()
}

func (s *AlertRuleService) Get(id string) (*models.AlertRule, error) {
	return s.ruleRepo.Get(id)
}

func (s *AlertRuleService) Create(rule *models.AlertRule) error {
	return s.ruleRepo.Create(rule)
}

func (s *AlertRuleService) Update(rule *models.AlertRule) error {
	return s.ruleRepo.Update(rule)
}

func (s *AlertRuleService) Delete(id string) error {
	return s.ruleRepo.Delete(id)
}

func (s *AlertRuleService) Toggle(id string, enabled bool) error {
	return s.ruleRepo.Toggle(id, enabled)
}

type AlertEventService struct {
	eventRepo *repository.AlertEventRepo
	hub       *WebSocketHub
}

func NewAlertEventService(eventRepo *repository.AlertEventRepo, hub *WebSocketHub) *AlertEventService {
	return &AlertEventService{eventRepo: eventRepo, hub: hub}
}

func (s *AlertEventService) List(
	page models.Pagination,
	status *models.AlertStatus,
	severity *models.AlertSeverity,
	ruleID string,
	dimensionType string,
	dimensionValue string,
) (*models.PaginatedResult, error) {
	return s.eventRepo.List(page, status, severity, ruleID, dimensionType, dimensionValue)
}

func (s *AlertEventService) Get(id int64) (*models.AlertEvent, error) {
	return s.eventRepo.Get(id)
}

func (s *AlertEventService) Create(event *models.AlertEvent) error {
	err := s.eventRepo.Create(event)
	if err != nil {
		return err
	}
	s.pushAlert(event)
	return nil
}

func (s *AlertEventService) Update(event *models.AlertEvent) error {
	return s.eventRepo.Update(event)
}

func (s *AlertEventService) Acknowledge(id int64, acknowledgedBy string) error {
	err := s.eventRepo.Acknowledge(id, acknowledgedBy)
	if err != nil {
		return err
	}
	event, err := s.eventRepo.Get(id)
	if err == nil {
		s.hub.Broadcast("alert_updated", event)
	}
	return nil
}

func (s *AlertEventService) Resolve(id int64) error {
	err := s.eventRepo.Resolve(id)
	if err != nil {
		return err
	}
	event, err := s.eventRepo.Get(id)
	if err == nil {
		s.hub.Broadcast("alert_resolved", event)
	}
	return nil
}

func (s *AlertEventService) GetStats() (*models.AlertStats, error) {
	return s.eventRepo.GetStats()
}

func (s *AlertEventService) pushAlert(event *models.AlertEvent) {
	pushMsg := models.AlertPushMessage{
		ID:             event.ID,
		Severity:       event.Severity,
		RuleName:       event.RuleName,
		DimensionType:  event.DimensionType,
		DimensionValue: event.DimensionValue,
		TriggerTime:    event.CreatedAt,
		CurrentValue:   event.CurrentValue,
		ThresholdValue: event.ThresholdValue,
		Status:         event.Status,
	}
	s.hub.Broadcast("alert_firing", pushMsg)
}

type AlertEngineService struct {
	ruleSvc        *AlertRuleService
	eventSvc       *AlertEventService
	alertEventRepo *repository.AlertEventRepo
	db             *gorm.DB
	lastFired      map[string]time.Time
}

func NewAlertEngineService(
	ruleSvc *AlertRuleService,
	eventSvc *AlertEventService,
	alertEventRepo *repository.AlertEventRepo,
	db *gorm.DB,
) *AlertEngineService {
	return &AlertEngineService{
		ruleSvc:        ruleSvc,
		eventSvc:       eventSvc,
		alertEventRepo: alertEventRepo,
		db:             db,
		lastFired:      make(map[string]time.Time),
	}
}

func (e *AlertEngineService) Evaluate() error {
	rules, err := e.ruleSvc.ListAllEnabled()
	if err != nil {
		return err
	}

	for _, rule := range rules {
		silentKey := "rule:" + rule.ID
		lastFired, hasLast := e.lastFired[silentKey]
		if hasLast && time.Since(lastFired) < time.Duration(rule.SilentPeriodSeconds)*time.Second {
			continue
		}

		switch rule.TriggerType {
		case models.TriggerTypeThreshold:
			e.evaluateThresholdRule(&rule)
		case models.TriggerTypeRate:
			e.evaluateRateRule(&rule)
		case models.TriggerTypeDuration:
			e.evaluateDurationRule(&rule)
		}
	}

	e.checkResolvedAlerts()
	return nil
}

func (e *AlertEngineService) evaluateThresholdRule(rule *models.AlertRule) {
	if rule.ThresholdTriggerConfig == nil {
		return
	}
	cfg := rule.ThresholdTriggerConfig
	window := time.Duration(cfg.WindowSeconds) * time.Second
	endTime := time.Now()
	startTime := endTime.Add(-window)

	dimType, dimValues := e.getDimensionValues(rule, startTime, endTime)

	for _, dimValue := range dimValues {
		count := e.getRejectCountByDimension(rule, dimType, dimValue, startTime, endTime)
		threshold := float64(cfg.Threshold)

		if float64(count) >= threshold {
			dimKey := rule.ID + ":" + dimType + ":" + dimValue
			active, _ := e.alertEventRepo.FindActiveByRuleAndDimension(rule.ID, dimType, dimValue)

			if active == nil {
				event := &models.AlertEvent{
					AlertRuleID:    rule.ID,
					RuleName:       rule.Name,
					Severity:       rule.Severity,
					Status:         models.StatusFiring,
					DimensionType:  dimType,
					DimensionValue: dimValue,
					CurrentValue:   float64(count),
					ThresholdValue: threshold,
					TriggerSnapshot: e.buildSnapshot(map[string]interface{}{
						"windowSeconds": cfg.WindowSeconds,
						"metric":        cfg.Metric,
						"rejectCount":   count,
					}),
				}
				_ = e.eventSvc.Create(event)
				e.lastFired["rule:"+rule.ID] = time.Now()
			} else {
				active.CurrentValue = float64(count)
				active.LastFiringAt = time.Now()
				_ = e.alertEventRepo.Update(active)
			}
			_ = dimKey
		}
	}
}

func (e *AlertEngineService) evaluateRateRule(rule *models.AlertRule) {
	if rule.RateTriggerConfig == nil {
		return
	}
	cfg := rule.RateTriggerConfig
	window := time.Duration(cfg.WindowSeconds) * time.Second
	endTime := time.Now()
	startTime := endTime.Add(-window)

	dimType, dimValues := e.getDimensionValues(rule, startTime, endTime)

	for _, dimValue := range dimValues {
		rejectCount, totalCount := e.getRejectAndTotalByDimension(rule, dimType, dimValue, startTime, endTime)
		var rejectRate float64
		if totalCount > 0 {
			rejectRate = float64(rejectCount) / float64(totalCount) * 100
		}

		if rejectRate >= cfg.ThresholdPercent {
			active, _ := e.alertEventRepo.FindActiveByRuleAndDimension(rule.ID, dimType, dimValue)

			if active == nil {
				event := &models.AlertEvent{
					AlertRuleID:    rule.ID,
					RuleName:       rule.Name,
					Severity:       rule.Severity,
					Status:         models.StatusFiring,
					DimensionType:  dimType,
					DimensionValue: dimValue,
					CurrentValue:   rejectRate,
					ThresholdValue: cfg.ThresholdPercent,
					TriggerSnapshot: e.buildSnapshot(map[string]interface{}{
						"windowSeconds":    cfg.WindowSeconds,
						"metric":           cfg.Metric,
						"rejectCount":      rejectCount,
						"totalCount":       totalCount,
						"rejectRatePercent": rejectRate,
					}),
				}
				_ = e.eventSvc.Create(event)
				e.lastFired["rule:"+rule.ID] = time.Now()
			} else {
				active.CurrentValue = rejectRate
				active.LastFiringAt = time.Now()
				_ = e.alertEventRepo.Update(active)
			}
		}
	}
}

func (e *AlertEngineService) evaluateDurationRule(rule *models.AlertRule) {
	if rule.DurationTriggerConfig == nil {
		return
	}
	cfg := rule.DurationTriggerConfig
	duration := time.Duration(cfg.DurationSeconds) * time.Second
	endTime := time.Now()
	startTime := endTime.Add(-duration)

	dimType, dimValues := e.getDimensionValues(rule, startTime, endTime)

	for _, dimValue := range dimValues {
		hasRejection := e.hasContinuousRejections(rule, dimType, dimValue, startTime, endTime)

		if hasRejection {
			active, _ := e.alertEventRepo.FindActiveByRuleAndDimension(rule.ID, dimType, dimValue)

			if active == nil {
				event := &models.AlertEvent{
					AlertRuleID:    rule.ID,
					RuleName:       rule.Name,
					Severity:       rule.Severity,
					Status:         models.StatusFiring,
					DimensionType:  dimType,
					DimensionValue: dimValue,
					CurrentValue:   float64(cfg.DurationSeconds),
					ThresholdValue: float64(cfg.DurationSeconds),
					TriggerSnapshot: e.buildSnapshot(map[string]interface{}{
						"durationSeconds": cfg.DurationSeconds,
						"metric":          cfg.Metric,
					}),
				}
				_ = e.eventSvc.Create(event)
				e.lastFired["rule:"+rule.ID] = time.Now()
			} else {
				active.LastFiringAt = time.Now()
				_ = e.alertEventRepo.Update(active)
			}
		}
	}
}

func (e *AlertEngineService) checkResolvedAlerts() {
	activeEvents, err := e.alertEventRepo.ListActiveEvents()
	if err != nil {
		return
	}

	for _, event := range activeEvents {
		rule, err := e.ruleSvc.Get(event.AlertRuleID)
		if err != nil || rule == nil {
			continue
		}

		resolved := false
		switch rule.TriggerType {
		case models.TriggerTypeThreshold:
			if rule.ThresholdTriggerConfig != nil {
				window := time.Duration(rule.ThresholdTriggerConfig.WindowSeconds) * time.Second
				endTime := time.Now()
				startTime := endTime.Add(-window)
				count := e.getRejectCountByDimension(rule, event.DimensionType, event.DimensionValue, startTime, endTime)
				if count < int64(rule.ThresholdTriggerConfig.Threshold) {
					resolved = true
				}
			}
		case models.TriggerTypeRate:
			if rule.RateTriggerConfig != nil {
				window := time.Duration(rule.RateTriggerConfig.WindowSeconds) * time.Second
				endTime := time.Now()
				startTime := endTime.Add(-window)
				rejectCount, totalCount := e.getRejectAndTotalByDimension(rule, event.DimensionType, event.DimensionValue, startTime, endTime)
				var rejectRate float64
				if totalCount > 0 {
					rejectRate = float64(rejectCount) / float64(totalCount) * 100
				}
				if rejectRate < rule.RateTriggerConfig.ThresholdPercent {
					resolved = true
				}
			}
		case models.TriggerTypeDuration:
			if rule.DurationTriggerConfig != nil {
				window := time.Duration(rule.DurationTriggerConfig.DurationSeconds) * time.Second
				endTime := time.Now()
				startTime := endTime.Add(-window)
				hasRejection := e.hasContinuousRejections(rule, event.DimensionType, event.DimensionValue, startTime, endTime)
				if !hasRejection {
					resolved = true
				}
			}
		}

		if resolved {
			_ = e.eventSvc.Resolve(event.ID)
		}
	}
}

func (e *AlertEngineService) getDimensionValues(rule *models.AlertRule, startTime, endTime time.Time) (string, []string) {
	switch rule.ScopeType {
	case models.ScopeAPI:
		return "api_path", []string{rule.ScopeValue}
	case models.ScopeTenant:
		return "tenant_id", []string{rule.ScopeValue}
	default:
		return e.getDistinctDimensions(rule, startTime, endTime)
	}
}

func (e *AlertEngineService) getDistinctDimensions(rule *models.AlertRule, startTime, endTime time.Time) (string, []string) {
	type dimRow struct {
		Value string
	}

	if rule.TriggerType == models.TriggerTypeRate && rule.ScopeType == models.ScopeGlobal {
		var results []dimRow
		sql := `SELECT DISTINCT COALESCE(tenant_id, 'unknown') as value FROM rate_limit_events WHERE timestamp >= ? AND timestamp <= ? LIMIT 100`
		e.db.Raw(sql, startTime, endTime).Scan(&results)
		values := make([]string, 0, len(results))
		for _, r := range results {
			values = append(values, r.Value)
		}
		return "tenant_id", values
	}

	var results []dimRow
	sql := `SELECT DISTINCT COALESCE(api_path, 'unknown') as value FROM rate_limit_events WHERE timestamp >= ? AND timestamp <= ? LIMIT 100`
	e.db.Raw(sql, startTime, endTime).Scan(&results)
	values := make([]string, 0, len(results))
	for _, r := range results {
		values = append(values, r.Value)
	}
	return "api_path", values
}

func (e *AlertEngineService) getRejectCountByDimension(rule *models.AlertRule, dimType, dimValue string, startTime, endTime time.Time) int64 {
	var count int64
	var whereSQL string
	var args []interface{}

	switch dimType {
	case "tenant_id":
		whereSQL = "tenant_id = ? AND allowed = false AND timestamp >= ? AND timestamp <= ?"
		args = []interface{}{dimValue, startTime, endTime}
	default:
		whereSQL = "api_path = ? AND allowed = false AND timestamp >= ? AND timestamp <= ?"
		args = []interface{}{dimValue, startTime, endTime}
	}

	e.db.Model(&models.RateLimitEvent{}).
		Where(whereSQL, args...).
		Count(&count)
	return count
}

func (e *AlertEngineService) getRejectAndTotalByDimension(rule *models.AlertRule, dimType, dimValue string, startTime, endTime time.Time) (int64, int64) {
	type result struct {
		Rejected int64
		Total    int64
	}
	var res result
	var whereSQL string
	var args []interface{}

	switch dimType {
	case "tenant_id":
		whereSQL = "tenant_id = ? AND timestamp >= ? AND timestamp <= ?"
		args = []interface{}{dimValue, startTime, endTime}
	default:
		whereSQL = "api_path = ? AND timestamp >= ? AND timestamp <= ?"
		args = []interface{}{dimValue, startTime, endTime}
	}

	e.db.Model(&models.RateLimitEvent{}).
		Select("COUNT(*) FILTER (WHERE allowed = false) as rejected, COUNT(*) as total").
		Where(whereSQL, args...).
		Scan(&res)
	return res.Rejected, res.Total
}

func (e *AlertEngineService) hasContinuousRejections(rule *models.AlertRule, dimType, dimValue string, startTime, endTime time.Time) bool {
	var count int64
	var whereSQL string
	var args []interface{}

	switch dimType {
	case "tenant_id":
		whereSQL = "tenant_id = ? AND allowed = false AND timestamp >= ? AND timestamp <= ?"
		args = []interface{}{dimValue, startTime, endTime}
	default:
		whereSQL = "api_path = ? AND allowed = false AND timestamp >= ? AND timestamp <= ?"
		args = []interface{}{dimValue, startTime, endTime}
	}

	e.db.Model(&models.RateLimitEvent{}).
		Where(whereSQL, args...).
		Count(&count)
	return count > 0
}

func (e *AlertEngineService) buildSnapshot(data map[string]interface{}) json.RawMessage {
	b, _ := json.Marshal(data)
	return b
}
