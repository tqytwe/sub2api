package service

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
)

// OAuthAutomationService owns only account policy automation. Notification
// delivery and provider credential operations remain in their own services.
type OAuthAutomationService struct {
	settings           SettingRepository
	accounts           AccountRepository
	autoGroups         GroupRepository
	autoAccounts       AccountConcurrencyRepository
	autoConfigMu       sync.Mutex
	autoConfig         atomic.Value
	autoBlocked        atomic.Bool
	autoResults        chan AccountConcurrencyResult
	autoSeen           map[int64]bool
	degradationCursor  int64
	fallbackSharedJobs atomic.Bool
	fallbackSLAReader  atomic.Value
	fallbackStatus     atomic.Value
	cancel             context.CancelFunc
	wg                 sync.WaitGroup
	lifecycle          sync.Mutex
}

func NewOAuthAutomationService(settings SettingRepository, accounts AccountRepository, groups GroupRepository) *OAuthAutomationService {
	s := &OAuthAutomationService{settings: settings, accounts: accounts, autoGroups: groups, autoResults: make(chan AccountConcurrencyResult, 1024), autoSeen: make(map[int64]bool)}
	s.autoAccounts, _ = accounts.(AccountConcurrencyRepository)
	s.autoConfig.Store(DefaultOAuthAutoConfig())
	return s
}

func (s *OAuthAutomationService) Start() {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.fallbackSharedJobs.Store(true)
	s.wg.Add(1)
	go s.runAutoConfig(ctx)
}
func (s *OAuthAutomationService) Stop() {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.wg.Wait()
		s.cancel = nil
	}
	s.fallbackSharedJobs.Store(false)
}
func (s *OAuthAutomationService) AttachOps(ops *OpsService, settings *SettingService) {
	s.setOpenAIAPIKeyFallbackSLAReader(ops, settings)
	ops.SetAutoConfigObserver(s.ObserveConcurrencyResult)
}

// History contains selected policy values, never prompts, answers, judge
// credentials or provider output. The live configuration retains its settings.
func OAuthAutoConfigHistorySnapshot(c OAuthAutoConfig) *OAuthAutoConfig {
	raw, _ := json.Marshal(c)
	var out OAuthAutoConfig
	_ = json.Unmarshal(raw, &out)
	out.QualityRule = nil
	return &out
}
