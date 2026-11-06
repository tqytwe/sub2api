package service

import (
	"context"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"time"
)

type PlayAdminCheckinFilter struct {
	UserID     int64  `json:"user_id"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
	RewardMode string `json:"reward_mode"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	AsOf       string `json:"as_of"`
}

type PlayAdminCheckinRecord struct {
	ID                int64     `json:"id"`
	UserID            int64     `json:"user_id"`
	Email             string    `json:"email"`
	Date              string    `json:"date"`
	StoredStreak      int       `json:"stored_streak"`
	ObservedStreak    int       `json:"observed_streak"`
	RewardAmount      string    `json:"reward_amount"`
	AdjustmentAmount  string    `json:"adjustment_amount"`
	LedgerAmount      string    `json:"ledger_amount"`
	GrowthEnergy      int64     `json:"growth_energy"`
	RewardMode        string    `json:"reward_mode"`
	EligibilityReason string    `json:"eligibility_reason"`
	SnapshotID        *int64    `json:"snapshot_id"`
	EvidenceStatus    string    `json:"evidence_status"`
	ReconciliationID  *int64    `json:"reconciliation_id"`
	CreatedAt         time.Time `json:"created_at"`
}

type PlayAdminCheckinPage struct {
	Items     []PlayAdminCheckinRecord `json:"items"`
	Total     int                      `json:"total"`
	Page      int                      `json:"page"`
	PageSize  int                      `json:"page_size"`
	StartDate string                   `json:"start_date"`
	EndDate   string                   `json:"end_date"`
	AsOf      string                   `json:"as_of"`
}

type PlayAdminCheckinRepository interface {
	ListAdminCheckins(context.Context, PlayAdminCheckinFilter) (*PlayAdminCheckinPage, error)
}

func (s *PlayService) ListAdminCheckins(ctx context.Context, filter PlayAdminCheckinFilter) (*PlayAdminCheckinPage, error) {
	invalid := infraerrors.BadRequest("PLAY_CHECKIN_QUERY_INVALID", "invalid check-in filters; use a date range of at most 366 days")
	now := s.serverNow()
	today := s.serverDate(now)
	if filter.StartDate == "" {
		filter.StartDate = today.AddDate(0, 0, -29).Format("2006-01-02")
	}
	if filter.EndDate == "" {
		filter.EndDate = today.Format("2006-01-02")
	}
	start, err := time.Parse("2006-01-02", filter.StartDate)
	if err != nil {
		return nil, invalid
	}
	end, err := time.Parse("2006-01-02", filter.EndDate)
	if err != nil || end.Before(start) || end.Sub(start) >= 366*24*time.Hour || filter.UserID < 0 {
		return nil, invalid
	}
	switch filter.RewardMode {
	case "", "energy", "redeemable", "unknown":
	default:
		return nil, invalid
	}
	if filter.Page == 0 {
		filter.Page = 1
	}
	if filter.PageSize == 0 {
		filter.PageSize = 20
	}
	if filter.Page < 1 || filter.Page > 10000 || filter.PageSize < 1 || filter.PageSize > 100 {
		return nil, invalid
	}
	if filter.AsOf == "" {
		filter.AsOf = now.UTC().Format(time.RFC3339Nano)
	} else {
		asOf, parseErr := time.Parse(time.RFC3339Nano, filter.AsOf)
		if parseErr != nil || asOf.After(now) || now.Sub(asOf) > 24*time.Hour {
			return nil, invalid
		}
	}
	repo, ok := s.repo.(PlayAdminCheckinRepository)
	if !ok {
		return nil, infraerrors.ServiceUnavailable("PLAY_CHECKIN_REPORT_UNAVAILABLE", "check-in report unavailable")
	}
	result, err := repo.ListAdminCheckins(ctx, filter)
	if err != nil {
		return nil, err
	}
	result.Page, result.PageSize = filter.Page, filter.PageSize
	result.StartDate, result.EndDate, result.AsOf = filter.StartDate, filter.EndDate, filter.AsOf
	if result.Items == nil {
		result.Items = []PlayAdminCheckinRecord{}
	}
	return result, nil
}
