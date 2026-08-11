//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// Covers the observation seams that let the forum mirror platform state. Signing
// and delivery of the resulting webhook is covered separately; these tests only
// assert that an observer is invoked with the right values, at the right time,
// and left alone when it should not fire.

type balanceChangeObserverStub struct {
	calls []balanceChangeObserverCall
}

type balanceChangeObserverCall struct {
	userID     int64
	newBalance float64
	change     float64
	reason     string
}

func (s *balanceChangeObserverStub) NotifyBalanceChanged(userID int64, newBalance, change float64, reason string) {
	s.calls = append(s.calls, balanceChangeObserverCall{
		userID:     userID,
		newBalance: newBalance,
		change:     change,
		reason:     reason,
	})
}

type vipChangeObserverStub struct {
	calls []vipChangeObserverCall
}

type vipChangeObserverCall struct {
	userID int64
	vip    PlayVIPStatus
	role   string
}

func (s *vipChangeObserverStub) NotifyVIPChanged(userID int64, vip PlayVIPStatus, role string) {
	s.calls = append(s.calls, vipChangeObserverCall{userID: userID, vip: vip, role: role})
}

type roleChangeObserverStub struct {
	calls []roleChangeObserverCall
}

type roleChangeObserverCall struct {
	userID  int64
	newRole string
}

func (s *roleChangeObserverStub) NotifyRoleChanged(userID int64, newRole string) {
	s.calls = append(s.calls, roleChangeObserverCall{userID: userID, newRole: newRole})
}

// expectRechargeApply queues one committed +25 recharge, mirroring the mock
// sequence the other ledger tests use.
func expectRechargeApply(mock sqlmock.Sqlmock, createdAt time.Time) {
	mock.ExpectBegin()
	mock.ExpectQuery(balanceLedgerSelectByKeyPattern()).
		WithArgs(int64(42), "payment_recharge:42:order-9101").
		WillReturnRows(balanceTransactionRows())
	mock.ExpectQuery("(?s)FROM users\\s+WHERE id = \\$1 AND deleted_at IS NULL\\s+FOR UPDATE").
		WithArgs(int64(42)).
		WillReturnRows(balanceLedgerUserStateRows().AddRow("10.00000000", "0.00000000", "0.00000000", "0.00000000"))
	expectWithdrawableSums(mock, 42, createdAt, "0.00000000", "0.00000000", "0.00000000")
	mock.ExpectExec("(?s)UPDATE users\\s+SET balance = \\$1,\\s+frozen_balance = \\$2,\\s+withdrawable_balance = \\$3,\\s+withdrawal_frozen_balance = \\$4").
		WithArgs("35.00000000", "0.00000000", "0.00000000", "0.00000000", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)INSERT INTO balance_transactions").
		WillReturnRows(balanceTransactionRows().AddRow(
			int64(9101), int64(42), 25.0, 10.0, 35.0, 0.0, 0.0, 0.0,
			0.0, 0.0, 0.0, 0.0, 0.0, 0.0,
			BalanceFlowTypePaymentRecharge, "order-9101", "payment_recharge:42:order-9101", "system", nil,
			"在线充值", `{}`, false, "high", createdAt,
		))
	expectFundBatchGrant(mock, 42, 9101, FundSourceKindOnlineRecharge, BalanceFlowTypePaymentRecharge, "order-9101", "25.00000000", true, createdAt)
	mock.ExpectCommit()
}

func applyRecharge(t *testing.T, svc *BalanceLedgerService) *BalanceTransaction {
	t.Helper()
	got, err := svc.ApplyDelta(context.Background(), BalanceLedgerApplyInput{
		UserID:         42,
		BalanceDelta:   25,
		SourceType:     BalanceFlowTypePaymentRecharge,
		SourceID:       "order-9101",
		IdempotencyKey: "payment_recharge:42:order-9101",
		Description:    "在线充值",
	})
	require.NoError(t, err)
	return got
}

func TestBalanceLedgerNotifiesObserverAfterCommit(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	observer := &balanceChangeObserverStub{}
	createdAt := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	svc := &BalanceLedgerService{db: db, now: func() time.Time { return createdAt }}
	svc.SetChangeObserver(observer)

	expectRechargeApply(mock, createdAt)
	applyRecharge(t, svc)

	require.Equal(t, []balanceChangeObserverCall{{
		userID:     42,
		newBalance: 35,
		change:     25,
		reason:     BalanceFlowTypePaymentRecharge,
	}}, observer.calls, "论坛应收到提交后的余额与变动额")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBalanceLedgerAppliesWithoutObserver(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	createdAt := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	svc := &BalanceLedgerService{db: db, now: func() time.Time { return createdAt }}

	// 论坛是可选卫星服务：未注册观察者不能影响主平台账变。
	expectRechargeApply(mock, createdAt)
	got := applyRecharge(t, svc)

	require.Equal(t, int64(9101), got.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBalanceLedgerSkipsObserverForUsageCharge(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	observer := &balanceChangeObserverStub{}
	createdAt := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	svc := &BalanceLedgerService{db: db, now: func() time.Time { return createdAt }}
	svc.SetChangeObserver(observer)

	// 逐请求计费在每次调用时都会触发。转发它等于给每个 API 请求外挂一次
	// 出站 HTTP 请求，而论坛只是展示余额，用户下次登录即可对齐。
	mock.ExpectBegin()
	mock.ExpectQuery(balanceLedgerSelectByKeyPattern()).
		WithArgs(int64(42), "usage:42:req-notify").
		WillReturnRows(balanceTransactionRows())
	mock.ExpectQuery("(?s)FROM users\\s+WHERE id = \\$1 AND deleted_at IS NULL\\s+FOR UPDATE").
		WithArgs(int64(42)).
		WillReturnRows(balanceLedgerUserStateRows().AddRow("10.00000000", "0.00000000", "0.00000000", "0.00000000"))
	expectWithdrawableSums(mock, 42, createdAt, "0.00000000", "0.00000000", "0.00000000")
	mock.ExpectQuery("(?s)FROM withdrawable_entitlements\\s+WHERE user_id = \\$1\\s+AND status = 'active'").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "remaining_amount", "available_at"}))
	mock.ExpectExec("(?s)UPDATE users\\s+SET balance = \\$1,\\s+frozen_balance = \\$2,\\s+withdrawable_balance = \\$3,\\s+withdrawal_frozen_balance = \\$4").
		WithArgs("9.50000000", "0.00000000", "0.00000000", "0.00000000", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)INSERT INTO balance_transactions").
		WillReturnRows(balanceTransactionRows().AddRow(
			int64(9102), int64(42), -0.5, 10.0, 9.5, 0.0, 0.0, 0.0,
			0.0, 0.0, 0.0, 0.0, 0.0, 0.0,
			BalanceFlowTypeUsageCharge, "req-notify", "usage:42:req-notify", "system", nil,
			"API 消耗扣费", `{}`, false, "high", createdAt,
		))
	expectEmptyFundBatchConsumption(mock, 42)
	mock.ExpectCommit()

	got, err := svc.ApplyDelta(context.Background(), BalanceLedgerApplyInput{
		UserID:         42,
		BalanceDelta:   -0.5,
		SourceType:     BalanceFlowTypeUsageCharge,
		SourceID:       "req-notify",
		IdempotencyKey: "usage:42:req-notify",
		Description:    "API 消耗扣费",
	})
	require.NoError(t, err)
	require.Equal(t, 9.5, *got.BalanceAfter, "扣费本身仍须照常落库")
	require.Empty(t, observer.calls, "逐请求计费不应外发论坛通知")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBalanceLedgerSkipsObserverOnIdempotentReplay(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	observer := &balanceChangeObserverStub{}
	createdAt := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	svc := &BalanceLedgerService{db: db, now: func() time.Time { return createdAt }}
	svc.SetChangeObserver(observer)

	// 重放只回放原交易、不搬动资金。再次通知会让论坛把同一笔变动记两次。
	mock.ExpectBegin()
	mock.ExpectQuery(balanceLedgerSelectByKeyPattern()).
		WithArgs(int64(42), "payment_recharge:42:order-9101").
		WillReturnRows(balanceTransactionRows().AddRow(
			int64(9101), int64(42), 25.0, 10.0, 35.0, 0.0, 0.0, 0.0,
			0.0, 0.0, 0.0, 0.0, 0.0, 0.0,
			BalanceFlowTypePaymentRecharge, "order-9101", "payment_recharge:42:order-9101", "system", nil,
			"在线充值", `{}`, false, "high", createdAt,
		))
	mock.ExpectCommit()

	got := applyRecharge(t, svc)

	require.Equal(t, int64(9101), got.ID)
	require.Empty(t, observer.calls, "幂等重放不应重复通知论坛")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBalanceLedgerApplyDeltaInSQLTxDoesNotNotify(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	observer := &balanceChangeObserverStub{}
	createdAt := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	svc := &BalanceLedgerService{db: db, now: func() time.Time { return createdAt }}
	svc.SetChangeObserver(observer)

	// 这条路径在调用方事务仍打开时就返回，此处通知可能announce一笔随后回滚的变动。
	expectRechargeApply(mock, createdAt)

	tx, err := db.Begin()
	require.NoError(t, err)
	got, err := svc.ApplyDeltaInSQLTx(context.Background(), tx, BalanceLedgerApplyInput{
		UserID:         42,
		BalanceDelta:   25,
		SourceType:     BalanceFlowTypePaymentRecharge,
		SourceID:       "order-9101",
		IdempotencyKey: "payment_recharge:42:order-9101",
		Description:    "在线充值",
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	require.Equal(t, int64(9101), got.ID)
	require.Empty(t, observer.calls, "调用方持有事务时不应由本服务通知")
	require.NoError(t, mock.ExpectationsWereMet())
}

// vipSyncMembershipRepo 只实现 PlayMembershipRepository，故意不实现可选的
// PlayMembershipAdminRepository，以此固定"论坛同步不依赖该断言"这一行为。
type vipSyncMembershipRepo struct {
	PlayRepository
	before float64
	after  float64
	synced int
}

func (r *vipSyncMembershipRepo) GetMembershipPaidTotal(context.Context, int64) (float64, error) {
	if r.synced == 0 {
		return r.before, nil
	}
	return r.after, nil
}

func (r *vipSyncMembershipRepo) SyncMembershipOrderContribution(context.Context, int64, int64, string, float64, float64, *time.Time, string) error {
	r.synced++
	return nil
}

func (r *vipSyncMembershipRepo) ListTeamLeaderboardBase(context.Context, time.Time, time.Time, int) ([]PlayTeamLeaderboardBase, error) {
	return nil, nil
}

func (r *vipSyncMembershipRepo) GetTeamLeaderboardRank(context.Context, int64, time.Time, time.Time) (int, int, decimal.Decimal, error) {
	return 0, 0, decimal.Zero, nil
}

func TestSyncMembershipOrderNotifiesForumOnTierUpgrade(t *testing.T) {
	t.Parallel()

	observer := &vipChangeObserverStub{}
	// 默认档位下 0 元为 V0、100 元为 V2。settingService 为 nil 时 GetRuntime 返回
	// 空 runtime，GetVIPTier 会回落到默认档位，因此边界依然是真实的。
	repo := &vipSyncMembershipRepo{before: 0, after: 100}
	svc := &PlayService{repo: repo, userRepo: &userRepoStub{user: &User{ID: 42, Role: RoleUser}}}
	svc.SetVIPChangeObserver(observer)

	err := svc.SyncMembershipOrder(context.Background(), 9101, 42, "recharge", 100, 0, nil, "paid")
	require.NoError(t, err)

	require.Len(t, observer.calls, 1, "跨档应通知论坛")
	require.Equal(t, int64(42), observer.calls[0].userID)
	require.Equal(t, 2, observer.calls[0].vip.Tier, "应上报变更后的目标档位")
	require.Equal(t, RoleUser, observer.calls[0].role)
}

func TestSyncMembershipOrderSkipsForumWhenTierUnchanged(t *testing.T) {
	t.Parallel()

	observer := &vipChangeObserverStub{}
	// 两个金额都落在 V1 区间（50–100），累计额变了但档位没变。
	repo := &vipSyncMembershipRepo{before: 50, after: 80}
	svc := &PlayService{repo: repo, userRepo: &userRepoStub{user: &User{ID: 42, Role: RoleUser}}}
	svc.SetVIPChangeObserver(observer)

	err := svc.SyncMembershipOrder(context.Background(), 9102, 42, "recharge", 30, 0, nil, "paid")
	require.NoError(t, err)

	require.Empty(t, observer.calls, "同档内充值不应外发论坛通知")
}

func TestPlayServiceNotifiesVIPChangeWithRole(t *testing.T) {
	t.Parallel()

	observer := &vipChangeObserverStub{}
	users := &userRepoStub{user: &User{ID: 42, Role: RoleAdmin}}
	svc := &PlayService{userRepo: users}
	svc.SetVIPChangeObserver(observer)

	status := PlayVIPStatus{Tier: 2, Label: "V2", RechargeBonusPct: 4}
	svc.notifyVIPChanged(context.Background(), 42, status)

	require.Equal(t, []vipChangeObserverCall{{userID: 42, vip: status, role: RoleAdmin}}, observer.calls)
}

func TestPlayServiceNotifiesVIPChangeWhenRoleLookupFails(t *testing.T) {
	t.Parallel()

	observer := &vipChangeObserverStub{}
	svc := &PlayService{userRepo: &userRepoStub{getErr: ErrUserNotFound}}
	svc.SetVIPChangeObserver(observer)

	// 等级才是论坛真正消费的字段。查角色失败就整条丢弃，会让论坛徽章一直停在
	// 旧等级，直到用户下次登录。
	svc.notifyVIPChanged(context.Background(), 42, PlayVIPStatus{Tier: 1, Label: "V1"})

	require.Len(t, observer.calls, 1)
	require.Equal(t, 1, observer.calls[0].vip.Tier)
	require.Empty(t, observer.calls[0].role)
}

func TestPlayServiceVIPNotifyWithoutObserverIsNoop(t *testing.T) {
	t.Parallel()

	users := &userRepoStub{user: &User{ID: 42, Role: RoleUser}}
	svc := &PlayService{userRepo: users}

	require.NotPanics(t, func() {
		svc.notifyVIPChanged(context.Background(), 42, PlayVIPStatus{Tier: 1})
	})
}

func TestAdminServiceUpdateUserNotifiesRoleChange(t *testing.T) {
	base := &userRepoStub{user: &User{ID: 42, Email: "u@example.com", Role: RoleUser}}
	repo := &rpmUserRepoStub{userRepoStub: base}
	observer := &roleChangeObserverStub{}
	svc := &adminServiceImpl{
		userRepo:             repo,
		redeemCodeRepo:       &redeemRepoStub{},
		authCacheInvalidator: &authCacheInvalidatorStub{},
	}
	svc.SetRoleChangeObserver(observer)

	updated, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{Role: RoleAdmin})
	require.NoError(t, err)
	require.Equal(t, RoleAdmin, updated.Role)

	require.Equal(t, []roleChangeObserverCall{{userID: 42, newRole: RoleAdmin}}, observer.calls,
		"平台管理员在论坛也应是管理员")
}

func TestAdminServiceUpdateUserSkipsRoleObserverWhenRoleUnchanged(t *testing.T) {
	base := &userRepoStub{user: &User{ID: 42, Email: "u@example.com", Role: RoleAdmin}}
	repo := &rpmUserRepoStub{userRepoStub: base}
	observer := &roleChangeObserverStub{}
	svc := &adminServiceImpl{userRepo: repo, redeemCodeRepo: &redeemRepoStub{}}
	svc.SetRoleChangeObserver(observer)

	newName := "renamed"
	_, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{Username: &newName})
	require.NoError(t, err)

	require.Empty(t, observer.calls, "仅改用户名不应触发论坛角色同步")
}

// roleUpdateFailingUserRepoStub 让角色写库失败，用于验证观察点位于持久化之后。
type roleUpdateFailingUserRepoStub struct {
	*rpmUserRepoStub
	err error
}

func (s *roleUpdateFailingUserRepoStub) Update(context.Context, *User, UserUpdateFields) error {
	return s.err
}

func TestAdminServiceUpdateUserSkipsRoleObserverWhenPersistFails(t *testing.T) {
	base := &userRepoStub{user: &User{ID: 42, Email: "u@example.com", Role: RoleUser}}
	repo := &roleUpdateFailingUserRepoStub{
		rpmUserRepoStub: &rpmUserRepoStub{userRepoStub: base},
		err:             errors.New("persist failed"),
	}
	observer := &roleChangeObserverStub{}
	svc := &adminServiceImpl{userRepo: repo, redeemCodeRepo: &redeemRepoStub{}}
	svc.SetRoleChangeObserver(observer)

	// 观察点在写库成功之后：写失败时论坛不能被告知一个并未生效的角色。
	_, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{Role: RoleAdmin})
	require.Error(t, err)
	require.Empty(t, observer.calls, "写库失败不应同步论坛角色")
}

func TestForumSSOServiceSatisfiesObserverInterfaces(t *testing.T) {
	t.Parallel()

	// ProvideForumSSOService 把同一个服务注册到三个观察者接口上，其中角色一路
	// 走运行时类型断言。这里静态断言实现仍然匹配，签名漂移会在此处失败，
	// 而不是在生产环境静默跳过同步。
	var (
		_ balanceLedgerChangeObserver = (*ForumSSOService)(nil)
		_ playVIPChangeObserver       = (*ForumSSOService)(nil)
		_ userRoleChangeObserver      = (*ForumSSOService)(nil)
	)

	// 未配置论坛时所有通知都应静默短路，nil 接收者不能 panic。
	var svc *ForumSSOService
	require.NotPanics(t, func() {
		svc.NotifyBalanceChanged(1, 2, 3, BalanceFlowTypePaymentRecharge)
		svc.NotifyVIPChanged(1, PlayVIPStatus{Tier: 1}, RoleUser)
		svc.NotifyRoleChanged(1, RoleAdmin)
	})
}
