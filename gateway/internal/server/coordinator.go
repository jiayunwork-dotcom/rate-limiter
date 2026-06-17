package server

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/ratelimiter/gateway/internal/adaptive"
	"github.com/ratelimiter/gateway/internal/algorithms"
	"github.com/ratelimiter/gateway/internal/config"
	"github.com/ratelimiter/gateway/internal/dimensions"
	"github.com/ratelimiter/gateway/internal/distributed"
	"github.com/ratelimiter/gateway/internal/fallback"
	"github.com/ratelimiter/gateway/internal/metrics"
	"github.com/ratelimiter/gateway/internal/queue"
	"github.com/ratelimiter/gateway/internal/quota"
	"github.com/ratelimiter/gateway/pkg/models"
)

type Coordinator struct {
	sync.RWMutex
	localLimiters    map[models.AlgorithmType]algorithms.Limiter
	distStore        *distributed.RedisStore
	ruleStore        *config.Store
	quotaStore       *config.QuotaStore
	dimExtractor     *dimensions.Extractor
	hieraChecker     *quota.HierarchicalChecker
	shaper           *queue.Shaper
	modeSwitcher     *fallback.ModeSwitcher
	adaptiveMgr      *adaptive.AdaptiveManager
	metrics          *metrics.Collector
	eventBuffer      *fallback.EventBuffer
	nodeID           string
	running          bool
	ctx              context.Context
	cancel           context.CancelFunc
}

func NewCoordinator(
	nodeID string,
	redisStore *distributed.RedisStore,
	ruleStore *config.Store,
	quotaStore *config.QuotaStore,
	adaptiveMgr *adaptive.AdaptiveManager,
	collector *metrics.Collector,
	eventFlushCb func([]*models.RateLimitEvent) error,
) *Coordinator {

	ctx, cancel := context.WithCancel(context.Background())

	localLimiters := map[models.AlgorithmType]algorithms.Limiter{
		models.AlgorithmTokenBucket:   algorithms.NewTokenBucket(),
		models.AlgorithmLeakyBucket:   algorithms.NewLeakyBucket(),
		models.AlgorithmFixedWindow:   algorithms.NewFixedWindow(),
		models.AlgorithmSlidingWindow: algorithms.NewSlidingWindow(),
		models.AlgorithmSlidingLog:    algorithms.NewSlidingLog(),
	}

	quotaMgr := quota.NewManager()
	hiera := quota.NewHierarchicalChecker(quotaMgr)

	shaper := queue.NewShaper(1000, 10)

	instanceCount := int64(5)
	modeSwitcher := fallback.NewModeSwitcher(redisStore, instanceCount)
	modeSwitcher.SetOnModeChange(func(old, new models.Mode) {
		collector.SetMode(nodeID, new == models.ModeDistributed)
	})

	eventBuffer := fallback.NewEventBuffer(1000, eventFlushCb)

	return &Coordinator{
		localLimiters: localLimiters,
		distStore:     redisStore,
		ruleStore:     ruleStore,
		quotaStore:    quotaStore,
		dimExtractor:  dimensions.NewExtractor(),
		hieraChecker:  hiera,
		shaper:        shaper,
		modeSwitcher:  modeSwitcher,
		adaptiveMgr:   adaptiveMgr,
		metrics:       collector,
		eventBuffer:   eventBuffer,
		nodeID:        nodeID,
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (c *Coordinator) Start() error {
	c.Lock()
	if c.running {
		c.Unlock()
		return nil
	}
	c.running = true
	c.Unlock()

	c.modeSwitcher.Start()
	c.shaper.Start()
	c.adaptiveMgr.Start(c.ctx)

	if err := c.ruleStore.StartSubscriber(c.ctx); err != nil {
		return err
	}

	go c.rebalanceLoop()

	return nil
}

func (c *Coordinator) Stop() {
	c.cancel()
	c.modeSwitcher.Stop()
	c.shaper.Stop()
	c.eventBuffer.Flush()
}

func (c *Coordinator) Process(ctx context.Context, req *models.RequestContext) (*models.RateLimitResult, error) {
	if req == nil {
		return nil, errors.New("nil request")
	}

	rules := c.ruleStore.GetRules()
	mode := c.modeSwitcher.GetMode()

	quotaResults, err := c.hieraChecker.Check(ctx, req, mode)
	if err != nil {
		return nil, fmt.Errorf("quota check: %w", err)
	}

	for _, qr := range quotaResults {
		if !qr.Allowed {
			c.recordEvent(req, qr)
			return qr, nil
		}
	}

	var lastAllowed *models.RateLimitResult
	matchedRules := 0

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if !c.dimExtractor.MatchesRule(rule, req) {
			continue
		}

		matchedRules++
		c.metrics.RecordRuleMatch(rule.ID, true)

		adjustedRule := c.applyAdaptiveCoefficient(rule)

		result, err := c.checkRule(ctx, bucketKeyFor(rule, req), req, adjustedRule, mode)
		if err != nil {
			continue
		}

		if !result.Allowed {
			hasShaping := rule.ShapingConfig != nil && rule.ShapingConfig.Enabled
			if hasShaping {
				queueStart := time.Now()
				shaped, err := c.shaper.Enqueue(ctx,
					c.dimExtractor.GenerateBucketKeys(rule, req)[0],
					req, rule)
				if err == nil {
					queueD := time.Since(queueStart)
					c.metrics.RecordQueueLatency(rule.ID, req.Priority, queueD)
					shaped.TriggeredLevel = result.TriggeredLevel
					c.recordEvent(req, shaped)
					return shaped, nil
				}
			}
			c.recordEvent(req, result)
			return result, nil
		}

		lastAllowed = result
	}

	if matchedRules == 0 {
		result := &models.RateLimitResult{
			Allowed:   true,
			Limit:     math.MaxInt64,
			Remaining: math.MaxInt64,
			ResetTime: time.Now().Add(60 * time.Second),
			RuleID:    "default-allow",
			Mode:      mode,
		}
		c.metrics.RecordAllowed(req.TenantID, req.UserID, req.APIPath, "default", "none")
		return result, nil
	}

	if lastAllowed != nil {
		c.recordEvent(req, lastAllowed)
	}
	return lastAllowed, nil
}

func (c *Coordinator) checkRule(
	ctx context.Context,
	bucketKey string,
	req *models.RequestContext,
	rule *models.RuleConfig,
	mode models.Mode,
) (*models.RateLimitResult, error) {

	bucketKeys := c.dimExtractor.GenerateBucketKeys(rule, req)
	var firstDenied *models.RateLimitResult
	var lastResult *models.RateLimitResult

	for _, bk := range bucketKeys {
		var result *models.RateLimitResult
		var err error

		start := time.Now()
		if mode == models.ModeDistributed && c.distStore != nil {
			result, err = c.distStore.Check(ctx, bk, time.Now(), rule)
			c.metrics.RecordRedisOp(string(rule.Algorithm), time.Since(start))
			if err != nil {
				result, err = c.checkLocal(ctx, bk, req, rule)
			}
		} else {
			result, err = c.checkLocal(ctx, bk, req, rule)
		}
		if err != nil {
			continue
		}

		result.Mode = mode
		lastResult = result

		if !result.Allowed {
			if firstDenied == nil {
				firstDenied = result
			}
			if rule.Dimensions.CombineMode != "AND" {
				c.metrics.RecordRejected(req.TenantID, req.UserID, req.APIPath,
					rule.ID, string(rule.Algorithm), string(result.TriggeredLevel))
				return result, nil
			}
		}
	}

	if firstDenied != nil {
		return firstDenied, nil
	}
	if lastResult != nil {
		c.metrics.RecordAllowed(req.TenantID, req.UserID, req.APIPath,
			rule.ID, string(rule.Algorithm))
	}
	return lastResult, nil
}

func (c *Coordinator) checkLocal(
	ctx context.Context,
	bucketKey string,
	req *models.RequestContext,
	rule *models.RuleConfig,
) (*models.RateLimitResult, error) {
	limiter, ok := c.localLimiters[rule.Algorithm]
	if !ok {
		return nil, fmt.Errorf("unknown algorithm: %s", rule.Algorithm)
	}
	return limiter.Allow(ctx, bucketKey, req, rule)
}

func (c *Coordinator) applyAdaptiveCoefficient(rule *models.RuleConfig) *models.RuleConfig {
	if c.adaptiveMgr == nil {
		return rule
	}
	coeff := c.adaptiveMgr.GetCoefficient()
	if coeff >= 1.0 {
		return rule
	}

	adjusted := &models.RuleConfig{}
	*adjusted = *rule
	adjusted.Limit = int64(float64(rule.Limit) * coeff)
	if adjusted.Limit < 1 {
		adjusted.Limit = 1
	}

	if adjusted.TokenBucketConfig != nil {
		tbCopy := *adjusted.TokenBucketConfig
		tbCopy.Capacity = int64(float64(tbCopy.Capacity) * coeff)
		tbCopy.RefillRate = int64(float64(tbCopy.RefillRate) * coeff)
		if tbCopy.Capacity < 1 {
			tbCopy.Capacity = 1
		}
		if tbCopy.RefillRate < 1 {
			tbCopy.RefillRate = 1
		}
		adjusted.TokenBucketConfig = &tbCopy
	}
	if adjusted.LeakyBucketConfig != nil {
		lbCopy := *adjusted.LeakyBucketConfig
		lbCopy.Capacity = int64(float64(lbCopy.Capacity) * coeff)
		lbCopy.OutRate = int64(float64(lbCopy.OutRate) * coeff)
		if lbCopy.Capacity < 1 {
			lbCopy.Capacity = 1
		}
		if lbCopy.OutRate < 1 {
			lbCopy.OutRate = 1
		}
		adjusted.LeakyBucketConfig = &lbCopy
	}

	return adjusted
}

func (c *Coordinator) recordEvent(req *models.RequestContext, result *models.RateLimitResult) {
	c.metrics.SetAdaptiveCoeff("global", c.adaptiveMgr.GetCoefficient())

	event := &models.RateLimitEvent{
		Timestamp:      time.Now(),
		RequestID:      req.RequestID,
		Allowed:        result.Allowed,
		RuleID:         result.RuleID,
		Limit:          result.Limit,
		Remaining:      result.Remaining,
		APIPath:        req.APIPath,
		Method:         req.Method,
		UserID:         req.UserID,
		TenantID:       req.TenantID,
		ClientIP:       req.ClientIP,
		TriggeredLevel: result.TriggeredLevel,
		Mode:           result.Mode,
	}

	c.eventBuffer.Add(event)
}

func (c *Coordinator) RecordBackendMetrics(latencyMs int64, isError bool) {
	if c.adaptiveMgr != nil {
		c.adaptiveMgr.RecordRequest(latencyMs, isError)
	}
}

func (c *Coordinator) GetMode() models.Mode {
	return c.modeSwitcher.GetMode()
}

func (c *Coordinator) GetHealth() models.BackendHealth {
	if c.adaptiveMgr != nil {
		return c.adaptiveMgr.GetHealth()
	}
	return models.BackendHealth{}
}

func (c *Coordinator) rebalanceLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if c.distStore != nil && c.modeSwitcher.GetMode() == models.ModeDistributed {
				c.distStore.ReclaimLocalTokens()
			}
		}
	}
}

func bucketKeyFor(rule *models.RuleConfig, req *models.RequestContext) string {
	return fmt.Sprintf("rule:%s", rule.ID)
}
