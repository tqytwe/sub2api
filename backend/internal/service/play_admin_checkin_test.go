package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type adminCheckinRepo struct {
	PlayRepository
	filter PlayAdminCheckinFilter
	calls  int
}

func (r *adminCheckinRepo) ListAdminCheckins(_ context.Context, filter PlayAdminCheckinFilter) (*PlayAdminCheckinPage, error) {
	r.filter = filter
	r.calls++
	return &PlayAdminCheckinPage{Items: []PlayAdminCheckinRecord{}, Total: 0}, nil
}
func TestAdminCheckinQueryValidatesBoundedWindowAndPagination(t *testing.T) {
	repo := &adminCheckinRepo{}
	svc := NewPlayService(repo, nil, nil, nil, nil, nil)
	svc.now = func() time.Time { return time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC) }
	page, err := svc.ListAdminCheckins(context.Background(), PlayAdminCheckinFilter{})
	require.NoError(t, err)
	require.Equal(t, 1, page.Page)
	require.Equal(t, 20, page.PageSize)
	require.Equal(t, "2026-10-06", repo.filter.StartDate)
	require.Equal(t, "2026-11-04", repo.filter.EndDate)
	require.NotEmpty(t, page.AsOf)
	for _, filter := range []PlayAdminCheckinFilter{
		{StartDate: "invalid"},
		{StartDate: "2026-11-04", EndDate: "2026-11-03"},
		{StartDate: "2025-01-01", EndDate: "2026-11-04"},
		{UserID: -1},
		{Page: 1000000000},
		{RewardMode: "anything"},
	} {
		_, err := svc.ListAdminCheckins(context.Background(), filter)
		require.Error(t, err)
	}
	require.Equal(t, 1, repo.calls, "invalid queries never reach storage")
}
