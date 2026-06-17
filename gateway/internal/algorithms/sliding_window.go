package algorithms

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/ratelimiter/gateway/pkg/models"
)

type SlidingWindow struct {
	baseLimiter
	sync.RWMutex
	windows map[string]*slidingWindowState
}

type slidingWindowState struct {
	requests  []int64
	limit     int64
	windowSec int64
}

func NewSlidingWindow() *SlidingWindow {
	return &SlidingWindow{
		windows: make(map[string]*slidingWindowState),
	}
}

func (sw *SlidingWindow) Allow(ctx context.Context, bucketKey string, req *models.RequestContext, rule *models.RuleConfig) (*models.RateLimitResult, error) {
	sw.Lock()
	defer sw.Unlock()

	windowSec := rule.WindowSeconds
	if windowSec <= 0 {
		windowSec = 60
	}

	state, exists := sw.windows[bucketKey]
	if !exists {
		state = &slidingWindowState{
			requests:  make([]int64, 0),
			limit:     rule.Limit,
			windowSec: windowSec,
		}
		sw.windows[bucketKey] = state
	}

	nowMs := time.Now().UnixNano() / int64(time.Millisecond)
	windowStartMs := nowMs - (state.windowSec * 1000)

	idx := sort.Search(len(state.requests), func(i int) bool {
		return state.requests[i] > windowStartMs
	})
	state.requests = state.requests[idx:]

	allowed := false
	var retryAfter int64
	remaining := state.limit - int64(len(state.requests))
	if remaining < 0 {
		remaining = 0
	}

	if int64(len(state.requests)) < state.limit {
		state.requests = append(state.requests, nowMs)
		remaining = state.limit - int64(len(state.requests))
		allowed = true
	} else {
		oldestIdx := len(state.requests) - int(state.limit)
		if oldestIdx >= 0 && oldestIdx < len(state.requests) {
			oldestTime := state.requests[oldestIdx]
			waitMs := oldestTime + (state.windowSec * 1000) - nowMs
			retryAfter = waitMs / 1000
			if waitMs%1000 > 0 {
				retryAfter++
			}
		}
		if retryAfter <= 0 {
			retryAfter = 1
		}
	}

	return &models.RateLimitResult{
		Allowed:    allowed,
		Limit:      state.limit,
		Remaining:  remaining,
		ResetTime:  time.Now().Add(time.Duration(state.windowSec) * time.Second),
		RetryAfter: retryAfter,
		RuleID:     rule.ID,
		Algorithm:  models.AlgorithmSlidingWindow,
	}, nil
}
