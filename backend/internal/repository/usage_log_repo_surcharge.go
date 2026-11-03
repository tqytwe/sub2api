package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

func (r *usageLogRepository) GetBillingSurchargeReport(ctx context.Context, startTime, endTime, todayStart time.Time, reportParams usagestats.BillingSurchargeReportParams) (*usagestats.BillingSurchargeReport, error) {
	params := reportParams.Pagination
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}
	limit := params.Limit()
	todayEnd := todayStart.AddDate(0, 0, 1)
	search := strings.TrimSpace(reportParams.Search)

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
	where, whereArgs := billingSurchargeReportWhere(startTime, endTime, search)
	countQuery := `
		SELECT COUNT(*)
		FROM usage_logs ul
		LEFT JOIN users u ON u.id = ul.user_id
		LEFT JOIN api_keys ak ON ak.id = ul.api_key_id
		LEFT JOIN groups g ON g.id = ul.group_id
		WHERE ` + where
	if err := scanSingleRow(ctx, r.sql, countQuery, whereArgs, &total); err != nil {
		return nil, err
	}
	sortColumn := billingSurchargeSortColumn(params.SortBy)
	sortOrder := params.NormalizedSortOrder("desc")
	whereArgs = append(whereArgs, limit, params.Offset())
	limitPlaceholder, offsetPlaceholder := len(whereArgs)-1, len(whereArgs)

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
		WHERE ` + where + fmt.Sprintf(" ORDER BY %s %s, ul.id DESC LIMIT $%d OFFSET $%d", sortColumn, sortOrder, limitPlaceholder, offsetPlaceholder)
	rows, err := r.sql.QueryContext(ctx, detailQuery, whereArgs...)
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

func billingSurchargeReportWhere(startTime, endTime time.Time, search string) (string, []any) {
	where := "ul.billing_surcharge_cost > 0 AND ul.created_at >= $1 AND ul.created_at < $2"
	args := []any{startTime, endTime}
	if search != "" {
		args = append(args, "%"+strings.TrimSpace(search)+"%")
		where += " AND " + billingSurchargeSearchClause("ul", len(args))
	}
	return where, args
}

func billingSurchargeSearchClause(alias string, placeholder int) string {
	value := fmt.Sprintf("$%d", placeholder)
	return fmt.Sprintf("(u.email ILIKE %[1]s OR ak.name ILIKE %[1]s OR %[2]s.model ILIKE %[1]s OR COALESCE(g.name, '') ILIKE %[1]s OR %[2]s.request_id ILIKE %[1]s OR %[2]s.id::text ILIKE %[1]s)", value, alias)
}

func billingSurchargeSortColumn(value string) string {
	switch value {
	case "user":
		return "COALESCE(u.email, '')"
	case "model":
		return "ul.model"
	case "actual_cost":
		return "ul.actual_cost"
	case "billing_surcharge_cost":
		return "ul.billing_surcharge_cost"
	case "billed_cost":
		return usageLogChargedCostExprUL
	default:
		return "ul.created_at"
	}
}
