package algorithms

import (
	"context"
	"sync"
	"time"

	"github.com/ratelimiter/gateway/pkg/models"
)

type LeakyBucket struct {
	baseLimiter
	sync.RWMutex
	buckets map[string]*leakyBucketState
}

type leakyBucketState struct {
	queue     []time.Time
	capacity  int64
	outRate   int64
	lastLeak  time.Time
}

func NewLeakyBucket() *LeakyBucket {
	return &LeakyBucket{
		buckets: make(map[string]*leakyBucketState),
	}
}

func (lb *LeakyBucket) Allow(ctx context.Context, bucketKey string, req *models.RequestContext, rule *models.RuleConfig) (*models.RateLimitResult, error) {
	lb.Lock()
	defer lb.Unlock()

	config := rule.LeakyBucketConfig
	if config == nil {
		config = &models.LeakyBucketConfig{
			OutRate:  rule.Limit / rule.WindowSeconds,
			Capacity: rule.Limit,
		}
		if config.OutRate <= 0 {
			config.OutRate = 1
		}
	}

	state, exists := lb.buckets[bucketKey]
	if !exists {
		state = &leakyBucketState{
			queue:    make([]time.Time, 0),
			capacity: config.Capacity,
			outRate:  config.OutRate,
			lastLeak: time.Now(),
		}
		lb.buckets[bucketKey] = state
	}

	now := time.Now()
	interval := time.Second / time.Duration(state.outRate)
	elapsed := now.Sub(state.lastLeak)
	leaked := int64(elapsed / interval)
	if leaked > 0 {
		if leaked >= int64(len(state.queue)) {
			state.queue = state.queue[:0]
		} else {
			state.queue = state.queue[leaked:]
		}
		state.lastLeak = state.lastLeak.Add(interval * time.Duration(leaked))
	}

	allowed := false
	var retryAfter int64
	currentSize := int64(len(state.queue))

	if currentSize < state.capacity {
		state.queue = append(state.queue, now)
		currentSize++
		allowed = true
	} else {
		queueDuration := time.Duration(state.capacity) * interval
		retryAfter = int64(queueDuration.Seconds())
		if retryAfter <= 0 {
			retryAfter = 1
		}
	}

	remaining := state.capacity - currentSize
	if remaining < 0 {
		remaining = 0
	}

	resetSeconds := rule.WindowSeconds
	if resetSeconds <= 0 {
		resetSeconds = 60
	}

	return &models.RateLimitResult{
		Allowed:    allowed,
		Limit:      state.capacity,
		Remaining:  remaining,
		ResetTime:  b.calcResetTime(resetSeconds),
		RetryAfter: retryAfter,
		RuleID:     rule.ID,
		Algorithm:  models.AlgorithmLeakyBucket,
	}, nil
}
