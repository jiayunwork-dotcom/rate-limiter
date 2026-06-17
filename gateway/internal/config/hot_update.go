package config

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/ratelimiter/gateway/pkg/models"
)

type RuleChangeType string

const (
	RuleChangeUpsert RuleChangeType = "upsert"
	RuleChangeDelete RuleChangeType = "delete"
	RuleChangeToggle RuleChangeType = "toggle"
	RuleChangeRollback RuleChangeType = "rollback"
	ChannelRuleUpdates = "rl:rule:updates"
)

type RuleChangeMessage struct {
	Type       RuleChangeType      `json:"type"`
	Rule       *models.RuleConfig  `json:"rule,omitempty"`
	RuleID     string              `json:"rule_id,omitempty"`
	Enabled    bool                `json:"enabled,omitempty"`
	Version    int64               `json:"version,omitempty"`
	Timestamp  time.Time           `json:"timestamp"`
	GrayRatio  float64             `json:"gray_ratio,omitempty"`
}

type RuleVersionSnapshot struct {
	Version int64
	Rule    *models.RuleConfig
	SavedAt time.Time
}

type Store struct {
	sync.RWMutex
	rules          map[string]*models.RuleConfig
	ruleVersions   map[string][]*RuleVersionSnapshot
	grayRules      map[string]float64
	publisher      func(ctx context.Context, channel string, msg interface{}) error
	subscriber     func(ctx context.Context, channel string) (<-chan *RuleChangeMessage, error)
	nodeID         string
}

func NewStore(nodeID string) *Store {
	return &Store{
		rules:        make(map[string]*models.RuleConfig),
		ruleVersions: make(map[string][]*RuleVersionSnapshot),
		grayRules:    make(map[string]float64),
		nodeID:       nodeID,
	}
}

func (s *Store) SetPublisher(pub func(ctx context.Context, channel string, msg interface{}) error) {
	s.publisher = pub
}

func (s *Store) SetSubscriber(sub func(ctx context.Context, channel string) (<-chan *RuleChangeMessage, error)) {
	s.subscriber = sub
}

func (s *Store) GetRules() []*models.RuleConfig {
	s.RLock()
	defer s.RUnlock()
	result := make([]*models.RuleConfig, 0, len(s.rules))
	for _, r := range s.rules {
		if s.shouldApplyRule(r) {
			result = append(result, r)
		}
	}
	return result
}

func (s *Store) GetRule(id string) *models.RuleConfig {
	s.RLock()
	defer s.RUnlock()
	r, ok := s.rules[id]
	if !ok {
		return nil
	}
	if !s.shouldApplyRule(r) {
		return nil
	}
	return r
}

func (s *Store) UpsertRule(ctx context.Context, rule *models.RuleConfig) error {
	s.Lock()
	rule.UpdatedAt = time.Now()
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = rule.UpdatedAt
	}
	if rule.Version == 0 {
		rule.Version = 1
		if existing, ok := s.rules[rule.ID]; ok {
			rule.Version = existing.Version + 1
		}
	}

	s.rules[rule.ID] = rule
	s.ruleVersions[rule.ID] = append(s.ruleVersions[rule.ID], &RuleVersionSnapshot{
		Version: rule.Version,
		Rule:    copyRule(rule),
		SavedAt: time.Now(),
	})
	if len(s.ruleVersions[rule.ID]) > 50 {
		s.ruleVersions[rule.ID] = s.ruleVersions[rule.ID][len(s.ruleVersions[rule.ID])-50:]
	}

	isGray := rule.GrayReleaseConfig != nil && rule.GrayReleaseConfig.Enabled
	if isGray {
		s.grayRules[rule.ID] = rule.GrayReleaseConfig.TrafficRatio
	} else {
		delete(s.grayRules, rule.ID)
	}
	s.Unlock()

	if s.publisher != nil {
		msg := &RuleChangeMessage{
			Type:      RuleChangeUpsert,
			Rule:      copyRule(rule),
			Timestamp: time.Now(),
		}
		if isGray {
			msg.GrayRatio = rule.GrayReleaseConfig.TrafficRatio
		}
		if err := s.publisher(ctx, ChannelRuleUpdates, msg); err != nil {
			return fmt.Errorf("publish rule update: %w", err)
		}
	}
	return nil
}

func (s *Store) DeleteRule(ctx context.Context, id string) error {
	s.Lock()
	delete(s.rules, id)
	delete(s.grayRules, id)
	s.Unlock()

	if s.publisher != nil {
		msg := &RuleChangeMessage{
			Type:      RuleChangeDelete,
			RuleID:    id,
			Timestamp: time.Now(),
		}
		if err := s.publisher(ctx, ChannelRuleUpdates, msg); err != nil {
			return fmt.Errorf("publish rule delete: %w", err)
		}
	}
	return nil
}

func (s *Store) ToggleRule(ctx context.Context, id string, enabled bool) error {
	s.Lock()
	rule, ok := s.rules[id]
	if !ok {
		s.Unlock()
		return fmt.Errorf("rule %s not found", id)
	}
	rule.Enabled = enabled
	rule.Version++
	rule.UpdatedAt = time.Now()
	s.Unlock()

	if s.publisher != nil {
		msg := &RuleChangeMessage{
			Type:      RuleChangeToggle,
			RuleID:    id,
			Enabled:   enabled,
			Version:   rule.Version,
			Timestamp: time.Now(),
		}
		if err := s.publisher(ctx, ChannelRuleUpdates, msg); err != nil {
			return fmt.Errorf("publish rule toggle: %w", err)
		}
	}
	return nil
}

func (s *Store) RollbackRule(ctx context.Context, id string, version int64) error {
	s.RLock()
	versions, ok := s.ruleVersions[id]
	s.RUnlock()
	if !ok {
		return fmt.Errorf("rule %s versions not found", id)
	}

	var target *RuleVersionSnapshot
	for _, v := range versions {
		if v.Version == version {
			target = v
			break
		}
	}
	if target == nil {
		return fmt.Errorf("version %d not found for rule %s", version, id)
	}

	rule := copyRule(target.Rule)
	rule.Version++
	rule.UpdatedAt = time.Now()

	s.Lock()
	s.rules[id] = rule
	s.ruleVersions[id] = append(s.ruleVersions[id], &RuleVersionSnapshot{
		Version: rule.Version,
		Rule:    copyRule(rule),
		SavedAt: time.Now(),
	})
	s.Unlock()

	if s.publisher != nil {
		msg := &RuleChangeMessage{
			Type:      RuleChangeRollback,
			Rule:      copyRule(rule),
			Version:   version,
			Timestamp: time.Now(),
		}
		if err := s.publisher(ctx, ChannelRuleUpdates, msg); err != nil {
			return fmt.Errorf("publish rule rollback: %w", err)
		}
	}
	return nil
}

func (s *Store) GetRuleVersions(id string) []*RuleVersionSnapshot {
	s.RLock()
	defer s.RUnlock()
	return s.ruleVersions[id]
}

func (s *Store) shouldApplyRule(rule *models.RuleConfig) bool {
	cfg := rule.GrayReleaseConfig
	if cfg == nil || !cfg.Enabled {
		return true
	}

	s.RLock()
	ratio, ok := s.grayRules[rule.ID]
	s.RUnlock()
	if !ok {
		ratio = cfg.TrafficRatio
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return r.Float64() < ratio
}

func (s *Store) StartSubscriber(ctx context.Context) error {
	if s.subscriber == nil {
		return nil
	}

	ch, err := s.subscriber(ctx, ChannelRuleUpdates)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				s.applyChange(msg)
			}
		}
	}()
	return nil
}

func (s *Store) applyChange(msg *RuleChangeMessage) {
	switch msg.Type {
	case RuleChangeUpsert, RuleChangeRollback:
		if msg.Rule != nil {
			s.Lock()
			s.rules[msg.Rule.ID] = msg.Rule
			if msg.Rule.GrayReleaseConfig != nil && msg.Rule.GrayReleaseConfig.Enabled {
				s.grayRules[msg.Rule.ID] = msg.GrayRatio
			} else {
				delete(s.grayRules, msg.Rule.ID)
			}
			s.Unlock()
		}
	case RuleChangeDelete:
		s.Lock()
		delete(s.rules, msg.RuleID)
		delete(s.grayRules, msg.RuleID)
		s.Unlock()
	case RuleChangeToggle:
		s.Lock()
		if rule, ok := s.rules[msg.RuleID]; ok {
			rule.Enabled = msg.Enabled
		}
		s.Unlock()
	}
}

func (s *Store) BulkLoad(rules []*models.RuleConfig) {
	s.Lock()
	defer s.Unlock()
	for _, r := range rules {
		s.rules[r.ID] = r
		if r.GrayReleaseConfig != nil && r.GrayReleaseConfig.Enabled {
			s.grayRules[r.ID] = r.GrayReleaseConfig.TrafficRatio
		}
	}
}

func copyRule(r *models.RuleConfig) *models.RuleConfig {
	data, _ := json.Marshal(r)
	var copy models.RuleConfig
	json.Unmarshal(data, &copy)
	return &copy
}

type QuotaChangeType string

const (
	QuotaChangeUpsert QuotaChangeType = "upsert"
	QuotaChangeDelete QuotaChangeType = "delete"
	ChannelQuotaUpdates = "rl:quota:updates"
)

type QuotaChangeMessage struct {
	Type      QuotaChangeType     `json:"type"`
	Quota     *models.QuotaConfig `json:"quota,omitempty"`
	Level     models.QuotaLevel   `json:"level"`
	Identifier string             `json:"identifier"`
	Timestamp time.Time           `json:"timestamp"`
}

type QuotaStore struct {
	sync.RWMutex
	quotas    map[string]*models.QuotaConfig
	publisher func(ctx context.Context, channel string, msg interface{}) error
}

func NewQuotaStore() *QuotaStore {
	return &QuotaStore{
		quotas: make(map[string]*models.QuotaConfig),
	}
}

func (qs *QuotaStore) SetPublisher(pub func(ctx context.Context, channel string, msg interface{}) error) {
	qs.publisher = pub
}

func (qs *QuotaStore) Upsert(ctx context.Context, quota *models.QuotaConfig) error {
	key := string(quota.Level) + ":" + quota.Identifier
	qs.Lock()
	qs.quotas[key] = quota
	qs.Unlock()

	if qs.publisher != nil {
		msg := &QuotaChangeMessage{
			Type:      QuotaChangeUpsert,
			Quota:     quota,
			Level:     quota.Level,
			Identifier: quota.Identifier,
			Timestamp: time.Now(),
		}
		return qs.publisher(ctx, ChannelQuotaUpdates, msg)
	}
	return nil
}

func (qs *QuotaStore) Get(level models.QuotaLevel, identifier string) *models.QuotaConfig {
	qs.RLock()
	defer qs.RUnlock()
	return qs.quotas[string(level)+":"+identifier]
}

func (qs *QuotaStore) GetAll() []*models.QuotaConfig {
	qs.RLock()
	defer qs.RUnlock()
	result := make([]*models.QuotaConfig, 0, len(qs.quotas))
	for _, q := range qs.quotas {
		result = append(result, q)
	}
	return result
}
