package service

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/qualityqueue"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/stretchr/testify/require"
)

type qualityRuntimeQueue struct {
	score                  time.Time
	enqueued, acknowledged int
	err                    error
}

func (q *qualityRuntimeQueue) Enqueue(_ context.Context, _ int64, at time.Time) error {
	q.enqueued++
	q.score = at
	return q.err
}
func (q *qualityRuntimeQueue) Score(context.Context, int64) (time.Time, error) { return q.score, q.err }
func (q *qualityRuntimeQueue) Pending(context.Context, time.Time) ([]qualityqueue.Signal, error) {
	return nil, q.err
}
func (q *qualityRuntimeQueue) Acknowledge(_ context.Context, signal qualityqueue.Signal) error {
	if q.score.Equal(signal.ObservedAt) {
		q.acknowledged++
	}
	return q.err
}
func (*qualityRuntimeQueue) Close() error { return nil }

type qualityRuntimeEventPlans struct {
	qualityRuntimeQuotaPlans
	plan              *ScheduledTestPlan
	lostClaim         bool
	coveredAfterClaim bool
	finishErr         error
	started           time.Time
}

func (r *qualityRuntimeEventPlans) ListByAccountID(context.Context, int64) ([]*ScheduledTestPlan, error) {
	return []*ScheduledTestPlan{r.plan}, nil
}
func (r *qualityRuntimeEventPlans) GetByID(context.Context, int64) (*ScheduledTestPlan, error) {
	return r.plan, nil
}
func (r *qualityRuntimeEventPlans) ClaimQualityRun(_ context.Context, p *ScheduledTestPlan, _ time.Time) (time.Time, bool, error) {
	if r.coveredAfterClaim {
		now := time.Now()
		r.plan.LastRunAt = &now
	}
	return p.UpdatedAt, !r.lostClaim, nil
}
func (r *qualityRuntimeEventPlans) FinishQualityPlan(_ context.Context, _ *ScheduledTestPlan, started, next time.Time) error {
	if r.finishErr != nil {
		return r.finishErr
	}
	r.finishes++
	r.started = started
	r.plan.LastRunAt = &started
	return nil
}

func TestQuality5xxObservesOAuthFailuresWithoutProbeRecursion(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		kind   string
		probe  bool
		want   int
	}{
		{"oauth 503", 503, AccountTypeOAuth, false, 1},
		{"oauth 401 unchanged", 401, AccountTypeOAuth, false, 0},
		{"oauth 429", 429, AccountTypeOAuth, false, 0},
		{"api key", 503, AccountTypeAPIKey, false, 0},
		{"quality probe", 503, AccountTypeOAuth, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := &qualityRuntimeQueue{}
			svc := &RateLimitService{qualityTrigger: &quality5xxTrigger{queue: q}}
			ctx := context.Background()
			if tc.probe {
				ctx = context.WithValue(ctx, qualityIsolationProbeKey{}, true)
			}
			svc.observeQualityResponse(ctx, &Account{ID: 7, Type: tc.kind}, &http.Response{StatusCode: tc.status}, nil)
			require.Equal(t, tc.want, q.enqueued)
		})
	}
}

func TestQuality5xxOpenAIRawAndSemanticResponsesReachQueue(t *testing.T) {
	q := &qualityRuntimeQueue{}
	limiter := &RateLimitService{qualityTrigger: &quality5xxTrigger{queue: q}}
	upstream := &geminiCompatHTTPUpstreamStub{response: &http.Response{StatusCode: http.StatusServiceUnavailable}}
	gateway := &OpenAIGatewayService{rateLimitService: limiter, httpUpstream: upstream}
	account := &Account{ID: 7, Type: AccountTypeOAuth, Platform: PlatformOpenAI}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://upstream.invalid/v1/responses", nil)
	require.NoError(t, err)
	_, err = gateway.doOpenAIUpstream(req, "", account)
	require.NoError(t, err)
	require.Equal(t, 1, q.enqueued)
	payload := []byte(`{"type":"response.failed","response":{"error":{"code":"server_error","message":"upstream error"}}}`)
	status, _ := gateway.handleOpenAIStreamTerminalAccountSideEffects(nil, account, payload, "upstream error", nil)
	require.Equal(t, http.StatusBadGateway, status)
	require.Equal(t, 2, q.enqueued, "an HTTP 200 stream's semantic 5xx must also queue a test")
	probeReq := req.WithContext(context.WithValue(req.Context(), qualityIsolationProbeKey{}, true))
	_, err = gateway.doOpenAIUpstream(probeReq, "", account)
	require.NoError(t, err)
	require.Equal(t, 2, q.enqueued)
}

func TestQuality5xxOpenAIRawResponseAndFastpathRecordOncePerRetry(t *testing.T) {
	q := &qualityRuntimeQueue{}
	limiter := &RateLimitService{qualityTrigger: &quality5xxTrigger{queue: q}}
	upstream := &geminiCompatHTTPUpstreamStub{response: &http.Response{StatusCode: http.StatusServiceUnavailable}}
	gateway := &OpenAIGatewayService{rateLimitService: limiter, httpUpstream: upstream}
	account := &Account{ID: 7, Type: AccountTypeOAuth, Platform: PlatformOpenAI}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://upstream.invalid/v1/responses", nil)
	require.NoError(t, err)
	for attempt := 1; attempt <= 2; attempt++ {
		response, err := gateway.doOpenAIUpstream(req, "", account)
		require.NoError(t, err)
		gateway.handleOpenAIAccountUpstreamError(req.Context(), account, response.StatusCode, response.Header, []byte(`{"error":{"code":"server_is_overloaded","message":"server is overloaded"}}`), "a")
		require.Equal(t, attempt, q.enqueued, "each independent retry response must produce one signal, without repeated status-handler observations")
	}
}

func TestQuality5xxImagesSeparatesProviderSemanticFailureFromSynthesizedVerdict(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		synthesized bool
		probe       bool
		want        int
	}{
		{"provider semantic 5xx", http.StatusOK, false, false, 1},
		{"model text inferred 5xx", http.StatusOK, true, false, 0},
		{"raw response already observed", http.StatusServiceUnavailable, false, false, 1},
		{"quality probe", http.StatusOK, false, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := &qualityRuntimeQueue{}
			limiter := &RateLimitService{qualityTrigger: &quality5xxTrigger{queue: q}}
			gateway := &OpenAIGatewayService{rateLimitService: limiter}
			account := &Account{ID: 7, Type: AccountTypeOAuth, Platform: PlatformOpenAI}
			ctx := context.Background()
			if tc.probe {
				ctx = context.WithValue(ctx, qualityIsolationProbeKey{}, true)
			}
			response := &http.Response{StatusCode: tc.status}
			limiter.observeQualityResponse(ctx, account, response, nil)
			failure := &OpenAIImagesUpstreamError{StatusCode: http.StatusServiceUnavailable, Code: "image_generation_unavailable", SynthesizedFromModelText: tc.synthesized}
			_ = gateway.handleOpenAIImagesOAuthResponseError(requestledger.Receipt{}, ctx, nil, account, "gpt-image-2", "", response, 0, failure)
			require.Equal(t, tc.want, q.enqueued)
		})
	}
}

func TestQuality5xxAcknowledgesOnlyCommittedFinishedOrCoveredSignals(t *testing.T) {
	for _, scenario := range []string{"success", "commit failure", "finish failure", "lost claim", "covered after lost claim", "newer signal during probe"} {
		t.Run(scenario, func(t *testing.T) {
			observed := time.Now().Add(-time.Second)
			q := &qualityRuntimeQueue{score: observed}
			plan := qualityProbeRunnerPlan([]string{"a"}, 1)
			plan.PelicanConfig.Quality.TriggerOnUpstream5xx = true
			repo := &qualityRuntimeEventPlans{plan: plan}
			switch scenario {
			case "commit failure":
				repo.commitError = errors.New("commit failed")
			case "finish failure":
				repo.finishErr = errors.New("finish failed")
			case "lost claim":
				repo.lostClaim = true
			case "covered after lost claim":
				repo.lostClaim = true
				repo.coveredAfterClaim = true
			}
			runner := &ScheduledTestRunnerService{planRepo: repo, scheduledSvc: NewScheduledTestService(repo, nil), qualityTrigger: &quality5xxTrigger{queue: q}}
			runner.runQualityProbe = func(ctx context.Context, _ int64, _, _ string) (*ScheduledTestResult, error) {
				require.NotNil(t, ctx.Value(qualityIsolationProbeKey{}))
				if scenario == "newer signal during probe" {
					q.score = time.Now().Add(time.Second)
				}
				return probeAnswer("42"), nil
			}
			before := time.Now()
			err := runner.processQualitySignal(context.Background(), qualityqueue.Signal{AccountID: 7, ObservedAt: observed})
			if scenario == "commit failure" || scenario == "finish failure" || scenario == "lost claim" {
				require.Error(t, err)
				require.Zero(t, q.acknowledged)
			} else {
				require.NoError(t, err)
				if scenario == "newer signal during probe" {
					require.Zero(t, q.acknowledged)
					require.False(t, plan.PelicanConfig.Quality.AutoRestore)
					require.Equal(t, "inconclusive", repo.outcome)
				} else {
					require.Equal(t, 1, q.acknowledged)
				}
			}
			if scenario == "success" {
				require.False(t, repo.started.Before(before))
				require.Equal(t, quality5xxSource, repo.results[0].PelicanConfig.TriggerSource)
			}
		})
	}
}

func TestQualityRuntimeManualAndEventQuotaBypass(t *testing.T) {
	for _, source := range []string{"manual", quality5xxSource} {
		t.Run(source, func(t *testing.T) {
			plan := qualityProbeRunnerPlan([]string{"a"}, 1)
			if source == "manual" {
				plan.ManualQualityRun = true
			} else {
				plan.TriggerSource = source
			}
			svc := NewScheduledTestService(&qualityRuntimeQuotaPlans{}, nil)
			svc.templateRepo = &qualityRuntimeQuotaTemplates{}
			runner := &ScheduledTestRunnerService{scheduledSvc: svc}
			available, err := runner.qualityTemplateQuotaAvailable(context.Background(), plan)
			require.NoError(t, err)
			require.True(t, available)
		})
	}
}

func TestQuality5xxWebSocketProbeKeepsContextInNilGinSideEffects(t *testing.T) {
	q := &qualityRuntimeQueue{}
	gateway := &OpenAIGatewayService{rateLimitService: &RateLimitService{qualityTrigger: &quality5xxTrigger{queue: q}}}
	account := &Account{ID: 7, Type: AccountTypeOAuth, Platform: PlatformOpenAI}
	ctx := context.WithValue(context.Background(), qualityIsolationProbeKey{}, true)
	payload := []byte(`{"type":"response.failed","response":{"error":{"status_code":529,"code":"server_is_overloaded","message":"server is overloaded"}}}`)
	gateway.handleOpenAIWSFailureAccountSideEffects(ctx, account, "a", nil, payload)
	require.Zero(t, q.enqueued, "nil Gin context must not strip the quality probe recursion guard")
}

func TestQuality5xxWebSocket529EnqueuesOneSignal(t *testing.T) {
	q := &qualityRuntimeQueue{}
	gateway := &OpenAIGatewayService{rateLimitService: &RateLimitService{qualityTrigger: &quality5xxTrigger{queue: q}}}
	account := &Account{ID: 7, Type: AccountTypeOAuth, Platform: PlatformOpenAI}
	payload := []byte(`{"type":"response.failed","response":{"error":{"status_code":529,"code":"server_is_overloaded","message":"server is overloaded"}}}`)
	gateway.handleOpenAIWSFailureAccountSideEffects(context.Background(), account, "a", nil, payload)
	require.Equal(t, 1, q.enqueued, "one real terminal response must not manufacture newer queue episodes")
}

type qualityRuntimeLifecycleQueue struct {
	signal qualityqueue.Signal
	acks   atomic.Int32
	closed chan struct{}
}

func (*qualityRuntimeLifecycleQueue) Enqueue(context.Context, int64, time.Time) error { return nil }
func (q *qualityRuntimeLifecycleQueue) Score(context.Context, int64) (time.Time, error) {
	return q.signal.ObservedAt, nil
}
func (q *qualityRuntimeLifecycleQueue) Pending(context.Context, time.Time) ([]qualityqueue.Signal, error) {
	return []qualityqueue.Signal{q.signal}, nil
}
func (q *qualityRuntimeLifecycleQueue) Acknowledge(context.Context, qualityqueue.Signal) error {
	q.acks.Add(1)
	return nil
}
func (q *qualityRuntimeLifecycleQueue) Close() error { close(q.closed); return nil }

func TestQuality5xxStopCancelsActiveProbeWithoutAcknowledgingSignal(t *testing.T) {
	q := &qualityRuntimeLifecycleQueue{signal: qualityqueue.Signal{AccountID: 7, ObservedAt: time.Now().Add(-time.Second)}, closed: make(chan struct{})}
	plan := qualityProbeRunnerPlan([]string{"a"}, 1)
	plan.PelicanConfig.Quality.TriggerOnUpstream5xx = true
	repo := &qualityRuntimeEventPlans{plan: plan}
	repo.commitError = errors.New("fixture persistence unavailable")
	runner := &ScheduledTestRunnerService{planRepo: repo, scheduledSvc: NewScheduledTestService(repo, nil), qualityTrigger: &quality5xxTrigger{queue: q}}
	started := make(chan struct{})
	canceled := make(chan struct{})
	runner.runQualityProbe = func(ctx context.Context, _ int64, _, _ string) (*ScheduledTestResult, error) {
		close(started)
		<-ctx.Done()
		close(canceled)
		return nil, ctx.Err()
	}
	runner.startQualityTriggers()
	t.Cleanup(runner.Stop)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("event probe never started")
	}
	stopped := make(chan struct{})
	go func() { runner.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("event worker did not stop after cancellation")
	}
	select {
	case <-canceled:
	default:
		t.Fatal("running provider probe was not canceled")
	}
	select {
	case <-q.closed:
	default:
		t.Fatal("dedicated queue pool was not closed")
	}
	require.Zero(t, q.acks.Load(), "a stopped or unsaved probe cannot consume the signal")
}
