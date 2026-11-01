package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type qualityHistoryReadRepo struct {
	ScheduledTestResultRepository
	results []*ScheduledTestResult
}

func (r *qualityHistoryReadRepo) ListByPlanID(context.Context, int64, int) ([]*ScheduledTestResult, error) {
	return r.results, nil
}

func TestListResultsRedactsQualityHistoryContent(t *testing.T) {
	quality := &ScheduledTestResult{
		Status:       "success",
		ResponseText: "private response",
		PelicanConfig: &PelicanTestConfig{
			Prompt: "private prompt",
			Quality: &QualityPolicy{
				ExpectedAnswer: "private answer",
				Judge:          &QualityJudgeConfig{Kind: "jev", Prompt: "private rubric"},
			},
		},
	}
	repo := &qualityHistoryReadRepo{results: []*ScheduledTestResult{quality}}

	results, err := (&ScheduledTestService{resultRepo: repo}).ListResults(context.Background(), 7, 50)

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Empty(t, results[0].ResponseText)
	require.Empty(t, results[0].PelicanConfig.Prompt)
	require.Empty(t, results[0].PelicanConfig.Quality.ExpectedAnswer)
	require.Empty(t, results[0].PelicanConfig.Quality.Judge.Prompt)
}

func TestListResultsPreservesLegacyConnectivityContent(t *testing.T) {
	connectivity := &ScheduledTestResult{Status: "success", ResponseText: "connectivity response"}
	repo := &qualityHistoryReadRepo{results: []*ScheduledTestResult{connectivity}}

	results, err := (&ScheduledTestService{resultRepo: repo}).ListResults(context.Background(), 7, 50)

	require.NoError(t, err)
	require.Equal(t, "connectivity response", results[0].ResponseText)
}
