package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type qualityProbeRunnerRepo struct {
	ScheduledTestPlanRepository
	results           []*ScheduledTestResult
	outcome           string
	commits, finishes int
}

func (r *qualityProbeRunnerRepo) ClaimQualityRun(_ context.Context, p *ScheduledTestPlan, _ time.Time) (time.Time, bool, error) {
	return p.UpdatedAt, true, nil
}
func (r *qualityProbeRunnerRepo) FinishQualityRun(context.Context, int64, time.Time, time.Time, time.Time) error {
	r.finishes++
	return nil
}
func (r *qualityProbeRunnerRepo) CommitQualityRun(ctx context.Context, plan *ScheduledTestPlan, outcome string, result *ScheduledTestResult, max int) (string, error) {
	return r.CommitQualityRound(ctx, plan, outcome, []*ScheduledTestResult{result}, max)
}
func (r *qualityProbeRunnerRepo) CommitQualityRound(_ context.Context, _ *ScheduledTestPlan, outcome string, results []*ScheduledTestResult, _ int) (string, error) {
	r.results = results
	r.outcome = outcome
	r.commits++
	return "observed", nil
}
func qualityProbeRunnerPlan(models []string, parallel int) *ScheduledTestPlan {
	return &ScheduledTestPlan{ID: 9, AccountID: 7, ModelID: "a", Enabled: true, MaxResults: 50, UpdatedAt: time.Now(), CronExpression: "0 * * * *", PelicanConfig: &PelicanTestConfig{ModelIDs: models, ParallelCount: parallel, Prompt: "say 42", Quality: &QualityPolicy{Action: "disable_scheduling", ExpectedAnswer: "42", AutoRestore: true, Judge: &QualityJudgeConfig{Kind: "expected"}}}}
}
func qualityProbeRunner(repo *qualityProbeRunnerRepo, run func(context.Context, int64, string, string) (*ScheduledTestResult, error)) *ScheduledTestRunnerService {
	return &ScheduledTestRunnerService{scheduledSvc: NewScheduledTestService(repo, nil), runQualityProbe: run}
}
func probeAnswer(answer string) *ScheduledTestResult {
	return &ScheduledTestResult{Status: "success", ResponseText: answer, StartedAt: time.Now(), FinishedAt: time.Now()}
}

func TestQualityMultiProbeWaitsForWholeRoundAndBoundsWorkers(t *testing.T) {
	repo := &qualityProbeRunnerRepo{}
	plan := qualityProbeRunnerPlan([]string{"a", "b"}, 5)
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	started := make(chan struct{}, 10)
	var mu sync.Mutex
	active, maxActive := 0, 0
	calls := map[string]int{}
	runner := qualityProbeRunner(repo, func(ctx context.Context, _ int64, model, prompt string) (*ScheduledTestResult, error) {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		calls[model]++
		mu.Unlock()
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		mu.Lock()
		active--
		mu.Unlock()
		return probeAnswer("42"), nil
	})
	done := make(chan struct{})
	go func() { runner.runQualityPlan(context.Background(), plan); close(done) }()
	for i := 0; i < 8; i++ {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatal("did not start all eight workers")
		}
	}
	select {
	case <-done:
		t.Fatal("round committed before probes finished")
	default:
	}
	close(release)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("round did not finish")
	}
	require.Equal(t, 8, maxActive)
	require.Equal(t, map[string]int{"a": 5, "b": 5}, calls)
	require.Equal(t, 1, repo.commits)
	require.Equal(t, 1, repo.finishes)
	require.Equal(t, "passed", repo.outcome)
	require.Len(t, repo.results, 10)
	round := repo.results[0].QualityRoundID
	require.NotEmpty(t, round)
	models := map[string]int{}
	for _, result := range repo.results {
		require.Equal(t, round, result.QualityRoundID)
		require.Equal(t, []string{"a", "b"}, result.PelicanConfig.ModelIDs)
		models[result.PelicanConfig.ModelID]++
		require.Empty(t, result.ResponseText)
		require.Empty(t, result.PelicanConfig.Prompt)
	}
	require.Equal(t, calls, models)
	require.Equal(t, "say 42", plan.PelicanConfig.Prompt)
}

func TestQualityMultiProbeAggregatesAnswersBeforeSingleAction(t *testing.T) {
	for _, tc := range []struct {
		name, answer, outcome string
		transport             bool
	}{
		{"all correct", "42", "passed", false},
		{"known wrong dominates unknown", "wrong", "failed", true},
		{"partial correct never restores", "42", "inconclusive", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &qualityProbeRunnerRepo{}
			runner := qualityProbeRunner(repo, func(_ context.Context, _ int64, model, _ string) (*ScheduledTestResult, error) {
				if model == "b" && tc.transport {
					return nil, errors.New("private provider diagnostics")
				}
				return probeAnswer(tc.answer), nil
			})
			runner.runQualityPlan(context.Background(), qualityProbeRunnerPlan([]string{"a", "b"}, 2))
			require.Equal(t, tc.outcome, repo.outcome)
			require.Equal(t, 1, repo.commits)
			require.Len(t, repo.results, 4)
			for _, r := range repo.results {
				require.NotContains(t, r.ErrorMessage, "private")
				require.Empty(t, r.ResponseText)
			}
		})
	}
}

func TestQualityMultiProbeCancellationPersistsEveryPlannedUnknownSample(t *testing.T) {
	repo := &qualityProbeRunnerRepo{}
	calls := 0
	runner := qualityProbeRunner(repo, func(context.Context, int64, string, string) (*ScheduledTestResult, error) {
		calls++
		return nil, context.Canceled
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner.runQualityPlan(ctx, qualityProbeRunnerPlan([]string{"a", "b"}, 3))
	require.Zero(t, calls)
	require.Equal(t, "inconclusive", repo.outcome)
	require.Len(t, repo.results, 6)
	require.Equal(t, 1, repo.finishes)
	for _, r := range repo.results {
		require.Equal(t, "unknown", r.QualityJudgment["verdict"])
	}
}

func TestQualityMultiProbeKeepsLegacySingleProbeDefault(t *testing.T) {
	repo := &qualityProbeRunnerRepo{}
	plan := qualityProbeRunnerPlan(nil, 0)
	plan.ModelID = ""
	modelUsed := "unset"
	runner := qualityProbeRunner(repo, func(_ context.Context, _ int64, model, _ string) (*ScheduledTestResult, error) {
		modelUsed = model
		return probeAnswer("42"), nil
	})
	runner.runQualityPlan(context.Background(), plan)
	require.Equal(t, "", modelUsed)
	require.Len(t, repo.results, 1)
	require.Equal(t, "passed", repo.outcome)
}

func TestQualityMultiProbeJevDoesNotRejectItsOwnParallelSamples(t *testing.T) {
	var calls atomic.Int32
	runner, _, _, _ := qualityJevFixture(t, func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		time.Sleep(20 * time.Millisecond)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(qualityJevReply("correct", 0.99, 0.99)))}, nil
	})
	repo := &qualityProbeRunnerRepo{}
	runner.scheduledSvc = NewScheduledTestService(repo, nil)
	runner.runQualityProbe = func(context.Context, int64, string, string) (*ScheduledTestResult, error) {
		return probeAnswer("42"), nil
	}
	plan := qualityProbeRunnerPlan([]string{"a", "b"}, 4)
	plan.PelicanConfig.Quality.Judge = &QualityJudgeConfig{Kind: "jev", Prompt: "compare answer"}
	runner.runQualityPlan(context.Background(), plan)
	require.Equal(t, int32(8), calls.Load())
	require.Equal(t, "passed", repo.outcome)
	require.Len(t, repo.results, 8)
	for _, result := range repo.results {
		require.Equal(t, "correct", result.QualityJudgment["verdict"])
	}
}
