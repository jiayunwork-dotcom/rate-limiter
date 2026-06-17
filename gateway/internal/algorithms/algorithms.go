package algorithms

import (
	"context"
	"time"

	"github.com/ratelimiter/gateway/pkg/models"
)

type Limiter interface {
	Allow(ctx context.Context, bucketKey string, req *models.RequestContext, rule *models.RuleConfig) (*models.RateLimitResult, error)
}

type baseLimiter struct{}

func (b *baseLimiter) calcResetTime(windowSeconds int64) time.Time {
	return time.Now().Add(time.Duration(windowSeconds) * time.Second)
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
