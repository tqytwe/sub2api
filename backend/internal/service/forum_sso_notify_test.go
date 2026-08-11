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

// ---------------------------------------------------------------------------
// Fix #5: deliverAsync 重试 context — 只做静态断言，实际 HTTP 交互在
// forum_sso_sign_test.go 的跨实现 pin 测试中覆盖。
// ---------------------------------------------------------------------------

func TestDeliverAsyncContextIsWideEnoughForAllAttempts(t *testing.T) {
	t.Parallel()
	// 总预算 = maxAttempts × (httpTimeout + 2s)，必须 > httpTimeout（单次）。
	total := time.Duration(forumWebhookMaxAttempts) * (forumWebhookTimeout + 2*time.Second)
	require.Greater(t, total, forumWebhookTimeout,
		"每次重试都应有足够的 context 预算")
	// 应为两次完整超时 + margin，而不仅仅是一次超时加一点儿。
	require.GreaterOrEqual(t, total, 2*forumWebhookTimeout,
		"重试预算应覆盖至少两次完整请求超时")
}

// ---------------------------------------------------------------------------
// Fix #6: AlreadyPaid — BalanceTransaction.Replayed 标志在幂等重放时设置。
// ---------------------------------------------------------------------------

func TestBalanceLedgerIdempotentReplaySetsReplayedFlag(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	createdAt := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	svc := &BalanceLedgerService{db: db, now: func() time.Time { return createdAt }}

	// 首次应用
	expectRechargeApply(mock, createdAt)
	first := applyRecharge(t, svc)
	require.False(t, first.Replayed, "首次应用不是重放")

	// 幂等重放：同一 idempotency key 返回已有交易
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

	second := applyRecharge(t, svc)
	require.True(t, second.Replayed, "幂等重放应设置 Replayed = true")
	require.Equal(t, first.ID, second.ID, "重放应返回同一交易 ID")
	require.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// Fix #4: 封号/删除吊销 SSO token —— 使用 tokenRevokerStub 验证接缝。
// ---------------------------------------------------------------------------

type tokenRevokerStub struct {
	revokedUserIDs []int64
	err            error
}

func (s *tokenRevokerStub) RevokeUserTokens(_ context.Context, userID int64) error {
	s.revokedUserIDs = append(s.revokedUserIDs, userID)
	return s.err
}

// waitForRevocations 等待 tokenRevokerStub 收到 n 次吊销，超时则 fail。
func waitForRevocations(t *testing.T, stub *tokenRevokerStub, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(stub.revokedUserIDs) >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected %d token revocations, got %d", n, len(stub.revokedUserIDs))
}

func TestAdminServiceRevokesForumTokenOnDisable(t *testing.T) {
	base := &userRepoStub{user: &User{ID: 42, Email: "u@example.com", Role: RoleUser, Status: StatusActive}}
	repo := &rpmUserRepoStub{userRepoStub: base}
	revoker := &tokenRevokerStub{}
	svc := &adminServiceImpl{
		userRepo:       repo,
		redeemCodeRepo: &redeemRepoStub{},
	}
	svc.SetTokenRevoker(revoker)

	disabled := StatusDisabled
	_, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{Status: disabled})
	require.NoError(t, err)

	waitForRevocations(t, revoker, 1)
	require.Equal(t, []int64{42}, revoker.revokedUserIDs, "封号应吊销论坛 token")
}

func TestAdminServiceDoesNotRevokeTokenWhenStatusUnchanged(t *testing.T) {
	base := &userRepoStub{user: &User{ID: 42, Email: "u@example.com", Role: RoleUser, Status: StatusActive}}
	repo := &rpmUserRepoStub{userRepoStub: base}
	revoker := &tokenRevokerStub{}
	svc := &adminServiceImpl{
		userRepo:       repo,
		redeemCodeRepo: &redeemRepoStub{},
	}
	svc.SetTokenRevoker(revoker)

	// 仅改用户名，状态不变
	newName := "renamed"
	_, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{Username: &newName})
	require.NoError(t, err)

	// 等待一段时间确认没有异步吊销
	time.Sleep(50 * time.Millisecond)
	require.Empty(t, revoker.revokedUserIDs, "仅改名不应吊销论坛 token")
}

func TestAdminServiceDoesNotRevokeTokenWhenReenabling(t *testing.T) {
	// 恢复账号（disabled → active）不应触发吊销
	base := &userRepoStub{user: &User{ID: 42, Email: "u@example.com", Role: RoleUser, Status: StatusDisabled}}
	repo := &rpmUserRepoStub{userRepoStub: base}
	revoker := &tokenRevokerStub{}
	svc := &adminServiceImpl{
		userRepo:       repo,
		redeemCodeRepo: &redeemRepoStub{},
	}
	svc.SetTokenRevoker(revoker)

	_, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{Status: StatusActive})
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	require.Empty(t, revoker.revokedUserIDs, "解封不应吊销论坛 token")
}

func TestAdminServiceRevokesForumTokenOnDelete(t *testing.T) {
	repo := &userRepoStub{user: &User{ID: 55, Role: RoleUser}}
	revoker := &tokenRevokerStub{}
	svc := &adminServiceImpl{
		userRepo:       repo,
		redeemCodeRepo: &redeemRepoStub{},
	}
	svc.SetTokenRevoker(revoker)

	err := svc.DeleteUser(context.Background(), 55)
	require.NoError(t, err)

	waitForRevocations(t, revoker, 1)
	require.Equal(t, []int64{55}, revoker.revokedUserIDs, "删除账号应吊销论坛 token")
}

func TestAdminServiceNoTokenRevokerIsNoop(t *testing.T) {
	// 未注册 tokenRevoker 时不能 panic
	base := &userRepoStub{user: &User{ID: 42, Email: "u@example.com", Role: RoleUser, Status: StatusActive}}
	repo := &rpmUserRepoStub{userRepoStub: base}
	svc := &adminServiceImpl{userRepo: repo, redeemCodeRepo: &redeemRepoStub{}}

	disabled := StatusDisabled
	require.NotPanics(t, func() {
		_, _ = svc.UpdateUser(context.Background(), 42, &UpdateUserInput{Status: disabled})
	})
}

// ---------------------------------------------------------------------------
// Fix #7: PublishVIPConfig 批量通知论坛。
// ---------------------------------------------------------------------------

// publishVIPConfigAdminRepo 实现 PlayMembershipAdminRepository + PlayRepository
// 只提供 PublishVIPConfig 路径所需的方法，其余全部 panic。
type publishVIPConfigAdminRepo struct {
	vipSyncMembershipRepo                          // embeds base PlayRepository stubs
	version                int64
	totals                 map[int64]decimal.Decimal
	publishedVersion       int64
}

func (r *publishVIPConfigAdminRepo) GetVIPConfigVersion(_ context.Context) (int64, error) {
	return r.version, nil
}

func (r *publishVIPConfigAdminRepo) ListMembershipPaidTotals(_ context.Context) (map[int64]decimal.Decimal, error) {
	return r.totals, nil
}

func (r *publishVIPConfigAdminRepo) PublishVIPConfig(_ context.Context, expectedVersion, _ int64, _, _ string, _, _, _ int, _ []PlayMembershipTierChange) (int64, error) {
	r.publishedVersion = expectedVersion + 1
	return r.publishedVersion, nil
}

// Stub methods required by PlayMembershipAdminRepository interface but not needed for this test path.
func (r *publishVIPConfigAdminRepo) MembershipAdminOverview(_ context.Context, _ float64) (int, decimal.Decimal, error) {
	return 0, decimal.Zero, nil
}
func (r *publishVIPConfigAdminRepo) ListMembershipAdminRows(_ context.Context, _ string, _ *bool, _ float64, _, _ int) ([]PlayMembershipAdminRow, int, error) {
	return nil, 0, nil
}
func (r *publishVIPConfigAdminRepo) GetMembershipAdminRow(_ context.Context, _ int64) (*PlayMembershipAdminRow, error) {
	return nil, nil
}
func (r *publishVIPConfigAdminRepo) ListMembershipContributions(_ context.Context, _ int64, _ int) ([]PlayMembershipContribution, error) {
	return nil, nil
}
func (r *publishVIPConfigAdminRepo) ListMembershipTierHistory(_ context.Context, _ int64, _ int) ([]PlayMembershipTierChange, error) {
	return nil, nil
}
func (r *publishVIPConfigAdminRepo) CountRecentMembershipTierChanges(_ context.Context, _ time.Time) (int, int, error) {
	return 0, 0, nil
}
func (r *publishVIPConfigAdminRepo) RecordMembershipTierChange(_ context.Context, _ PlayMembershipTierChange) error {
	return nil
}

func TestPublishVIPConfigNotifiesForumForAffectedUsers(t *testing.T) {
	t.Parallel()

	observer := &vipChangeObserverStub{}
	// user 1: 100 元 → V2 under default tiers; user 2: 10 元 → V0 (unchanged in new cfg).
	// New tiers move V1 threshold from 50→80, so user 1 stays V2, user 2 stays V0.
	// To get a tier change: user 3 at 60 元 goes V1→V0 when V1 threshold moves to 80.
	totals := map[int64]decimal.Decimal{
		1: decimal.NewFromFloat(100), // V2 → V2 (unchanged)
		2: decimal.NewFromFloat(10),  // V0 → V0 (unchanged)
		3: decimal.NewFromFloat(60),  // V1 → V0 (downgraded)
	}
	repo := &publishVIPConfigAdminRepo{
		version: 1,
		totals:  totals,
	}
	svc := &PlayService{
		repo:     repo,
		userRepo: &userRepoStub{user: &User{ID: 3, Role: RoleUser}},
	}
	svc.SetVIPChangeObserver(observer)

	// New tiers: V1 threshold = 80 (was 50), so user 3 at 60 moves V1 → V0.
	newTiers := []PlayVIPTier{
		{Tier: 0, Label: "V0", MinRecharge: 0, RechargeBonusPct: 0, ColorKey: "neutral"},
		{Tier: 1, Label: "V1", MinRecharge: 80, RechargeBonusPct: 2, ColorKey: "emerald"},
		{Tier: 2, Label: "V2", MinRecharge: 100, RechargeBonusPct: 4, ColorKey: "sky"},
	}
	_, err := svc.PublishVIPConfig(context.Background(), newTiers, 1, 99,
		"raise V1 threshold for testing purposes ok")
	require.NoError(t, err)

	require.Len(t, observer.calls, 1, "应通知受档位影响的用户（user 3）")
	require.Equal(t, int64(3), observer.calls[0].userID)
	require.Equal(t, 0, observer.calls[0].vip.Tier, "user 3 应降到 V0")
}

func TestPublishVIPConfigSkipsForumWhenNoUsersAffected(t *testing.T) {
	t.Parallel()

	observer := &vipChangeObserverStub{}
	// totals empty → no users to affect
	repo := &publishVIPConfigAdminRepo{version: 1, totals: map[int64]decimal.Decimal{}}
	svc := &PlayService{repo: repo}
	svc.SetVIPChangeObserver(observer)

	_, err := svc.PublishVIPConfig(context.Background(), defaultPlayVIPTiers(), 1, 99,
		"no users in the system at all this is a test")
	require.NoError(t, err)
	require.Empty(t, observer.calls, "无受影响用户时不应通知论坛")
}

// ---------------------------------------------------------------------------
// Fix #8: retryBackoff — 指数退避不超过上限。
// ---------------------------------------------------------------------------

func TestRetryBackoffExponentialCappedAtMax(t *testing.T) {
	t.Parallel()

	prev := retryBackoff(1)
	require.Equal(t, forumPaymentRetryInitial, prev, "第一次重试应使用初始间隔")

	for attempt := 2; attempt <= 10; attempt++ {
		d := retryBackoff(attempt)
		require.LessOrEqual(t, d, forumPaymentRetryMax, "退避不应超过上限")
		if prev < forumPaymentRetryMax {
			require.Greater(t, d, prev, "未到上限前应递增")
		}
		prev = d
	}
}

func TestForumSSOServiceSatisfiesForumTokenRevoker(t *testing.T) {
	t.Parallel()
	// ForumSSOService 现在同时实现三个观察者接口和 forumTokenRevoker。
	var _ forumTokenRevoker = (*ForumSSOService)(nil)
}

