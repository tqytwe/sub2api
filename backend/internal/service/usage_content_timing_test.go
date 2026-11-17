package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/requesttiming"
	"github.com/stretchr/testify/require"
)

type usageTimingPageStub struct {
	UsageLogRepository
	ids []int64
	err error
}

func (r *usageTimingPageStub) ReadUsageTimingObservations(_ context.Context, ids []int64) (map[int64]UsageTimingObservation, error) {
	r.ids = append([]int64(nil), ids...)
	return map[int64]UsageTimingObservation{
		7:  {Content: requesttiming.ContentObservation{Status: "observed"}, Queue: &requesttiming.QueueObservation{Status: "measured", ServerDurationMs: 200}},
		99: {Content: requesttiming.ContentObservation{Status: "observed"}},
	}, r.err
}

func TestUsageTimingEnrichmentIsBoundedToAuthorizedPage(t *testing.T) {
	repo := &usageTimingPageStub{}
	s := NewUsageService(repo, nil, nil, nil)
	logs := []UsageLog{{ID: 7}, {ID: 8}}
	s.EnrichContentTiming(context.Background(), logs)
	require.Equal(t, []int64{7, 8}, repo.ids)
	require.Equal(t, "observed", logs[0].ContentTiming.Status)
	require.Equal(t, int64(200), logs[0].QueueTiming.ServerDurationMs)
	require.Nil(t, logs[1].ContentTiming)
	require.Nil(t, logs[1].QueueTiming)
	repo.err = errors.New("diagnostics unavailable")
	logs = []UsageLog{{ID: 7}}
	s.EnrichContentTiming(context.Background(), logs)
	require.Nil(t, logs[0].ContentTiming, "diagnostic failures do not fail the usage list or fabricate measurements")
	require.Nil(t, logs[0].QueueTiming)
	repo.ids = nil
	s.EnrichContentTiming(context.Background(), make([]UsageLog, 1001))
	require.Nil(t, repo.ids, "oversized batches are not queried")
}
