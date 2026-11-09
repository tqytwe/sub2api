package service

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type AffiliateOperationsQuery struct {
	Month                                        string
	Page, PageSize                               int
	UserID, InviteeID, SourceOrderID, CampaignID int64
	Status, Action, Search                       string
	Scope, StartDate, EndDate                    string
	StartAt, EndAt                               *time.Time
	Admin                                        bool
}

type AffiliateOperationsPage[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

type AffiliateOperationsTrend struct {
	Date               string `json:"date"`
	InvitedCount       int64  `json:"invited_count"`
	EffectivePaidCount int64  `json:"effective_paid_count"`
}

type AffiliateOperationsSummary struct {
	Reconciliation           AffiliateOperationsReconciliation `json:"reconciliation"`
	Month                    string                            `json:"month"`
	InvitedCount             int64                             `json:"invited_count"`
	EffectivePaidCount       int64                             `json:"effective_paid_count"`
	PendingCount             int64                             `json:"pending_count"`
	ExceptionsCount          int64                             `json:"exceptions_count"`
	ConversionRate           float64                           `json:"conversion_rate"`
	CohortEffectivePaidCount int64                             `json:"cohort_effective_paid_count"`
	AvailableQuota           float64                           `json:"available_quota"`
	FrozenQuota              float64                           `json:"frozen_quota"`
	GrossEarned              *float64                          `json:"gross_earned"`
	RevokedAmount            *float64                          `json:"revoked_amount"`
	NetEarned                *float64                          `json:"net_earned"`
	MonthNetEarned           *float64                          `json:"month_net_earned"`
	Amounts                  []AffiliateOperationsMoney        `json:"amounts"`
	PeriodAmounts            []AffiliateOperationsMoney        `json:"period_amounts"`
	TransferredAmount        float64                           `json:"transferred_amount"`
	WithdrawnAmount          float64                           `json:"withdrawn_amount"`
	Currency                 string                            `json:"currency"`
	MonetaryScope            string                            `json:"monetary_scope"`
	Trend                    []AffiliateOperationsTrend        `json:"trend"`
	UpdatedAt                time.Time                         `json:"updated_at"`
}

// Registration is independently observable; it must never stand in for a
// missing historical binding timestamp or establish paid-invite eligibility.
type AffiliateOperationsReconciliation struct {
	RelationshipCount            int64 `json:"relationship_count"`
	RegisteredMonthCount         int64 `json:"registered_month_count"`
	RegisteredEffectivePaidCount int64 `json:"registered_effective_paid_count"`
	KnownBindingCount            int64 `json:"known_binding_count"`
	UnknownBindingCount          int64 `json:"unknown_binding_count"`
	EffectivePaidCount           int64 `json:"effective_paid_count"`
	UnpaidCount                  int64 `json:"unpaid_count"`
	RefundedCount                int64 `json:"refunded_count"`
	RiskExcludedCount            int64 `json:"risk_excluded_count"`
	DisabledCount                int64 `json:"disabled_count"`
	PendingCount                 int64 `json:"pending_count"`
	MissingFactCount             int64 `json:"missing_fact_count"`
	PendingPaymentCount          int64 `json:"pending_payment_count"`
}

type AffiliateOperationsMoney struct {
	SourceType    string  `json:"source_type"`
	Currency      string  `json:"currency"`
	GrossEarned   float64 `json:"gross_earned"`
	RevokedAmount float64 `json:"revoked_amount"`
	NetEarned     float64 `json:"net_earned"`
}

type AffiliateOperationsInvitee struct {
	UserID         int64      `json:"user_id"`
	InviterID      int64      `json:"inviter_id,omitempty"`
	InviteeID      int64      `json:"invitee_id,omitempty"`
	InviterEmail   string     `json:"inviter_email,omitempty"`
	InviteeEmail   string     `json:"invitee_email,omitempty"`
	DisplayName    string     `json:"display_name"`
	InvitedAt      *time.Time `json:"invited_at,omitempty"`
	FirstPaidAt    *time.Time `json:"first_paid_at,omitempty"`
	FirstPaidMonth *string    `json:"first_paid_month,omitempty"`
	Status         string     `json:"status"`
	Reason         string     `json:"reason"`
	TotalRebate    float64    `json:"total_rebate"`
	RiskCaseID     *int64     `json:"risk_case_id,omitempty"`
}

type AffiliateOperationsEarning struct {
	ID                    int64      `json:"id"`
	UserID                int64      `json:"user_id,omitempty"`
	Email                 string     `json:"email,omitempty"`
	InviteeID             *int64     `json:"invitee_id,omitempty"`
	InviteeEmail          string     `json:"invitee_email,omitempty"`
	SourceUserDisplayName string     `json:"source_user_display_name,omitempty"`
	SourceOrderID         *int64     `json:"source_order_id,omitempty"`
	Action                string     `json:"action"`
	SourceType            string     `json:"source_type"`
	Amount                float64    `json:"amount"`
	Currency              string     `json:"currency"`
	Status                string     `json:"status"`
	FrozenUntil           *time.Time `json:"frozen_until,omitempty"`
	BalanceAfter          *float64   `json:"balance_after,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
}

type AffiliateOperationsRank struct {
	Rank               int64  `json:"rank"`
	DisplayName        string `json:"display_name"`
	EffectivePaidCount int64  `json:"effective_paid_count"`
	IsMe               bool   `json:"is_me"`
	UserID             int64  `json:"user_id,omitempty"`
	Email              string `json:"email,omitempty"`
	Username           string `json:"username,omitempty"`
}

type AffiliateOperationsLeaderboard struct {
	Month     string                    `json:"month"`
	Timezone  string                    `json:"timezone"`
	Items     []AffiliateOperationsRank `json:"items"`
	MyRank    *AffiliateOperationsRank  `json:"my_rank"`
	UpdatedAt time.Time                 `json:"updated_at"`
}

type AffiliateOperationsPromoter struct {
	UserID             int64                      `json:"user_id"`
	Email              string                     `json:"email"`
	Username           string                     `json:"username"`
	AffCode            string                     `json:"aff_code"`
	InvitedCount       int64                      `json:"invited_count"`
	EffectivePaidCount int64                      `json:"effective_paid_count"`
	AvailableQuota     float64                    `json:"available_quota"`
	FrozenQuota        float64                    `json:"frozen_quota"`
	GrossEarned        *float64                   `json:"gross_earned"`
	NetEarned          *float64                   `json:"net_earned"`
	Currency           string                     `json:"currency"`
	Amounts            []AffiliateOperationsMoney `json:"amounts"`
}

type AffiliateOperationsAudit struct {
	ID        int64          `json:"id"`
	UserID    *int64         `json:"user_id,omitempty"`
	ActorID   *int64         `json:"actor_id,omitempty"`
	Action    string         `json:"action"`
	Detail    map[string]any `json:"detail"`
	CreatedAt time.Time      `json:"created_at"`
}

type AffiliateOperationsRepository interface {
	GetAffiliateOperationsSummary(context.Context, AffiliateOperationsQuery) (*AffiliateOperationsSummary, error)
	ListAffiliateOperationsInvitees(context.Context, AffiliateOperationsQuery) (*AffiliateOperationsPage[AffiliateOperationsInvitee], error)
	ListAffiliateOperationsEarnings(context.Context, AffiliateOperationsQuery) (*AffiliateOperationsPage[AffiliateOperationsEarning], error)
	GetAffiliateOperationsLeaderboard(context.Context, AffiliateOperationsQuery) (*AffiliateOperationsLeaderboard, error)
	ListAffiliateOperationsPromoters(context.Context, AffiliateOperationsQuery) (*AffiliateOperationsPage[AffiliateOperationsPromoter], error)
	ListAffiliateOperationsAudits(context.Context, AffiliateOperationsQuery) (*AffiliateOperationsPage[AffiliateOperationsAudit], error)
	ListAffiliateOperationsCampaigns(context.Context, AffiliateOperationsQuery) (*AffiliateOperationsPage[ReferralCampaignProgress], error)
}

func NormalizeAffiliateOperationsQuery(q AffiliateOperationsQuery, now time.Time) (AffiliateOperationsQuery, error) {
	if q.Month == "" {
		q.Month = now.In(time.FixedZone("Asia/Shanghai", 8*3600)).Format("2006-01")
	}
	month, err := time.Parse("2006-01", q.Month)
	if err != nil || month.Format("2006-01") != q.Month || q.UserID < 0 || q.InviteeID < 0 || q.SourceOrderID < 0 || q.CampaignID < 0 {
		return q, infraerrors.BadRequest("AFFILIATE_OPERATIONS_INVALID_FILTER", "invalid month or identity filter")
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = 20
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
	q.Search = strings.TrimSpace(q.Search)
	if q.Scope != "" && q.Scope != "joined" && q.Scope != "ended" {
		return q, infraerrors.BadRequest("AFFILIATE_OPERATIONS_INVALID_FILTER", "invalid campaign scope")
	}
	loc := time.FixedZone("Asia/Shanghai", 8*3600)
	if q.StartDate != "" {
		d, e := time.ParseInLocation("2006-01-02", q.StartDate, loc)
		if e != nil {
			return q, infraerrors.BadRequest("AFFILIATE_OPERATIONS_INVALID_FILTER", "invalid start date")
		}
		q.StartAt = &d
	}
	if q.EndDate != "" {
		d, e := time.ParseInLocation("2006-01-02", q.EndDate, loc)
		if e != nil {
			return q, infraerrors.BadRequest("AFFILIATE_OPERATIONS_INVALID_FILTER", "invalid end date")
		}
		d = d.AddDate(0, 0, 1)
		q.EndAt = &d
	}
	if q.StartAt != nil && q.EndAt != nil && !q.StartAt.Before(*q.EndAt) {
		return q, infraerrors.BadRequest("AFFILIATE_OPERATIONS_INVALID_FILTER", "invalid date range")
	}
	switch q.Status {
	case "", "pending_evidence", "unpaid", "effective", "refunded", "risk_excluded", "disabled", "exceptions":
	default:
		return q, infraerrors.BadRequest("AFFILIATE_OPERATIONS_INVALID_FILTER", "invalid invitation status")
	}
	switch q.Action {
	case "", "accrue", "revoke", "transfer", "withdraw":
	default:
		return q, infraerrors.BadRequest("AFFILIATE_OPERATIONS_INVALID_FILTER", "invalid ledger action")
	}
	return q, nil
}

// Operations exposes only the optional read model, keeping old integrations compatible.
func (s *AffiliateService) Operations() (AffiliateOperationsRepository, error) {
	if s != nil {
		if repo, ok := s.repo.(AffiliateOperationsRepository); ok {
			return repo, nil
		}
	}
	return nil, infraerrors.ServiceUnavailable("AFFILIATE_OPERATIONS_UNAVAILABLE", "affiliate operations are unavailable")
}

func MaskAffiliateOperationsName(value string) string {
	value = strings.TrimSpace(value)
	if i := strings.IndexByte(value, '@'); i >= 0 {
		value = value[:i]
	}
	r := []rune(value)
	if len(r) == 0 {
		return "***"
	}
	return string(r[0]) + "***"
}

// AffiliateOperationsQueryFromValues never trusts an ownership filter supplied
// by a regular user. Their authenticated subject is the only owner boundary.
func AffiliateOperationsQueryFromValues(values url.Values, userID int64, admin bool) (AffiliateOperationsQuery, error) {
	q := AffiliateOperationsQuery{UserID: userID, Admin: admin, Month: values.Get("month"), Status: values.Get("status"), Action: values.Get("action"), Search: values.Get("search"), Scope: values.Get("scope"), StartDate: values.Get("start_date"), EndDate: values.Get("end_date")}
	for key, dest := range map[string]*int{"page": &q.Page, "page_size": &q.PageSize} {
		if value := values.Get(key); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil {
				return q, infraerrors.BadRequest("AFFILIATE_OPERATIONS_INVALID_FILTER", "invalid pagination")
			}
			*dest = n
		}
	}
	ids := map[string]*int64{"campaign_id": &q.CampaignID}
	if admin {
		ids["user_id"] = &q.UserID
		ids["invitee_id"] = &q.InviteeID
		ids["source_order_id"] = &q.SourceOrderID
	}
	for key, dest := range ids {
		if value := values.Get(key); value != "" {
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil || n <= 0 {
				return q, infraerrors.BadRequest("AFFILIATE_OPERATIONS_INVALID_FILTER", "invalid identity filter")
			}
			*dest = n
		}
	}
	return NormalizeAffiliateOperationsQuery(q, time.Now())
}

func (s *AffiliateService) ReadAffiliateOperations(ctx context.Context, operation string, q AffiliateOperationsQuery) (any, error) {
	q, err := NormalizeAffiliateOperationsQuery(q, time.Now())
	if err != nil {
		return nil, err
	}
	if !q.Admin && q.UserID <= 0 {
		return nil, infraerrors.Forbidden("AFFILIATE_OPERATIONS_FORBIDDEN", "authenticated user required")
	}
	repo, err := s.Operations()
	if err != nil {
		return nil, err
	}
	switch operation {
	case "summary", "overview":
		return repo.GetAffiliateOperationsSummary(ctx, q)
	case "invitees":
		return repo.ListAffiliateOperationsInvitees(ctx, q)
	case "earnings":
		return repo.ListAffiliateOperationsEarnings(ctx, q)
	case "leaderboard":
		return repo.GetAffiliateOperationsLeaderboard(ctx, q)
	case "campaigns":
		if !q.Admin {
			return repo.ListAffiliateOperationsCampaigns(ctx, q)
		}
	case "promoters":
		if q.Admin {
			return repo.ListAffiliateOperationsPromoters(ctx, q)
		}
	case "audits":
		if q.Admin {
			return repo.ListAffiliateOperationsAudits(ctx, q)
		}
	case "exceptions":
		if q.Admin {
			if q.Status == "" {
				q.Status = "exceptions"
			}
			return repo.ListAffiliateOperationsInvitees(ctx, q)
		}
	}
	return nil, infraerrors.NotFound("AFFILIATE_OPERATIONS_NOT_FOUND", "unknown affiliate operation")
}
