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
	require.Equal(t, "failed", page.Items[0].Status)
}

func TestListQualityHistoryPropagatesRepositoryError(t *testing.T) {
	want := errors.New("history unavailable")
	_, err := (&ScheduledTestService{resultRepo: &qualityHistoryResultRepoStub{err: want}}).ListQualityHistory(context.Background(), 0)
	require.ErrorIs(t, err, want)
}
