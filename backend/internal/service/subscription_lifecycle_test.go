package service

import (
	"regexp"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// Existing term/revoke tests retain their original mutation and rollback
// expectations; these additional expectations cover the new preceding event.
func expectSubscriptionLifecycle(mock sqlmock.Sqlmock, id, userID, groupID int64, kind string) {
	mock.ExpectQuery(`(?s)SELECT id,status,expires_at,.*FROM subscription_package_entitlements.*ORDER BY id FOR UPDATE`).WithArgs(userID, groupID).WillReturnRows(sqlmock.NewRows([]string{"id", "status", "expires_at", "quota_available"}))
	now := time.Now()
	mock.ExpectQuery(`(?s)SELECT us.starts_at,us.expires_at,.*FROM user_subscriptions us JOIN groups.*FOR UPDATE OF us`).WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"starts_at", "expires_at", "daily_window_start", "weekly_window_start", "monthly_window_start", "daily_usage_usd", "weekly_usage_usd", "monthly_usage_usd", "daily_limit_usd", "weekly_limit_usd", "monthly_limit_usd", "now"}).AddRow(now, now.Add(24*time.Hour), nil, nil, nil, 0, 0, 0, nil, nil, nil, now))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO gateway_subscription_events")).WithArgs(id, userID, groupID, kind, sqlmock.AnyArg(), sqlmock.AnyArg(), "[]", true).WillReturnResult(sqlmock.NewResult(1, 1))
}
