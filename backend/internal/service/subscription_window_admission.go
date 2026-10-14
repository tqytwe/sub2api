package service

import (
	"context"
	"database/sql"
	"time"
)

// SubscriptionWindowLimitsTx uses the existing calendar and one-time-window
// rules under the caller's subscription row lock. It neither reserves quota nor
// changes the existing inclusive limit boundary.
func SubscriptionWindowLimitsTx(ctx context.Context, tx *sql.Tx, id int64) error {
	sub, group, now, err := subscriptionWindowSnapshot(ctx, tx, id)
	if err != nil {
		return err
	}
	oldDaily, oldWeekly, oldMonthly := sub.DailyWindowStart, sub.WeeklyWindowStart, sub.MonthlyWindowStart
	if start, ok := sub.automaticDailyWindowStartAt(now); ok {
		sub.DailyWindowStart = &start
		sub.DailyUsageUSD = 0
	}
	if sub.canAutomaticallyResetWeeklyAt(now) {
		start, _ := sub.automaticWindowStartAt(sub.WeeklyWindowStart, 7*24*time.Hour, now)
		sub.WeeklyWindowStart = &start
		sub.WeeklyUsageUSD = 0
	}
	if sub.canAutomaticallyResetMonthlyAt(now) {
		start, _ := sub.automaticWindowStartAt(sub.MonthlyWindowStart, 30*24*time.Hour, now)
		sub.MonthlyWindowStart = &start
		sub.MonthlyUsageUSD = 0
	}
	if err := subscriptionWindowLimitError(sub, group); err != nil {
		return err
	}
	if sub.DailyWindowStart != oldDaily || sub.WeeklyWindowStart != oldWeekly || sub.MonthlyWindowStart != oldMonthly {
		_, err = tx.ExecContext(ctx, `UPDATE user_subscriptions SET daily_window_start=$2,weekly_window_start=$3,monthly_window_start=$4,daily_usage_usd=$5,weekly_usage_usd=$6,monthly_usage_usd=$7,updated_at=NOW() WHERE id=$1`, id, sub.DailyWindowStart, sub.WeeklyWindowStart, sub.MonthlyWindowStart, sub.DailyUsageUSD, sub.WeeklyUsageUSD, sub.MonthlyUsageUSD)
	}
	return err
}

func subscriptionWindowLimitError(sub *UserSubscription, group *Group) error {
	if !sub.CheckDailyLimit(group, 0) {
		return ErrDailyLimitExceeded
	}
	if !sub.CheckWeeklyLimit(group, 0) {
		return ErrWeeklyLimitExceeded
	}
	if !sub.CheckMonthlyLimit(group, 0) {
		return ErrMonthlyLimitExceeded
	}
	return nil
}

func subscriptionWindowSnapshot(ctx context.Context, runner packageQuotaRunner, id int64) (*UserSubscription, *Group, time.Time, error) {
	rows, err := runner.QueryContext(ctx, `SELECT us.starts_at,us.expires_at,us.daily_window_start,us.weekly_window_start,us.monthly_window_start,
 us.daily_usage_usd,us.weekly_usage_usd,us.monthly_usage_usd,g.daily_limit_usd,g.weekly_limit_usd,g.monthly_limit_usd,clock_timestamp()
 FROM user_subscriptions us JOIN groups g ON g.id=us.group_id AND g.deleted_at IS NULL WHERE us.id=$1 FOR UPDATE OF us`, id)
	if err != nil {
		return nil, nil, time.Time{}, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, nil, time.Time{}, err
		}
		return nil, nil, time.Time{}, ErrSubscriptionNotFound
	}
	sub, group := &UserSubscription{}, &Group{}
	var now time.Time
	err = rows.Scan(&sub.StartsAt, &sub.ExpiresAt, &sub.DailyWindowStart, &sub.WeeklyWindowStart, &sub.MonthlyWindowStart, &sub.DailyUsageUSD, &sub.WeeklyUsageUSD, &sub.MonthlyUsageUSD, &group.DailyLimitUSD, &group.WeeklyLimitUSD, &group.MonthlyLimitUSD, &now)
	return sub, group, now, err
}

func subscriptionEffectiveQuotaAvailable(sub *UserSubscription, group *Group, now time.Time) bool {
	if sub.canAutomaticallyResetDailyAt(now) {
		sub.DailyUsageUSD = 0
	}
	if sub.canAutomaticallyResetWeeklyAt(now) {
		sub.WeeklyUsageUSD = 0
	}
	if sub.canAutomaticallyResetMonthlyAt(now) {
		sub.MonthlyUsageUSD = 0
	}
	return subscriptionWindowLimitError(sub, group) == nil
}
