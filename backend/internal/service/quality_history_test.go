package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type qualityHistoryResultRepoStub struct {
	items []*QualityHistoryResult
	err   error
}

func (s *qualityHistoryResultRepoStub) Create(context.Context, *ScheduledTestResult) (*ScheduledTestResult, error) {
	return nil, nil
}
func (s *qualityHistoryResultRepoStub) ListByPlanID(context.Context, int64, int) ([]*ScheduledTestResult, error) {
	return nil, nil
}
func (s *qualityHistoryResultRepoStub) ListQualityHistory(context.Context, int64, int) ([]*QualityHistoryResult, error) {
	return s.items, s.err
}
func (s *qualityHistoryResultRepoStub) PruneOldResults(context.Context, int64, int) error { return nil }

func TestListQualityHistoryNormalizesRoundsAndCursor(t *testing.T) {
	items := make([]*QualityHistoryResult, 0, 101)
	for i := 0; i < 101; i++ {
		items = append(items, &QualityHistoryResult{
			ScheduledTestResult: ScheduledTestResult{ID: int64(1000 - i)},
			PassedCount:         2,
			TotalCount:          2,
		})
	}
	items[0].PelicanConfig = &PelicanTestConfig{ModelIDs: []string{"a", "b"}, ParallelCount: 2}
	page, err := (&ScheduledTestService{resultRepo: &qualityHistoryResultRepoStub{items: items}}).ListQualityHistory(context.Background(), 0)
	require.NoError(t, err)
	require.Len(t, page.Items, 100)
	require.Equal(t, int64(901), page.NextCursor)
	require.Equal(t, 4, page.Items[0].TotalCount)
	require.Equal(t, "inconclusive", page.Items[0].Status)
}

func TestListQualityHistoryPropagatesRepositoryError(t *testing.T) {
	want := errors.New("history unavailable")
	_, err := (&ScheduledTestService{resultRepo: &qualityHistoryResultRepoStub{err: want}}).ListQualityHistory(context.Background(), 0)
	require.ErrorIs(t, err, want)
}

func TestListQualityHistoryPreservesInconclusiveRounds(t *testing.T) {
	for _, tc := range []struct {
		name, storedStatus, want string
		passed, skipped, total   int
	}{
		{"unknown judge", "inconclusive", "inconclusive", 0, 0, 1},
		{"correct and unknown", "inconclusive", "inconclusive", 1, 0, 2},
		{"correct and skipped", "inconclusive", "inconclusive", 1, 1, 2},
		{"incorrect and unknown", "failed", "failed", 0, 0, 2},
		{"correct round", "success", "success", 2, 0, 2},
		{"invalid complete model distribution", "inconclusive", "inconclusive", 2, 0, 2},
		{"all skipped", "inconclusive", "skipped", 0, 2, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := &QualityHistoryResult{
				ScheduledTestResult: ScheduledTestResult{Status: tc.storedStatus},
				PassedCount:         tc.passed, SkippedCount: tc.skipped, TotalCount: tc.total,
			}
			page, err := (&ScheduledTestService{resultRepo: &qualityHistoryResultRepoStub{
				items: []*QualityHistoryResult{item},
			}}).ListQualityHistory(context.Background(), 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, page.Items[0].Status)
		})
	}
}

func TestListQualityHistoryMultiModelDefaultsAndCanonicalModelsCannotFakeCompletion(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		models                        []string
		parallel, passed, total, want int
	}{
		{"omitted count", []string{"first", "second"}, 0, 1, 1, 2},
		{"duplicate and whitespace models", []string{" first ", "first", "second", " "}, 2, 2, 2, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := &QualityHistoryResult{ScheduledTestResult: ScheduledTestResult{Status: "success", PelicanConfig: &PelicanTestConfig{ModelIDs: tc.models, ParallelCount: tc.parallel}}, PassedCount: tc.passed, TotalCount: tc.total}
			page, err := (&ScheduledTestService{resultRepo: &qualityHistoryResultRepoStub{items: []*QualityHistoryResult{item}}}).ListQualityHistory(context.Background(), 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, page.Items[0].TotalCount)
			require.Equal(t, "inconclusive", page.Items[0].Status)
		})
	}
}
