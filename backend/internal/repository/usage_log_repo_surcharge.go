package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

func (r *usageLogRepository) GetBillingSurchargeReport(ctx context.Context, startTime, endTime, todayStart time.Time, params pagination.PaginationParams) (*usagestats.BillingSurchargeReport, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}
	limit := params.Limit()
	todayEnd := todayStart.AddDate(0, 0, 1)

	summary := usagestats.BillingSurchargeSummary{}
	summaryQuery := `
		SELECT
			COALESCE(SUM(billing_surcharge_cost) FILTER (WHERE billing_surcharge_cost > 0), 0) AS total_surcharge_cost,
			COALESCE(SUM(billing_surcharge_cost) FILTER (WHERE billing_surcharge_cost > 0 AND created_at >= $1 AND created_at < $2), 0) AS today_surcharge_cost,
			COALESCE(SUM(billing_surcharge_cost) FILTER (WHERE billing_surcharge_cost > 0 AND created_at >= $3 AND created_at < $4), 0) AS range_surcharge_cost,
			COALESCE(SUM(` + usageLogChargedCostExpr + `) FILTER (WHERE billing_surcharge_cost > 0), 0) AS total_billed_cost,
			COALESCE(SUM(` + usageLogChargedCostExpr + `) FILTER (WHERE billing_surcharge_cost > 0 AND created_at >= $1 AND created_at < $2), 0) AS today_billed_cost,
			COALESCE(SUM(` + usageLogChargedCostExpr + `) FILTER (WHERE billing_surcharge_cost > 0 AND created_at >= $3 AND created_at < $4), 0) AS range_billed_cost,
			COUNT(*) FILTER (WHERE billing_surcharge_cost > 0) AS total_requests,
			COUNT(*) FILTER (WHERE billing_surcharge_cost > 0 AND created_at >= $1 AND created_at < $2) AS today_requests,
			COUNT(*) FILTER (WHERE billing_surcharge_cost > 0 AND created_at >= $3 AND created_at < $4) AS range_requests
		FROM usage_logs
	`
	if err := scanSingleRow(
		ctx,
		r.sql,
		summaryQuery,
		[]any{todayStart, todayEnd, startTime, endTime},
		&summary.TotalSurchargeCost,
		&summary.TodaySurchargeCost,
		&summary.RangeSurchargeCost,
		&summary.TotalBilledCost,
		&summary.TodayBilledCost,
		&summary.RangeBilledCost,
		&summary.TotalRequests,
		&summary.TodayRequests,
		&summary.RangeRequests,
	); err != nil {
		return nil, err
	}

	var total int64
	countQuery := `
		SELECT COUNT(*)
		FROM usage_logs
		WHERE billing_surcharge_cost > 0
		  AND created_at >= $1
		  AND created_at < $2
	`
	if err := scanSingleRow(ctx, r.sql, countQuery, []any{startTime, endTime}, &total); err != nil {
		return nil, err
	}

	detailQuery := `
		SELECT
			ul.id,
			ul.request_id,
			ul.user_id,
			u.email,
			ul.api_key_id,
			ak.name,
			ul.account_id,
			ul.group_id,
			g.name,
			ul.model,
			ul.actual_cost,
			ul.billing_surcharge_cost,
			` + usageLogChargedCostExprUL + ` AS billed_cost,
			ul.billing_surcharge_mode,
			ul.billing_surcharge_value,
			ul.created_at
		FROM usage_logs ul
		LEFT JOIN users u ON u.id = ul.user_id
		LEFT JOIN api_keys ak ON ak.id = ul.api_key_id
		LEFT JOIN groups g ON g.id = ul.group_id
		WHERE ul.billing_surcharge_cost > 0
		  AND ul.created_at >= $1
		  AND ul.created_at < $2
		ORDER BY ul.created_at DESC, ul.id DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.sql.QueryContext(ctx, detailQuery, startTime, endTime, limit, params.Offset())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]usagestats.BillingSurchargeDetail, 0, limit)
	for rows.Next() {
		var item usagestats.BillingSurchargeDetail
		var userEmail sql.NullString
		var apiKeyName sql.NullString
		var groupID sql.NullInt64
		var groupName sql.NullString
		if err := rows.Scan(
			&item.ID,
			&item.RequestID,
			&item.UserID,
			&userEmail,
			&item.APIKeyID,
			&apiKeyName,
			&item.AccountID,
			&groupID,
			&groupName,
			&item.Model,
			&item.ActualCost,
			&item.BillingSurchargeCost,
			&item.BilledCost,
			&item.BillingSurchargeMode,
			&item.BillingSurchargeValue,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		if userEmail.Valid {
			value := userEmail.String
			item.UserEmail = &value
		}
		if apiKeyName.Valid {
			value := apiKeyName.String
			item.APIKeyName = &value
		}
		if groupID.Valid {
			value := groupID.Int64
			item.GroupID = &value
		}
		if groupName.Valid {
			value := groupName.String
			item.GroupName = &value
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	pages := int((total + int64(limit) - 1) / int64(limit))
	if pages < 1 {
		pages = 1
	}
	return &usagestats.BillingSurchargeReport{
		Summary: summary,
		Items:   items,
		Pagination: &pagination.PaginationResult{
			Total:    total,
			Page:     params.Page,
			PageSize: limit,
			Pages:    pages,
		},
	}, nil
}
