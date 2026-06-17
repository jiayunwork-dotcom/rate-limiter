package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/ratelimiter/gateway/internal/adaptive"
	"github.com/ratelimiter/gateway/internal/config"
	"github.com/ratelimiter/gateway/internal/distributed"
	"github.com/ratelimiter/gateway/internal/metrics"
	"github.com/ratelimiter/gateway/internal/server"
	"github.com/ratelimiter/gateway/pkg/models"
)

func main() {
	viper.SetConfigName("gateway")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/ratelimiter")
	viper.AddConfigPath(".")
	viper.SetDefault("http_addr", ":8080")
	viper.SetDefault("grpc_addr", ":9090")
	viper.SetDefault("redis_addr", "localhost:6379")
	viper.SetDefault("redis_db", 0)
	viper.SetDefault("postgres_dsn", "host=localhost port=5432 user=ratelimiter password=ratelimiter dbname=ratelimiter sslmode=disable")
	viper.SetDefault("target_url", "http://localhost:3000")
	viper.SetDefault("node_id", uuid.New().String()[:8])
	viper.AutomaticEnv()

	nodeID := viper.GetString("node_id")
	log.Printf("Starting rate limiter gateway node: %s", nodeID)

	redisAddr := viper.GetString("redis_addr")
	redisStore := distributed.NewRedisStore(
		redisAddr,
		viper.GetString("redis_password"),
		viper.GetInt("redis_db"),
		nodeID,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisStore.Ping(ctx); err != nil {
		log.Printf("Warning: Redis not available, will operate in local mode: %v", err)
	}

	db, err := gorm.Open(postgres.Open(viper.GetString("postgres_dsn")), &gorm.Config{})
	if err != nil {
		log.Printf("Warning: PostgreSQL not available: %v", err)
	}

	ruleStore := config.NewStore(nodeID)
	setupRuleStore(ruleStore, redisStore, db)

	quotaStore := config.NewQuotaStore()
	quotaStore.SetPublisher(func(ctx context.Context, channel string, msg interface{}) error {
		return redisStore.Publish(ctx, channel, msg)
	})

	adaptiveCfg := models.AdaptiveConfig{
		Enabled:             true,
		TargetP99LatencyMs:  200,
		ErrorRateThreshold:  0.05,
		MinCoefficient:      0.3,
		MaxCoefficient:      1.0,
		TighteningRatio:     0.7,
		RecoveryIntervalSec: 10,
		RecoveryStepPercent: 10,
		StablePeriodSec:     30,
		PIDConfig: models.PIDConfig{
			Kp:        0.5,
			Ki:        0.1,
			Kd:        0.2,
			Setpoint:  200,
			OutputMin: -30,
			OutputMax: 30,
		},
	}
	adaptiveMgr := adaptive.NewAdaptiveManager(adaptiveCfg)

	collector := metrics.NewCollector("ratelimiter")

	eventFlushCb := func(events []*models.RateLimitEvent) error {
		if db == nil {
			return nil
		}
		return saveEvents(db, events)
	}

	coordinator := server.NewCoordinator(
		nodeID,
		redisStore,
		ruleStore,
		quotaStore,
		adaptiveMgr,
		collector,
		eventFlushCb,
	)
	if err := coordinator.Start(); err != nil {
		log.Fatalf("Failed to start coordinator: %v", err)
	}
	defer coordinator.Stop()

	loadRulesFromDB(ruleStore, db)
	loadDefaultRules(ruleStore)

	gs, err := server.NewGatewayServer(
		nodeID,
		coordinator,
		collector,
		viper.GetString("target_url"),
	)
	if err != nil {
		log.Fatalf("Failed to create gateway server: %v", err)
	}

	grpcInt := server.NewGrpcInterceptor(coordinator)
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(grpcInt.UnaryInterceptor()),
		grpc.StreamInterceptor(grpcInt.StreamInterceptor()),
	)

	errCh := make(chan error, 2)
	go func() {
		log.Printf("HTTP gateway listening on %s", viper.GetString("http_addr"))
		errCh <- gs.Start(viper.GetString("http_addr"))
	}()
	go func() {
		addr := viper.GetString("grpc_addr")
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			errCh <- err
			return
		}
		log.Printf("gRPC gateway listening on %s", addr)
		errCh <- grpcServer.Serve(lis)
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
	gs.Shutdown(shutdownCtx)
	grpcServer.GracefulStop()
}

func setupRuleStore(ruleStore *config.Store, redisStore *distributed.RedisStore, db *gorm.DB) {
	ruleStore.SetPublisher(func(ctx context.Context, channel string, msg interface{}) error {
		return redisStore.Publish(ctx, channel, msg)
	})

	ruleStore.SetSubscriber(func(ctx context.Context, channel string) (<-chan *config.RuleChangeMessage, error) {
		pubsub := redisStore.Subscribe(ctx, channel)
		out := make(chan *config.RuleChangeMessage, 100)
		go func() {
			defer close(out)
			defer pubsub.Close()
			ch := pubsub.Channel()
			for msg := range ch {
				var rcm config.RuleChangeMessage
				if err := json.Unmarshal([]byte(msg.Payload), &rcm); err != nil {
					continue
				}
				select {
				case out <- &rcm:
				case <-ctx.Done():
					return
				}
			}
		}()
		return out, nil
	})
}

func loadRulesFromDB(ruleStore *config.Store, db *gorm.DB) {
	if db == nil {
		return
	}
	type DBRule struct {
		ID        string
		Name      string
		APIPath   string
		Method    string
		Algorithm string
		Enabled   bool
		ConfigJSON string
	}
	var rules []DBRule
	db.Table("rate_limit_rules").Where("enabled = ?", true).Find(&rules)
	for _, r := range rules {
		var rc models.RuleConfig
		if err := json.Unmarshal([]byte(r.ConfigJSON), &rc); err == nil {
			rc.ID = r.ID
			rc.Name = r.Name
			rc.APIPath = r.APIPath
			rc.Method = r.Method
			rc.Algorithm = models.AlgorithmType(r.Algorithm)
			rc.Enabled = r.Enabled
			rulesArr := []*models.RuleConfig{&rc}
			ruleStore.BulkLoad(rulesArr)
		}
	}
}

func loadDefaultRules(ruleStore *config.Store) {
	existingRules := ruleStore.GetRules()
	if len(existingRules) > 0 {
		return
	}

	rule1 := &models.RuleConfig{
		ID:            "global-default",
		Name:          "Global Default Limit",
		APIPath:       "*",
		Method:        "*",
		Algorithm:     models.AlgorithmTokenBucket,
		Enabled:       true,
		Limit:         1000,
		WindowSeconds: 60,
		Dimensions: models.RuleDimensions{
			Dimensions: []models.Dimension{
				{Type: models.DimensionTenantID},
			},
			CombineMode: "OR",
		},
		TokenBucketConfig: &models.TokenBucketConfig{
			RefillRate:   16,
			Capacity:     1000,
			TokensPerReq: 1,
		},
		ShapingConfig: &models.ShapingConfig{
			Enabled:          true,
			MaxQueueDepth:    100,
			MaxWaitMs:        2000,
			PriorityEnabled:  true,
		},
	}
	ruleStore.BulkLoad([]*models.RuleConfig{rule1})
}

type rateLimitEventDB struct {
	gorm.Model
	Timestamp      time.Time
	RequestID      string
	Allowed        bool
	RuleID         string
	RuleName       string
	Limit          int64
	Remaining      int64
	APIPath        string
	Method         string
	UserID         string
	TenantID       string
	ClientIP       string
	TriggeredLevel string
	Mode           string
}

func saveEvents(db *gorm.DB, events []*models.RateLimitEvent) error {
	records := make([]rateLimitEventDB, 0, len(events))
	for _, e := range events {
		records = append(records, rateLimitEventDB{
			Timestamp:      e.Timestamp,
			RequestID:      e.RequestID,
			Allowed:        e.Allowed,
			RuleID:         e.RuleID,
			RuleName:       e.RuleName,
			Limit:          e.Limit,
			Remaining:      e.Remaining,
			APIPath:        e.APIPath,
			Method:         e.Method,
			UserID:         e.UserID,
			TenantID:       e.TenantID,
			ClientIP:       e.ClientIP,
			TriggeredLevel: string(e.TriggeredLevel),
			Mode:           string(e.Mode),
		})
	}
	return db.Table("rate_limit_events").Create(&records).Error
}
