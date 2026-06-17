package algorithms

import (
	"context"
	"sync"
	"time"

	"github.com/ratelimiter/gateway/pkg/models"
)

type TokenBucket struct {
	baseLimiter
	sync.RWMutex
	buckets map[string]*tokenBucketState
}

type tokenBucketState struct {
	tokens     int64
	lastRefill time.Time
	capacity   int64
	refillRate int64
}

func NewTokenBucket() *TokenBucket {
	return &TokenBucket{
		buckets: make(map[string]*tokenBucketState),
	}
}

func (tb *TokenBucket) Allow(ctx context.Context, bucketKey string, req *models.RequestContext, rule *models.RuleConfig) (*models.RateLimitResult, error) {
	tb.Lock()
	defer tb.Unlock()

	config := rule.TokenBucketConfig
	if config == nil {
		config = &models.TokenBucketConfig{
			RefillRate:   rule.Limit / rule.WindowSeconds,
			Capacity:     rule.Limit,
			TokensPerReq: 1,
		}
		if config.RefillRate <= 0 {
			config.RefillRate = 1
		}
	}

	state, exists := tb.buckets[bucketKey]
	if !exists {
		state = &tokenBucketState{
			tokens:     config.Capacity,
			lastRefill: time.Now(),
			capacity:   config.Capacity,
			refillRate: config.RefillRate,
		}
		tb.buckets[bucketKey] = state
	}

	now := time.Now()
	elapsed := now.Sub(state.lastRefill).Seconds()
	refillTokens := int64(elapsed * float64(state.refillRate))
	if refillTokens > 0 {
		state.tokens = min64(state.capacity, state.tokens+refillTokens)
		state.lastRefill = now
	}

	allowed := false
	remaining := state.tokens
	tokensNeeded := config.TokensPerReq

	if state.tokens >= tokensNeeded {
		state.tokens -= tokensNeeded
		remaining = state.tokens
		allowed = true
	}

	var retryAfter int64
	if !allowed {
		tokensToRefill := tokensNeeded - state.tokens
		retryAfter = int64(float64(tokensToRefill) / float64(state.refillRate))
		if retryAfter <= 0 {
			retryAfter = 1
		}
	}

	resetSeconds := rule.WindowSeconds
	if resetSeconds <= 0 {
		resetSeconds = 60
	}

	return &models.RateLimitResult{
		Allowed:    allowed,
		Limit:      config.Capacity,
		Remaining:  remaining,
		ResetTime:  b.calcResetTime(resetSeconds),
		RetryAfter: retryAfter,
		RuleID:     rule.ID,
		Algorithm:  models.AlgorithmTokenBucket,
	}, nil
}
