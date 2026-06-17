package fallback

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ratelimiter/gateway/pkg/models"
)

type HealthChecker interface {
	Ping(ctx context.Context) error
}

type ModeSwitcher struct {
	sync.RWMutex
	currentMode   models.Mode
	checker       HealthChecker
	instanceCount int64
	consecutiveFails int32
	consecutiveOKs   int32
	failThreshold    int32
	okThreshold      int32
	checkInterval    time.Duration
	onModeChange     func(oldMode, newMode models.Mode)
	ctx             context.Context
	cancel          context.CancelFunc
	pendingCounts   int64
}

func NewModeSwitcher(checker HealthChecker, instanceCount int64) *ModeSwitcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &ModeSwitcher{
		currentMode:   models.ModeDistributed,
		checker:       checker,
		instanceCount: instanceCount,
		failThreshold: 3,
		okThreshold:   5,
		checkInterval: 2 * time.Second,
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (m *ModeSwitcher) SetOnModeChange(cb func(old, new models.Mode)) {
	m.onModeChange = cb
}

func (m *ModeSwitcher) SetThresholds(fail, ok int32, interval time.Duration) {
	m.Lock()
	m.failThreshold = fail
	m.okThreshold = ok
	m.checkInterval = interval
	m.Unlock()
}

func (m *ModeSwitcher) Start() {
	go m.healthCheckLoop()
}

func (m *ModeSwitcher) Stop() {
	m.cancel()
}

func (m *ModeSwitcher) GetMode() models.Mode {
	m.RLock()
	defer m.RUnlock()
	return m.currentMode
}

func (m *ModeSwitcher) GetInstanceCount() int64 {
	return m.instanceCount
}

func (m *ModeSwitcher) AddPendingCount(n int64) {
	atomic.AddInt64(&m.pendingCounts, n)
}

func (m *ModeSwitcher) GetAndClearPendingCounts() int64 {
	return atomic.SwapInt64(&m.pendingCounts, 0)
}

func (m *ModeSwitcher) healthCheckLoop() {
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.performCheck()
		}
	}
}

func (m *ModeSwitcher) performCheck() {
	ctx, cancel := context.WithTimeout(m.ctx, 500*time.Millisecond)
	defer cancel()

	err := m.checker.Ping(ctx)

	m.Lock()
	defer m.Unlock()

	oldMode := m.currentMode

	if err != nil {
		atomic.StoreInt32(&m.consecutiveOKs, 0)
		newFails := atomic.AddInt32(&m.consecutiveFails, 1)
		if newFails >= m.failThreshold && m.currentMode == models.ModeDistributed {
			m.currentMode = models.ModeLocal
			if m.onModeChange != nil {
				m.onModeChange(oldMode, m.currentMode)
			}
		}
	} else {
		atomic.StoreInt32(&m.consecutiveFails, 0)
		newOKs := atomic.AddInt32(&m.consecutiveOKs, 1)
		if newOKs >= m.okThreshold && m.currentMode == models.ModeLocal {
			m.currentMode = models.ModeDistributed
			if m.onModeChange != nil {
				m.onModeChange(oldMode, m.currentMode)
			}
		}
	}
}

func (m *ModeSwitcher) ForceMode(mode models.Mode) {
	m.Lock()
	old := m.currentMode
	m.currentMode = mode
	m.Unlock()
	if m.onModeChange != nil {
		m.onModeChange(old, mode)
	}
}

type EventBuffer struct {
	sync.RWMutex
	events    []*models.RateLimitEvent
	maxSize   int
	flushCb   func([]*models.RateLimitEvent) error
	lastFlush time.Time
}

func NewEventBuffer(maxSize int, flushCb func([]*models.RateLimitEvent) error) *EventBuffer {
	return &EventBuffer{
		events:    make([]*models.RateLimitEvent, 0, maxSize),
		maxSize:   maxSize,
		flushCb:   flushCb,
		lastFlush: time.Now(),
	}
}

func (eb *EventBuffer) Add(e *models.RateLimitEvent) {
	eb.Lock()
	eb.events = append(eb.events, e)
	shouldFlush := len(eb.events) >= eb.maxSize ||
		time.Since(eb.lastFlush) >= 10*time.Second
	events := eb.events
	if shouldFlush {
		eb.events = make([]*models.RateLimitEvent, 0, eb.maxSize)
		eb.lastFlush = time.Now()
	} else {
		events = nil
	}
	eb.Unlock()

	if events != nil && eb.flushCb != nil {
		go func(evts []*models.RateLimitEvent) {
			_ = eb.flushCb(evts)
		}(events)
	}
}

func (eb *EventBuffer) Flush() error {
	eb.Lock()
	events := eb.events
	eb.events = make([]*models.RateLimitEvent, 0, eb.maxSize)
	eb.lastFlush = time.Now()
	eb.Unlock()

	if len(events) > 0 && eb.flushCb != nil {
		return eb.flushCb(events)
	}
	return nil
}

func (eb *EventBuffer) GetPendingCount() int {
	eb.RLock()
	defer eb.RUnlock()
	return len(eb.events)
}
