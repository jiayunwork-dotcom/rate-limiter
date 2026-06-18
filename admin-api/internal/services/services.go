package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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
	eventRepo     *repository.AlertEventRepo
	hub           *WebSocketHub
	suppressionSvc *AlertSuppressionService
	aggregationSvc *AlertAggregationService
}

func NewAlertEventService(eventRepo *repository.AlertEventRepo, hub *WebSocketHub) *AlertEventService {
	return &AlertEventService{eventRepo: eventRepo, hub: hub}
}

func (s *AlertEventService) SetSuppressionService(svc *AlertSuppressionService) {
	s.suppressionSvc = svc
}

func (s *AlertEventService) SetAggregationService(svc *AlertAggregationService) {
	s.aggregationSvc = svc
}

func (s *AlertEventService) List(
	page models.Pagination,
	status *models.AlertStatus,
	severity *models.AlertSeverity,
	ruleID string,
	dimensionType string,
	dimensionValue string,
	includeSuppressed bool,
) (*models.PaginatedResult, error) {
	return s.eventRepo.List(page, status, severity, ruleID, dimensionType, dimensionValue, includeSuppressed)
}

func (s *AlertEventService) Get(id int64) (*models.AlertEvent, error) {
	return s.eventRepo.Get(id)
}

func (s *AlertEventService) Create(event *models.AlertEvent) error {
	if s.suppressionSvc != nil {
		suppressed, ruleID := s.suppressionSvc.CheckSuppression(event)
		if suppressed {
			event.Suppressed = true
			event.SuppressedByRuleID = ruleID
		}
	}

	err := s.eventRepo.Create(event)
	if err != nil {
		return err
	}

	if !event.Suppressed {
		if s.aggregationSvc != nil {
			aggregated, _ := s.aggregationSvc.ProcessAlert(event)
			if aggregated {
				return nil
			}
		}
		s.pushAlert(event)
	}
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
				now := time.Now()
				event := &models.AlertEvent{
					AlertRuleID:    rule.ID,
					RuleName:       rule.Name,
					Severity:       rule.Severity,
					Status:         models.StatusFiring,
					DimensionType:  dimType,
					DimensionValue: dimValue,
					CurrentValue:   float64(count),
					ThresholdValue: threshold,
					FiringStartedAt: now,
					LastFiringAt:    now,
					TriggerSnapshot: e.buildSnapshot(map[string]interface{}{
						"windowSeconds": cfg.WindowSeconds,
						"metric":        cfg.Metric,
						"rejectCount":   count,
					}),
				}
				_ = e.eventSvc.Create(event)
				e.lastFired["rule:"+rule.ID] = now
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
				now := time.Now()
				event := &models.AlertEvent{
					AlertRuleID:    rule.ID,
					RuleName:       rule.Name,
					Severity:       rule.Severity,
					Status:         models.StatusFiring,
					DimensionType:  dimType,
					DimensionValue: dimValue,
					CurrentValue:   rejectRate,
					ThresholdValue: cfg.ThresholdPercent,
					FiringStartedAt: now,
					LastFiringAt:    now,
					TriggerSnapshot: e.buildSnapshot(map[string]interface{}{
						"windowSeconds":    cfg.WindowSeconds,
						"metric":           cfg.Metric,
						"rejectCount":      rejectCount,
						"totalCount":       totalCount,
						"rejectRatePercent": rejectRate,
					}),
				}
				_ = e.eventSvc.Create(event)
				e.lastFired["rule:"+rule.ID] = now
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
				now := time.Now()
				event := &models.AlertEvent{
					AlertRuleID:    rule.ID,
					RuleName:       rule.Name,
					Severity:       rule.Severity,
					Status:         models.StatusFiring,
					DimensionType:  dimType,
					DimensionValue: dimValue,
					CurrentValue:   float64(cfg.DurationSeconds),
					ThresholdValue: float64(cfg.DurationSeconds),
					FiringStartedAt: now,
					LastFiringAt:    now,
					TriggerSnapshot: e.buildSnapshot(map[string]interface{}{
						"durationSeconds": cfg.DurationSeconds,
						"metric":          cfg.Metric,
					}),
				}
				_ = e.eventSvc.Create(event)
				e.lastFired["rule:"+rule.ID] = now
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
	type rangeResult struct {
		FirstTs time.Time
		LastTs  time.Time
		Count   int64
	}

	var dimColumn string
	switch dimType {
	case "tenant_id":
		dimColumn = "tenant_id"
	default:
		dimColumn = "api_path"
	}

	var res rangeResult
	sql := `SELECT MIN(timestamp) as first_ts, MAX(timestamp) as last_ts, COUNT(*) as count 
	        FROM rate_limit_events 
	        WHERE ` + dimColumn + ` = ? AND allowed = false AND timestamp >= ? AND timestamp <= ?`
	e.db.Raw(sql, dimValue, startTime, endTime).Scan(&res)

	totalWindow := endTime.Sub(startTime)
	if totalWindow <= 0 {
		return false
	}

	coverageThreshold := totalWindow * 7 / 10

	if res.Count < 3 {
		return false
	}

	coverage := res.LastTs.Sub(res.FirstTs)
	if coverage < coverageThreshold {
		return false
	}

	totalSeconds := int64(totalWindow.Seconds())
	numBuckets := numBucketsForWindow(totalSeconds)
	bucketSize := totalWindow / time.Duration(numBuckets)
	coveredBuckets := 0

	for i := int64(0); i < numBuckets; i++ {
		bucketStart := startTime.Add(time.Duration(i) * bucketSize)
		bucketEnd := bucketStart.Add(bucketSize)
		var bucketCount int64
		e.db.Model(&models.RateLimitEvent{}).
			Where(dimColumn+" = ? AND allowed = false AND timestamp >= ? AND timestamp < ?",
				dimValue, bucketStart, bucketEnd).
			Count(&bucketCount)
		if bucketCount > 0 {
			coveredBuckets++
		}
	}

	return coveredBuckets*10 >= int(numBuckets)*7
}

func numBucketsForWindow(totalSeconds int64) int64 {
	if totalSeconds <= 30 {
		return 6
	}
	if totalSeconds <= 60 {
		return 10
	}
	if totalSeconds <= 300 {
		return 15
	}
	return 20
}

func (e *AlertEngineService) buildSnapshot(data map[string]interface{}) json.RawMessage {
	b, _ := json.Marshal(data)
	return b
}

type AlertAggregationRuleService struct {
	ruleRepo *repository.AlertAggregationRuleRepo
}

func NewAlertAggregationRuleService(ruleRepo *repository.AlertAggregationRuleRepo) *AlertAggregationRuleService {
	return &AlertAggregationRuleService{ruleRepo: ruleRepo}
}

func (s *AlertAggregationRuleService) List(page models.Pagination, enabled *bool) (*models.PaginatedResult, error) {
	return s.ruleRepo.List(page, enabled)
}

func (s *AlertAggregationRuleService) ListAllEnabled() ([]models.AlertAggregationRule, error) {
	return s.ruleRepo.ListAllEnabled()
}

func (s *AlertAggregationRuleService) Get(id string) (*models.AlertAggregationRule, error) {
	return s.ruleRepo.Get(id)
}

func (s *AlertAggregationRuleService) Create(rule *models.AlertAggregationRule) error {
	return s.ruleRepo.Create(rule)
}

func (s *AlertAggregationRuleService) Update(rule *models.AlertAggregationRule) error {
	return s.ruleRepo.Update(rule)
}

func (s *AlertAggregationRuleService) Delete(id string) error {
	return s.ruleRepo.Delete(id)
}

func (s *AlertAggregationRuleService) Toggle(id string, enabled bool) error {
	return s.ruleRepo.Toggle(id, enabled)
}

type AlertSuppressionRuleService struct {
	ruleRepo *repository.AlertSuppressionRuleRepo
}

func NewAlertSuppressionRuleService(ruleRepo *repository.AlertSuppressionRuleRepo) *AlertSuppressionRuleService {
	return &AlertSuppressionRuleService{ruleRepo: ruleRepo}
}

func (s *AlertSuppressionRuleService) List(page models.Pagination, enabled *bool) (*models.PaginatedResult, error) {
	return s.ruleRepo.List(page, enabled)
}

func (s *AlertSuppressionRuleService) ListAllEnabled() ([]models.AlertSuppressionRule, error) {
	return s.ruleRepo.ListAllEnabled()
}

func (s *AlertSuppressionRuleService) Get(id string) (*models.AlertSuppressionRule, error) {
	return s.ruleRepo.Get(id)
}

func (s *AlertSuppressionRuleService) Create(rule *models.AlertSuppressionRule) error {
	return s.ruleRepo.Create(rule)
}

func (s *AlertSuppressionRuleService) Update(rule *models.AlertSuppressionRule) error {
	return s.ruleRepo.Update(rule)
}

func (s *AlertSuppressionRuleService) Delete(id string) error {
	return s.ruleRepo.Delete(id)
}

func (s *AlertSuppressionRuleService) Toggle(id string, enabled bool) error {
	return s.ruleRepo.Toggle(id, enabled)
}

type AlertAggregationService struct {
	aggregationRuleSvc  *AlertAggregationRuleService
	groupRepo           *repository.AlertAggregationGroupRepo
	aggEventRepo        *repository.AlertAggregationEventRepo
	eventRepo           *repository.AlertEventRepo
	hub                 *WebSocketHub
}

func NewAlertAggregationService(
	aggregationRuleSvc *AlertAggregationRuleService,
	groupRepo *repository.AlertAggregationGroupRepo,
	aggEventRepo *repository.AlertAggregationEventRepo,
	eventRepo *repository.AlertEventRepo,
	hub *WebSocketHub,
) *AlertAggregationService {
	return &AlertAggregationService{
		aggregationRuleSvc: aggregationRuleSvc,
		groupRepo:          groupRepo,
		aggEventRepo:       aggEventRepo,
		eventRepo:          eventRepo,
		hub:                hub,
	}
}

func (s *AlertAggregationService) ProcessAlert(event *models.AlertEvent) (bool, *models.AlertAggregationGroup) {
	rules, err := s.aggregationRuleSvc.ListAllEnabled()
	if err != nil || len(rules) == 0 {
		return false, nil
	}

	for _, rule := range rules {
		dimValue := s.getAggregationDimensionValue(event, rule.DimensionType)
		if dimValue == "" {
			continue
		}

		group, err := s.groupRepo.FindActiveGroup(rule.ID, rule.DimensionType, dimValue)
		now := time.Now()
		windowEnds := now.Add(time.Duration(rule.WindowSeconds) * time.Second)

		if err != nil || group == nil || now.After(group.WindowEndsAt) {
			group = &models.AlertAggregationGroup{
				AggregationRuleID: rule.ID,
				DimensionType:     rule.DimensionType,
				DimensionValue:    dimValue,
				TriggerCount:      1,
				FirstTriggeredAt:  now,
				LastTriggeredAt:   now,
				WindowEndsAt:      windowEnds,
				Severity:          event.Severity,
				Status:            models.StatusFiring,
				UniqueValues:      []string{event.DimensionValue},
			}
			if err := s.groupRepo.Create(group); err != nil {
				continue
			}
		} else {
			group.TriggerCount++
			group.LastTriggeredAt = now
			group.WindowEndsAt = windowEnds

			hasValue := false
			for _, v := range group.UniqueValues {
				if v == event.DimensionValue {
					hasValue = true
					break
				}
			}
			if !hasValue {
				group.UniqueValues = append(group.UniqueValues, event.DimensionValue)
			}

			if s.compareSeverity(event.Severity, group.Severity) > 0 {
				group.Severity = event.Severity
			}

			if err := s.groupRepo.Update(group); err != nil {
				continue
			}
		}

		aggEvent := &models.AlertAggregationEvent{
			AggregationGroupID: group.ID,
			AlertEventID:       event.ID,
		}
		_ = s.aggEventRepo.Create(aggEvent)

		s.pushAggregationUpdate(group, event.DimensionValue)

		return true, group
	}

	return false, nil
}

func (s *AlertAggregationService) getAggregationDimensionValue(event *models.AlertEvent, dimType models.AggregationDimensionType) string {
	switch dimType {
	case models.AggregateByRule:
		return event.AlertRuleID
	default:
		if event.DimensionType == string(dimType) {
			return event.DimensionValue
		}
		return ""
	}
}

func (s *AlertAggregationService) compareSeverity(a, b models.AlertSeverity) int {
	severityOrder := map[models.AlertSeverity]int{
		models.SeverityInfo:     1,
		models.SeverityWarning:  2,
		models.SeverityCritical: 3,
	}
	return severityOrder[a] - severityOrder[b]
}

func (s *AlertAggregationService) pushAggregationUpdate(group *models.AlertAggregationGroup, latestDim string) {
	msg := models.AggregationPushMessage{
		GroupID:           group.ID,
		AggregationRuleID: group.AggregationRuleID,
		DimensionType:     group.DimensionType,
		DimensionValue:    group.DimensionValue,
		TriggerCount:      group.TriggerCount,
		FirstTriggeredAt:  group.FirstTriggeredAt,
		LastTriggeredAt:   group.LastTriggeredAt,
		Severity:          group.Severity,
		Status:            group.Status,
		LatestDimension:   latestDim,
		UniqueValues:      group.UniqueValues,
	}
	s.hub.Broadcast("alert_aggregation_updated", msg)
}

func (s *AlertAggregationService) ListActiveGroups(page models.Pagination) (*models.PaginatedResult, error) {
	return s.groupRepo.ListActiveGroups(page)
}

func (s *AlertAggregationService) GetGroupEvents(groupID int64, page models.Pagination) (*models.PaginatedResult, error) {
	return s.groupRepo.GetEventsForGroup(groupID, page)
}

type AlertSuppressionService struct {
	suppressionRuleSvc *AlertSuppressionRuleService
	eventRepo          *repository.AlertEventRepo
}

func NewAlertSuppressionService(
	suppressionRuleSvc *AlertSuppressionRuleService,
	eventRepo *repository.AlertEventRepo,
) *AlertSuppressionService {
	return &AlertSuppressionService{
		suppressionRuleSvc: suppressionRuleSvc,
		eventRepo:          eventRepo,
	}
}

func (s *AlertSuppressionService) CheckSuppression(event *models.AlertEvent) (bool, string) {
	rules, err := s.suppressionRuleSvc.ListAllEnabled()
	if err != nil || len(rules) == 0 {
		return false, ""
	}

	activeEvents, err := s.eventRepo.ListActiveEvents()
	if err != nil {
		return false, ""
	}

	activeFiringEvents := make([]models.AlertEvent, 0)
	for _, e := range activeEvents {
		if e.Status == models.StatusFiring && !e.Suppressed {
			activeFiringEvents = append(activeFiringEvents, e)
		}
	}

	if len(activeFiringEvents) == 0 {
		return false, ""
	}

	for _, rule := range rules {
		if !s.matchesSourceSeverity(event.Severity, rule.SourceSeverity) {
			continue
		}

		for _, sourceEvent := range activeFiringEvents {
			if sourceEvent.ID == event.ID {
				continue
			}

			if rule.SourceRuleID != "" && sourceEvent.AlertRuleID != rule.SourceRuleID {
				continue
			}

			if !s.matchesTargetSeverity(event.Severity, rule.TargetSeverity) {
				continue
			}

			if rule.TargetDimensionType != "" && event.DimensionType != rule.TargetDimensionType {
				continue
			}

			if !s.dimensionsMatch(&sourceEvent, event, rule.MatchDimensionFields) {
				continue
			}

			return true, rule.ID
		}
	}

	return false, ""
}

func (s *AlertSuppressionService) matchesSourceSeverity(targetSeverity, sourceSeverity models.AlertSeverity) bool {
	severityOrder := map[models.AlertSeverity]int{
		models.SeverityInfo:     1,
		models.SeverityWarning:  2,
		models.SeverityCritical: 3,
	}
	return severityOrder[sourceSeverity] > severityOrder[targetSeverity]
}

func (s *AlertSuppressionService) matchesTargetSeverity(eventSeverity, targetSeverity models.AlertSeverity) bool {
	severityOrder := map[models.AlertSeverity]int{
		models.SeverityInfo:     1,
		models.SeverityWarning:  2,
		models.SeverityCritical: 3,
	}
	return severityOrder[eventSeverity] <= severityOrder[targetSeverity]
}

func (s *AlertSuppressionService) dimensionsMatch(source, target *models.AlertEvent, matchFields string) bool {
	if matchFields == "" {
		return true
	}

	fields := strings.Split(matchFields, ",")
	for _, field := range fields {
		field = strings.TrimSpace(field)
		switch field {
		case "dimension_type":
			if source.DimensionType != target.DimensionType {
				return false
			}
		case "dimension_value":
			if source.DimensionValue != target.DimensionValue {
				return false
			}
		case "alert_rule_id":
			if source.AlertRuleID != target.AlertRuleID {
				return false
			}
		}
	}
	return true
}

type AuditService struct {
	auditRepo          *repository.AuditRepo
	ruleSvc            *RuleService
	quotaSvc           *QuotaService
	alertRuleSvc       *AlertRuleService
	aggregationRuleSvc *AlertAggregationRuleService
	suppressionRuleSvc *AlertSuppressionRuleService
}

func NewAuditService(auditRepo *repository.AuditRepo) *AuditService {
	return &AuditService{auditRepo: auditRepo}
}

func (s *AuditService) SetRuleService(svc *RuleService) {
	s.ruleSvc = svc
}

func (s *AuditService) SetQuotaService(svc *QuotaService) {
	s.quotaSvc = svc
}

func (s *AuditService) SetAlertRuleService(svc *AlertRuleService) {
	s.alertRuleSvc = svc
}

func (s *AuditService) SetAggregationRuleService(svc *AlertAggregationRuleService) {
	s.aggregationRuleSvc = svc
}

func (s *AuditService) SetSuppressionRuleService(svc *AlertSuppressionRuleService) {
	s.suppressionRuleSvc = svc
}

func (s *AuditService) RecordLog(
	operator string,
	opType models.AuditOperationType,
	resType models.AuditResourceType,
	resID string,
	beforeSnapshot, afterSnapshot interface{},
	requestIP string,
) error {
	var beforeJSON, afterJSON, diffJSON json.RawMessage

	if beforeSnapshot != nil {
		data, err := json.Marshal(beforeSnapshot)
		if err == nil {
			beforeJSON = data
		}
	}

	if afterSnapshot != nil {
		data, err := json.Marshal(afterSnapshot)
		if err == nil {
			afterJSON = data
		}
	}

	diff := computeDiff(beforeSnapshot, afterSnapshot)
	if diff != nil {
		data, err := json.Marshal(diff)
		if err == nil {
			diffJSON = data
		}
	}

	log := &models.AuditLog{
		Operator:       operator,
		OperationType:  opType,
		ResourceType:   resType,
		ResourceID:     resID,
		BeforeSnapshot: beforeJSON,
		AfterSnapshot:  afterJSON,
		DiffSummary:    diffJSON,
		RequestIP:      requestIP,
	}

	return s.auditRepo.Create(log)
}

func computeDiff(before, after interface{}) map[string]models.DiffField {
	if before == nil || after == nil {
		return nil
	}

	beforeMap := make(map[string]interface{})
	afterMap := make(map[string]interface{})

	beforeBytes, _ := json.Marshal(before)
	afterBytes, _ := json.Marshal(after)
	json.Unmarshal(beforeBytes, &beforeMap)
	json.Unmarshal(afterBytes, &afterMap)

	diff := make(map[string]models.DiffField)
	allKeys := make(map[string]bool)

	for k := range beforeMap {
		allKeys[k] = true
	}
	for k := range afterMap {
		allKeys[k] = true
	}

	skipFields := map[string]bool{
		"createdAt": true,
		"updatedAt": true,
		"version":   true,
		"created_at": true,
		"updated_at": true,
	}

	for k := range allKeys {
		if skipFields[k] {
			continue
		}
		bVal, bOk := beforeMap[k]
		aVal, aOk := afterMap[k]

		if !bOk {
			diff[k] = models.DiffField{OldValue: nil, NewValue: aVal}
		} else if !aOk {
			diff[k] = models.DiffField{OldValue: bVal, NewValue: nil}
		} else {
			bStr, _ := json.Marshal(bVal)
			aStr, _ := json.Marshal(aVal)
			if string(bStr) != string(aStr) {
				diff[k] = models.DiffField{OldValue: bVal, NewValue: aVal}
			}
		}
	}

	if len(diff) == 0 {
		return nil
	}
	return diff
}

func (s *AuditService) List(query models.AuditLogQuery) (*models.PaginatedResult, error) {
	return s.auditRepo.List(query)
}

func (s *AuditService) ExportCSV(query models.AuditLogQuery) ([]byte, error) {
	logs, err := s.auditRepo.ListAll(query)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	buf.WriteString("时间,操作人,操作类型,资源类型,资源ID,变更字段列表\n")

	for _, log := range logs {
		var diffFields []string
		if len(log.DiffSummary) > 0 {
			var diffMap map[string]interface{}
			if err := json.Unmarshal(log.DiffSummary, &diffMap); err == nil {
				for k := range diffMap {
					diffFields = append(diffFields, k)
				}
			}
		}

		createdAt := log.CreatedAt.Format("2006-01-02 15:04:05")
		opType := string(log.OperationType)
		resType := string(log.ResourceType)
		fields := strings.Join(diffFields, ",")

		row := fmt.Sprintf("%s,%s,%s,%s,%s,%s\n",
			csvEscape(createdAt),
			csvEscape(log.Operator),
			csvEscape(opType),
			csvEscape(resType),
			csvEscape(log.ResourceID),
			csvEscape(fields),
		)
		buf.WriteString(row)
	}

	return buf.Bytes(), nil
}

func csvEscape(s string) string {
	if strings.Contains(s, ",") || strings.Contains(s, "\"") || strings.Contains(s, "\n") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}

func (s *AuditService) Get(id int64) (*models.AuditLog, error) {
	return s.auditRepo.Get(id)
}

func (s *AuditService) GetTimeline(resType models.AuditResourceType, resID string) ([]models.TimelineNode, error) {
	return s.auditRepo.GetTimeline(resType, resID)
}

func (s *AuditService) GetStats() (*models.AuditStats, error) {
	return s.auditRepo.GetStats()
}

func (s *AuditService) ListOperators() ([]string, error) {
	return s.auditRepo.ListOperators()
}

func (s *AuditService) Rollback(auditLogID int64, operator string, requestIP string) error {
	log, err := s.auditRepo.Get(auditLogID)
	if err != nil {
		return fmt.Errorf("audit log not found: %w", err)
	}

	if log.OperationType == models.AuditOpCreate || log.OperationType == models.AuditOpRollback {
		return fmt.Errorf("operation type %s cannot be rolled back", log.OperationType)
	}

	var beforeSnapshot map[string]interface{}
	if len(log.BeforeSnapshot) > 0 {
		if err := json.Unmarshal(log.BeforeSnapshot, &beforeSnapshot); err != nil {
			return fmt.Errorf("failed to parse before snapshot: %w", err)
		}
	}

	switch log.ResourceType {
	case models.AuditResRule:
		return s.rollbackRule(log, beforeSnapshot, operator, requestIP)
	case models.AuditResAlertRule:
		return s.rollbackAlertRule(log, beforeSnapshot, operator, requestIP)
	case models.AuditResAggregationRule:
		return s.rollbackAggregationRule(log, beforeSnapshot, operator, requestIP)
	case models.AuditResSuppressionRule:
		return s.rollbackSuppressionRule(log, beforeSnapshot, operator, requestIP)
	case models.AuditResQuota:
		return s.rollbackQuota(log, beforeSnapshot, operator, requestIP)
	default:
		return fmt.Errorf("unsupported resource type: %s", log.ResourceType)
	}
}

func (s *AuditService) rollbackRule(log *models.AuditLog, before map[string]interface{}, operator, requestIP string) error {
	if s.ruleSvc == nil {
		return fmt.Errorf("rule service not available")
	}

	switch log.OperationType {
	case models.AuditOpDelete:
		var rule models.RateLimitRule
		beforeBytes, _ := json.Marshal(before)
		if err := json.Unmarshal(beforeBytes, &rule); err != nil {
			return err
		}
		if err := s.ruleSvc.Create(&rule); err != nil {
			return err
		}
		return s.RecordLog(operator, models.AuditOpRollback, models.AuditResRule, log.ResourceID, nil, rule, requestIP)

	case models.AuditOpUpdate, models.AuditOpToggle:
		var rule models.RateLimitRule
		beforeBytes, _ := json.Marshal(before)
		if err := json.Unmarshal(beforeBytes, &rule); err != nil {
			return err
		}
		rule.ID = log.ResourceID
		afterRule, _ := s.ruleSvc.Get(log.ResourceID)
		if err := s.ruleSvc.Update(&rule); err != nil {
			return err
		}
		return s.RecordLog(operator, models.AuditOpRollback, models.AuditResRule, log.ResourceID, afterRule, rule, requestIP)
	}

	return fmt.Errorf("unsupported operation type: %s", log.OperationType)
}

func (s *AuditService) rollbackAlertRule(log *models.AuditLog, before map[string]interface{}, operator, requestIP string) error {
	if s.alertRuleSvc == nil {
		return fmt.Errorf("alert rule service not available")
	}

	switch log.OperationType {
	case models.AuditOpDelete:
		var rule models.AlertRule
		beforeBytes, _ := json.Marshal(before)
		if err := json.Unmarshal(beforeBytes, &rule); err != nil {
			return err
		}
		if err := s.alertRuleSvc.Create(&rule); err != nil {
			return err
		}
		return s.RecordLog(operator, models.AuditOpRollback, models.AuditResAlertRule, log.ResourceID, nil, rule, requestIP)

	case models.AuditOpUpdate, models.AuditOpToggle:
		var rule models.AlertRule
		beforeBytes, _ := json.Marshal(before)
		if err := json.Unmarshal(beforeBytes, &rule); err != nil {
			return err
		}
		rule.ID = log.ResourceID
		afterRule, _ := s.alertRuleSvc.Get(log.ResourceID)
		if err := s.alertRuleSvc.Update(&rule); err != nil {
			return err
		}
		return s.RecordLog(operator, models.AuditOpRollback, models.AuditResAlertRule, log.ResourceID, afterRule, rule, requestIP)
	}

	return fmt.Errorf("unsupported operation type: %s", log.OperationType)
}

func (s *AuditService) rollbackAggregationRule(log *models.AuditLog, before map[string]interface{}, operator, requestIP string) error {
	if s.aggregationRuleSvc == nil {
		return fmt.Errorf("aggregation rule service not available")
	}

	switch log.OperationType {
	case models.AuditOpDelete:
		var rule models.AlertAggregationRule
		beforeBytes, _ := json.Marshal(before)
		if err := json.Unmarshal(beforeBytes, &rule); err != nil {
			return err
		}
		if err := s.aggregationRuleSvc.Create(&rule); err != nil {
			return err
		}
		return s.RecordLog(operator, models.AuditOpRollback, models.AuditResAggregationRule, log.ResourceID, nil, rule, requestIP)

	case models.AuditOpUpdate, models.AuditOpToggle:
		var rule models.AlertAggregationRule
		beforeBytes, _ := json.Marshal(before)
		if err := json.Unmarshal(beforeBytes, &rule); err != nil {
			return err
		}
		rule.ID = log.ResourceID
		afterRule, _ := s.aggregationRuleSvc.Get(log.ResourceID)
		if err := s.aggregationRuleSvc.Update(&rule); err != nil {
			return err
		}
		return s.RecordLog(operator, models.AuditOpRollback, models.AuditResAggregationRule, log.ResourceID, afterRule, rule, requestIP)
	}

	return fmt.Errorf("unsupported operation type: %s", log.OperationType)
}

func (s *AuditService) rollbackSuppressionRule(log *models.AuditLog, before map[string]interface{}, operator, requestIP string) error {
	if s.suppressionRuleSvc == nil {
		return fmt.Errorf("suppression rule service not available")
	}

	switch log.OperationType {
	case models.AuditOpDelete:
		var rule models.AlertSuppressionRule
		beforeBytes, _ := json.Marshal(before)
		if err := json.Unmarshal(beforeBytes, &rule); err != nil {
			return err
		}
		if err := s.suppressionRuleSvc.Create(&rule); err != nil {
			return err
		}
		return s.RecordLog(operator, models.AuditOpRollback, models.AuditResSuppressionRule, log.ResourceID, nil, rule, requestIP)

	case models.AuditOpUpdate, models.AuditOpToggle:
		var rule models.AlertSuppressionRule
		beforeBytes, _ := json.Marshal(before)
		if err := json.Unmarshal(beforeBytes, &rule); err != nil {
			return err
		}
		rule.ID = log.ResourceID
		afterRule, _ := s.suppressionRuleSvc.Get(log.ResourceID)
		if err := s.suppressionRuleSvc.Update(&rule); err != nil {
			return err
		}
		return s.RecordLog(operator, models.AuditOpRollback, models.AuditResSuppressionRule, log.ResourceID, afterRule, rule, requestIP)
	}

	return fmt.Errorf("unsupported operation type: %s", log.OperationType)
}

func (s *AuditService) rollbackQuota(log *models.AuditLog, before map[string]interface{}, operator, requestIP string) error {
	if s.quotaSvc == nil {
		return fmt.Errorf("quota service not available")
	}

	switch log.OperationType {
	case models.AuditOpDelete:
		var quota models.QuotaConfig
		beforeBytes, _ := json.Marshal(before)
		if err := json.Unmarshal(beforeBytes, &quota); err != nil {
			return err
		}
		if err := s.quotaSvc.Upsert(&quota); err != nil {
			return err
		}
		return s.RecordLog(operator, models.AuditOpRollback, models.AuditResQuota, log.ResourceID, nil, quota, requestIP)

	case models.AuditOpUpdate, models.AuditOpToggle:
		var quota models.QuotaConfig
		beforeBytes, _ := json.Marshal(before)
		if err := json.Unmarshal(beforeBytes, &quota); err != nil {
			return err
		}
		afterQuota, _ := s.quotaSvc.List()
		_ = afterQuota
		id, err := strconv.ParseInt(log.ResourceID, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid quota id: %w", err)
		}
		quota.ID = id
		if err := s.quotaSvc.Upsert(&quota); err != nil {
			return err
		}
		return s.RecordLog(operator, models.AuditOpRollback, models.AuditResQuota, log.ResourceID, nil, quota, requestIP)
	}

	return fmt.Errorf("unsupported operation type: %s", log.OperationType)
}
