package service

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

type priorityLatencySignalReaderStub struct {
	UsageLogRepository
	signals map[int64]PrioritySchedulingSignal
}

func (s *priorityLatencySignalReaderStub) ReadPrioritySchedulingSignals(context.Context, PrioritySchedulingQuery) (map[int64]PrioritySchedulingSignal, error) {
	return s.signals, nil
}

type priorityLatencyRefreshReader struct {
	UsageLogRepository
	started chan struct{}
	release chan struct{}
	signals map[int64]PrioritySchedulingSignal
}

func (s *priorityLatencyRefreshReader) ReadPrioritySchedulingSignals(context.Context, PrioritySchedulingQuery) (map[int64]PrioritySchedulingSignal, error) {
	close(s.started)
	<-s.release
	return s.signals, nil
}

type priorityHistoryReaderFunc struct {
	UsageLogRepository
	read func(context.Context, PrioritySchedulingQuery) (map[int64]PrioritySchedulingSignal, error)
}

func (f priorityHistoryReaderFunc) ReadPrioritySchedulingSignals(ctx context.Context, query PrioritySchedulingQuery) (map[int64]PrioritySchedulingSignal, error) {
	return f.read(ctx, query)
}

func priorityLatencyAdmissionConfig(mode string) PrioritySchedulingConfig {
	cfg := DefaultPrioritySchedulingConfig()
	cfg.Enabled = true
	cfg.LatencyPenaltyMode = mode
	cfg.Models = []string{"gpt-test"}
	cfg.MinSamples = 1
	cfg.TargetTTFTMs = 1000
	return cfg
}

func priorityLatencyAdmissionAccount() *Account {
	return &Account{ID: 42, Name: "priority-test", Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
}

func waitPriorityLatencyAdmission(t *testing.T, svc *OpenAIGatewayService, req OpenAIAccountScheduleRequest, account *Account, cfg PrioritySchedulingConfig) error {
	t.Helper()
	_ = svc.priorityLatencyAdmissionWithConfig(req, account, cfg)
	// The first call starts the bounded stale-while-revalidate read.
	time.Sleep(20 * time.Millisecond)
	return svc.priorityLatencyAdmissionWithConfig(req, account, cfg)
}

func TestPriorityLatencyAdmissionStopsConfirmedSlowAccount(t *testing.T) {
	svc := &OpenAIGatewayService{usageLogRepo: &priorityLatencySignalReaderStub{
		signals: map[int64]PrioritySchedulingSignal{42: {Samples: 3, P90TTFTMs: 2500}},
	}}
	req := OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequestedModel: "gpt-test", UseUpstreamTokenCost: true}
	err := waitPriorityLatencyAdmission(t, svc, req, priorityLatencyAdmissionAccount(), priorityLatencyAdmissionConfig(PriorityLatencyStop))
	if !errors.Is(err, ErrPriorityLatencyUnavailable) {
		t.Fatalf("expected local latency admission denial, got %v", err)
	}
	var denied *OpenAITurnAdmissionError
	if !errors.As(err, &denied) || denied.Reason != "latency_penalty" {
		t.Fatalf("expected latency_penalty admission error, got %v", err)
	}
}

func TestPriorityLatencyAdmissionRejectsUnknownHistoryInStopMode(t *testing.T) {
	svc := &OpenAIGatewayService{usageLogRepo: &priorityLatencySignalReaderStub{signals: map[int64]PrioritySchedulingSignal{}}}
	req := OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequestedModel: "gpt-test", UseUpstreamTokenCost: true}
	err := waitPriorityLatencyAdmission(t, svc, req, priorityLatencyAdmissionAccount(), priorityLatencyAdmissionConfig(PriorityLatencyStop))
	if !errors.Is(err, ErrPriorityLatencyUnavailable) {
		t.Fatalf("expected retryable unknown-history denial, got %v", err)
	}
	var denied *OpenAITurnAdmissionError
	if !errors.As(err, &denied) || denied.Reason != "latency_history_unavailable" {
		t.Fatalf("expected latency_history_unavailable admission error, got %v", err)
	}
}

func TestPriorityLatencyAdmissionDeprioritizeDoesNotReject(t *testing.T) {
	svc := &OpenAIGatewayService{}
	req := OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequestedModel: "gpt-test", UseUpstreamTokenCost: true}
	err := svc.priorityLatencyAdmissionWithConfig(req, priorityLatencyAdmissionAccount(), priorityLatencyAdmissionConfig(PriorityLatencyDeprioritize))
	if err != nil {
		t.Fatalf("deprioritize mode must not be a hard admission gate: %v", err)
	}
}

func TestPriorityLatencyAdmissionDoesNotUseExpiredStopSignalDuringRefresh(t *testing.T) {
	req := OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequestedModel: "gpt-test", UseUpstreamTokenCost: true}
	cfg := priorityLatencyAdmissionConfig(PriorityLatencyStop)
	reader := &priorityLatencyRefreshReader{
		started: make(chan struct{}),
		release: make(chan struct{}),
		signals: map[int64]PrioritySchedulingSignal{42: {Samples: 3, P90TTFTMs: 2500}},
	}
	svc := &OpenAIGatewayService{usageLogRepo: reader}
	key := prioritySignalCacheKey(req, cfg, []int64{42})
	svc.priorityScheduling.entries = map[string]*prioritySignalEntry{
		key: {
			accountIDs: []int64{42},
			signals:    map[int64]PrioritySchedulingSignal{42: {Samples: 3, P90TTFTMs: 500}},
			observedAt: time.Now().Add(-31 * time.Second),
			expiresAt:  time.Now().Add(-time.Second),
		},
	}

	admitted := make(chan error, 1)
	go func() {
		admitted <- svc.priorityLatencyAdmissionWithConfig(req, priorityLatencyAdmissionAccount(), cfg)
	}()
	select {
	case <-reader.started:
	case <-time.After(time.Second):
		t.Fatal("expired signal refresh did not start")
	}
	select {
	case err := <-admitted:
		var denied *OpenAITurnAdmissionError
		if !errors.As(err, &denied) || denied.Reason != "latency_history_unavailable" {
			t.Fatalf("expired signal must fail closed while refresh is pending, got %v", err)
		}
	case <-time.After(20 * time.Millisecond):
		t.Fatal("admission waited on an unbounded history refresh")
	}
	close(reader.release)
}

func TestPriorityHistoryRefreshRejectsOversizedScopeAndAccountSet(t *testing.T) {
	var calls atomic.Int32
	svc := &OpenAIGatewayService{usageLogRepo: priorityHistoryReaderFunc{read: func(context.Context, PrioritySchedulingQuery) (map[int64]PrioritySchedulingSignal, error) {
		calls.Add(1)
		return map[int64]PrioritySchedulingSignal{}, nil
	}}}
	cfg := DefaultPrioritySchedulingConfig()
	items := make([]openAIAccountCandidateScore, priorityHistoryMaxAccounts+1)
	for i := range items {
		items[i].account = &Account{ID: int64(i + 1)}
	}
	if signals, ready := svc.prioritySignals(OpenAIAccountScheduleRequest{RequestedModel: "oversized"}, cfg, items); ready || signals != nil {
		t.Fatalf("oversized account query must remain unavailable: ready=%v signals=%v", ready, signals)
	}
	if calls.Load() != 0 || len(svc.priorityScheduling.entries) != 0 {
		t.Fatalf("oversized account query must not start SQL or allocate a cache scope: calls=%d scopes=%d", calls.Load(), len(svc.priorityScheduling.entries))
	}
	for i := 0; i < priorityHistoryMaxScopes; i++ {
		req := OpenAIAccountScheduleRequest{RequestedModel: fmt.Sprintf("model-%d", i)}
		svc.prioritySignals(req, cfg, []openAIAccountCandidateScore{{account: &Account{ID: int64(i + 1)}}})
	}
	before := len(svc.priorityScheduling.entries)
	svc.prioritySignals(OpenAIAccountScheduleRequest{RequestedModel: "over-scope"}, cfg, []openAIAccountCandidateScore{{account: &Account{ID: 9999}}})
	if before != priorityHistoryMaxScopes || len(svc.priorityScheduling.entries) != priorityHistoryMaxScopes {
		t.Fatalf("history scope cache must remain bounded at %d with LRU replacement: before=%d after=%d", priorityHistoryMaxScopes, before, len(svc.priorityScheduling.entries))
	}
}

func TestPriorityHistoryRefreshBudgetsLaunchesAndActiveReaders(t *testing.T) {
	started := make(chan struct{}, 8)
	release := make(chan struct{})
	svc := &OpenAIGatewayService{usageLogRepo: priorityHistoryReaderFunc{read: func(ctx context.Context, _ PrioritySchedulingQuery) (map[int64]PrioritySchedulingSignal, error) {
		started <- struct{}{}
		select {
		case <-release:
			return map[int64]PrioritySchedulingSignal{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}}
	cfg := DefaultPrioritySchedulingConfig()
	for i := 0; i < priorityHistoryMaxActive; i++ {
		svc.prioritySignals(OpenAIAccountScheduleRequest{RequestedModel: fmt.Sprintf("active-%d", i)}, cfg, []openAIAccountCandidateScore{{account: &Account{ID: int64(i + 1)}}})
	}
	for i := 0; i < priorityHistoryMaxActive; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("expected active history query to start")
		}
	}
	svc.prioritySignals(OpenAIAccountScheduleRequest{RequestedModel: "active-overflow"}, cfg, []openAIAccountCandidateScore{{account: &Account{ID: 99}}})
	select {
	case <-started:
		t.Fatal("history refresh exceeded active reader budget")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
}

func TestPriorityHistoryRefreshAllowsTwoLaunchBurstPerSecond(t *testing.T) {
	state := &prioritySchedulingState{}
	base := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		if !state.canStartRefresh(base) {
			t.Fatalf("launch %d should fit the two-start burst", i+1)
		}
		state.launches[state.launchNext] = base
		state.launchNext = (state.launchNext + 1) % len(state.launches)
	}
	if state.canStartRefresh(base.Add(500 * time.Millisecond)) {
		t.Fatal("third launch inside one second must be rejected")
	}
	if !state.canStartRefresh(base.Add(time.Second)) {
		t.Fatal("a new launch should be allowed after the rolling one-second window")
	}
}

func TestPriorityHistoryRefreshUsesColdTTLAndKeepsRecentSignalsOnFailure(t *testing.T) {
	var calls atomic.Int32
	reader := priorityHistoryReaderFunc{read: func(context.Context, PrioritySchedulingQuery) (map[int64]PrioritySchedulingSignal, error) {
		if calls.Add(1) == 1 {
			return map[int64]PrioritySchedulingSignal{7: {Samples: 1, P90TTFTMs: 500}}, nil
		}
		return nil, errors.New("temporary read failure")
	}}
	svc := &OpenAIGatewayService{usageLogRepo: reader}
	cfg := DefaultPrioritySchedulingConfig()
	req := OpenAIAccountScheduleRequest{RequestedModel: "cold-scope"}
	items := []openAIAccountCandidateScore{{account: &Account{ID: 7}}}
	svc.prioritySignals(req, cfg, items)
	key := prioritySignalCacheKey(req, cfg, []int64{7})
	waitForPriorityEntry(t, svc, key, func(entry *prioritySignalEntry) bool { return !entry.observedAt.IsZero() })
	entry := svc.priorityScheduling.entries[key]
	if ttl := entry.expiresAt.Sub(entry.observedAt); ttl <= 0 || ttl > priorityHistoryColdTTL {
		t.Fatalf("cold history should refresh after at most %v, got %v", priorityHistoryColdTTL, ttl)
	}
	entry.expiresAt = time.Now().Add(-time.Second)
	before := entry.signals[7]
	if signals, ready := svc.prioritySignals(req, cfg, items); !ready || signals[7] != before {
		t.Fatalf("recent cached evidence should remain available during refresh: ready=%v signals=%v", ready, signals)
	}
	waitForPriorityEntry(t, svc, key, func(entry *prioritySignalEntry) bool { return !entry.refreshing && calls.Load() == 2 })
	entry = svc.priorityScheduling.entries[key]
	if entry.signals[7] != before || entry.observedAt.IsZero() {
		t.Fatalf("failed refresh must preserve recent signal evidence: %+v", entry)
	}
	if retry := entry.expiresAt.Sub(time.Now()); retry <= 0 || retry > priorityHistoryRetry {
		t.Fatalf("failed refresh retry must be bounded by %v, got %v", priorityHistoryRetry, retry)
	}
}

func TestPriorityHistorySignalsExpireAfterMaximumAge(t *testing.T) {
	cfg := priorityLatencyAdmissionConfig(PriorityLatencyStop)
	req := OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequestedModel: "gpt-test", UseUpstreamTokenCost: true}
	account := priorityLatencyAdmissionAccount()
	svc := &OpenAIGatewayService{usageLogRepo: priorityLatencySignalReaderStub{signals: map[int64]PrioritySchedulingSignal{42: {Samples: 3, P90TTFTMs: 500}}}}
	key := prioritySignalCacheKey(req, cfg, []int64{42})
	svc.priorityScheduling.entries = map[string]*prioritySignalEntry{key: {
		accountIDs: []int64{42}, signals: map[int64]PrioritySchedulingSignal{42: {Samples: 3, P90TTFTMs: 500}},
		observedAt: time.Now().Add(-priorityHistoryMaxAge - time.Second), expiresAt: time.Now().Add(time.Hour),
	}}
	if err := svc.priorityLatencyAdmissionWithConfig(req, account, cfg); err == nil {
		t.Fatal("stop admission must not trust evidence older than the maximum history age")
	}
}

func waitForPriorityEntry(t *testing.T, svc *OpenAIGatewayService, key string, ready func(*prioritySignalEntry) bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		svc.priorityScheduling.mu.Lock()
		entry := svc.priorityScheduling.entries[key]
		ok := entry != nil && ready(entry)
		svc.priorityScheduling.mu.Unlock()
		if ok {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("priority history entry did not reach expected state")
}
