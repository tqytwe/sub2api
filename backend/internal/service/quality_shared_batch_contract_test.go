package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func sharedQualityContractRun(t *testing.T, batch *QualitySharedBatch, id int64, verdict string) *QualityBatchRun {
	t.Helper()
	cfg := &PelicanTestConfig{QuestionKind: QualityRandomQuestionKind, ModelIDs: []string{"model"}, ParallelCount: 1, ReasoningEffort: "medium", Quality: &QualityPolicy{Action: "disable_scheduling", Judge: &QualityJudgeConfig{Kind: "expected"}}}
	samples, err := sharedQualitySamples(cfg, batch)
	require.NoError(t, err)
	samples[0].ModelID = "model"
	samples[0].QualityBatch.Route, samples[0].QualityBatch.MappedModel = "openai:account", "model"
	r := &ScheduledTestResult{Status: "success", PelicanConfig: samples[0], StartedAt: batch.StartedAt.Add(time.Second), FinishedAt: batch.StartedAt.Add(2 * time.Second), QualityJudgment: map[string]any{"verdict": verdict}}
	if verdict == "incorrect" {
		r.Status, r.ErrorMessage = "failed", "answer_mismatch"
	}
	return &QualityBatchRun{Apply: true, Plan: &ScheduledTestPlan{ID: id, AccountID: id, PelicanConfig: cfg}, Results: []*ScheduledTestResult{r}}
}

func TestQualitySharedBatchRequiresComparablePassingPeer(t *testing.T) {
	batch, err := newQualitySharedBatch(time.Now())
	require.NoError(t, err)
	a, b := sharedQualityContractRun(t, batch, 1, "incorrect"), sharedQualityContractRun(t, batch, 2, "incorrect")
	require.Equal(t, "inconclusive", EvaluateQualityBatch(a, []*QualityBatchRun{a, b}))
	require.Equal(t, "incorrect", a.Results[0].QualityJudgment["verdict"])
	require.Equal(t, "no_passing_peer", a.Results[0].PelicanConfig.QualityBatch.Decision)
	b.Results[0].Status, b.Results[0].ErrorMessage = "success", ""
	b.Results[0].QualityJudgment["verdict"] = "correct"
	require.Equal(t, "failed", EvaluateQualityBatch(a, []*QualityBatchRun{a, b}))
	require.Equal(t, "peer_passed", a.Results[0].PelicanConfig.QualityBatch.Decision)
	for name, change := range map[string]func(*QualityBatchRun){
		"same account":           func(r *QualityBatchRun) { r.Plan.AccountID = 1 },
		"different effort":       func(r *QualityBatchRun) { r.Results[0].PelicanConfig.ReasoningEffort = "high" },
		"different route":        func(r *QualityBatchRun) { r.Results[0].PelicanConfig.QualityBatch.Route = "openai:bps" },
		"different mapped model": func(r *QualityBatchRun) { r.Results[0].PelicanConfig.QualityBatch.MappedModel = "other" },
		"late":                   func(r *QualityBatchRun) { r.Results[0].StartedAt = batch.StartDeadline },
		"stale":                  func(r *QualityBatchRun) { r.Apply = false },
	} {
		t.Run(name, func(t *testing.T) {
			b := sharedQualityContractRun(t, batch, 2, "correct")
			change(b)
			require.Equal(t, "inconclusive", EvaluateQualityBatch(a, []*QualityBatchRun{a, b}))
		})
	}
}

func TestQuality401ClassificationRequiresCompleteTrustedRound(t *testing.T) {
	plan := &ScheduledTestPlan{ModelID: "a", PelicanConfig: &PelicanTestConfig{ModelIDs: []string{"a", "b"}, ParallelCount: 2, Quality: &QualityPolicy{OAuth401MaxRounds: 3}}}
	results := make([]*ScheduledTestResult, 4)
	for i := range results {
		results[i] = &ScheduledTestResult{Status: "failed", ErrorMessage: "API returned 401: unavailable", DirectOAuth401: true}
	}
	require.True(t, QualityOAuth401Evidence(plan, results).AllDirect401)
	require.False(t, QualityOAuth401Evidence(plan, results[:3]).AllDirect401)
	results[0].DirectOAuth401 = false
	require.False(t, QualityOAuth401Evidence(plan, results).AllDirect401)
	for _, body := range []string{`{"error":{"message":"not invalid_grant"}}`, `{"error":{"code":"other","message":"token_invalidated"}}`, `invalid_grant`} {
		require.False(t, qualityOAuth401RevokedCode([]byte(body)))
	}
	require.True(t, qualityOAuth401RevokedCode([]byte(`{"error":{"code":"token_invalidated"}}`)))
	var injected ScheduledTestResult
	require.NoError(t, json.Unmarshal([]byte(`{"DirectOAuth401":true,"direct_oauth_401":true}`), &injected))
	require.False(t, injected.DirectOAuth401)
}

func TestQualityBatchLedgerRedactsPendingAndCollectedSnapshots(t *testing.T) {
	batch, err := newQualitySharedBatch(time.Now())
	require.NoError(t, err)
	run := sharedQualityContractRun(t, batch, 7, "correct")
	run.Plan.PelicanConfig.Prompt = "private draft question"
	run.Plan.PelicanConfig.Quality.ExpectedAnswer = "private reference"
	run.Plan.PelicanConfig.Quality.Judge.Prompt = "private rubric"
	run.Results[0].ResponseText = "private upstream response"
	clean := SanitizedQualityBatchRun(run)
	raw, err := json.Marshal(clean)
	require.NoError(t, err)
	for _, private := range []string{"private draft question", "private reference", "private rubric", "private upstream response"} {
		require.NotContains(t, string(raw), private)
	}
	require.NotEmpty(t, clean.Results[0].PelicanConfig.QualityBatch.ComparisonKey)
	require.Equal(t, "private draft question", run.Plan.PelicanConfig.Prompt)
	require.Equal(t, "private upstream response", run.Results[0].ResponseText)
}

type qualityPresetEditorContract struct {
	qualityPresetServiceRepo
	saved *QualityRulePreset
}

func (r *qualityPresetEditorContract) UpdateQualityPreset(_ context.Context, p *QualityRulePreset) (*QualityRulePreset, error) {
	r.saved = p
	return p, nil
}

func TestQualityPresetEditRetainsIdentityWithoutDispatch(t *testing.T) {
	repo := &qualityPresetEditorContract{}
	svc := NewScheduledTestService(repo, nil)
	input := &QualityRulePreset{Name: "edited", ModelID: "model", CronExpression: "*/30 * * * *", Enabled: true, MaxResults: 50, PelicanConfig: &PelicanTestConfig{QuestionKind: "text", Prompt: "say 21", ActualPrompt: "runtime prompt", RandomQuestion: &QualityRandomQuestion{ID: "runtime", Family: "paths", Version: "1"}, QualityBatch: &QualityBatchEvidence{ID: "runtime"}, ParallelCount: 1, Quality: &QualityPolicy{Action: "observe_only", ExpectedAnswer: "21", Judge: &QualityJudgeConfig{Kind: "jev", Prompt: "saved rubric"}}}}
	saved, err := svc.UpdateQualityPreset(context.Background(), 7, input)
	require.NoError(t, err)
	require.Equal(t, int64(7), saved.ID)
	require.Equal(t, "edited", repo.saved.Name)
	require.Nil(t, repo.saved.PelicanConfig.RandomQuestion)
	require.Nil(t, repo.saved.PelicanConfig.QualityBatch)
	require.Empty(t, repo.saved.PelicanConfig.ActualPrompt)
}

type qualitySharedRunnerRepo struct {
	qualityProbeRunnerRepo
	batch *QualitySharedBatch
	run   *QualityBatchRun
	err   error
}

func (r *qualitySharedRunnerRepo) StartQualityBatch(_ context.Context, b *QualitySharedBatch, _ *ScheduledTestPlan, _ time.Time) (*QualitySharedBatch, error) {
	r.batch = b
	return b, r.err
}
func (r *qualitySharedRunnerRepo) CollectQualityBatch(_ context.Context, run *QualityBatchRun) error {
	r.run = run
	return nil
}
func (r *qualitySharedRunnerRepo) FinalizeQualityBatches(context.Context) error { return nil }

func TestQualitySharedBatchRunnerPersistsBeforeCallsAndDefersAction(t *testing.T) {
	for _, available := range []bool{false, true} {
		t.Run(map[bool]string{false: "storage unavailable", true: "durable collection"}[available], func(t *testing.T) {
			repo := &qualitySharedRunnerRepo{}
			if !available {
				repo.err = fmt.Errorf("fixture storage unavailable")
			}
			calls := 0
			runner := qualityProbeRunner(&repo.qualityProbeRunnerRepo, func(ctx context.Context, _ int64, _ string, prompt string) (*ScheduledTestResult, error) {
				calls++
				require.NotEmpty(t, prompt)
				e, ok := ctx.Value(qualityProbeEvidenceKey{}).(*qualityProbeEvidence)
				require.True(t, ok)
				e.Batch.Route, e.Batch.MappedModel = "openai:account", "model"
				return &ScheduledTestResult{Status: "success", ResponseText: "wrong", StartedAt: time.Now(), FinishedAt: time.Now()}, nil
			})
			runner.scheduledSvc = NewScheduledTestService(repo, nil)
			plan := &ScheduledTestPlan{ID: 7, AccountID: 8, ModelID: "model", Enabled: true, CronExpression: "*/30 * * * *", MaxResults: 10, PelicanConfig: &PelicanTestConfig{ModelIDs: []string{"model"}, QuestionKind: QualityRandomQuestionKind, ParallelCount: 1, Quality: &QualityPolicy{Action: "disable_scheduling", Judge: &QualityJudgeConfig{Kind: "expected"}}}}
			runner.runQualityPlan(context.Background(), plan)
			if !available {
				require.Zero(t, calls)
				require.Nil(t, repo.run)
				return
			}
			require.Equal(t, 1, calls)
			require.NotNil(t, repo.run)
			require.Empty(t, repo.outcome, "collection cannot immediately apply an uncalibrated wrong answer")
			require.Empty(t, repo.run.Results[0].ResponseText)
			require.Empty(t, repo.run.Results[0].PelicanConfig.Prompt)
			require.Empty(t, repo.run.Results[0].PelicanConfig.Quality.ExpectedAnswer)
			require.NotEmpty(t, repo.run.Results[0].PelicanConfig.QualityBatch.ComparisonKey)
		})
	}
}
