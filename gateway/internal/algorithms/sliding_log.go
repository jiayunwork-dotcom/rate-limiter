package algorithms

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/ratelimiter/gateway/pkg/models"
)

type SlidingLog struct {
	baseLimiter
	sync.RWMutex
	logs map[string][]int64
}

func NewSlidingLog() *SlidingLog {
	return &SlidingLog{
		logs: make(map[string][]int64),
	}
}

func (sl *SlidingLog) Allow(ctx context.Context, bucketKey string, req *models.RequestContext, rule *models.RuleConfig) (*models.RateLimitResult, error) {
	sl.Lock()
	defer sl.Unlock()

	windowSec := rule.WindowSeconds
	if windowSec <= 0 {
		windowSec = 60
	}

	logs, exists := sl.logs[bucketKey]
	if !exists {
		logs = make([]int64, 0)
	}

	nowMs := time.Now().UnixNano() / int64(time.Millisecond)
	windowStartMs := nowMs - (windowSec * 1000)

	idx := sort.Search(len(logs), func(i int) bool {
		return logs[i] > windowStartMs
	})
	logs = logs[idx:]

	allowed := false
	var retryAfter int64
	remaining := rule.Limit - int64(len(logs))
	if remaining < 0 {
		remaining = 0
	}

	if int64(len(logs)) < rule.Limit {
		logs = append(logs, nowMs)
		sort.Slice(logs, func(i, j int) bool {
			return logs[i] < logs[j]
		})
		remaining = rule.Limit - int64(len(logs))
		allowed = true
	} else {
		idx := len(logs) - int(rule.Limit)
		if idx >= 0 && idx < len(logs) {
			oldestTime := logs[idx]
			waitMs := oldestTime + (windowSec * 1000) - nowMs
			retryAfter = waitMs / 1000
			if waitMs%1000 > 0 {
				retryAfter++
			}
		}
		if retryAfter <= 0 {
			retryAfter = 1
		}
	}

	sl.logs[bucketKey] = logs

	return &models.RateLimitResult{
		Allowed:    allowed,
		Limit:      rule.Limit,
		Remaining:  remaining,
		ResetTime:  time.Now().Add(time.Duration(windowSec) * time.Second),
		RetryAfter: retryAfter,
		RuleID:     rule.ID,
		Algorithm:  models.AlgorithmSlidingLog,
	}, nil
}

func (sl *SlidingLog) Cleanup() {
	sl.Lock()
	defer sl.Unlock()

	nowMs := time.Now().UnixNano() / int64(time.Millisecond)
	maxAgeMs := int64(24 * 60 * 60 * 1000)

	for key, logs := range sl.logs {
		cutoff := nowMs - maxAgeMs
		idx := sort.Search(len(logs), func(i int) bool {
			return logs[i] > cutoff
		})
		if idx > 0 {
			sl.logs[key] = logs[idx:]
		}
		if len(sl.logs[key]) == 0 {
			delete(sl.logs, key)
		}
	}
}
