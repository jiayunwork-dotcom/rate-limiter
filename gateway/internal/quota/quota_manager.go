package quota

import (
	"context"
	"sync"
	"time"

	"github.com/ratelimiter/gateway/pkg/models"
)

type Manager struct {
	sync.RWMutex
	quotas map[string]*models.QuotaConfig
}

func NewManager() *Manager {
	return &Manager{
		quotas: make(map[string]*models.QuotaConfig),
	}
}

func (m *Manager) SetQuota(quota *models.QuotaConfig) {
	m.Lock()
	defer m.Unlock()
	key := string(quota.Level) + ":" + quota.Identifier
	m.quotas[key] = quota
}

func (m *Manager) GetQuota(level models.QuotaLevel, identifier string) *models.QuotaConfig {
	m.RLock()
	defer m.RUnlock()
	key := string(level) + ":" + identifier
	if q, ok := m.quotas[key]; ok {
		return q
	}
	return m.getDefaultQuota(level, identifier)
}

func (m *Manager) getDefaultQuota(level models.QuotaLevel, identifier string) *models.QuotaConfig {
	switch level {
	case models.QuotaLevelGlobal:
		return &models.QuotaConfig{
			Level:         models.QuotaLevelGlobal,
			Identifier:    "global",
			Limit:         100000,
			WindowSeconds: 60,
		}
	case models.QuotaLevelTenant:
		return &models.QuotaConfig{
			Level:         models.QuotaLevelTenant,
			Identifier:    identifier,
			Limit:         10000,
			WindowSeconds: 60,
			InheritFrom:   true,
		}
	case models.QuotaLevelUser:
		return &models.QuotaConfig{
			Level:         models.QuotaLevelUser,
			Identifier:    identifier,
			Limit:         1000,
			WindowSeconds: 60,
			InheritFrom:   true,
		}
	case models.QuotaLevelAPI:
		return &models.QuotaConfig{
			Level:         models.QuotaLevelAPI,
			Identifier:    identifier,
			Limit:         100,
			WindowSeconds: 60,
			InheritFrom:   true,
		}
	default:
		return &models.QuotaConfig{
			Level:         level,
			Identifier:    identifier,
			Limit:         100,
			WindowSeconds: 60,
		}
	}
}

type HierarchicalChecker struct {
	sync.RWMutex
	counters    map[string]*hierarchyCounter
	quotaMgr    *Manager
	instanceCnt int64
}

type hierarchyCounter struct {
	count     int64
	windowStart time.Time
}

func NewHierarchicalChecker(quotaMgr *Manager) *HierarchicalChecker {
	return &HierarchicalChecker{
		counters:    make(map[string]*hierarchyCounter),
		quotaMgr:    quotaMgr,
		instanceCnt: 1,
	}
}

func (hc *HierarchicalChecker) SetInstanceCount(n int64) {
	hc.Lock()
	defer hc.Unlock()
	hc.instanceCnt = n
}

func (hc *HierarchicalChecker) Check(ctx context.Context, req *models.RequestContext, mode models.Mode) ([]*models.RateLimitResult, error) {
	hc.Lock()
	defer hc.Unlock()

	results := make([]*models.RateLimitResult, 0)

	levels := []models.QuotaLevel{
		models.QuotaLevelGlobal,
		models.QuotaLevelTenant,
		models.QuotaLevelUser,
		models.QuotaLevelAPI,
	}

	identifiers := hc.getIdentifiers(req)

	for i, level := range levels {
		ident := identifiers[i]
		if ident == "" {
			continue
		}

		quota := hc.quotaMgr.GetQuota(level, ident)
		key := string(level) + ":" + ident

		limit := quota.Limit
		if mode == models.ModeLocal && hc.instanceCnt > 1 {
			limit = limit / hc.instanceCnt
			if limit < 1 {
				limit = 1
			}
		}

		counter, exists := hc.counters[key]
		now := time.Now()
		windowSec := quota.WindowSeconds
		if windowSec <= 0 {
			windowSec = 60
		}

		if !exists {
			counter = &hierarchyCounter{
				count:       0,
				windowStart: now,
			}
			hc.counters[key] = counter
		}

		elapsed := now.Sub(counter.windowStart).Seconds()
		if int64(elapsed) >= windowSec {
			passedWindows := int64(elapsed) / windowSec
			counter.windowStart = counter.windowStart.Add(time.Duration(passedWindows*windowSec) * time.Second)
			counter.count = 0
		}

		allowed := counter.count < limit
		remaining := limit - counter.count
		if remaining < 0 {
			remaining = 0
		}

		var retryAfter int64
		if !allowed {
			windowEnd := counter.windowStart.Add(time.Duration(windowSec) * time.Second)
			retryAfter = int64(windowEnd.Sub(now).Seconds())
			if retryAfter <= 0 {
				retryAfter = 1
			}
		} else {
			counter.count++
			remaining = limit - counter.count
		}

		result := &models.RateLimitResult{
			Allowed:        allowed,
			Limit:          limit,
			Remaining:      remaining,
			ResetTime:      counter.windowStart.Add(time.Duration(windowSec) * time.Second),
			RetryAfter:     retryAfter,
			RuleID:         "quota-" + string(level),
			Algorithm:      models.AlgorithmFixedWindow,
			Mode:           mode,
			TriggeredLevel: level,
		}
		results = append(results, result)

		if !allowed {
			break
		}
	}

	return results, nil
}

func (hc *HierarchicalChecker) getIdentifiers(req *models.RequestContext) [4]string {
	var result [4]string
	result[0] = "global"
	result[1] = req.TenantID
	if req.TenantID != "" && req.UserID != "" {
		result[2] = req.TenantID + ":" + req.UserID
	}
	if req.APIPath != "" {
		result[3] = req.Method + ":" + req.APIPath
	}
	return result
}

func (hc *HierarchicalChecker) GetUsage(level models.QuotaLevel, identifier string) (count int64, limit int64, usage float64) {
	hc.RLock()
	defer hc.RUnlock()

	key := string(level) + ":" + identifier
	quota := hc.quotaMgr.GetQuota(level, identifier)

	limit = quota.Limit
	counter, ok := hc.counters[key]
	if ok {
		count = counter.count
	}
	if limit > 0 {
		usage = float64(count) / float64(limit)
	}
	return
}
