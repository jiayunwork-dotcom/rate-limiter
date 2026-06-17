package distributed

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/ratelimiter/gateway/pkg/models"
)

var (
	slidingWindowLua = `
local key = KEYS[1]
local now_ms = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local req_id = ARGV[4]

local window_start = now_ms - window_ms

redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

local count = redis.call('ZCARD', key)

if count < limit then
    redis.call('ZADD', key, now_ms, req_id)
    redis.call('EXPIRE', key, math.ceil(window_ms / 1000) + 1)
    return {1, limit - count - 1, 0}
else
    local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
    local retry_after = 0
    if #oldest >= 2 then
        local oldest_ms = tonumber(oldest[2])
        retry_after = math.ceil((oldest_ms + window_ms - now_ms) / 1000)
        if retry_after < 1 then retry_after = 1 end
    end
    return {0, 0, retry_after}
end
`

	tokenBucketLua = `
local key = KEYS[1]
local now_ms = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local capacity = tonumber(ARGV[3])
local tokens_needed = tonumber(ARGV[4])

local data = redis.call('HMGET', key, 'tokens', 'last_refill')
local tokens = tonumber(data[1])
local last_refill = tonumber(data[2])

if tokens == nil then
    tokens = capacity
    last_refill = now_ms
end

local elapsed = (now_ms - last_refill) / 1000
local refill = math.floor(elapsed * refill_rate)
if refill > 0 then
    tokens = math.min(capacity, tokens + refill)
    last_refill = now_ms
end

local allowed = 0
local retry_after = 0
local remaining = tokens

if tokens >= tokens_needed then
    tokens = tokens - tokens_needed
    remaining = tokens
    allowed = 1
else
    local deficit = tokens_needed - tokens
    retry_after = math.ceil(deficit / refill_rate)
    if retry_after < 1 then retry_after = 1 end
end

redis.call('HMSET', key, 'tokens', tokens, 'last_refill', last_refill)
redis.call('EXPIRE', key, math.ceil(capacity / refill_rate) + 60)

return {allowed, remaining, retry_after}
`

	fixedWindowLua = `
local key = KEYS[1]
local now_sec = tonumber(ARGV[1])
local window_sec = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])

local window_start = math.floor(now_sec / window_sec) * window_sec
local wm_key = key .. ':wm'

local stored_wm = redis.call('GET', wm_key)
if stored_wm == false or tonumber(stored_wm) ~= window_start then
    redis.call('SET', key, 0)
    redis.call('SET', wm_key, window_start)
end

local count = tonumber(redis.call('GET', key))
if count == nil then count = 0 end

local allowed = 0
local retry_after = 0
local remaining = 0

if count < limit then
    count = redis.call('INCR', key)
    remaining = limit - count
    allowed = 1
else
    local window_end = window_start + window_sec
    retry_after = window_end - now_sec
    if retry_after < 1 then retry_after = 1 end
end

redis.call('EXPIRE', key, window_sec * 2)
redis.call('EXPIRE', wm_key, window_sec * 2)

return {allowed, remaining, retry_after, window_start + window_sec}
`
)

type RedisStore struct {
	client       *redis.Client
	scriptCache  map[string]*redis.Script
	cacheMu      sync.RWMutex
	localTokens  map[string]*localTokenBucket
	localTokenMu sync.Mutex
	nodeID       string
}

type localTokenBucket struct {
	available    int64
	lastRefill   time.Time
	shardSize    int64
}

func NewRedisStore(addr, password string, db int, nodeID string) *RedisStore {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     100,
		MinIdleConns: 10,
		ReadTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
	})

	store := &RedisStore{
		client:      client,
		scriptCache: make(map[string]*redis.Script),
		localTokens: make(map[string]*localTokenBucket),
		nodeID:      nodeID,
	}

	store.scriptCache["sliding_window"] = redis.NewScript(slidingWindowLua)
	store.scriptCache["token_bucket"] = redis.NewScript(tokenBucketLua)
	store.scriptCache["fixed_window"] = redis.NewScript(fixedWindowLua)

	return store
}

func (s *RedisStore) Check(ctx context.Context, key string, now time.Time, rule *models.RuleConfig) (*models.RateLimitResult, error) {
	switch rule.Algorithm {
	case models.AlgorithmSlidingWindow, models.AlgorithmSlidingLog:
		return s.checkSlidingWindow(ctx, key, now, rule)
	case models.AlgorithmTokenBucket:
		return s.checkTokenBucket(ctx, key, now, rule)
	case models.AlgorithmFixedWindow:
		return s.checkFixedWindow(ctx, key, now, rule)
	case models.AlgorithmLeakyBucket:
		return s.checkLeakyBucket(ctx, key, now, rule)
	default:
		return s.checkFixedWindow(ctx, key, now, rule)
	}
}

func (s *RedisStore) checkSlidingWindow(ctx context.Context, key string, now time.Time, rule *models.RuleConfig) (*models.RateLimitResult, error) {
	script := s.scriptCache["sliding_window"]
	windowSec := rule.WindowSeconds
	if windowSec <= 0 {
		windowSec = 60
	}
	nowMs := now.UnixNano() / int64(time.Millisecond)
	windowMs := windowSec * 1000
	reqID := fmt.Sprintf("%d-%s", nowMs, s.nodeID)

	res, err := script.Run(ctx, s.client, []string{key},
		nowMs, windowMs, rule.Limit, reqID,
	).Slice()
	if err != nil {
		return nil, err
	}

	allowed := res[0].(int64) == 1
	remaining := res[1].(int64)
	retryAfter := res[2].(int64)

	return &models.RateLimitResult{
		Allowed:    allowed,
		Limit:      rule.Limit,
		Remaining:  remaining,
		ResetTime:  now.Add(time.Duration(windowSec) * time.Second),
		RetryAfter: retryAfter,
		RuleID:     rule.ID,
		Algorithm:  rule.Algorithm,
		Mode:       models.ModeDistributed,
	}, nil
}

func (s *RedisStore) checkTokenBucket(ctx context.Context, key string, now time.Time, rule *models.RuleConfig) (*models.RateLimitResult, error) {
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

	s.localTokenMu.Lock()
	lt, ok := s.localTokens[key]
	if !ok {
		lt = &localTokenBucket{
			available:  0,
			lastRefill: now,
			shardSize:  max64(config.Capacity/10, 5),
		}
		s.localTokens[key] = lt
	}
	s.localTokenMu.Unlock()

	s.localTokenMu.Lock()
	if lt.available >= config.TokensPerReq {
		lt.available -= config.TokensPerReq
		s.localTokenMu.Unlock()
		return &models.RateLimitResult{
			Allowed:   true,
			Limit:     config.Capacity,
			Remaining: lt.available,
			ResetTime: now.Add(time.Duration(rule.WindowSeconds) * time.Second),
			RuleID:    rule.ID,
			Algorithm: rule.Algorithm,
			Mode:      models.ModeDistributed,
		}, nil
	}
	s.localTokenMu.Unlock()

	script := s.scriptCache["token_bucket"]
	nowMs := now.UnixNano() / int64(time.Millisecond)

	shardReq := lt.shardSize + config.TokensPerReq

	res, err := script.Run(ctx, s.client, []string{key},
		nowMs, config.RefillRate, config.Capacity, shardReq,
	).Slice()
	if err != nil {
		return nil, err
	}

	allowed := res[0].(int64) == 1
	remaining := res[1].(int64)
	retryAfter := res[2].(int64)

	if allowed {
		s.localTokenMu.Lock()
		lt.available += lt.shardSize
		if lt.available > config.Capacity {
			lt.available = config.Capacity
		}
		s.localTokenMu.Unlock()
	}

	return &models.RateLimitResult{
		Allowed:    allowed,
		Limit:      config.Capacity,
		Remaining:  remaining,
		ResetTime:  now.Add(time.Duration(rule.WindowSeconds) * time.Second),
		RetryAfter: retryAfter,
		RuleID:     rule.ID,
		Algorithm:  rule.Algorithm,
		Mode:       models.ModeDistributed,
	}, nil
}

func (s *RedisStore) checkFixedWindow(ctx context.Context, key string, now time.Time, rule *models.RuleConfig) (*models.RateLimitResult, error) {
	script := s.scriptCache["fixed_window"]
	windowSec := rule.WindowSeconds
	if windowSec <= 0 {
		windowSec = 60
	}
	nowSec := now.Unix()

	res, err := script.Run(ctx, s.client, []string{key},
		nowSec, windowSec, rule.Limit,
	).Slice()
	if err != nil {
		return nil, err
	}

	allowed := res[0].(int64) == 1
	remaining := res[1].(int64)
	retryAfter := res[2].(int64)
	resetSec := res[3].(int64)

	return &models.RateLimitResult{
		Allowed:    allowed,
		Limit:      rule.Limit,
		Remaining:  remaining,
		ResetTime:  time.Unix(resetSec, 0),
		RetryAfter: retryAfter,
		RuleID:     rule.ID,
		Algorithm:  rule.Algorithm,
		Mode:       models.ModeDistributed,
	}, nil
}

func (s *RedisStore) checkLeakyBucket(ctx context.Context, key string, now time.Time, rule *models.RuleConfig) (*models.RateLimitResult, error) {
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

	queueKey := key + ":queue"
	metaKey := key + ":meta"

	nowMs := now.UnixNano() / int64(time.Millisecond)

	meta, err := s.client.HMGet(ctx, metaKey, "last_leak", "out_rate", "capacity").Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}

	lastLeak := nowMs
	if meta[0] != nil {
		fmt.Sscanf(meta[0].(string), "%d", &lastLeak)
	}

	intervalMs := 1000 / config.OutRate
	elapsedMs := nowMs - lastLeak
	leakedCount := elapsedMs / intervalMs
	if leakedCount > 0 {
		if leakedCount > 1000000 {
			leakedCount = 1000000
		}
		s.client.LTrim(ctx, queueKey, leakedCount, -1)
		s.client.HSet(ctx, metaKey, "last_leak", lastLeak+leakedCount*intervalMs)
	}

	queueLen, err := s.client.LLen(ctx, queueKey).Result()
	if err != nil {
		return nil, err
	}

	windowSec := rule.WindowSeconds
	if windowSec <= 0 {
		windowSec = 60
	}

	if queueLen < config.Capacity {
		s.client.RPush(ctx, queueKey, nowMs)
		s.client.Expire(ctx, queueKey, time.Duration(windowSec*2)*time.Second)
		s.client.Expire(ctx, metaKey, time.Duration(windowSec*2)*time.Second)

		return &models.RateLimitResult{
			Allowed:   true,
			Limit:     config.Capacity,
			Remaining: config.Capacity - queueLen - 1,
			ResetTime: now.Add(time.Duration(windowSec) * time.Second),
			RuleID:    rule.ID,
			Algorithm: rule.Algorithm,
			Mode:      models.ModeDistributed,
		}, nil
	}

	retryAfter := (config.Capacity * intervalMs) / 1000
	if retryAfter < 1 {
		retryAfter = 1
	}

	return &models.RateLimitResult{
		Allowed:    false,
		Limit:      config.Capacity,
		Remaining:  0,
		ResetTime:  now.Add(time.Duration(windowSec) * time.Second),
		RetryAfter: retryAfter,
		RuleID:     rule.ID,
		Algorithm:  rule.Algorithm,
		Mode:       models.ModeDistributed,
	}, nil
}

func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *RedisStore) Publish(ctx context.Context, channel string, msg interface{}) error {
	return s.client.Publish(ctx, channel, msg).Err()
}

func (s *RedisStore) Subscribe(ctx context.Context, channel string) *redis.PubSub {
	return s.client.Subscribe(ctx, channel)
}

func (s *RedisStore) GetClient() *redis.Client {
	return s.client
}

func (s *RedisStore) ReclaimLocalTokens() {
	s.localTokenMu.Lock()
	defer s.localTokenMu.Unlock()
	for key, lt := range s.localTokens {
		ctx := context.Background()
		if lt.available > lt.shardSize*2 {
			returnTokens := lt.available - lt.shardSize
			script := s.scriptCache["token_bucket"]
			nowMs := time.Now().UnixNano() / int64(time.Millisecond)
			script.Run(ctx, s.client, []string{key},
				nowMs, 1, 1, 0,
			).Result()
			_ = returnTokens
		}
	}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
