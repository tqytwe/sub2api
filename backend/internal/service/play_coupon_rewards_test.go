package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

type playCouponRewardIssuer struct {
	result          *CouponRewardIssueResult
	err             error
	poolUnavailable bool
	poolErr         error
	requests        []CouponRewardDrawRequest
	replayResult    *CouponRewardIssueResult
	replayErr       error
	replayKeys      []string
	inTx            bool
}

type playCouponReadinessIssuer struct {
	*playCouponRewardIssuer
	ready    bool
	readyErr error
	checked  bool
}

func (i *playCouponReadinessIssuer) CouponRewardPoolReady(_ context.Context, _ CouponRewardActivity) (bool, error) {
	i.checked = true
	return i.ready, i.readyErr
}

func (i *playCouponRewardIssuer) DrawAndIssueInTx(ctx context.Context, request CouponRewardDrawRequest) (*CouponRewardIssueResult, error) {
	i.inTx = dbent.TxFromContext(ctx) != nil
	i.requests = append(i.requests, request)
	if i.err != nil {
		return nil, i.err
	}
	return i.result, nil
}

func (i *playCouponRewardIssuer) GetCouponRewardIssueByIdempotency(_ context.Context, _ int64, idempotencyKey string) (*CouponRewardIssueResult, error) {
	i.replayKeys = append(i.replayKeys, idempotencyKey)
	if i.replayErr != nil {
		return nil, i.replayErr
	}
	return i.replayResult, nil
}

func (i *playCouponRewardIssuer) GetPublishedRewardPool(_ context.Context, activity CouponRewardActivity) (*CouponRewardPoolVersion, error) {
	if i.poolErr != nil {
		return nil, i.poolErr
	}
	if i.poolUnavailable {
		return nil, ErrCouponRewardPoolUnavailable
	}
	return &CouponRewardPoolVersion{
		Activity: activity,
		Status:   CouponRewardPoolStatusPublished,
	}, nil
}

type playCouponQuizRepo struct {
	PlayRepository
	questions      []PlayQuizQuestionDB
	attempt        *PlayQuizAttemptDB
	inserted       []PlayQuizAttemptDB
	insertedInTx   bool
	insertErr      error
	ledgerEntries  []PlayRewardLedgerEntry
	balanceUpdates []float64
}

func (r *playCouponQuizRepo) ListQuizQuestions(context.Context, string) ([]PlayQuizQuestionDB, error) {
	return append([]PlayQuizQuestionDB(nil), r.questions...), nil
}

func (r *playCouponQuizRepo) GetQuizAttempt(context.Context, int64, time.Time) (*PlayQuizAttemptDB, error) {
	return r.attempt, nil
}

func (r *playCouponQuizRepo) InsertQuizAttempt(ctx context.Context, _ int64, _ time.Time, score, total int, reward float64, _ map[string]any) error {
	r.insertedInTx = dbent.TxFromContext(ctx) != nil
	if r.insertErr != nil {
		return r.insertErr
	}
	if r.attempt != nil {
		return ErrPlayQuizAlreadyDone
	}
	attempt := PlayQuizAttemptDB{Score: score, Total: total, RewardAmount: reward}
	r.attempt = &attempt
	r.inserted = append(r.inserted, attempt)
	return nil
}

func (r *playCouponQuizRepo) InsertRewardLedger(_ context.Context, entry PlayRewardLedgerEntry) error {
	r.ledgerEntries = append(r.ledgerEntries, entry)
	return nil
}

func (r *playCouponQuizRepo) UpdatePlayBalance(_ context.Context, _ int64, amount float64) error {
	r.balanceUpdates = append(r.balanceUpdates, amount)
	return nil
}

func TestCouponRewardBranchWeightsAreFixedByActivity(t *testing.T) {
	t.Parallel()

	cases := []struct {
		activity     CouponRewardActivity
		couponDraws  int
		balanceDraws int
		boundaryDraw int64
	}{
		{activity: CouponRewardActivityBlindbox, couponDraws: 6000, balanceDraws: 4000, boundaryDraw: 6000},
		{activity: CouponRewardActivityQuiz, couponDraws: 8000, balanceDraws: 2000, boundaryDraw: 8000},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.activity), func(t *testing.T) {
			t.Parallel()
			couponCount := 0
			balanceCount := 0
			for draw := int64(0); draw < couponWeightBasisPoints; draw++ {
				rewardType, err := couponRewardTypeAt(tc.activity, draw)
				require.NoError(t, err)
				switch rewardType {
				case PlayRewardTypeCoupon:
					couponCount++
				case PlayRewardTypeBalance:
					balanceCount++
				default:
					t.Fatalf("unexpected reward type %q", rewardType)
				}
			}
			require.Equal(t, tc.couponDraws, couponCount)
			require.Equal(t, tc.balanceDraws, balanceCount)

			rewardType, err := couponRewardTypeAt(tc.activity, tc.boundaryDraw)
			require.NoError(t, err)
			require.Equal(t, PlayRewardTypeBalance, rewardType)
		})
	}
}

func TestCouponPoolReadinessUsesLiveIssuanceGateWhenSupported(t *testing.T) {
	t.Parallel()

	issuer := &playCouponReadinessIssuer{
		playCouponRewardIssuer: &playCouponRewardIssuer{},
		ready:                  false,
	}
	svc := &PlayService{couponRewardIssuer: issuer}

	ready, err := svc.couponRewardPoolReady(context.Background(), CouponRewardActivityBlindbox)
	require.NoError(t, err)
	require.False(t, ready)
	require.True(t, issuer.checked)
}

func TestCouponPoolUnavailablePreventsBlindboxOpenAndSetsUnavailableStatus(t *testing.T) {
	pool := defaultBlindboxPool()
	repo := &blindboxOpenRepo{lockedBalance: 1}
	userRepo := &blindboxOpenUserRepo{user: &User{ID: 42, Balance: 1}}
	issuer := &playCouponRewardIssuer{poolUnavailable: true}
	svc := NewPlayService(repo, userRepo, nil, newCouponRewardSettingService(t, pool, true), nil, nil)
	svc.SetCouponRewardIssuer(issuer)

	status, err := svc.GetBlindboxStatus(context.Background(), 42)
	require.NoError(t, err)
	require.False(t, status.CouponPoolReady)
	require.False(t, status.CanOpen)

	_, err = svc.OpenBlindbox(context.Background(), 42, "blindbox-no-pool")
	require.ErrorIs(t, err, ErrCouponRewardPoolUnavailable)
	require.Empty(t, issuer.requests)
	require.Empty(t, repo.records)
	require.Empty(t, repo.balanceUpdates)
	require.Empty(t, userRepo.balanceUpdates)
}

func TestCouponPoolUnavailablePreventsQuizSubmissionAndSetsUnavailableStatus(t *testing.T) {
	repo := &playCouponQuizRepo{questions: newPlayCouponQuizQuestions(5)}
	issuer := &playCouponRewardIssuer{poolUnavailable: true}
	svc := NewPlayService(repo, nil, nil, newCouponQuizSettingService(), nil, nil)
	svc.SetCouponRewardIssuer(issuer)

	today, err := svc.GetQuizToday(context.Background(), 42, "en")
	require.NoError(t, err)
	require.False(t, today.CouponPoolReady)
	require.Empty(t, today.Questions)

	_, err = svc.SubmitQuiz(context.Background(), 42, "en", newPlayCouponQuizAnswers(repo.questions))
	require.ErrorIs(t, err, ErrCouponRewardPoolUnavailable)
	require.Empty(t, issuer.requests)
	require.Empty(t, repo.inserted)
	require.Empty(t, repo.ledgerEntries)
	require.Empty(t, repo.balanceUpdates)
}

func TestQuizTodayRestoresCouponRewardFromCompletedAttempt(t *testing.T) {
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	repo := &playCouponQuizRepo{
		questions: newPlayCouponQuizQuestions(5),
		attempt:   &PlayQuizAttemptDB{Score: 5, Total: 5, RewardAmount: 0},
	}
	issuer := &playCouponRewardIssuer{replayResult: newPlayCouponRewardIssueResult(now, "quiz-coupon-v1")}
	svc := NewPlayService(repo, nil, nil, newCouponQuizSettingService(), nil, nil)
	svc.now = func() time.Time { return now }
	svc.SetCouponRewardIssuer(issuer)

	today, err := svc.GetQuizToday(context.Background(), 42, "en")

	require.NoError(t, err)
	require.True(t, today.AlreadySubmitted)
	require.Equal(t, PlayRewardTypeCoupon, today.PreviousRewardType)
	require.NotNil(t, today.PreviousCoupon)
	require.Equal(t, "充值满10减1", today.PreviousCoupon.Name)
	require.Equal(t, "quiz-coupon-v1", today.PreviousCouponPoolVersion)
	require.Equal(t, []string{"quiz:42:2026-07-27"}, issuer.replayKeys)
}

func TestQuizTodayRestoresCompletedCouponWhenPoolBecomesUnavailable(t *testing.T) {
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	repo := &playCouponQuizRepo{
		attempt: &PlayQuizAttemptDB{Score: 5, Total: 5, RewardAmount: 0},
	}
	issuer := &playCouponRewardIssuer{
		poolUnavailable: true,
		replayResult:    newPlayCouponRewardIssueResult(now, "quiz-coupon-v1"),
	}
	svc := NewPlayService(repo, nil, nil, newCouponQuizSettingService(), nil, nil)
	svc.now = func() time.Time { return now }
	svc.SetCouponRewardIssuer(issuer)

	today, err := svc.GetQuizToday(context.Background(), 42, "en")

	require.NoError(t, err)
	require.False(t, today.CouponPoolReady)
	require.True(t, today.AlreadySubmitted)
	require.Equal(t, PlayRewardTypeCoupon, today.PreviousRewardType)
	require.NotNil(t, today.PreviousCoupon)
	require.Equal(t, "充值满10减1", today.PreviousCoupon.Name)
}

func TestBlindboxCouponRewardIssuesCouponAndChargesCostInOneTransaction(t *testing.T) {
	pool := defaultBlindboxPool()
	settings := newCouponRewardSettingService(t, pool, true)
	repo := &blindboxOpenRepo{lockedBalance: 1}
	userRepo := &blindboxOpenUserRepo{user: &User{ID: 42, Balance: 1}}
	client, mock := newCouponRewardEntClient(t)

	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	issuer := &playCouponRewardIssuer{result: newPlayCouponRewardIssueResult(now, "blindbox-coupon-v1")}
	svc := NewPlayService(repo, userRepo, nil, settings, nil, client)
	svc.now = func() time.Time { return now }
	svc.rewardDrawSource = func(max int64) (int64, error) {
		require.Equal(t, int64(couponWeightBasisPoints), max)
		return 0, nil
	}
	svc.SetCouponRewardIssuer(issuer)

	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := svc.OpenBlindbox(context.Background(), 42, "blindbox-coupon-success")
	require.NoError(t, err)
	require.Equal(t, PlayRewardTypeCoupon, result.RewardType)
	require.Zero(t, result.RewardAmount)
	require.InDelta(t, -pool.Cost, result.NetAmount, 1e-12)
	require.Equal(t, "blindbox-coupon-v1", result.CouponPoolVersion)
	require.NotNil(t, result.Coupon)
	require.Equal(t, int64(701), result.Coupon.UserCouponID)
	require.Equal(t, "充值满10减1", result.Coupon.Name)
	require.Equal(t, now.Add(72*time.Hour), result.Coupon.ExpiresAt)
	require.Len(t, issuer.requests, 1)
	require.True(t, issuer.inTx)
	require.Equal(t, CouponRewardActivityBlindbox, issuer.requests[0].Activity)
	require.Equal(t, int64(42), issuer.requests[0].UserID)
	require.Equal(t, "2026-07-27", issuer.requests[0].SourceRef)
	require.Equal(t, now, issuer.requests[0].IssuedAt)
	require.Len(t, repo.records, 1)
	require.Zero(t, repo.records[0].Reward)
	require.Len(t, repo.ledgerEntries, 1)
	require.InDelta(t, -pool.Cost, repo.ledgerEntries[0].Amount, 1e-12)
	require.Equal(t, []float64{-pool.Cost}, repo.balanceUpdates)
	require.True(t, repo.recordInTx)
	require.True(t, repo.ledgerInTx)
	require.True(t, repo.balanceInTx)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBlindboxCouponIssueFailureRollsBackWithoutBalanceFallback(t *testing.T) {
	pool := defaultBlindboxPool()
	settings := newCouponRewardSettingService(t, pool, true)
	repo := &blindboxOpenRepo{lockedBalance: 1}
	userRepo := &blindboxOpenUserRepo{user: &User{ID: 42, Balance: 1}}
	client, mock := newCouponRewardEntClient(t)
	issuer := &playCouponRewardIssuer{err: ErrCouponRewardPoolUnavailable}
	svc := NewPlayService(repo, userRepo, nil, settings, nil, client)
	svc.rewardDrawSource = func(int64) (int64, error) { return 0, nil }
	svc.SetCouponRewardIssuer(issuer)

	mock.ExpectBegin()
	mock.ExpectRollback()

	_, err := svc.OpenBlindbox(context.Background(), 42, "blindbox-coupon-failure")
	require.ErrorIs(t, err, ErrCouponRewardPoolUnavailable)
	require.True(t, issuer.inTx)
	require.Len(t, issuer.requests, 1)
	require.Empty(t, repo.records)
	require.Empty(t, repo.ledgerEntries)
	require.Empty(t, repo.balanceUpdates)
	require.Empty(t, userRepo.balanceUpdates)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBlindboxBalanceBranchKeepsOriginalPoolWeights(t *testing.T) {
	pool := defaultBlindboxPool()
	settings := newCouponRewardSettingService(t, pool, true)
	repo := &blindboxOpenRepo{lockedBalance: 1}
	userRepo := &blindboxOpenUserRepo{user: &User{ID: 42, Balance: 1}}
	client, mock := newCouponRewardEntClient(t)
	issuer := &playCouponRewardIssuer{result: newPlayCouponRewardIssueResult(time.Now(), "unused")}
	svc := NewPlayService(repo, userRepo, nil, settings, nil, client)
	draws := []int64{6000, blindboxWeightTotal - 1}
	svc.rewardDrawSource = func(int64) (int64, error) { return draws[0], nil }
	svc.blindboxDrawSource = func(int64) (int64, error) {
		draw := draws[1]
		draws = draws[1:]
		return draw, nil
	}
	svc.SetCouponRewardIssuer(issuer)

	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := svc.OpenBlindbox(context.Background(), 42, "blindbox-balance-branch")
	require.NoError(t, err)
	require.Equal(t, PlayRewardTypeBalance, result.RewardType)
	require.Equal(t, 20.0, result.RewardAmount)
	require.InDelta(t, 19.5, result.NetAmount, 1e-12)
	require.Nil(t, result.Coupon)
	require.Empty(t, issuer.requests)
	require.Equal(t, []float64{19.5}, repo.balanceUpdates)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBlindboxOpenReplaysCompletedCouponWithoutIssuingOrChargingAgain(t *testing.T) {
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	rawKey := "coupon-retry-after-lost-response"
	scopedKey, err := scopeBlindboxIdempotencyKey(42, rawKey)
	require.NoError(t, err)
	repo := &blindboxOpenRepo{records: []PlayBlindboxOpenRecord{{
		UserID:         42,
		Date:           now,
		Cost:           0.5,
		Reward:         0,
		IdempotencyKey: scopedKey,
		PoolVersion:    "season-1-v1",
		OpenSource:     "paid",
	}}}
	issuer := &playCouponRewardIssuer{replayResult: newPlayCouponRewardIssueResult(now, "blindbox-coupon-v1")}
	svc := NewPlayService(repo, nil, nil, NewSettingService(&blindboxOpenSettingRepo{}, nil), nil, nil)
	svc.now = func() time.Time { return now }
	svc.SetCouponRewardIssuer(issuer)
	svc.rewardDrawSource = func(int64) (int64, error) {
		t.Fatal("completed blindbox retry must not redraw the reward branch")
		return 0, nil
	}

	result, err := svc.OpenBlindbox(context.Background(), 42, rawKey)

	require.NoError(t, err)
	require.Equal(t, PlayRewardTypeCoupon, result.RewardType)
	require.Zero(t, result.RewardAmount)
	require.InDelta(t, -0.5, result.NetAmount, 1e-12)
	require.Equal(t, "blindbox-coupon-v1", result.CouponPoolVersion)
	require.NotNil(t, result.Coupon)
	require.Equal(t, int64(701), result.Coupon.UserCouponID)
	require.Equal(t, "充值满10减1", result.Coupon.Name)
	require.Equal(t, []string{scopedKey}, issuer.replayKeys)
	require.Empty(t, issuer.requests)
	require.Empty(t, repo.ledgerEntries)
	require.Empty(t, repo.balanceUpdates)
}

func TestQuizCouponRewardIssuesAfterCompleteSubmissionInOneTransaction(t *testing.T) {
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	repo := &playCouponQuizRepo{questions: newPlayCouponQuizQuestions(5)}
	settings := newCouponQuizSettingService()
	client, mock := newCouponRewardEntClient(t)
	issuer := &playCouponRewardIssuer{result: newPlayCouponRewardIssueResult(now, "quiz-coupon-v1")}
	svc := NewPlayService(repo, nil, nil, settings, nil, client)
	svc.now = func() time.Time { return now }
	svc.rewardDrawSource = func(int64) (int64, error) { return 0, nil }
	svc.SetCouponRewardIssuer(issuer)

	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := svc.SubmitQuiz(context.Background(), 42, "en", newPlayCouponQuizAnswers(repo.questions))
	require.NoError(t, err)
	require.Equal(t, 5, result.Score)
	require.Equal(t, 5, result.Total)
	require.Equal(t, PlayRewardTypeCoupon, result.RewardType)
	require.Zero(t, result.RewardAmount)
	require.Equal(t, "quiz-coupon-v1", result.CouponPoolVersion)
	require.NotNil(t, result.Coupon)
	require.Equal(t, int64(701), result.Coupon.UserCouponID)
	require.Len(t, issuer.requests, 1)
	require.True(t, issuer.inTx)
	require.Equal(t, CouponRewardActivityQuiz, issuer.requests[0].Activity)
	require.Equal(t, "2026-07-27", issuer.requests[0].SourceRef)
	require.Len(t, repo.inserted, 1)
	require.Zero(t, repo.inserted[0].RewardAmount)
	require.True(t, repo.insertedInTx)
	require.Empty(t, repo.ledgerEntries)
	require.Empty(t, repo.balanceUpdates)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuizCouponClaimsAttemptBeforeIssuingCoupon(t *testing.T) {
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	repo := &playCouponQuizRepo{
		questions: newPlayCouponQuizQuestions(5),
		insertErr: ErrPlayQuizAlreadyDone,
	}
	client, mock := newCouponRewardEntClient(t)
	issuer := &playCouponRewardIssuer{result: newPlayCouponRewardIssueResult(now, "quiz-coupon-v1")}
	svc := NewPlayService(repo, nil, nil, newCouponQuizSettingService(), nil, client)
	svc.now = func() time.Time { return now }
	svc.rewardDrawSource = func(int64) (int64, error) { return 0, nil }
	svc.SetCouponRewardIssuer(issuer)

	mock.ExpectBegin()
	mock.ExpectRollback()

	_, err := svc.SubmitQuiz(context.Background(), 42, "en", newPlayCouponQuizAnswers(repo.questions))
	require.ErrorIs(t, err, ErrPlayQuizAlreadyDone)
	require.True(t, repo.insertedInTx)
	require.Empty(t, issuer.requests, "a concurrent completed attempt must not issue a second coupon")
	require.Empty(t, repo.ledgerEntries)
	require.Empty(t, repo.balanceUpdates)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuizCouponIssueFailureRollsBackAttemptWithoutBalanceFallback(t *testing.T) {
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	repo := &playCouponQuizRepo{questions: newPlayCouponQuizQuestions(5)}
	client, mock := newCouponRewardEntClient(t)
	issuer := &playCouponRewardIssuer{err: errors.New("coupon stock unavailable")}
	svc := NewPlayService(repo, nil, nil, newCouponQuizSettingService(), nil, client)
	svc.now = func() time.Time { return now }
	svc.rewardDrawSource = func(int64) (int64, error) { return 0, nil }
	svc.SetCouponRewardIssuer(issuer)

	mock.ExpectBegin()
	mock.ExpectRollback()

	_, err := svc.SubmitQuiz(context.Background(), 42, "en", newPlayCouponQuizAnswers(repo.questions))
	require.ErrorContains(t, err, "coupon stock unavailable")
	require.True(t, issuer.inTx)
	require.Len(t, issuer.requests, 1)
	// The attempt is claimed before the coupon issuer. sqlmock verifies that
	// the enclosing transaction rolls back, so this in-memory call is not a
	// persisted completion after the failed issue.
	require.Len(t, repo.inserted, 1)
	require.True(t, repo.insertedInTx)
	require.Empty(t, repo.ledgerEntries)
	require.Empty(t, repo.balanceUpdates)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuizBalanceBranchUsesFullCompletionReward(t *testing.T) {
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	repo := &playCouponQuizRepo{questions: newPlayCouponQuizQuestions(5)}
	client, mock := newCouponRewardEntClient(t)
	issuer := &playCouponRewardIssuer{result: newPlayCouponRewardIssueResult(now, "unused")}
	svc := NewPlayService(repo, nil, nil, newCouponQuizSettingService(), nil, client)
	svc.now = func() time.Time { return now }
	svc.rewardDrawSource = func(int64) (int64, error) { return 8000, nil }
	svc.SetCouponRewardIssuer(issuer)

	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := svc.SubmitQuiz(context.Background(), 42, "en", newPlayCouponQuizAnswers(repo.questions))
	require.NoError(t, err)
	require.Equal(t, PlayRewardTypeBalance, result.RewardType)
	require.InDelta(t, 0.5, result.RewardAmount, 1e-12)
	require.Nil(t, result.Coupon)
	require.Empty(t, issuer.requests)
	require.Len(t, repo.inserted, 1)
	require.InDelta(t, 0.5, repo.inserted[0].RewardAmount, 1e-12)
	require.Len(t, repo.ledgerEntries, 1)
	require.InDelta(t, 0.5, repo.ledgerEntries[0].Amount, 1e-12)
	require.Equal(t, []float64{0.5}, repo.balanceUpdates)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuizBalanceBranchUsesFullCompletionRewardForOneCorrectAnswer(t *testing.T) {
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	repo := &playCouponQuizRepo{questions: newPlayCouponQuizQuestions(5)}
	client, mock := newCouponRewardEntClient(t)
	issuer := &playCouponRewardIssuer{result: newPlayCouponRewardIssueResult(now, "unused")}
	svc := NewPlayService(repo, nil, nil, newCouponQuizSettingService(), nil, client)
	svc.now = func() time.Time { return now }
	svc.rewardDrawSource = func(int64) (int64, error) { return 8000, nil }
	svc.SetCouponRewardIssuer(issuer)
	answers := newPlayCouponQuizAnswers(repo.questions)
	for i := 1; i < len(answers); i++ {
		answers[i].ChoiceIndex = 1
	}

	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := svc.SubmitQuiz(context.Background(), 42, "en", answers)
	require.NoError(t, err)
	require.Equal(t, 1, result.Score)
	require.Equal(t, 5, result.Total)
	require.Equal(t, PlayRewardTypeBalance, result.RewardType)
	require.InDelta(t, 0.5, result.RewardAmount, 1e-12)
	require.Nil(t, result.Coupon)
	require.Empty(t, issuer.requests)
	require.Len(t, repo.inserted, 1)
	require.InDelta(t, 0.5, repo.inserted[0].RewardAmount, 1e-12)
	require.Len(t, repo.ledgerEntries, 1)
	require.InDelta(t, 0.5, repo.ledgerEntries[0].Amount, 1e-12)
	require.Equal(t, 1, repo.ledgerEntries[0].Detail["score"])
	require.Equal(t, []float64{0.5}, repo.balanceUpdates)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuizZeroScoreRecordsAttemptWithoutCouponOrBalance(t *testing.T) {
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	repo := &playCouponQuizRepo{questions: newPlayCouponQuizQuestions(5)}
	issuer := &playCouponRewardIssuer{result: newPlayCouponRewardIssueResult(now, "unused")}
	svc := NewPlayService(repo, nil, nil, newCouponQuizSettingService(), nil, nil)
	svc.now = func() time.Time { return now }
	svc.rewardDrawSource = func(int64) (int64, error) {
		t.Fatal("zero-score quiz must not enter the coupon/balance draw")
		return 0, nil
	}
	svc.SetCouponRewardIssuer(issuer)
	answers := newPlayCouponQuizAnswers(repo.questions)
	for i := range answers {
		answers[i].ChoiceIndex = 1
	}

	result, err := svc.SubmitQuiz(context.Background(), 42, "en", answers)
	require.NoError(t, err)
	require.Zero(t, result.Score)
	require.Equal(t, PlayRewardTypeNone, result.RewardType)
	require.Zero(t, result.RewardAmount)
	require.Nil(t, result.Coupon)
	require.Empty(t, issuer.requests)
	require.Len(t, repo.inserted, 1)
	require.Zero(t, repo.inserted[0].RewardAmount)
	require.False(t, repo.insertedInTx)
	require.Empty(t, repo.ledgerEntries)
	require.Empty(t, repo.balanceUpdates)
}

func TestQuizRequiresAllDailyAnswersBeforeRewardDraw(t *testing.T) {
	repo := &playCouponQuizRepo{questions: newPlayCouponQuizQuestions(5)}
	issuer := &playCouponRewardIssuer{result: newPlayCouponRewardIssueResult(time.Now(), "unused")}
	svc := NewPlayService(repo, nil, nil, newCouponQuizSettingService(), nil, nil)
	svc.rewardDrawSource = func(int64) (int64, error) {
		t.Fatal("incomplete quiz must not enter the coupon/balance draw")
		return 0, nil
	}
	svc.SetCouponRewardIssuer(issuer)

	answers := newPlayCouponQuizAnswers(repo.questions)
	_, err := svc.SubmitQuiz(context.Background(), 42, "en", answers[:len(answers)-1])
	require.ErrorIs(t, err, ErrPlayQuizInvalidAnswer)
	require.Empty(t, issuer.requests)
	require.Empty(t, repo.inserted)
	require.Empty(t, repo.ledgerEntries)
	require.Empty(t, repo.balanceUpdates)
}

func TestQuizRejectsDuplicateQuestionAnswersBeforeIssuingReward(t *testing.T) {
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	repo := &playCouponQuizRepo{questions: newPlayCouponQuizQuestions(5)}
	issuer := &playCouponRewardIssuer{result: newPlayCouponRewardIssueResult(now, "unused")}
	svc := NewPlayService(repo, nil, nil, newCouponQuizSettingService(), nil, nil)
	svc.now = func() time.Time { return now }
	svc.SetCouponRewardIssuer(issuer)
	answers := newPlayCouponQuizAnswers(repo.questions)
	answers[1] = answers[0]

	_, err := svc.SubmitQuiz(context.Background(), 42, "en", answers)
	require.ErrorIs(t, err, ErrPlayQuizInvalidAnswer)
	require.Empty(t, issuer.requests)
	require.Empty(t, repo.inserted)
	require.Empty(t, repo.ledgerEntries)
	require.Empty(t, repo.balanceUpdates)
}

func newCouponRewardSettingService(t *testing.T, pool PlayBlindboxPool, enabled bool) *SettingService {
	t.Helper()
	poolJSON, err := json.Marshal(pool)
	require.NoError(t, err)
	return NewSettingService(&blindboxOpenSettingRepo{values: map[string]string{
		SettingKeyPlayBlindboxEnabled:    fmt.Sprintf("%t", enabled),
		SettingKeyPlayBlindboxCost:       fmt.Sprintf("%g", pool.Cost),
		SettingKeyPlayBlindboxPoolJSON:   string(poolJSON),
		SettingKeyPlayBlindboxDailyLimit: "10",
	}}, nil)
}

func newCouponQuizSettingService() *SettingService {
	return NewSettingService(&blindboxOpenSettingRepo{values: map[string]string{
		SettingKeyPlayQuizEnabled:          "true",
		SettingKeyPlayQuizRewardPerCorrect: "0.1",
		SettingKeyPlayQuizQuestionsPerDay:  "5",
	}}, nil)
}

func newCouponRewardEntClient(t *testing.T) (*dbent.Client, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	return client, mock
}

func newPlayCouponRewardIssueResult(issuedAt time.Time, poolVersion string) *CouponRewardIssueResult {
	validFrom := issuedAt.UTC()
	expiresAt := validFrom.Add(72 * time.Hour)
	return &CouponRewardIssueResult{
		PoolVersionID: 401,
		PoolVersion:   poolVersion,
		PoolEntryID:   501,
		TemplateID:    601,
		UserCouponID:  701,
		Coupon: UserCoupon{
			ID:           701,
			TemplateID:   601,
			TemplateName: "充值满10减1",
			UserID:       42,
			Status:       UserCouponStatusAvailable,
			TermsSnapshot: CouponTermsSnapshot{
				TemplateID:         601,
				TemplateVersion:    1,
				TemplateKey:        "recharge-10-off-1",
				Name:               "充值满10减1",
				BenefitType:        CouponBenefitTypeFixedAmount,
				BenefitValue:       1,
				Currency:           "CNY",
				ApplicableScopes:   []CouponScope{CouponScopeBalance},
				MinimumOrderAmount: 10,
				ValidityMode:       CouponValidityModeRelativeDays,
				ValidityDays:       3,
			},
			ValidFrom: validFrom,
			ExpiresAt: expiresAt,
		},
		ValidFrom: validFrom,
		ExpiresAt: expiresAt,
	}
}

func newPlayCouponQuizQuestions(count int) []PlayQuizQuestionDB {
	questions := make([]PlayQuizQuestionDB, 0, count)
	for i := 0; i < count; i++ {
		questions = append(questions, PlayQuizQuestionDB{
			ID:           int64(i + 1),
			Language:     "en",
			Prompt:       fmt.Sprintf("Question %d", i+1),
			OptionsJSON:  fmt.Sprintf(`["Correct %d","Wrong %d"]`, i+1, i+1),
			CorrectIndex: 0,
		})
	}
	return questions
}

func newPlayCouponQuizAnswers(questions []PlayQuizQuestionDB) []PlayQuizAnswer {
	answers := make([]PlayQuizAnswer, 0, len(questions))
	for _, question := range questions {
		answers = append(answers, PlayQuizAnswer{QuestionID: question.ID, ChoiceIndex: question.CorrectIndex})
	}
	return answers
}
