package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/ratelimiter/admin-api/internal/handlers"
	"github.com/ratelimiter/admin-api/internal/models"
	"github.com/ratelimiter/admin-api/internal/repository"
	"github.com/ratelimiter/admin-api/internal/services"
)

func main() {
	viper.SetConfigName("admin-api")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/ratelimiter")
	viper.AddConfigPath(".")
	viper.SetDefault("http_addr", ":8081")
	viper.SetDefault("redis_addr", "localhost:6379")
	viper.SetDefault("redis_db", 0)
	viper.SetDefault("postgres_dsn", "host=localhost port=5432 user=ratelimiter password=ratelimiter dbname=ratelimiter sslmode=disable")
	viper.AutomaticEnv()

	dsn := viper.GetString("postgres_dsn")
	var db *gorm.DB
	var err error
	for retries := 0; retries < 10; retries++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			break
		}
		log.Printf("Waiting for PostgreSQL (attempt %d): %v", retries+1, err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	if err := autoMigrateDB(db); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	log.Println("Database migration completed")

	rdb := redis.NewClient(&redis.Options{
		Addr:         viper.GetString("redis_addr"),
		Password:     viper.GetString("redis_password"),
		DB:           viper.GetInt("redis_db"),
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Redis not available: %v", err)
	} else {
		log.Println("Connected to Redis")
	}

	ruleRepo := repository.NewRuleRepo(db)
	eventRepo := repository.NewEventRepo(db)
	quotaRepo := repository.NewQuotaRepo(db)
	tenantRepo := repository.NewTenantRepo(db)
	adaptiveRepo := repository.NewAdaptiveRepo(db)
	templateRepo := repository.NewTemplateRepo(db)
	alertRuleRepo := repository.NewAlertRuleRepo(db)
	alertEventRepo := repository.NewAlertEventRepo(db)
	aggregationRuleRepo := repository.NewAlertAggregationRuleRepo(db)
	suppressionRuleRepo := repository.NewAlertSuppressionRuleRepo(db)
	aggregationGroupRepo := repository.NewAlertAggregationGroupRepo(db)
	aggregationEventRepo := repository.NewAlertAggregationEventRepo(db)
	auditRepo := repository.NewAuditRepo(db)

	wsHub := services.NewWebSocketHub()
	go wsHub.Run()

	ruleSvc := services.NewRuleService(ruleRepo, rdb)
	eventSvc := services.NewEventService(eventRepo)
	quotaSvc := services.NewQuotaService(quotaRepo, tenantRepo, eventRepo, rdb)
	adaptiveSvc := services.NewAdaptiveService(adaptiveRepo, rdb)
	templateSvc := services.NewTemplateService(templateRepo)
	alertRuleSvc := services.NewAlertRuleService(alertRuleRepo)
	alertEventSvc := services.NewAlertEventService(alertEventRepo, wsHub)
	aggregationRuleSvc := services.NewAlertAggregationRuleService(aggregationRuleRepo)
	suppressionRuleSvc := services.NewAlertSuppressionRuleService(suppressionRuleRepo)
	auditSvc := services.NewAuditService(auditRepo)

	auditSvc.SetRuleService(ruleSvc)
	auditSvc.SetQuotaService(quotaSvc)
	auditSvc.SetAlertRuleService(alertRuleSvc)
	auditSvc.SetAggregationRuleService(aggregationRuleSvc)
	auditSvc.SetSuppressionRuleService(suppressionRuleSvc)

	alertSuppressionSvc := services.NewAlertSuppressionService(suppressionRuleSvc, alertEventRepo)
	alertAggregationSvc := services.NewAlertAggregationService(
		aggregationRuleSvc,
		aggregationGroupRepo,
		aggregationEventRepo,
		alertEventRepo,
		wsHub,
	)

	alertEventSvc.SetSuppressionService(alertSuppressionSvc)
	alertEventSvc.SetAggregationService(alertAggregationSvc)

	alertEngine := services.NewAlertEngineService(alertRuleSvc, alertEventSvc, alertEventRepo, db)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := alertEngine.Evaluate(); err != nil {
				log.Printf("Alert engine evaluation error: %v", err)
			}
		}
	}()

	h := handlers.NewHandler(ruleSvc, eventSvc, quotaSvc, adaptiveSvc, templateSvc, alertRuleSvc, alertEventSvc,
		aggregationRuleSvc, suppressionRuleSvc, alertAggregationSvc, alertSuppressionSvc, auditSvc, wsHub)

	auditMiddleware := handlers.NewAuditMiddleware(ruleSvc, quotaSvc, alertRuleSvc,
		aggregationRuleSvc, suppressionRuleSvc, auditSvc)

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery(), gin.Logger(), handlers.CORSMiddleware())

	api := engine.Group("/api/v1")
	api.Use(auditMiddleware.Middleware())
	{
		api.GET("/health", h.Health)

		rules := api.Group("/rules")
		{
			rules.GET("", h.ListRules)
			rules.POST("", h.CreateRule)
			rules.POST("/bulk-toggle", h.BulkToggleRules)
			rules.GET("/:id", h.GetRule)
			rules.PUT("/:id", h.UpdateRule)
			rules.DELETE("/:id", h.DeleteRule)
			rules.PATCH("/:id/toggle", h.ToggleRule)
			rules.GET("/:id/versions", h.GetRuleVersions)
			rules.POST("/:id/rollback", h.RollbackRule)
		}

		events := api.Group("/events")
		{
			events.GET("", h.ListEvents)
		}

		dashboard := api.Group("/dashboard")
		{
			dashboard.GET("/traffic", h.GetTrafficSeries)
			dashboard.GET("/tenant-share", h.GetTenantShare)
			dashboard.GET("/heatmap", h.GetHeatmap)
		}

		quotas := api.Group("/quotas")
		{
			quotas.GET("", h.ListQuotas)
			quotas.GET("/tree", h.GetQuotaTree)
			quotas.POST("", h.UpsertQuota)
			quotas.DELETE("/:id", h.DeleteQuota)
		}

		adaptive := api.Group("/adaptive")
		{
			adaptive.GET("/status", h.GetAdaptiveStatus)
			adaptive.PUT("/config", h.UpdateAdaptiveConfig)
			adaptive.POST("/override", h.OverrideAdaptiveCoeff)
			adaptive.DELETE("/override", h.ClearAdaptiveOverride)
		}

		templates := api.Group("/templates")
		{
			templates.GET("", h.ListTemplates)
			templates.GET("/all", h.ListAllTemplates)
			templates.POST("", h.CreateTemplate)
			templates.GET("/:id", h.GetTemplate)
			templates.PUT("/:id", h.UpdateTemplate)
			templates.DELETE("/:id", h.DeleteTemplate)
		}

		alertRules := api.Group("/alert-rules")
		{
			alertRules.GET("", h.ListAlertRules)
			alertRules.POST("", h.CreateAlertRule)
			alertRules.GET("/:id", h.GetAlertRule)
			alertRules.PUT("/:id", h.UpdateAlertRule)
			alertRules.DELETE("/:id", h.DeleteAlertRule)
			alertRules.PATCH("/:id/toggle", h.ToggleAlertRule)
		}

		alertEvents := api.Group("/alert-events")
		{
			alertEvents.GET("", h.ListAlertEvents)
			alertEvents.GET("/stats", h.GetAlertStats)
			alertEvents.GET("/:id", h.GetAlertEvent)
			alertEvents.POST("/:id/acknowledge", h.AcknowledgeAlert)
		}

		aggregationRules := api.Group("/alert-aggregation-rules")
		{
			aggregationRules.GET("", h.ListAggregationRules)
			aggregationRules.POST("", h.CreateAggregationRule)
			aggregationRules.GET("/:id", h.GetAggregationRule)
			aggregationRules.PUT("/:id", h.UpdateAggregationRule)
			aggregationRules.DELETE("/:id", h.DeleteAggregationRule)
			aggregationRules.PATCH("/:id/toggle", h.ToggleAggregationRule)
		}

		suppressionRules := api.Group("/alert-suppression-rules")
		{
			suppressionRules.GET("", h.ListSuppressionRules)
			suppressionRules.POST("", h.CreateSuppressionRule)
			suppressionRules.GET("/:id", h.GetSuppressionRule)
			suppressionRules.PUT("/:id", h.UpdateSuppressionRule)
			suppressionRules.DELETE("/:id", h.DeleteSuppressionRule)
			suppressionRules.PATCH("/:id/toggle", h.ToggleSuppressionRule)
		}

		aggregationGroups := api.Group("/alert-aggregation-groups")
		{
			aggregationGroups.GET("", h.ListAggregationGroups)
			aggregationGroups.GET("/:id/events", h.GetAggregationGroupEvents)
		}

		auditLogs := api.Group("/audit-logs")
		{
			auditLogs.GET("", h.ListAuditLogs)
			auditLogs.GET("/stats", h.GetAuditStats)
			auditLogs.GET("/operators", h.ListAuditOperators)
			auditLogs.GET("/timeline", h.GetAuditTimeline)
			auditLogs.GET("/:id", h.GetAuditLog)
			auditLogs.POST("/:id/rollback", h.RollbackAuditOperation)
		}
	}

	engine.GET("/ws/alerts", h.WebSocketEndpoint)

	engine.GET("/metrics", gin.WrapH(promhttp.Handler()))

	srv := &http.Server{
		Addr:         viper.GetString("http_addr"),
		Handler:      engine,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("Admin API listening on %s", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		log.Printf("Server error: %v", err)
	case sig := <-sigCh:
		log.Printf("Received signal: %v", sig)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
}

func autoMigrateDB(db *gorm.DB) error {
	err := db.AutoMigrate(
		&models.AlertRule{},
		&models.AlertEvent{},
		&models.AlertAggregationRule{},
		&models.AlertSuppressionRule{},
		&models.AlertAggregationGroup{},
		&models.AlertAggregationEvent{},
		&models.AuditLog{},
	)
	if err != nil {
		return err
	}

	indexSQL := []string{
		`CREATE INDEX IF NOT EXISTS idx_alert_rules_enabled ON alert_rules(enabled)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_rules_trigger_type ON alert_rules(trigger_type)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_rules_scope ON alert_rules(scope_type, scope_value)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_events_status ON alert_events(status)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_events_severity ON alert_events(severity)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_events_rule ON alert_events(alert_rule_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_events_dimension ON alert_events(dimension_type, dimension_value, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_events_created ON alert_events(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_events_status_created ON alert_events(status, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_events_suppressed ON alert_events(suppressed)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_aggregation_rules_enabled ON alert_aggregation_rules(enabled)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_suppression_rules_enabled ON alert_suppression_rules(enabled)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_aggregation_groups_rule_dim ON alert_aggregation_groups(aggregation_rule_id, dimension_type, dimension_value, status)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_aggregation_groups_window_ends ON alert_aggregation_groups(window_ends_at)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_aggregation_events_group ON alert_aggregation_events(aggregation_group_id)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_aggregation_events_event ON alert_aggregation_events(alert_event_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_operator ON audit_logs(operator, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_type ON audit_logs(resource_type, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_id ON audit_logs(resource_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_operation_type ON audit_logs(operation_type, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_composite ON audit_logs(operator, resource_type, resource_id, created_at DESC)`,
	}

	for _, sql := range indexSQL {
		if err := db.Exec(sql).Error; err != nil {
			log.Printf("Warning: failed to create index: %v", err)
		}
	}

	initAlertRules := []models.AlertRule{
		{
			ID:                   "rule-api-high-reject",
			Name:                 "API高拒绝率告警",
			Description:          "当某个API的拒绝次数在60秒内超过100次时触发告警",
			Severity:             models.SeverityCritical,
			Enabled:              true,
			TriggerType:          models.TriggerTypeThreshold,
			TriggerConfigJSON:    []byte(`{"windowSeconds": 60, "threshold": 100, "metric": "reject_count"}`),
			ScopeType:            models.ScopeGlobal,
			NotificationJSON:     []byte(`[]`),
			SilentPeriodSeconds:  300,
			RetentionHours:       168,
			CreatedAt:            time.Now(),
			UpdatedAt:            time.Now(),
		},
		{
			ID:                   "rule-tenant-reject-rate",
			Name:                 "租户拒绝率告警",
			Description:          "当某个租户的拒绝率在5分钟内超过20%时触发告警",
			Severity:             models.SeverityWarning,
			Enabled:              true,
			TriggerType:          models.TriggerTypeRate,
			TriggerConfigJSON:    []byte(`{"windowSeconds": 300, "thresholdPercent": 20, "metric": "reject_rate"}`),
			ScopeType:            models.ScopeGlobal,
			NotificationJSON:     []byte(`[]`),
			SilentPeriodSeconds:  600,
			RetentionHours:       168,
			CreatedAt:            time.Now(),
			UpdatedAt:            time.Now(),
		},
	}

	for _, rule := range initAlertRules {
		var count int64
		db.Model(&models.AlertRule{}).Where("id = ?", rule.ID).Count(&count)
		if count == 0 {
			if err := db.Create(&rule).Error; err != nil {
				log.Printf("Warning: failed to create alert rule %s: %v", rule.ID, err)
			}
		}
	}

	initAggregationRules := []models.AlertAggregationRule{
		{
			ID:            "agg-api-path-5min",
			Name:          "按API路径聚合(5分钟)",
			DimensionType: models.AggregateByAPI,
			WindowSeconds: 300,
			Enabled:       true,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:            "agg-tenant-5min",
			Name:          "按租户聚合(5分钟)",
			DimensionType: models.AggregateByTenant,
			WindowSeconds: 300,
			Enabled:       true,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	}

	for _, rule := range initAggregationRules {
		var count int64
		db.Model(&models.AlertAggregationRule{}).Where("id = ?", rule.ID).Count(&count)
		if count == 0 {
			if err := db.Create(&rule).Error; err != nil {
				log.Printf("Warning: failed to create aggregation rule %s: %v", rule.ID, err)
			}
		}
	}

	initSuppressionRules := []models.AlertSuppressionRule{
		{
			ID:                   "suppress-critical-suppresses-warning-info",
			Name:                 "Critical告警抑制Warning和Info",
			SourceSeverity:       models.SeverityCritical,
			SourceStatus:         models.StatusFiring,
			SourceRuleID:         "",
			TargetSeverity:       models.SeverityInfo,
			TargetDimensionType:  "",
			MatchDimensionFields: "dimension_value",
			Enabled:              true,
			CreatedAt:            time.Now(),
			UpdatedAt:            time.Now(),
		},
	}

	for _, rule := range initSuppressionRules {
		var count int64
		db.Model(&models.AlertSuppressionRule{}).Where("id = ?", rule.ID).Count(&count)
		if count == 0 {
			if err := db.Create(&rule).Error; err != nil {
				log.Printf("Warning: failed to create suppression rule %s: %v", rule.ID, err)
			}
		}
	}

	return nil
}
