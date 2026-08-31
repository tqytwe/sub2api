package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// MembershipReconciliationOptions controls the explicit historical review job.
// Execute is false by default so operators can inspect a report first.
type MembershipReconciliationOptions struct {
	Execute      bool
	AfterOrderID int64
	Limit        int
	ReviewerID   *int64
}

type MembershipReconciliationItem struct {
	OrderID     int64  `json:"order_id"`
	State       string `json:"state"`
	Reason      string `json:"reason"`
	OrderStatus string `json:"order_status"`
}

type MembershipReconciliationReport struct {
	Execute       bool                           `json:"execute"`
	Scanned       int                            `json:"scanned"`
	Verified      int                            `json:"verified"`
	PendingReview int                            `json:"pending_review"`
	Skipped       int                            `json:"skipped"`
	Items         []MembershipReconciliationItem `json:"items"`
}

type membershipReconciliationService struct {
	db *sql.DB
}

// NewMembershipReconciliationService creates a standalone, operator-invoked
// historical review service. It is intentionally not part of application boot.
func NewMembershipReconciliationService(db *sql.DB) *membershipReconciliationService {
	return &membershipReconciliationService{db: db}
}

type membershipReviewRow struct {
	orderID                 int64
	status                  string
	orderType               string
	currency                sql.NullString
	listAmount              sql.NullFloat64
	gatewayBaseAmount       sql.NullFloat64
	qualifyingRecharge      sql.NullFloat64
	amount                  sql.NullFloat64
	refundAmount            sql.NullFloat64
	subscriptionSnapshotRaw []byte
}

func (s *membershipReconciliationService) Reconcile(ctx context.Context, opts MembershipReconciliationOptions) (*MembershipReconciliationReport, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("membership reconciliation database is unavailable")
	}
	if opts.Limit <= 0 || opts.Limit > 5000 {
		opts.Limit = 500
	}
	report := &MembershipReconciliationReport{Execute: opts.Execute, Items: make([]MembershipReconciliationItem, 0)}
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.order_id, p.status, p.order_type, p.payment_currency,
		       p.list_amount, p.gateway_base_amount, p.qualifying_recharge_amount,
		       p.amount, p.refund_amount, p.subscription_snapshot
		FROM play_membership_verified_contributions c
		JOIN payment_orders p ON p.id = c.order_id
		WHERE c.qualification_state = 'pending_review'
		  AND c.order_id > $1
		ORDER BY c.order_id ASC
		LIMIT $2`, opts.AfterOrderID, opts.Limit)
	if err != nil {
		return nil, fmt.Errorf("list membership review rows: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var row membershipReviewRow
		if err := rows.Scan(&row.orderID, &row.status, &row.orderType, &row.currency, &row.listAmount, &row.gatewayBaseAmount, &row.qualifyingRecharge, &row.amount, &row.refundAmount, &row.subscriptionSnapshotRaw); err != nil {
			return nil, fmt.Errorf("scan membership review row: %w", err)
		}
		report.Scanned++
		item := MembershipReconciliationItem{OrderID: row.orderID, OrderStatus: row.status}
		if reason := membershipReviewReason(row); reason != "" {
			item.State = "pending_review"
			item.Reason = reason
			report.PendingReview++
			if opts.Execute {
				if err := s.markPendingReview(ctx, row.orderID, reason); err != nil {
					return nil, err
				}
			}
		} else {
			item.State = "verified"
			report.Verified++
			if opts.Execute {
				refundBusiness := 0.0
				if row.amount.Valid && row.amount.Float64 > 0 && row.refundAmount.Valid && row.refundAmount.Float64 > 0 {
					refundBusiness = row.qualifyingRecharge.Float64 * row.refundAmount.Float64 / row.amount.Float64
				}
				if refundBusiness > row.qualifyingRecharge.Float64 {
					refundBusiness = row.qualifyingRecharge.Float64
				}
				if err := s.markVerified(ctx, row, refundBusiness, opts.ReviewerID); err != nil {
					return nil, err
				}
			}
		}
		report.Items = append(report.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate membership review rows: %w", err)
	}
	return report, nil
}

func membershipReviewReason(row membershipReviewRow) string {
	status := strings.ToUpper(strings.TrimSpace(row.status))
	if status != "COMPLETED" && status != "REFUNDED" && status != "PARTIALLY_REFUNDED" {
		return "refund_or_payment_not_final"
	}
	if !strings.EqualFold(strings.TrimSpace(row.currency.String), "CNY") {
		return "missing_or_unsupported_currency"
	}
	if !row.listAmount.Valid || !row.gatewayBaseAmount.Valid || !row.qualifyingRecharge.Valid || row.listAmount.Float64 <= 0 || row.gatewayBaseAmount.Float64 < 0 || row.qualifyingRecharge.Float64 < 0 {
		return "missing_settlement_snapshot"
	}
	if row.orderType == "subscription" && len(row.subscriptionSnapshotRaw) == 0 {
		return "missing_subscription_snapshot"
	}
	return ""
}

func (s *membershipReconciliationService) markPendingReview(ctx context.Context, orderID int64, reason string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE play_membership_order_contributions SET qualification_state='pending_review', qualification_source='historical_review', qualification_reason=$2, reviewed_at=NULL, reviewed_by=NULL, updated_at=NOW() WHERE order_id=$1 AND qualification_state='pending_review'`, orderID, reason)
	if err != nil {
		return fmt.Errorf("mark membership order %d pending review: %w", orderID, err)
	}
	return nil
}

func (s *membershipReconciliationService) markVerified(ctx context.Context, row membershipReviewRow, refundBusiness float64, reviewerID *int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE play_membership_order_contributions SET paid_amount=$2, refund_amount=$3, net_amount=GREATEST($2-$3,0), status=$4, qualification_state='verified', qualification_source='historical_snapshot', qualification_reason=NULL, reviewed_at=NOW(), reviewed_by=$5, updated_at=NOW() WHERE order_id=$1 AND qualification_state='pending_review'`, row.orderID, row.qualifyingRecharge.Float64, refundBusiness, strings.ToUpper(strings.TrimSpace(row.status)), reviewerID)
	if err != nil {
		return fmt.Errorf("verify membership order %d: %w", row.orderID, err)
	}
	return nil
}
