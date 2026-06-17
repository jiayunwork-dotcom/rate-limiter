package adaptive

import (
	"context"
	"sync"
	"time"

	"github.com/ratelimiter/gateway/pkg/models"
)

type PIDController struct {
	sync.RWMutex
	config models.PIDConfig
	integral float64
	prevError float64
	initialized bool
}

func NewPIDController(cfg models.PIDConfig) *PIDController {
	return &PIDController{
		config: cfg,
	}
}

func (p *PIDController) Compute(measurement float64) float64 {
	p.Lock()
	defer p.Unlock()

	error := p.config.Setpoint - measurement

	if !p.initialized {
		p.prevError = error
		p.initialized = true
	}

	p.integral += error
	derivative := error - p.prevError

	output := p.config.Kp*error + p.config.Ki*p.integral + p.config.Kd*derivative

	if output > p.config.OutputMax {
		output = p.config.OutputMax
	} else if output < p.config.OutputMin {
		output = p.config.OutputMin
	}

	p.prevError = error
	return output
}

func (p *PIDController) Reset() {
	p.Lock()
	defer p.Unlock()
	p.integral = 0
	p.prevError = 0
	p.initialized = false
}

func (p *PIDController) UpdateConfig(cfg models.PIDConfig) {
	p.Lock()
	defer p.Unlock()
	p.config = cfg
}

type AdaptiveManager struct {
	sync.RWMutex
	config       models.AdaptiveConfig
	enabled      bool
	coefficient  float64
	baseCoeff    float64
	health       models.BackendHealth
	latencies    []int64
	errorCounts  []int
	totalCounts  []int
	pid          *PIDController
	stableSince  *time.Time
	lastTighten  time.Time
	lastRecovery time.Time
}

func NewAdaptiveManager(cfg models.AdaptiveConfig) *AdaptiveManager {
	m := &AdaptiveManager{
		config:       cfg,
		enabled:      cfg.Enabled,
		coefficient:  1.0,
		baseCoeff:    1.0,
		latencies:    make([]int64, 0, 1000),
		errorCounts:  make([]int, 0, 1000),
		totalCounts:  make([]int, 0, 1000),
		lastTighten:  time.Now(),
		lastRecovery: time.Now(),
	}
	m.pid = NewPIDController(cfg.PIDConfig)
	return m
}

func (m *AdaptiveManager) RecordRequest(latencyMs int64, isError bool) {
	m.Lock()
	defer m.Unlock()

	now := time.Now()

	m.latencies = append(m.latencies, latencyMs)
	cutoff := now.Add(-1 * time.Minute).UnixNano() / int64(time.Millisecond)
	for len(m.latencies) > 0 {
		if len(m.latencies) > 5000 {
			m.latencies = m.latencies[len(m.latencies)-5000:]
			break
		}
		break
	}
	_ = cutoff

	m.totalCounts = append(m.totalCounts, 1)
	errCount := 0
	if isError {
		errCount = 1
	}
	m.errorCounts = append(m.errorCounts, errCount)

	if len(m.totalCounts) > 1000 {
		m.totalCounts = m.totalCounts[len(m.totalCounts)-1000:]
		m.errorCounts = m.errorCounts[len(m.errorCounts)-1000:]
	}
}

func (m *AdaptiveManager) evaluate() {
	if !m.enabled {
		return
	}

	p99 := m.calculateP99()
	errorRate := m.calculateErrorRate()

	now := time.Now()

	coeffFromHeuristic := m.applyHeuristicRules(p99, errorRate, now)
	coeffFromPID := m.applyPID(p99)

	m.coefficient = (coeffFromHeuristic + coeffFromPID) / 2
	if m.coefficient < m.config.MinCoefficient {
		m.coefficient = m.config.MinCoefficient
	}
	if m.coefficient > m.config.MaxCoefficient {
		m.coefficient = m.config.MaxCoefficient
	}

	m.health = models.BackendHealth{
		P99LatencyMs: p99,
		ErrorRate:    errorRate,
		CurrentCoeff: m.coefficient,
		LastUpdated:  now,
		StableSince:  m.stableSince,
	}
}

func (m *AdaptiveManager) applyHeuristicRules(p99 int64, errorRate float64, now time.Time) float64 {
	needTighten := false

	if p99 > m.config.TargetP99LatencyMs {
		needTighten = true
	}
	if errorRate > m.config.ErrorRateThreshold {
		needTighten = true
	}

	if needTighten {
		m.stableSince = nil
		if now.Sub(m.lastTighten) >= 5*time.Second {
			m.coefficient *= m.config.TighteningRatio
			m.lastTighten = now
		}
		return m.coefficient
	}

	if m.stableSince == nil {
		m.stableSince = &now
	} else if now.Sub(*m.stableSince) >= time.Duration(m.config.StablePeriodSec)*time.Second {
		recoveryInterval := time.Duration(m.config.RecoveryIntervalSec) * time.Second
		if now.Sub(m.lastRecovery) >= recoveryInterval {
			m.coefficient += m.config.RecoveryStepPercent / 100
			if m.coefficient > m.baseCoeff {
				m.coefficient = m.baseCoeff
			}
			m.lastRecovery = now
		}
	}

	return m.coefficient
}

func (m *AdaptiveManager) applyPID(p99 int64) float64 {
	if m.config.PIDConfig.Setpoint == 0 {
		return m.coefficient
	}

	pidOutput := m.pid.Compute(float64(p99))
	normalized := 1.0 + pidOutput/100.0
	return normalized
}

func (m *AdaptiveManager) calculateP99() int64 {
	if len(m.latencies) == 0 {
		return 0
	}

	sorted := make([]int64, len(m.latencies))
	copy(sorted, m.latencies)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j-1] > sorted[j]; j-- {
			sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
		}
	}

	idx := int(float64(len(sorted)-1) * 0.99)
	return sorted[idx]
}

func (m *AdaptiveManager) calculateErrorRate() float64 {
	total := 0
	errs := 0
	for i := range m.totalCounts {
		total += m.totalCounts[i]
		if i < len(m.errorCounts) {
			errs += m.errorCounts[i]
		}
	}
	if total == 0 {
		return 0
	}
	return float64(errs) / float64(total)
}

func (m *AdaptiveManager) GetCoefficient() float64 {
	m.RLock()
	defer m.RUnlock()
	m.evaluate()
	return m.coefficient
}

func (m *AdaptiveManager) GetHealth() models.BackendHealth {
	m.RLock()
	defer m.RUnlock()
	m.evaluate()
	return m.health
}

func (m *AdaptiveManager) SetEnabled(enabled bool) {
	m.Lock()
	defer m.Unlock()
	m.enabled = enabled
}

func (m *AdaptiveManager) OverrideCoefficient(coeff float64) {
	m.Lock()
	defer m.Unlock()
	if coeff < m.config.MinCoefficient {
		coeff = m.config.MinCoefficient
	}
	if coeff > m.config.MaxCoefficient {
		coeff = m.config.MaxCoefficient
	}
	m.coefficient = coeff
}

func (m *AdaptiveManager) UpdateConfig(cfg models.AdaptiveConfig) {
	m.Lock()
	defer m.Unlock()
	m.config = cfg
	m.enabled = cfg.Enabled
	m.pid.UpdateConfig(cfg.PIDConfig)
}

func (m *AdaptiveManager) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.Lock()
				m.evaluate()
				m.Unlock()
			}
		}
	}()
}
