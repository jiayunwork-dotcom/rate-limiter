package queue

import (
	"container/heap"
	"context"
	"sync"
	"time"

	"github.com/ratelimiter/gateway/pkg/models"
)

type queuedRequest struct {
	req        *models.RequestContext
	rule       *models.RuleConfig
	priority   int
	enqueuedAt time.Time
	resultCh   chan *models.RateLimitResult
	index      int
}

type priorityQueue []*queuedRequest

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	if pq[i].priority != pq[j].priority {
		return pq[i].priority > pq[j].priority
	}
	return pq[i].enqueuedAt.Before(pq[j].enqueuedAt)
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*queuedRequest)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[0 : n-1]
	return item
}

type Shaper struct {
	sync.Mutex
	queues       map[string]*priorityQueue
	maxDepth     int
	running      bool
	stopCh       chan struct{}
	tickInterval time.Duration
	backpressureCb func(queueSize int, maxSize int)
}

func NewShaper(maxDepth int, tickMs int64) *Shaper {
	return &Shaper{
		queues:       make(map[string]*priorityQueue),
		maxDepth:     maxDepth,
		tickInterval: time.Duration(tickMs) * time.Millisecond,
		stopCh:       make(chan struct{}),
	}
}

func (s *Shaper) SetBackpressureCallback(cb func(int, int)) {
	s.backpressureCb = cb
}

func (s *Shaper) Start() {
	s.Lock()
	if s.running {
		s.Unlock()
		return
	}
	s.running = true
	s.Unlock()

	go s.processLoop()
}

func (s *Shaper) Stop() {
	s.Lock()
	if !s.running {
		s.Unlock()
		return
	}
	s.running = false
	s.Unlock()
	close(s.stopCh)
}

func (s *Shaper) Enqueue(ctx context.Context, bucketKey string, req *models.RequestContext, rule *models.RuleConfig) (*models.RateLimitResult, error) {
	cfg := rule.ShapingConfig
	if cfg == nil {
		cfg = &models.ShapingConfig{
			Enabled:         true,
			MaxQueueDepth:   s.maxDepth,
			MaxWaitMs:       5000,
			PriorityEnabled: true,
		}
	}

	s.Lock()
	pq, exists := s.queues[bucketKey]
	if !exists {
		tmp := make(priorityQueue, 0)
		pq = &tmp
		s.queues[bucketKey] = pq
	}

	maxDepth := cfg.MaxQueueDepth
	if maxDepth <= 0 {
		maxDepth = s.maxDepth
	}

	if pq.Len() >= maxDepth {
		s.Unlock()
		return &models.RateLimitResult{
			Allowed:   false,
			RuleID:    rule.ID,
			Algorithm: rule.Algorithm,
			Queued:    false,
		}, nil
	}

	priority := req.Priority
	if !cfg.PriorityEnabled {
		priority = 0
	}

	resultCh := make(chan *models.RateLimitResult, 1)
	qr := &queuedRequest{
		req:        req,
		rule:       rule,
		priority:   priority,
		enqueuedAt: time.Now(),
		resultCh:   resultCh,
	}
	heap.Push(pq, qr)
	totalDepth := 0
	for _, q := range s.queues {
		totalDepth += q.Len()
	}
	s.Unlock()

	if s.backpressureCb != nil && totalDepth > maxDepth*3/4 {
		s.backpressureCb(totalDepth, maxDepth*len(s.queues))
	}

	maxWait := time.Duration(cfg.MaxWaitMs) * time.Millisecond
	if maxWait <= 0 {
		maxWait = 5 * time.Second
	}

	select {
	case res := <-resultCh:
		return res, nil
	case <-time.After(maxWait):
		s.Lock()
		if qr.index >= 0 {
			heap.Remove(pq, qr.index)
		}
		s.Unlock()
		return &models.RateLimitResult{
			Allowed:    false,
			RuleID:     rule.ID,
			Algorithm:  rule.Algorithm,
			Queued:     true,
			RetryAfter: int64(maxWait.Seconds()) + 1,
		}, nil
	case <-ctx.Done():
		s.Lock()
		if qr.index >= 0 {
			heap.Remove(pq, qr.index)
		}
		s.Unlock()
		return &models.RateLimitResult{
			Allowed:   false,
			RuleID:    rule.ID,
			Algorithm: rule.Algorithm,
			Queued:    true,
		}, nil
	}
}

func (s *Shaper) processLoop() {
	ticker := time.NewTicker(s.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.processTick()
		}
	}
}

func (s *Shaper) processTick() {
	s.Lock()
	defer s.Unlock()

	for key, pq := range s.queues {
		if pq.Len() == 0 {
			continue
		}

		item := heap.Pop(pq).(*queuedRequest)
		delayMs := int64(time.Since(item.enqueuedAt).Milliseconds())

		select {
		case item.resultCh <- &models.RateLimitResult{
			Allowed:       true,
			RuleID:        item.rule.ID,
			Algorithm:     item.rule.Algorithm,
			Queued:        true,
			QueueDelayMs:  delayMs,
		}:
		default:
		}

		if pq.Len() == 0 {
			delete(s.queues, key)
		}
	}
}

func (s *Shaper) GetQueueDepth(bucketKey string) int {
	s.Lock()
	defer s.Unlock()
	pq, ok := s.queues[bucketKey]
	if !ok {
		return 0
	}
	return pq.Len()
}

func (s *Shaper) GetTotalDepth() int {
	s.Lock()
	defer s.Unlock()
	total := 0
	for _, pq := range s.queues {
		total += pq.Len()
	}
	return total
}
