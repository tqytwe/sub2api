package service

import (
	"context"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"time"
)

type PlayAdminArenaPeriod struct {
	ID                 int64      `json:"id"`
	Name               string     `json:"name"`
	PeriodType         string     `json:"period_type"`
	Status             string     `json:"status"`
	StartAt            time.Time  `json:"start_at"`
	EndAt              time.Time  `json:"end_at"`
	SettledAt          *time.Time `json:"settled_at"`
	AccountingVersion  int        `json:"accounting_version"`
	RewardPolicySource string     `json:"reward_policy_source"`
	PaidUsers          int        `json:"paid_users"`
	PaidAmount         string     `json:"paid_amount"`
}
type PlayAdminArenaPeriods struct {
	Items    []PlayAdminArenaPeriod `json:"items"`
	Total    int                    `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}
type PlayAdminArenaPeriodsRepository interface {
	ListAdminArenaPeriods(context.Context, string, int, int) (*PlayAdminArenaPeriods, error)
}

func (s *PlayService) ListAdminArenaPeriods(ctx context.Context, kind string, page, size int) (*PlayAdminArenaPeriods, error) {
	if kind != "daily" && kind != "monthly" || page < 1 || page > 10000 || size < 1 || size > 100 {
		return nil, infraerrors.BadRequest("PLAY_ARENA_QUERY_INVALID", "invalid period type or pagination")
	}
	repo, ok := s.repo.(PlayAdminArenaPeriodsRepository)
	if !ok {
		return nil, infraerrors.ServiceUnavailable("PLAY_ARENA_REPORT_UNAVAILABLE", "period report unavailable")
	}
	result, err := repo.ListAdminArenaPeriods(ctx, kind, page, size)
	if err != nil {
		return nil, err
	}
	result.Page, result.PageSize = page, size
	return result, nil
}
