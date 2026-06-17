package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ratelimiter/admin-api/internal/models"
	"github.com/ratelimiter/admin-api/internal/services"
)

type Handler struct {
	rules      *services.RuleService
	events     *services.EventService
	quotas     *services.QuotaService
	adaptive   *services.AdaptiveService
	templates  *services.TemplateService
	alertRules *services.AlertRuleService
	alertEvents *services.AlertEventService
	wsHub      *services.WebSocketHub
}

func NewHandler(
	rules *services.RuleService,
	events *services.EventService,
	quotas *services.QuotaService,
	adaptive *services.AdaptiveService,
	templates *services.TemplateService,
	alertRules *services.AlertRuleService,
	alertEvents *services.AlertEventService,
	wsHub *services.WebSocketHub,
) *Handler {
	return &Handler{
		rules:      rules,
		events:     events,
		quotas:     quotas,
		adaptive:   adaptive,
		templates:  templates,
		alertRules: alertRules,
		alertEvents: alertEvents,
		wsHub:      wsHub,
	}
}

func (h *Handler) ListRules(c *gin.Context) {
	var page models.Pagination
	if err := c.ShouldBindQuery(&page); err != nil {
		page.Page = 1
		page.PageSize = 20
	}
	if page.Page <= 0 {
		page.Page = 1
	}
	if page.PageSize <= 0 {
		page.PageSize = 20
	}
	search := c.Query("search")
	var enabled *bool
	if e := c.Query("enabled"); e != "" {
		v := e == "true" || e == "1"
		enabled = &v
	}
	result, err := h.rules.List(page, search, enabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list rules",
			"details": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.rules.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *Handler) CreateRule(c *gin.Context) {
	var rule models.RateLimitRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.rules.Create(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

func (h *Handler) UpdateRule(c *gin.Context) {
	id := c.Param("id")
	var rule models.RateLimitRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule.ID = id
	if err := h.rules.Update(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *Handler) DeleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.rules.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ToggleRule(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.rules.Toggle(id, body.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "enabled": body.Enabled})
}

func (h *Handler) GetRuleVersions(c *gin.Context) {
	id := c.Param("id")
	versions, err := h.rules.GetVersions(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, versions)
}

func (h *Handler) RollbackRule(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		Version int64 `json:"version"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.rules.Rollback(id, body.Version); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "rolled_back", "to_version": body.Version})
}

func (h *Handler) BulkToggleRules(c *gin.Context) {
	var body struct {
		IDs     []string `json:"ids"`
		Enabled bool     `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.rules.BulkToggle(body.IDs, body.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"updated": len(body.IDs), "enabled": body.Enabled})
}

func (h *Handler) ListEvents(c *gin.Context) {
	var page models.Pagination
	if err := c.ShouldBindQuery(&page); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var startTime, endTime *time.Time
	if s := c.Query("start_time"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			startTime = &t
		}
	}
	if e := c.Query("end_time"); e != "" {
		if t, err := time.Parse(time.RFC3339, e); err == nil {
			endTime = &t
		}
	}

	ruleID := c.Query("rule_id")
	tenantID := c.Query("tenant_id")
	userID := c.Query("user_id")
	apiPath := c.Query("api_path")
	var allowed *bool
	if a := c.Query("allowed"); a != "" {
		v := a == "true" || a == "1"
		allowed = &v
	}

	result, err := h.events.List(page, startTime, endTime, ruleID, tenantID, userID, apiPath, allowed)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetTrafficSeries(c *gin.Context) {
	lastHours := 1
	if v := c.Query("hours"); v != "" {
		if h, err := strconv.Atoi(v); err == nil && h > 0 {
			lastHours = h
		}
	}
	intervalSec := 60
	if v := c.Query("interval"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			intervalSec = i
		}
	}
	apiPath := c.Query("api_path")
	tenantID := c.Query("tenant_id")

	points, err := h.events.TrafficSeries(lastHours, intervalSec, apiPath, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get traffic series",
			"details": err.Error(),
		})
		return
	}
	if points == nil {
		points = []models.TrafficPoint{}
	}
	c.JSON(http.StatusOK, points)
}

func (h *Handler) GetTenantShare(c *gin.Context) {
	lastHours := 24
	if v := c.Query("hours"); v != "" {
		if h, err := strconv.Atoi(v); err == nil && h > 0 {
			lastHours = h
		}
	}
	shares, err := h.events.TenantTrafficShare(lastHours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, shares)
}

func (h *Handler) GetHeatmap(c *gin.Context) {
	lastDays := 7
	if v := c.Query("days"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			lastDays = d
		}
	}
	points, err := h.events.Heatmap(lastDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, points)
}

func (h *Handler) ListQuotas(c *gin.Context) {
	quotas, err := h.quotas.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, quotas)
}

func (h *Handler) UpsertQuota(c *gin.Context) {
	var quota models.QuotaConfig
	if err := c.ShouldBindJSON(&quota); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.quotas.Upsert(&quota); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, quota)
}

func (h *Handler) DeleteQuota(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.quotas.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) GetQuotaTree(c *gin.Context) {
	tree, err := h.quotas.GetTree()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tree)
}

func (h *Handler) GetAdaptiveStatus(c *gin.Context) {
	component := c.DefaultQuery("component", "global")
	status, err := h.adaptive.GetStatus(component)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *Handler) UpdateAdaptiveConfig(c *gin.Context) {
	component := c.DefaultQuery("component", "global")
	var cfg models.AdaptiveConfigDB
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.adaptive.UpdateConfig(component, &cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

func (h *Handler) OverrideAdaptiveCoeff(c *gin.Context) {
	component := c.DefaultQuery("component", "global")
	var body struct {
		Coefficient float64 `json:"coefficient"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.adaptive.OverrideCoefficient(component, body.Coefficient); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"component": component, "coefficient": body.Coefficient})
}

func (h *Handler) ClearAdaptiveOverride(c *gin.Context) {
	component := c.DefaultQuery("component", "global")
	if err := h.adaptive.ClearOverride(component); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"component": component, "status": "cleared"})
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"service":   "admin-api",
		"timestamp": time.Now().UTC(),
	})
}

func (h *Handler) ListTemplates(c *gin.Context) {
	var page models.Pagination
	if err := c.ShouldBindQuery(&page); err != nil {
		page.Page = 1
		page.PageSize = 20
	}
	if page.Page <= 0 {
		page.Page = 1
	}
	if page.PageSize <= 0 {
		page.PageSize = 20
	}
	search := c.Query("search")
	result, err := h.templates.List(page, search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list templates",
			"details": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) ListAllTemplates(c *gin.Context) {
	templates, err := h.templates.ListAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list templates",
			"details": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, templates)
}

func (h *Handler) GetTemplate(c *gin.Context) {
	id := c.Param("id")
	template, err := h.templates.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}
	c.JSON(http.StatusOK, template)
}

func (h *Handler) CreateTemplate(c *gin.Context) {
	var template models.RuleTemplate
	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if template.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "template name is required"})
		return
	}
	if err := h.templates.Create(&template); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, template)
}

func (h *Handler) UpdateTemplate(c *gin.Context) {
	id := c.Param("id")
	var template models.RuleTemplate
	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	template.ID = id
	if err := h.templates.Update(&template); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, template)
}

func (h *Handler) DeleteTemplate(c *gin.Context) {
	id := c.Param("id")
	if err := h.templates.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListAlertRules(c *gin.Context) {
	var page models.Pagination
	if err := c.ShouldBindQuery(&page); err != nil {
		page.Page = 1
		page.PageSize = 20
	}
	if page.Page <= 0 {
		page.Page = 1
	}
	if page.PageSize <= 0 {
		page.PageSize = 20
	}
	search := c.Query("search")
	var enabled *bool
	if e := c.Query("enabled"); e != "" {
		v := e == "true" || e == "1"
		enabled = &v
	}
	result, err := h.alertRules.List(page, search, enabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list alert rules",
			"details": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetAlertRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.alertRules.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "alert rule not found"})
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *Handler) CreateAlertRule(c *gin.Context) {
	var rule models.AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.alertRules.Create(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

func (h *Handler) UpdateAlertRule(c *gin.Context) {
	id := c.Param("id")
	var rule models.AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule.ID = id
	if err := h.alertRules.Update(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *Handler) DeleteAlertRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.alertRules.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ToggleAlertRule(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.alertRules.Toggle(id, body.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "enabled": body.Enabled})
}

func (h *Handler) ListAlertEvents(c *gin.Context) {
	var page models.Pagination
	if err := c.ShouldBindQuery(&page); err != nil {
		page.Page = 1
		page.PageSize = 20
	}

	var status *models.AlertStatus
	if s := c.Query("status"); s != "" {
		sv := models.AlertStatus(s)
		status = &sv
	}
	var severity *models.AlertSeverity
	if s := c.Query("severity"); s != "" {
		sv := models.AlertSeverity(s)
		severity = &sv
	}
	ruleID := c.Query("rule_id")
	dimensionType := c.Query("dimension_type")
	dimensionValue := c.Query("dimension_value")

	result, err := h.alertEvents.List(page, status, severity, ruleID, dimensionType, dimensionValue)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list alert events",
			"details": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetAlertEvent(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	event, err := h.alertEvents.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "alert event not found"})
		return
	}
	c.JSON(http.StatusOK, event)
}

func (h *Handler) AcknowledgeAlert(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body struct {
		AcknowledgedBy string `json:"acknowledgedBy"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		body.AcknowledgedBy = "system"
	}
	if err := h.alertEvents.Acknowledge(id, body.AcknowledgedBy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "acknowledged"})
}

func (h *Handler) GetAlertStats(c *gin.Context) {
	stats, err := h.alertEvents.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *Handler) WebSocketEndpoint(c *gin.Context) {
	h.wsHub.HandleWebSocket(c.Writer, c.Request)
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,PATCH,OPTIONS")
		c.Header("Access-Control-Allow-Headers", strings.Join([]string{
			"Origin", "Content-Type", "Accept", "Authorization",
			"X-Requested-With", "X-Tenant-ID", "X-User-ID",
		}, ","))

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
