package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

const QualityBatchStartWindow = 2 * time.Minute
const QualityBatchLifetime = 12 * time.Minute

type QualityBatchQuestion struct {
	Prompt   string                 `json:"prompt"`
	Expected string                 `json:"expected"`
	Question *QualityRandomQuestion `json:"question"`
}

type QualitySharedBatch struct {
	ID            string                 `json:"id"`
	StartedAt     time.Time              `json:"started_at"`
	StartDeadline time.Time              `json:"start_deadline"`
	ClosesAt      time.Time              `json:"closes_at"`
	Questions     []QualityBatchQuestion `json:"questions"`
}

type QualityBatchEvidence struct {
	Route            string    `json:"route"`
	MappedModel      string    `json:"mapped_model"`
	ID               string    `json:"id"`
	StartedAt        time.Time `json:"started_at"`
	StartDeadline    time.Time `json:"start_deadline"`
	ClosesAt         time.Time `json:"closes_at"`
	ComparisonKey    string    `json:"comparison_key"`
	Decision         string    `json:"decision"`
	PeerAccounts     int       `json:"peer_accounts"`
	PeerPassed       int       `json:"peer_passed"`
	PeerFailed       int       `json:"peer_failed"`
	PeerInconclusive int       `json:"peer_inconclusive"`
}

type QualityBatchRun struct {
	CredentialsRevision string                 `json:"credentials_revision"`
	OAuth401            *QualityOAuth401Round  `json:"oauth_401,omitempty"`
	Apply               bool                   `json:"apply"`
	Plan                *ScheduledTestPlan     `json:"plan"`
	Until               time.Time              `json:"until"`
	Trigger             string                 `json:"trigger"`
	ObservedAt          *time.Time             `json:"observed_at,omitempty"`
	Results             []*ScheduledTestResult `json:"results,omitempty"`
}

type QualitySharedBatchRepository interface {
	StartQualityBatch(context.Context, *QualitySharedBatch, *ScheduledTestPlan, time.Time) (*QualitySharedBatch, error)
	CollectQualityBatch(context.Context, *QualityBatchRun) error
	FinalizeQualityBatches(context.Context) error
}

// SanitizedQualityBatchRun keeps the comparison key and safe 401 classification
// while excluding prompts, references, rubrics and provider bodies from the
// durable run ledger. It never changes the caller's active probe configuration.
func SanitizedQualityBatchRun(run *QualityBatchRun) *QualityBatchRun {
	copy := *run
	plan := *run.Plan
	plan.PelicanConfig = cloneQualityPelicanConfig(run.Plan.PelicanConfig)
	configResult := &ScheduledTestResult{PelicanConfig: plan.PelicanConfig}
	sanitizeQualityResult(configResult)
	plan.PelicanConfig = configResult.PelicanConfig
	copy.Plan = &plan
	if copy.OAuth401 == nil && len(run.Results) > 0 {
		copy.OAuth401 = QualityOAuth401Evidence(run.Plan, run.Results)
	}
	copy.Results = make([]*ScheduledTestResult, len(run.Results))
	for i, result := range run.Results {
		sample := *result
		sample.PelicanConfig = cloneQualityPelicanConfig(result.PelicanConfig)
		sanitizeQualityResult(&sample)
		copy.Results[i] = &sample
	}
	return &copy
}

func newQualitySharedBatch(now time.Time) (*QualitySharedBatch, error) {
	start := now.UTC().Truncate(time.Minute)
	samples, err := newQualityRandomSamples(&PelicanTestConfig{QuestionKind: QualityRandomQuestionKind, ParallelCount: 32, Quality: &QualityPolicy{}})
	if err != nil {
		return nil, err
	}
	b := &QualitySharedBatch{ID: "random-v" + qualityRandomGeneratorVersion + "-" + start.Format("20060102T1504Z"), StartedAt: start, StartDeadline: start.Add(QualityBatchStartWindow), ClosesAt: start.Add(QualityBatchLifetime)}
	for _, s := range samples {
		b.Questions = append(b.Questions, QualityBatchQuestion{Prompt: s.Prompt, Expected: s.Quality.ExpectedAnswer, Question: s.RandomQuestion})
	}
	return b, nil
}

func sharedQualitySamples(cfg *PelicanTestConfig, b *QualitySharedBatch) ([]*PelicanTestConfig, error) {
	if b == nil || cfg == nil || cfg.Quality == nil || QualityProbeParallelCount(cfg) > len(b.Questions) {
		return nil, fmt.Errorf("quality shared batch unavailable")
	}
	out := make([]*PelicanTestConfig, QualityProbeParallelCount(cfg))
	for i := range out {
		q := b.Questions[i]
		if q.Question == nil || q.Prompt == "" || q.Expected == "" {
			return nil, fmt.Errorf("quality shared question unavailable")
		}
		c := cloneQualityPelicanConfig(cfg)
		c.Prompt, c.Quality.ExpectedAnswer, c.RandomQuestion = q.Prompt, q.Expected, q.Question
		c.QualityBatch = &QualityBatchEvidence{ID: b.ID, StartedAt: b.StartedAt, StartDeadline: b.StartDeadline, ClosesAt: b.ClosesAt, Decision: "pending"}
		out[i] = c
	}
	return out, nil
}

func qualityComparisonKey(cfg *PelicanTestConfig) string {
	if cfg == nil || cfg.RandomQuestion == nil || cfg.Quality == nil || cfg.QualityBatch == nil {
		return ""
	}
	if cfg.QualityBatch.ComparisonKey != "" {
		return cfg.QualityBatch.ComparisonKey
	}
	channel := cfg.TestChannel
	if channel == "" {
		channel = "account"
	}
	value, _ := json.Marshal([]any{cfg.QualityBatch.ID, cfg.RandomQuestion, cfg.Prompt, cfg.Quality.ExpectedAnswer, cfg.ModelID, cfg.QualityBatch.MappedModel, cfg.QualityBatch.Route, cfg.ReasoningEffort, channel, cfg.Quality.Judge})
	return fmt.Sprintf("%x", sha256.Sum256(value))
}

func qualityBatchSampleEligible(r *ScheduledTestResult) bool {
	if r == nil || r.PelicanConfig == nil || r.PelicanConfig.QualityBatch == nil {
		return false
	}
	b := r.PelicanConfig.QualityBatch
	return b.Route != "" && b.MappedModel != "" && !r.FinishedAt.Before(r.StartedAt) && !r.StartedAt.Before(b.StartedAt) && r.StartedAt.Before(b.StartDeadline) && !r.FinishedAt.After(b.ClosesAt) && qualityComparisonKey(r.PelicanConfig) != ""
}

func qualitySharedOutcome(results []*ScheduledTestResult) string {
	if len(results) == 0 {
		return "inconclusive"
	}
	passed := true
	for _, r := range results {
		if r == nil {
			return "inconclusive"
		}
		v, _ := r.QualityJudgment["verdict"].(string)
		if r.Status == "failed" && r.ErrorMessage == "answer_mismatch" && v == "incorrect" {
			return "failed"
		}
		passed = passed && r.Status == "success" && v == "correct"
	}
	if passed {
		return "passed"
	}
	return "inconclusive"
}

// Raw verdicts remain unchanged; only account-action evidence requires a passing peer.
func EvaluateQualityBatch(run *QualityBatchRun, peers []*QualityBatchRun) string {
	run.Plan.QualityBatchEvaluated = true
	run.Plan.QualityModelOutcomes = map[string]string{}
	effective := make([]*ScheduledTestResult, 0, len(run.Results))
	for _, r := range run.Results {
		copy := *r
		cfg := r.PelicanConfig
		if cfg == nil || cfg.QualityBatch == nil {
			copy.Status, copy.QualityJudgment = "failed", nil
			effective = append(effective, &copy)
			continue
		}
		b := *cfg.QualityBatch
		b.ComparisonKey = qualityComparisonKey(cfg)
		b.PeerAccounts, b.PeerPassed, b.PeerFailed, b.PeerInconclusive = 0, 0, 0, 0
		accounts := map[int64][]*ScheduledTestResult{}
		for _, p := range peers {
			if !p.Apply || p.Plan.AccountID == run.Plan.AccountID {
				continue
			}
			for _, s := range p.Results {
				if qualityBatchSampleEligible(s) && qualityComparisonKey(s.PelicanConfig) == b.ComparisonKey {
					accounts[p.Plan.AccountID] = append(accounts[p.Plan.AccountID], s)
				}
			}
		}
		for _, samples := range accounts {
			b.PeerAccounts++
			switch qualitySharedOutcome(samples) {
			case "passed":
				b.PeerPassed++
			case "failed":
				b.PeerFailed++
			default:
				b.PeerInconclusive++
			}
		}
		switch {
		case !run.Apply || b.Route == "" || b.MappedModel == "":
			b.Decision = "inconclusive"
			copy.Status, copy.QualityJudgment = "failed", nil
		case !qualityBatchSampleEligible(r):
			b.Decision = "outside_window"
			copy.Status, copy.QualityJudgment = "failed", nil
		case qualitySharedOutcome([]*ScheduledTestResult{r}) == "passed":
			b.Decision = "passed"
		case b.PeerPassed > 0 && qualitySharedOutcome([]*ScheduledTestResult{r}) == "failed":
			b.Decision = "peer_passed"
		case qualitySharedOutcome([]*ScheduledTestResult{r}) == "failed":
			b.Decision = "no_passing_peer"
			copy.Status, copy.QualityJudgment = "failed", nil
		default:
			b.Decision = "inconclusive"
		}
		cfg.QualityBatch = &b
		effective = append(effective, &copy)
	}
	for _, model := range QualityProbeModels(run.Plan) {
		var samples []*ScheduledTestResult
		for _, r := range effective {
			if r.PelicanConfig != nil && r.PelicanConfig.ModelID == model {
				samples = append(samples, r)
			}
		}
		outcome := qualitySharedOutcome(samples)
		if len(samples) != QualityProbeParallelCount(run.Plan.PelicanConfig) && outcome != "failed" {
			outcome = "inconclusive"
		}
		run.Plan.QualityModelOutcomes[model] = outcome
	}
	if len(effective) != QualityProbeExpectedCount(run.Plan.PelicanConfig) {
		return "inconclusive"
	}
	return qualitySharedOutcome(effective)
}

type qualityProbeEvidenceKey struct{}
type qualityProbeEvidence struct {
	Batch       *QualityBatchEvidence
	NativeOAuth bool
	Direct401   bool
	Revoked401  bool
}

func recordQualityProbeRoute(ctx context.Context, account *Account, model, route string) {
	if e, _ := ctx.Value(qualityProbeEvidenceKey{}).(*qualityProbeEvidence); e != nil && e.Batch != nil {
		e.Batch.Route = route
		e.Batch.MappedModel = account.GetMappedModel(model)
	}
}

func qualityEvidenceDirect401(ctx context.Context) bool {
	e, _ := ctx.Value(qualityProbeEvidenceKey{}).(*qualityProbeEvidence)
	return e != nil && e.Direct401
}
func qualityEvidenceRevoked401(ctx context.Context) bool {
	e, _ := ctx.Value(qualityProbeEvidenceKey{}).(*qualityProbeEvidence)
	return e != nil && e.Revoked401
}
