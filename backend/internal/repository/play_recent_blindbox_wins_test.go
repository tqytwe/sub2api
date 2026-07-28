package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestListRecentBlindboxWinsIncludesBalanceAndCouponRewards(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	createdAt := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)FROM play_blindbox_opens b.*UNION ALL.*FROM coupon_reward_draws d.*ORDER BY created_at DESC.*LIMIT \$1`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"user_label", "reward_amount", "reward_type", "coupon_name", "created_at"}).
			AddRow("coupon-winner", 0.0, service.PlayRewardTypeCoupon, "充值20减1", createdAt.Add(time.Minute)).
			AddRow("cash-winner", 9.0, service.PlayRewardTypeBalance, "", createdAt))

	repo := NewPlayRepository(nil, db)
	wins, err := repo.ListRecentBlindboxWins(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, []service.PlayBlindboxRecentWin{{
		UserLabel:    "coupon-winner",
		RewardAmount: 0,
		RewardType:   service.PlayRewardTypeCoupon,
		CouponName:   "充值20减1",
		CreatedAt:    createdAt.Add(time.Minute),
	}, {
		UserLabel:    "cash-winner",
		RewardAmount: 9,
		RewardType:   service.PlayRewardTypeBalance,
		CreatedAt:    createdAt,
	}}, wins)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFindBlindboxOpenByIdempotencyScopesTheReadToItsOwner(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	openedAt := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)SELECT b\.open_date, b\.cost_amount, b\.reward_amount, b\.idempotency_key, b\.pool_version, b\.open_source.*FROM play_blindbox_opens b.*LEFT JOIN play_reward_ledger.*user_id = \$1 AND b\.idempotency_key = \$2`).
		WithArgs(int64(42), "blindbox:42:hash", service.PlayRewardSourceBlindbox).
		WillReturnRows(sqlmock.NewRows([]string{
			"open_date", "cost_amount", "reward_amount", "idempotency_key", "pool_version", "open_source", "detail",
		}).AddRow(openedAt, 0.5, 9.0, "blindbox:42:hash", "season-1-v1", "paid", `{
"vip_tier_snapshot":{"tier":3,"label":"V3","recharge_bonus_pct":6,"color_key":"indigo","perks":["blindbox_pool_upgrade"],"next_tier":4,"next_label":"V4","next_min_recharge":500,"amount_to_next":300},
"expected_reward":0.45,"rtp_cap":0.9}`))

	repo := NewPlayRepository(nil, db)
	record, err := repo.FindBlindboxOpenByIdempotency(context.Background(), 42, "blindbox:42:hash")

	require.NoError(t, err)
	expectedReward := 0.45
	rtpCap := 0.9
	require.Equal(t, &service.PlayBlindboxOpenRecord{
		UserID:         42,
		Date:           openedAt,
		Cost:           0.5,
		Reward:         9,
		IdempotencyKey: "blindbox:42:hash",
		PoolVersion:    "season-1-v1",
		OpenSource:     "paid",
		VIPTierSnapshot: &service.PlayVIPStatus{
			Tier:             3,
			Label:            "V3",
			RechargeBonusPct: 6,
			ColorKey:         "indigo",
			Perks:            []string{"blindbox_pool_upgrade"},
			NextTier:         4,
			NextLabel:        "V4",
			NextMinRecharge:  500,
			AmountToNext:     300,
		},
		ExpectedReward: &expectedReward,
		RTPCap:         &rtpCap,
	}, record)
	require.NoError(t, mock.ExpectationsWereMet())
}
