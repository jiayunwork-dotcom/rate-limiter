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

	ruleSvc := services.NewRuleService(ruleRepo, rdb)
	eventSvc := services.NewEventService(eventRepo)
	quotaSvc := services.NewQuotaService(quotaRepo, tenantRepo, eventRepo, rdb)
	adaptiveSvc := services.NewAdaptiveService(adaptiveRepo, rdb)

	h := handlers.NewHandler(ruleSvc, eventSvc, quotaSvc, adaptiveSvc)

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery(), gin.Logger(), handlers.CORSMiddleware())

	api := engine.Group("/api/v1")
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
	}

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
