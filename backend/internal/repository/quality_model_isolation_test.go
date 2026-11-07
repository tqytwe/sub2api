package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestQualityIsolationRecoveryRequiresCompleteConclusiveSamples(t *testing.T) {
	plan := &service.ScheduledTestPlan{ModelID: "model", PelicanConfig: &service.PelicanTestConfig{ModelIDs: []string{"model"}, ParallelCount: 2}}
	passed := &service.ScheduledTestResult{Status: "success", PelicanConfig: &service.PelicanTestConfig{ModelID: "model"}, QualityJudgment: map[string]any{"verdict": "correct"}}
	unknown := &service.ScheduledTestResult{Status: "skipped", PelicanConfig: &service.PelicanTestConfig{ModelID: "model"}, QualityJudgment: map[string]any{"verdict": "unknown"}}
	failed := &service.ScheduledTestResult{Status: "failed", ErrorMessage: "answer_mismatch", PelicanConfig: &service.PelicanTestConfig{ModelID: "model"}, QualityJudgment: map[string]any{"verdict": "incorrect"}}
	providerError := &service.ScheduledTestResult{Status: "failed", ErrorMessage: "upstream unavailable", PelicanConfig: &service.PelicanTestConfig{ModelID: "model"}, QualityJudgment: map[string]any{"verdict": "incorrect"}}
	for _, tc := range []struct {
		name    string
		samples []*service.ScheduledTestResult
		want    string
	}{
		{"complete passes", []*service.ScheduledTestResult{passed, passed}, "passed"},
		{"missing sample", []*service.ScheduledTestResult{passed}, "inconclusive"},
		{"unknown peer", []*service.ScheduledTestResult{passed, unknown}, "inconclusive"},
		{"provider error", []*service.ScheduledTestResult{passed, providerError}, "inconclusive"},
		{"definitive failure", []*service.ScheduledTestResult{passed, failed}, "failed"},
		{"failure with missing peer", []*service.ScheduledTestResult{failed}, "failed"},
		{"extra sample", []*service.ScheduledTestResult{passed, passed, passed}, "inconclusive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, qualityIsolationOutcomes(plan, tc.samples)["model"])
		})
	}
}
