package algorithms

import (
	"context"
	"sync"
	"time"

	"github.com/ratelimiter/gateway/pkg/models"
)

type FixedWindow struct {
	baseLimiter
	sync.RWMutex
	windows map[string]*fixedWindowState
}

type fixedWindowState struct {
	count     int64
	windowStart time.Time
	limit     int64
	windowSec int64
}

func NewFixedWindow() *FixedWindow {
	return &FixedWindow{
		windows: make(map[string]*fixedWindowState),
	}
}

func (fw *FixedWindow) Allow(ctx context.Context, bucketKey string, req *models.RequestContext, rule *models.RuleConfig) (*models.RateLimitResult, error) {
	fw.Lock()
	defer fw.Unlock()

	windowSec := rule.WindowSeconds
	if windowSec <= 0 {
		windowSec = 60
	}

	state, exists := fw.windows[bucketKey]
	if !exists {
		state = &fixedWindowState{
			count:       0,
			windowStart: time.Now(),
			limit:       rule.Limit,
			windowSec:   windowSec,
		}
		fw.windows[bucketKey] = state
	}

	now := time.Now()
	elapsed := now.Sub(state.windowStart).Seconds()

	if int64(elapsed) >= state.windowSec {
		passedWindows := int64(elapsed) / state.windowSec
		state.windowStart = state.windowStart.Add(time.Duration(passedWindows*state.windowSec) * time.Second)
		state.count = 0
	}

	allowed := false
	var retryAfter int64
	remaining := state.limit - state.count
	if remaining < 0 {
		remaining = 0
	}

	if state.count < state.limit {
		state.count++
		remaining = state.limit - state.count
		allowed = true
	} else {
		windowEnd := state.windowStart.Add(time.Duration(state.windowSec) * time.Second)
		retryAfter = int64(windowEnd.Sub(now).Seconds())
		if retryAfter <= 0 {
			retryAfter = 1
		}
	}

	return &models.RateLimitResult{
		Allowed:    allowed,
		Limit:      state.limit,
		Remaining:  remaining,
		ResetTime:  state.windowStart.Add(time.Duration(state.windowSec) * time.Second),
		RetryAfter: retryAfter,
		RuleID:     rule.ID,
		Algorithm:  models.AlgorithmFixedWindow,
	}, nil
}
