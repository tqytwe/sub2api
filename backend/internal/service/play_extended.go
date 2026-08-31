package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/shopspring/decimal"
)

func (s *PlayService) GetBlindboxStatus(ctx context.Context, userID int64) (*PlayBlindboxStatus, error) {
	rt := s.GetRuntime(ctx)
	now := s.serverNow()
	date := s.serverDate(now)
	out := &PlayBlindboxStatus{
		Enabled:                   rt.BlindboxEnabled,
		CouponPoolReady:           true,
		GrowthGovernanceAvailable: true,
		DailyLimit:                rt.BlindboxDailyLimit,
		ServerDate:                date.Format("2006-01-02"),
	}
	out.EffectiveLimit = rt.BlindboxDailyLimit
	if rt.BlindboxEnabled && userID > 0 {
		eligibility, err := s.growthEligibility(ctx, userID, now)
		if err != nil {
			return nil, err
		}
		out.GrowthEligibility = eligibility
		// Explorer status is intentionally reward-blind. Do not load or expose
		// the active pool, coupon previews, odds, or expected-value promises
		// until the server has established redeemable eligibility.
		if eligibility.RewardMode != PlayGrowthRewardRedeemable {
			opens, err := s.repo.CountBlindboxOpens(ctx, userID, date)
			if err != nil {
				return nil, err
			}
			out.OpensToday = opens
			out.CouponPoolReady = false
			return out, nil
		}
		_, governanceAvailable, governanceReason := s.growthGovernanceForStatus(ctx, userID, now)
		out.GrowthGovernanceAvailable = governanceAvailable
		out.GrowthGovernanceReason = governanceReason
		if !governanceAvailable {
			out.CouponPoolReady = false
			opens, countErr := s.repo.CountBlindboxOpens(ctx, userID, date)
			if countErr != nil {
				return nil, countErr
			}
			out.OpensToday = opens
			return out, nil
		}
	}

	vip := resolveVIPStatus(0, rt.VIPTiers)
	if userID > 0 {
		resolvedVIP, err := s.resolveBlindboxVIPStatus(ctx, userID, rt)
		if err != nil {
			return nil, err
		}
		vip = resolvedVIP
	}
	pool := resolveVIPBlindboxPool(rt.BlindboxPool, vip)
	nextPool := resolveNextVIPBlindboxPool(rt.BlindboxPool, vip)
	out.CouponWeightBP = 6000
	out.RedeemCodeWeightBP = 0
	out.BalanceWeightBP = 4000
	out.CostAmount = pool.Cost
	out.BlindboxPool = pool
	out.CurrentPool = pool
	out.NextPool = nextPool
	out.VIPTier = vip
	out.ExpectedReward = pool.ExpectedReward()
	out.PoolVersion = pool.Version
	out.RTPCap = pool.RTPCap
	if nextPool != nil {
		out.NextExpectedReward = nextPool.ExpectedReward()
	}
	if rt.BlindboxEnabled {
		ready, err := s.couponRewardPoolReady(ctx, CouponRewardActivityBlindbox)
		if err != nil {
			return nil, err
		}
		out.CouponPoolReady = ready
		if ready {
			couponWeightBP, redeemCodeWeightBP, balanceWeightBP, err := s.couponRewardSplit(ctx, CouponRewardActivityBlindbox)
			if err != nil {
				return nil, err
			}
			out.CouponWeightBP = couponWeightBP
			out.RedeemCodeWeightBP = redeemCodeWeightBP
			out.BalanceWeightBP = balanceWeightBP
		}
		prizes, err := s.couponRewardPrizePreview(ctx, CouponRewardActivityBlindbox)
		if err != nil {
			return nil, err
		}
		out.CouponPrizes = prizes
	}
	if userID <= 0 {
		return out, nil
	}
	// Preserve the public configured-pool response while the feature is off;
	// this is used by the landing page to render a disabled, non-actionable
	// preview. Authenticated requests also retain the historical shape here.
	if !rt.BlindboxEnabled {
		return out, nil
	}
	mods, err := s.resolvePlayEffectModifiers(ctx, userID, rt)
	if err != nil {
		return nil, err
	}
	if mods.BlindboxExtraOpens > 0 {
		out.EffectiveLimit = rt.BlindboxDailyLimit + mods.BlindboxExtraOpens
	}
	if boost, err := s.getRechargeBoostStatus(ctx, userID, rt); err != nil {
		return nil, err
	} else if boost.Active {
		out.RechargeBoostActive = true
	}
	if mods.CampaignActive {
		out.CampaignActive = true
	}
	opens, err := s.repo.CountBlindboxOpens(ctx, userID, date)
	if err != nil {
		return nil, err
	}
	out.OpensToday = opens
	out.CanOpen = out.CouponPoolReady && opens < out.EffectiveLimit
	return out, nil
}

func (s *PlayService) OpenBlindbox(ctx context.Context, userID int64, idempotencyKey string) (*PlayBlindboxOpenResult, error) {
	var err error
	idempotencyKey, err = scopeBlindboxIdempotencyKey(userID, idempotencyKey)
	if err != nil {
		return nil, err
	}
	if replay, err := s.replayBlindboxOpen(ctx, userID, idempotencyKey); err != nil {
		return nil, err
	} else if replay != nil {
		return replay, nil
	}

	rt := s.GetRuntime(ctx)
	if !rt.BlindboxEnabled {
		return nil, ErrPlayFeatureDisabled
	}
	now := s.serverNow()
	eligibility, err := s.growthEligibility(ctx, userID, now)
	if err != nil {
		return nil, err
	}
	if eligibility.RewardMode != PlayGrowthRewardRedeemable {
		return nil, newPlayGrowthRewardIneligibleError(eligibility)
	}
	growthGovernance, err := s.requireGrowthGovernanceForReward(ctx, userID, now)
	if err != nil {
		return nil, err
	}
	if err := s.requireCouponRewardPool(ctx, CouponRewardActivityBlindbox); err != nil {
		return nil, err
	}
	vip, err := s.resolveBlindboxVIPStatus(ctx, userID, rt)
	if err != nil {
		return nil, err
	}
	pool := resolveVIPBlindboxPool(rt.BlindboxPool, vip)
	if err := ValidateBlindboxPool(pool); err != nil {
		return nil, fmt.Errorf("blindbox pool not configured: %w", err)
	}
	cost := pool.Cost

	date := s.serverDate(now)
	dateKey := date.Format("2006-01-02")
	mods, err := s.resolvePlayEffectModifiers(ctx, userID, rt)
	if err != nil {
		return nil, err
	}
	effectiveLimit := rt.BlindboxDailyLimit + mods.BlindboxExtraOpens

	const openSource = "paid"

	if s.entClient == nil {
		return nil, fmt.Errorf("play service: ent client missing")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin blindbox open tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	balance, err := s.repo.LockBlindboxOpenUser(txCtx, userID)
	if err != nil {
		return nil, err
	}
	if replay, err := s.replayBlindboxOpen(txCtx, userID, idempotencyKey); err != nil {
		return nil, err
	} else if replay != nil {
		return replay, nil
	}
	if balance < cost {
		return nil, ErrPlayInsufficientBalance
	}
	opens, err := s.repo.CountBlindboxOpens(txCtx, userID, date)
	if err != nil {
		return nil, err
	}
	if opens >= effectiveLimit {
		return nil, ErrPlayBlindboxDailyLimit
	}
	rewardType, err := s.drawCouponRewardType(txCtx, CouponRewardActivityBlindbox)
	if err != nil {
		return nil, err
	}
	reward := 0.0
	switch rewardType {
	case PlayRewardTypeBalance:
		reward, err = s.pickBlindboxReward(pool)
		if err != nil {
			return nil, err
		}
	}
	net := reward - cost

	// The blindbox action must win its unique idempotency constraint before
	// it can issue a coupon or reserve a code. Keep the original seven-column
	// action insert intact, then append qualification evidence in this same tx.
	if err := s.repo.InsertBlindboxOpenRecord(txCtx, PlayBlindboxOpenRecord{
		UserID:         userID,
		Date:           date,
		Cost:           cost,
		Reward:         reward,
		IdempotencyKey: idempotencyKey,
		PoolVersion:    pool.Version,
		OpenSource:     openSource,
	}); err != nil {
		if errors.Is(err, ErrPlayRewardDuplicate) {
			return nil, ErrPlayRewardDuplicate
		}
		return nil, err
	}
	growthSnapshotID, err := s.createGrowthSnapshot(txCtx, PlayGrowthEligibilitySnapshot{
		UserID: userID, Source: PlayRewardSourceBlindbox, ActionID: idempotencyKey, ActivityDate: date, Eligibility: eligibility,
	})
	if err != nil {
		return nil, err
	}
	if growthSnapshotID > 0 {
		growthRepo, ok := s.growthQualificationRepository()
		if !ok {
			return nil, ErrPlayGrowthQualificationUnavailable
		}
		if err := growthRepo.LinkBlindboxGrowthEligibilitySnapshot(txCtx, userID, idempotencyKey, growthSnapshotID); err != nil {
			return nil, err
		}
	} else if s.requireGrowthQualification {
		return nil, ErrPlayGrowthQualificationUnavailable
	}
	if err := s.reserveGrowthRewardBudget(txCtx, growthGovernance, userID, PlayRewardSourceBlindbox, idempotencyKey, growthRewardBudgetCost(rewardType, reward)); err != nil {
		return nil, err
	}

	var couponIssue *CouponRewardIssueResult
	var redeemCode *RedeemCode
	switch rewardType {
	case PlayRewardTypeCoupon:
		couponIssue, err = s.issueCouponRewardInTx(txCtx, userID, CouponRewardActivityBlindbox, idempotencyKey, dateKey, now, eligibility)
		if err != nil {
			return nil, err
		}
	case PlayRewardTypeRedeem:
		redeemCode, err = s.issueRedeemCodeRewardInTx(txCtx, userID, CouponRewardActivityBlindbox, idempotencyKey, idempotencyKey, now, eligibility)
		if err != nil {
			return nil, err
		}
	}

	detail := map[string]any{
		"open_date":         dateKey,
		"cost_amount":       cost,
		"reward_amount":     reward,
		"net_amount":        net,
		"reward_type":       string(rewardType),
		"pool_version":      pool.Version,
		"open_source":       openSource,
		"vip_tier":          float64(vip.Tier),
		"vip_label":         vip.Label,
		"vip_color_key":     vip.ColorKey,
		"vip_tier_snapshot": vip,
		"expected_reward":   pool.ExpectedReward(),
		"rtp_cap":           pool.RTPCap,
	}
	if growthSnapshotID > 0 {
		detail["growth_eligibility_snapshot_id"] = growthSnapshotID
		detail["growth_rule_version"] = PlayGrowthQualificationRuleVersion()
		detail["growth_tier"] = eligibility.Tier
	}
	if couponIssue != nil {
		detail["coupon_pool_version_id"] = couponIssue.PoolVersionID
		detail["coupon_pool_version"] = couponIssue.PoolVersion
		detail["coupon_pool_entry_id"] = couponIssue.PoolEntryID
		detail["coupon_template_id"] = couponIssue.TemplateID
		detail["user_coupon_id"] = couponIssue.UserCouponID
		detail["coupon_expires_at"] = couponIssue.ExpiresAt.Format(time.RFC3339)
	}
	if redeemCode != nil {
		detail["redeem_code_id"] = redeemCode.ID
		detail["redeem_code_type"] = redeemCode.Type
		detail["redeem_code_batch"] = redeemCode.BatchName
		detail["redeem_code_status"] = redeemCode.Status
		detail["redeem_code_expires_at"] = redeemCode.ExpiresAt
	}

	if err := s.grantBalanceInTx(txCtx, userID, net, PlayRewardSourceBlindbox, idempotencyKey, detail, &growthSnapshotID, nil); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit blindbox open tx: %w", err)
	}

	return &PlayBlindboxOpenResult{
		CostAmount:        cost,
		RewardAmount:      reward,
		NetAmount:         net,
		RewardType:        rewardType,
		Coupon:            playCouponRewardSummary(couponIssue),
		RedeemCode:        playRedeemCodeRewardSummary(redeemCode),
		CouponPoolVersion: couponPoolVersion(couponIssue),
		OpensToday:        opens + 1,
		ServerDate:        dateKey,
		PoolVersion:       pool.Version,
		OpenSource:        openSource,
		VIPTier:           vip,
		ExpectedReward:    pool.ExpectedReward(),
		RTPCap:            pool.RTPCap,
	}, nil
}

// replayBlindboxOpen reconstructs a completed response before any current
// runtime gate, draw, or balance mutation. The same lookup is repeated after
// taking the user-row lock to serialize concurrent deliveries of one request.
func (s *PlayService) replayBlindboxOpen(ctx context.Context, userID int64, idempotencyKey string) (*PlayBlindboxOpenResult, error) {
	record, err := s.repo.FindBlindboxOpenByIdempotency(ctx, userID, idempotencyKey)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, nil
	}
	opens, err := s.repo.CountBlindboxOpens(ctx, userID, record.Date)
	if err != nil {
		return nil, err
	}

	var couponIssue *CouponRewardIssueResult
	var redeemCode *RedeemCode
	if reader, ok := s.couponRewardIssuer.(CouponRewardReplayReader); ok && reader != nil {
		couponIssue, err = reader.GetCouponRewardIssueByIdempotency(ctx, userID, idempotencyKey)
		if err != nil {
			return nil, err
		}
	}
	if reader, ok := s.redeemRewardIssuer.(RedeemCodeRewardReplayReader); ok && reader != nil {
		redeemCode, err = reader.GetRedeemCodeRewardByIssueRef(
			ctx,
			userID,
			string(CouponRewardActivityBlindbox),
			idempotencyKey,
		)
		if err != nil {
			return nil, err
		}
	}
	if couponIssue != nil && redeemCode != nil {
		return nil, fmt.Errorf("blindbox replay has conflicting coupon and redeem rewards")
	}
	rewardType := PlayRewardTypeBalance
	if couponIssue != nil {
		rewardType = PlayRewardTypeCoupon
	} else if redeemCode != nil {
		rewardType = PlayRewardTypeRedeem
	}
	vip := copyBlindboxVIPSnapshot(record.VIPTierSnapshot)
	return &PlayBlindboxOpenResult{
		CostAmount:        record.Cost,
		RewardAmount:      record.Reward,
		NetAmount:         record.Reward - record.Cost,
		RewardType:        rewardType,
		Coupon:            playCouponRewardSummary(couponIssue),
		RedeemCode:        playRedeemCodeRewardSummary(redeemCode),
		CouponPoolVersion: couponPoolVersion(couponIssue),
		OpensToday:        opens,
		ServerDate:        record.Date.Format("2006-01-02"),
		PoolVersion:       record.PoolVersion,
		OpenSource:        record.OpenSource,
		VIPTier:           vip,
		ExpectedReward:    blindboxReplaySnapshotFloat(record.ExpectedReward),
		RTPCap:            blindboxReplaySnapshotFloat(record.RTPCap),
	}, nil
}

func copyBlindboxVIPSnapshot(snapshot *PlayVIPStatus) PlayVIPStatus {
	if snapshot == nil {
		return PlayVIPStatus{}
	}
	copy := *snapshot
	copy.Perks = append([]string(nil), snapshot.Perks...)
	return copy
}

func blindboxReplaySnapshotFloat(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func (s *PlayService) resolveBlindboxVIPStatus(ctx context.Context, userID int64, rt PlayRuntime) (PlayVIPStatus, error) {
	vip := resolveVIPStatus(0, rt.VIPTiers)
	if userID <= 0 {
		return vip, nil
	}
	paidTotal, err := s.MembershipPaidTotal(ctx, userID)
	if err != nil {
		return vip, err
	}
	return resolveVIPStatus(paidTotal, rt.VIPTiers), nil
}

func scopeBlindboxIdempotencyKey(userID int64, raw string) (string, error) {
	normalized, err := NormalizeIdempotencyKey(raw)
	if err != nil {
		return "", err
	}
	if normalized == "" {
		random := make([]byte, 16)
		if _, err := rand.Read(random); err != nil {
			return "", fmt.Errorf("generate blindbox idempotency key: %w", err)
		}
		normalized = hex.EncodeToString(random)
	}
	return fmt.Sprintf("blindbox:%d:%s", userID, HashIdempotencyKey(normalized)), nil
}

func (s *PlayService) ListRecentBlindboxWins(ctx context.Context, limit int) ([]PlayBlindboxRecentWin, error) {
	rows, err := s.repo.ListRecentBlindboxWins(ctx, limit)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].UserLabel = maskBlindboxUserLabel(rows[i].UserLabel)
	}
	return rows, nil
}

// maskBlindboxUserLabel hides PII in the public win feed while keeping a recognizable stub.
func maskBlindboxUserLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return "***"
	}
	if strings.Contains(label, "@") {
		return maskEmail(label)
	}
	runes := []rune(label)
	if len(runes) <= 1 {
		return string(runes) + "***"
	}
	if len(runes) == 2 {
		return string(runes[0]) + "*"
	}
	return string(runes[0]) + "***" + string(runes[len(runes)-1])
}

func normalizeQuizLanguage(language string) string {
	lang := strings.ToLower(strings.TrimSpace(language))
	switch {
	case strings.HasPrefix(lang, "zh"):
		return "zh"
	case strings.HasPrefix(lang, "en"):
		return "en"
	default:
		return "en"
	}
}

func quizTemplateKey(q PlayQuizQuestionDB) string {
	return q.OptionsJSON + "\x00" + strconv.Itoa(q.CorrectIndex)
}

// dedupeQuizQuestionsByTemplate keeps one variant per unique stem/options set.
// The seeded zh/en pools repeat the same 10 templates with suffix-only variants;
// without dedupe the daily quiz can show five near-identical prompts.
func dedupeQuizQuestionsByTemplate(questions []PlayQuizQuestionDB, userID int64, date time.Time, language string) []PlayQuizQuestionDB {
	if len(questions) == 0 {
		return nil
	}
	if userID <= 0 {
		userID = 1
	}
	dayKey := date.Format("2006-01-02")
	groups := make(map[string][]PlayQuizQuestionDB)
	for _, q := range questions {
		key := quizTemplateKey(q)
		groups[key] = append(groups[key], q)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]PlayQuizQuestionDB, 0, len(keys))
	for _, key := range keys {
		group := groups[key]
		sort.SliceStable(group, func(i, j int) bool { return group[i].ID < group[j].ID })
		idx := quizDeterministicIndex(userID, dayKey, language, key, len(group))
		out = append(out, group[idx])
	}
	return out
}

func quizDeterministicIndex(userID int64, dayKey, language, salt string, size int) int {
	if size <= 1 {
		return 0
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(strconv.FormatInt(userID, 10)))
	_, _ = h.Write([]byte(":"))
	_, _ = h.Write([]byte(dayKey))
	_, _ = h.Write([]byte(":"))
	_, _ = h.Write([]byte(language))
	_, _ = h.Write([]byte(":"))
	_, _ = h.Write([]byte(salt))
	return int(h.Sum64() % uint64(size))
}

func (s *PlayService) pickDailyQuizQuestions(questions []PlayQuizQuestionDB, limit int, userID int64, date time.Time, language string) []PlayQuizQuestionDB {
	questions = dedupeQuizQuestionsByTemplate(questions, userID, date, language)
	if len(questions) == 0 {
		return nil
	}
	if limit <= 0 || limit > len(questions) {
		limit = len(questions)
	}
	if userID <= 0 {
		userID = 1
	}
	dayKey := date.Format("2006-01-02")
	type scoredQuestion struct {
		q     PlayQuizQuestionDB
		score uint64
	}
	scored := make([]scoredQuestion, 0, len(questions))
	for _, q := range questions {
		h := fnv.New64a()
		_, _ = h.Write([]byte(strconv.FormatInt(userID, 10)))
		_, _ = h.Write([]byte(":"))
		_, _ = h.Write([]byte(dayKey))
		_, _ = h.Write([]byte(":"))
		_, _ = h.Write([]byte(language))
		_, _ = h.Write([]byte(":"))
		_, _ = h.Write([]byte(strconv.FormatInt(q.ID, 10)))
		scored = append(scored, scoredQuestion{q: q, score: h.Sum64()})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].q.ID < scored[j].q.ID
		}
		return scored[i].score < scored[j].score
	})
	out := make([]PlayQuizQuestionDB, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, scored[i].q)
	}
	return out
}

func (s *PlayService) resolveDailyQuizQuestions(ctx context.Context, userID int64, language string, limit int) ([]PlayQuizQuestionDB, string, error) {
	now := s.serverNow()
	date := s.serverDate(now)
	quizPool, resolvedLanguage, err := s.getQuizPoolByLanguage(ctx, language)
	if err != nil {
		return nil, "", err
	}
	picked := s.pickDailyQuizQuestions(quizPool, limit, userID, date, resolvedLanguage)
	valid := make([]PlayQuizQuestionDB, 0, len(picked))
	for _, q := range picked {
		var options []string
		if err := json.Unmarshal([]byte(q.OptionsJSON), &options); err != nil || len(options) == 0 {
			continue
		}
		valid = append(valid, q)
	}
	return valid, resolvedLanguage, nil
}

func (s *PlayService) getQuizPoolByLanguage(ctx context.Context, language string) ([]PlayQuizQuestionDB, string, error) {
	lang := normalizeQuizLanguage(language)
	questions, err := s.repo.ListQuizQuestions(ctx, lang)
	if err != nil {
		return nil, "", err
	}
	if len(questions) == 0 && lang != "en" {
		questions, err = s.repo.ListQuizQuestions(ctx, "en")
		if err != nil {
			return nil, "", err
		}
		lang = "en"
	}
	return questions, lang, nil
}

func (s *PlayService) GetQuizToday(ctx context.Context, userID int64, language string) (*PlayQuizToday, error) {
	rt := s.GetRuntime(ctx)
	now := s.serverNow()
	date := s.serverDate(now)
	dateKey := date.Format("2006-01-02")
	out := &PlayQuizToday{
		Enabled:                   rt.QuizEnabled,
		CouponPoolReady:           true,
		GrowthGovernanceAvailable: true,
		RewardPerCorrect:          rt.QuizRewardPerCorrect,
		ServerDate:                dateKey,
	}
	if !rt.QuizEnabled {
		return out, nil
	}
	if userID > 0 {
		eligibility, err := s.growthEligibility(ctx, userID, s.serverNow())
		if err != nil {
			return nil, err
		}
		out.GrowthEligibility = eligibility
		if eligibility.RewardMode == PlayGrowthRewardRedeemable {
			_, governanceAvailable, governanceReason := s.growthGovernanceForStatus(ctx, userID, now)
			out.GrowthGovernanceAvailable = governanceAvailable
			out.GrowthGovernanceReason = governanceReason
			if !governanceAvailable {
				out.CouponPoolReady = false
			}
		}
	}
	var attempt *PlayQuizAttemptDB
	if userID > 0 {
		var err error
		attempt, err = s.repo.GetQuizAttempt(ctx, userID, date)
		if err != nil {
			return nil, err
		}
	}
	ready := false
	var err error
	if out.GrowthGovernanceAvailable {
		ready, err = s.couponRewardPoolReady(ctx, CouponRewardActivityQuiz)
		if err != nil {
			return nil, err
		}
		out.CouponPoolReady = ready
	}
	if attempt != nil {
		out.AlreadySubmitted = true
		out.PreviousScore = attempt.Score
		out.PreviousTotal = attempt.Total
		out.PreviousReward = attempt.RewardAmount
		out.PreviousRewardType = PlayRewardTypeNone
		if attempt.GrowthRewardMode == PlayGrowthRewardEnergy {
			out.PreviousGrowthEnergy = 1
		}
		if attempt.RewardAmount > 0 {
			out.PreviousRewardType = PlayRewardTypeBalance
		}
		if reader, ok := s.couponRewardIssuer.(CouponRewardReplayReader); ok && reader != nil {
			issue, issueErr := reader.GetCouponRewardIssueByIdempotency(
				ctx,
				userID,
				fmt.Sprintf("quiz:%d:%s", userID, dateKey),
			)
			if issueErr != nil {
				return nil, issueErr
			}
			if issue != nil {
				out.PreviousRewardType = PlayRewardTypeCoupon
				out.PreviousCoupon = playCouponRewardSummary(issue)
				out.PreviousCouponPoolVersion = couponPoolVersion(issue)
			}
		}
		if reader, ok := s.redeemRewardIssuer.(RedeemCodeRewardReplayReader); ok && reader != nil {
			code, codeErr := reader.GetRedeemCodeRewardByIssueRef(
				ctx,
				userID,
				string(CouponRewardActivityQuiz),
				fmt.Sprintf("quiz:%d:%s", userID, dateKey),
			)
			if codeErr != nil {
				return nil, codeErr
			}
			if code != nil {
				if out.PreviousCoupon != nil {
					return nil, fmt.Errorf("quiz replay has conflicting coupon and redeem rewards")
				}
				out.PreviousRewardType = PlayRewardTypeRedeem
				out.PreviousRedeemCode = playRedeemCodeRewardSummary(code)
				out.PreviousCouponPoolVersion = code.RewardPoolVersion
			}
		}
		return out, nil
	}
	if !ready && out.GrowthEligibility.RewardMode != PlayGrowthRewardEnergy && out.GrowthGovernanceAvailable {
		return out, nil
	}

	questions, _, err := s.resolveDailyQuizQuestions(ctx, userID, language, rt.QuizQuestionsPerDay)
	if err != nil {
		return nil, err
	}
	out.Questions = make([]PlayQuizQuestion, 0, len(questions))
	for _, q := range questions {
		var options []string
		if err := json.Unmarshal([]byte(q.OptionsJSON), &options); err != nil {
			continue
		}
		out.Questions = append(out.Questions, PlayQuizQuestion{
			ID:      q.ID,
			Prompt:  q.Prompt,
			Options: options,
		})
	}
	return out, nil
}

func (s *PlayService) SubmitQuiz(ctx context.Context, userID int64, language string, answers []PlayQuizAnswer) (*PlayQuizSubmitResult, error) {
	rt := s.GetRuntime(ctx)
	if !rt.QuizEnabled {
		return nil, ErrPlayFeatureDisabled
	}
	now := s.serverNow()
	growthEligibility, err := s.growthEligibility(ctx, userID, now)
	if err != nil {
		return nil, err
	}
	var growthGovernance *PlayGrowthGovernanceState
	if growthEligibility.RewardMode == PlayGrowthRewardRedeemable {
		if err := s.requireCouponRewardPool(ctx, CouponRewardActivityQuiz); err != nil {
			return nil, err
		}
	}
	if userID <= 0 {
		return nil, ErrPlayQuizInvalidAnswer
	}
	if len(answers) == 0 {
		return nil, ErrPlayQuizInvalidAnswer
	}

	date := s.serverDate(now)
	dateKey := date.Format("2006-01-02")
	idempotencyKey := fmt.Sprintf("quiz:%d:%s", userID, dateKey)
	if existing, err := s.repo.GetQuizAttempt(ctx, userID, date); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, ErrPlayQuizAlreadyDone
	}

	questions, _, err := s.resolveDailyQuizQuestions(ctx, userID, language, rt.QuizQuestionsPerDay)
	if err != nil {
		return nil, err
	}
	if len(questions) == 0 {
		return nil, ErrPlayQuizInvalidAnswer
	}
	if len(answers) != len(questions) {
		return nil, ErrPlayQuizInvalidAnswer
	}
	byID := make(map[int64]PlayQuizQuestionDB, len(questions))
	for _, q := range questions {
		byID[q.ID] = q
	}

	score := 0
	total := len(questions)
	answerDetail := make(map[string]any, len(answers))
	answeredQuestionIDs := make(map[int64]struct{}, len(answers))
	for _, ans := range answers {
		q, ok := byID[ans.QuestionID]
		if !ok {
			return nil, ErrPlayQuizInvalidAnswer
		}
		if _, duplicate := answeredQuestionIDs[ans.QuestionID]; duplicate {
			return nil, ErrPlayQuizInvalidAnswer
		}
		answeredQuestionIDs[ans.QuestionID] = struct{}{}
		var options []string
		if err := json.Unmarshal([]byte(q.OptionsJSON), &options); err != nil {
			return nil, ErrPlayQuizInvalidAnswer
		}
		if ans.ChoiceIndex < 0 || ans.ChoiceIndex >= len(options) {
			return nil, ErrPlayQuizInvalidAnswer
		}
		if ans.ChoiceIndex == q.CorrectIndex {
			score++
		}
		answerDetail[fmt.Sprintf("%d", ans.QuestionID)] = ans.ChoiceIndex
	}

	if growthEligibility.RewardMode == PlayGrowthRewardEnergy {
		var snapshotID int64
		if err := s.withPlayTx(ctx, func(txCtx context.Context) error {
			if err := s.repo.InsertQuizAttempt(txCtx, userID, date, score, total, 0, answerDetail); err != nil {
				return err
			}
			var snapshotErr error
			snapshotID, snapshotErr = s.createGrowthSnapshot(txCtx, PlayGrowthEligibilitySnapshot{UserID: userID, Source: PlayRewardSourceQuiz, ActionID: idempotencyKey, ActivityDate: date, Eligibility: growthEligibility})
			if snapshotErr != nil {
				return snapshotErr
			}
			if snapshotID > 0 {
				repo, ok := s.growthQualificationRepository()
				if !ok {
					return fmt.Errorf("growth qualification repository disappeared during quiz")
				}
				if err := repo.LinkGrowthEligibilitySnapshot(txCtx, PlayRewardSourceQuiz, userID, date, snapshotID); err != nil {
					return err
				}
				if err := s.insertGrowthEnergy(txCtx, PlayGrowthEnergyLedgerEntry{UserID: userID, Source: PlayRewardSourceQuiz, ActionID: idempotencyKey, Amount: 1, EligibilitySnapshotID: snapshotID}); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			if errors.Is(err, ErrPlayQuizAlreadyDone) || errors.Is(err, ErrPlayRewardDuplicate) {
				return nil, ErrPlayQuizAlreadyDone
			}
			return nil, err
		}
		return &PlayQuizSubmitResult{
			Score:             score,
			Total:             total,
			RewardType:        PlayRewardTypeNone,
			ServerDate:        dateKey,
			GrowthEnergy:      1,
			GrowthEligibility: growthEligibility,
		}, nil
	}

	// The quiz is a single completed daily attempt. Correct answers determine
	// whether it qualifies for a draw, while the balance branch keeps the
	// original full completion reward instead of paying per correct answer.
	completionReward := float64(total) * rt.QuizRewardPerCorrect
	reward := 0.0
	rewardType := PlayRewardTypeNone
	var couponIssue *CouponRewardIssueResult
	var redeemCode *RedeemCode

	if score > 0 {
		growthGovernance, err = s.requireGrowthGovernanceForReward(ctx, userID, now)
		if err != nil {
			return nil, err
		}
		rewardType, err = s.drawCouponRewardType(ctx, CouponRewardActivityQuiz)
		if err != nil {
			return nil, err
		}
		if rewardType == PlayRewardTypeBalance {
			reward = completionReward
		}
	}

	switch rewardType {
	case PlayRewardTypeCoupon:
		if s.entClient == nil {
			return nil, fmt.Errorf("play service: ent client missing")
		}
		tx, err := s.entClient.Tx(ctx)
		if err != nil {
			return nil, fmt.Errorf("begin quiz coupon tx: %w", err)
		}
		defer func() { _ = tx.Rollback() }()
		txCtx := dbent.NewTxContext(ctx, tx)

		// Claim the once-per-day attempt before issuing the coupon. The unique
		// attempt constraint serializes concurrent submissions, so a losing
		// request returns the normal already-completed result without entering
		// the coupon issuer or colliding on its idempotency key.
		if err := s.repo.InsertQuizAttempt(txCtx, userID, date, score, total, 0, answerDetail); err != nil {
			if errors.Is(err, ErrPlayQuizAlreadyDone) {
				return nil, ErrPlayQuizAlreadyDone
			}
			return nil, err
		}
		growthSnapshotID, err := s.createGrowthSnapshot(txCtx, PlayGrowthEligibilitySnapshot{UserID: userID, Source: PlayRewardSourceQuiz, ActionID: idempotencyKey, ActivityDate: date, Eligibility: growthEligibility})
		if err != nil {
			return nil, err
		}
		if growthSnapshotID > 0 {
			growthRepo, ok := s.growthQualificationRepository()
			if !ok {
				return nil, fmt.Errorf("growth qualification repository disappeared during quiz")
			}
			if err := growthRepo.LinkGrowthEligibilitySnapshot(txCtx, PlayRewardSourceQuiz, userID, date, growthSnapshotID); err != nil {
				return nil, err
			}
		}
		if err := s.reserveGrowthRewardBudget(txCtx, growthGovernance, userID, PlayRewardSourceQuiz, idempotencyKey, growthRewardBudgetCost(rewardType, reward)); err != nil {
			return nil, err
		}
		couponIssue, err = s.issueCouponRewardInTx(
			txCtx,
			userID,
			CouponRewardActivityQuiz,
			idempotencyKey,
			dateKey,
			now,
			growthEligibility,
		)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit quiz coupon tx: %w", err)
		}
		reward = 0
	case PlayRewardTypeRedeem:
		if s.entClient == nil {
			return nil, fmt.Errorf("play service: ent client missing")
		}
		tx, err := s.entClient.Tx(ctx)
		if err != nil {
			return nil, fmt.Errorf("begin quiz redeem code tx: %w", err)
		}
		defer func() { _ = tx.Rollback() }()
		txCtx := dbent.NewTxContext(ctx, tx)
		if err := s.repo.InsertQuizAttempt(txCtx, userID, date, score, total, 0, answerDetail); err != nil {
			if errors.Is(err, ErrPlayQuizAlreadyDone) {
				return nil, ErrPlayQuizAlreadyDone
			}
			return nil, err
		}
		growthSnapshotID, err := s.createGrowthSnapshot(txCtx, PlayGrowthEligibilitySnapshot{UserID: userID, Source: PlayRewardSourceQuiz, ActionID: idempotencyKey, ActivityDate: date, Eligibility: growthEligibility})
		if err != nil {
			return nil, err
		}
		if growthSnapshotID > 0 {
			growthRepo, ok := s.growthQualificationRepository()
			if !ok {
				return nil, fmt.Errorf("growth qualification repository disappeared during quiz")
			}
			if err := growthRepo.LinkGrowthEligibilitySnapshot(txCtx, PlayRewardSourceQuiz, userID, date, growthSnapshotID); err != nil {
				return nil, err
			}
		}
		if err := s.reserveGrowthRewardBudget(txCtx, growthGovernance, userID, PlayRewardSourceQuiz, idempotencyKey, growthRewardBudgetCost(rewardType, reward)); err != nil {
			return nil, err
		}
		redeemCode, err = s.issueRedeemCodeRewardInTx(
			txCtx,
			userID,
			CouponRewardActivityQuiz,
			idempotencyKey,
			idempotencyKey,
			now,
			growthEligibility,
		)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit quiz redeem code tx: %w", err)
		}
		reward = 0
	case PlayRewardTypeBalance:
		detail := map[string]any{
			"attempt_date": dateKey,
			"score":        score,
			"total":        total,
			"reward_type":  string(rewardType),
		}
		var growthSnapshotID int64
		if err := s.grantBalanceWithGrowthSnapshot(ctx, userID, reward, PlayRewardSourceQuiz, idempotencyKey, detail, &growthSnapshotID, func(txCtx context.Context) error {
			if err := s.repo.InsertQuizAttempt(txCtx, userID, date, score, total, reward, answerDetail); err != nil {
				return err
			}
			var snapshotErr error
			growthSnapshotID, snapshotErr = s.createGrowthSnapshot(txCtx, PlayGrowthEligibilitySnapshot{UserID: userID, Source: PlayRewardSourceQuiz, ActionID: idempotencyKey, ActivityDate: date, Eligibility: growthEligibility})
			if snapshotErr != nil {
				return snapshotErr
			}
			if growthSnapshotID > 0 {
				growthRepo, ok := s.growthQualificationRepository()
				if !ok {
					return fmt.Errorf("growth qualification repository disappeared during quiz")
				}
				if err := growthRepo.LinkGrowthEligibilitySnapshot(txCtx, PlayRewardSourceQuiz, userID, date, growthSnapshotID); err != nil {
					return err
				}
				detail["growth_eligibility_snapshot_id"] = growthSnapshotID
				detail["growth_rule_version"] = "v1"
				detail["growth_tier"] = growthEligibility.Tier
			}
			if err := s.reserveGrowthRewardBudget(txCtx, growthGovernance, userID, PlayRewardSourceQuiz, idempotencyKey, growthRewardBudgetCost(rewardType, reward)); err != nil {
				return err
			}
			return nil
		}); err != nil {
			if errors.Is(err, ErrPlayQuizAlreadyDone) || errors.Is(err, ErrPlayRewardDuplicate) {
				return nil, ErrPlayQuizAlreadyDone
			}
			return nil, err
		}
	case PlayRewardTypeNone:
		if _, ok := s.growthQualificationRepository(); !ok {
			if err := s.repo.InsertQuizAttempt(ctx, userID, date, score, total, 0, answerDetail); err != nil {
				if errors.Is(err, ErrPlayQuizAlreadyDone) {
					return nil, ErrPlayQuizAlreadyDone
				}
				return nil, err
			}
			break
		}
		var snapshotID int64
		err := s.withPlayTx(ctx, func(txCtx context.Context) error {
			if err := s.repo.InsertQuizAttempt(txCtx, userID, date, score, total, 0, answerDetail); err != nil {
				return err
			}
			var snapshotErr error
			snapshotID, snapshotErr = s.createGrowthSnapshot(txCtx, PlayGrowthEligibilitySnapshot{UserID: userID, Source: PlayRewardSourceQuiz, ActionID: idempotencyKey, ActivityDate: date, Eligibility: growthEligibility})
			if snapshotErr != nil {
				return snapshotErr
			}
			if snapshotID > 0 {
				growthRepo, ok := s.growthQualificationRepository()
				if !ok {
					return fmt.Errorf("growth qualification repository disappeared during quiz")
				}
				return growthRepo.LinkGrowthEligibilitySnapshot(txCtx, PlayRewardSourceQuiz, userID, date, snapshotID)
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, ErrPlayQuizAlreadyDone) {
				return nil, ErrPlayQuizAlreadyDone
			}
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported quiz reward type: %s", rewardType)
	}

	return &PlayQuizSubmitResult{
		Score:             score,
		Total:             total,
		RewardAmount:      reward,
		RewardType:        rewardType,
		Coupon:            playCouponRewardSummary(couponIssue),
		RedeemCode:        playRedeemCodeRewardSummary(redeemCode),
		CouponPoolVersion: couponPoolVersion(couponIssue),
		ServerDate:        dateKey,
		GrowthEligibility: growthEligibility,
	}, nil
}

func (s *PlayService) GetTeamMe(ctx context.Context, userID int64) (*PlayTeamMe, error) {
	rt := s.GetRuntime(ctx)
	out := &PlayTeamMe{Enabled: rt.AgentTeamEnabled}
	if !rt.AgentTeamEnabled || userID <= 0 {
		return out, nil
	}
	team, err := s.buildTeamSummary(ctx, userID)
	if err != nil {
		return nil, err
	}
	out.Team = team
	return out, nil
}

func (s *PlayService) CreateTeam(ctx context.Context, userID int64, name string) (*PlayTeamSummary, error) {
	rt := s.GetRuntime(ctx)
	if !rt.AgentTeamEnabled {
		return nil, ErrPlayFeatureDisabled
	}
	if _, ok := s.repo.(PlayTeamCompetitionLifecycleRepository); ok {
		return s.createTeamWithCompetitionAdmission(ctx, userID, name)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrPlayTeamNameRequired
	}
	code, err := generateTeamInviteCode()
	if err != nil {
		return nil, err
	}
	if s.entClient == nil {
		return nil, fmt.Errorf("play service: ent client missing")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create team tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	if existing, err := s.repo.LockActiveTeamMembership(txCtx, userID); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, ErrPlayTeamAlreadyJoined
	}
	team, err := s.repo.CreateTeam(txCtx, name, userID, code)
	if err != nil {
		return nil, err
	}
	if err := s.repo.JoinTeam(txCtx, team.ID, userID); err != nil {
		return nil, err
	}
	if err := s.repo.InsertTeamEvent(txCtx, PlayTeamEvent{
		TeamID:        team.ID,
		ActorUserID:   userID,
		SubjectUserID: userID,
		Type:          PlayTeamEventCreated,
		Detail:        map[string]any{},
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit create team tx: %w", err)
	}
	return s.buildTeamSummary(ctx, userID)
}

func (s *PlayService) JoinTeam(ctx context.Context, userID int64, inviteCode string) (*PlayTeamSummary, error) {
	rt := s.GetRuntime(ctx)
	if !rt.AgentTeamEnabled {
		return nil, ErrPlayFeatureDisabled
	}
	if _, ok := s.repo.(PlayTeamCompetitionLifecycleRepository); ok {
		return s.joinTeamWithCompetitionInvite(ctx, userID, inviteCode)
	}
	inviteCode = strings.TrimSpace(inviteCode)
	if inviteCode == "" {
		return nil, ErrPlayTeamNotFound
	}
	if s.entClient == nil {
		return nil, fmt.Errorf("play service: ent client missing")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin join team tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	if existing, err := s.repo.LockActiveTeamMembership(txCtx, userID); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, ErrPlayTeamAlreadyJoined
	}
	team, err := s.repo.GetTeamByInviteCode(txCtx, inviteCode)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, ErrPlayTeamNotFound
	}
	team, err = s.repo.LockTeam(txCtx, team.ID)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, ErrPlayTeamNotFound
	}
	if err := s.repo.JoinTeam(txCtx, team.ID, userID); err != nil {
		if errors.Is(err, ErrPlayTeamAlreadyJoined) {
			return nil, ErrPlayTeamAlreadyJoined
		}
		return nil, err
	}
	if err := s.repo.InsertTeamEvent(txCtx, PlayTeamEvent{
		TeamID:        team.ID,
		ActorUserID:   userID,
		SubjectUserID: userID,
		Type:          PlayTeamEventMemberJoined,
		Detail:        map[string]any{},
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		if isPlayTeamUniqueViolation(err) {
			return nil, ErrPlayTeamAlreadyJoined
		}
		return nil, fmt.Errorf("commit join team tx: %w", err)
	}
	return s.buildTeamSummary(ctx, userID)
}

func isPlayTeamUniqueViolation(err error) bool {
	type sqlStateError interface {
		SQLState() string
	}
	var stateErr sqlStateError
	return errors.As(err, &stateErr) && stateErr.SQLState() == "23505"
}

func (s *PlayService) LeaveTeam(ctx context.Context, userID int64) error {
	return s.leaveAndMaybeArchiveTeam(ctx, userID, false)
}

// ArchiveTeam closes a one-member captain membership and archives its team.
// It shares the leave transaction because an archived team must never retain an active member.
func (s *PlayService) ArchiveTeam(ctx context.Context, actorUserID int64) error {
	return s.leaveAndMaybeArchiveTeam(ctx, actorUserID, true)
}

func (s *PlayService) leaveAndMaybeArchiveTeam(ctx context.Context, userID int64, requireArchive bool) error {
	rt := s.GetRuntime(ctx)
	if !rt.AgentTeamEnabled {
		return ErrPlayFeatureDisabled
	}
	if s.entClient == nil {
		return fmt.Errorf("play service: ent client missing")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin leave team tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	currentTeam, err := s.repo.GetUserTeam(txCtx, userID)
	if err != nil {
		return err
	}
	if currentTeam == nil {
		return ErrPlayTeamNotMember
	}
	team, err := s.repo.LockTeam(txCtx, currentTeam.ID)
	if err != nil {
		return err
	}
	if team == nil {
		return ErrPlayTeamNotFound
	}
	membership, err := s.repo.LockActiveTeamMembership(txCtx, userID)
	if err != nil {
		return err
	}
	if membership == nil || membership.TeamID != team.ID {
		return ErrPlayTeamNotMember
	}
	if requireArchive && team.CaptainUserID != userID {
		return ErrPlayTeamCaptainRequired
	}

	archive := false
	if team.CaptainUserID == userID {
		memberCount, err := s.repo.CountActiveTeamMembers(txCtx, team.ID)
		if err != nil {
			return err
		}
		if memberCount > 1 {
			return ErrPlayTeamCaptainMustTransfer
		}
		archive = true
	}
	if err := s.repo.LeaveTeam(txCtx, team.ID, userID); err != nil {
		return err
	}
	if err := s.repo.InsertTeamEvent(txCtx, PlayTeamEvent{
		TeamID:        team.ID,
		ActorUserID:   userID,
		SubjectUserID: userID,
		Type:          PlayTeamEventMemberLeft,
		Detail:        map[string]any{},
	}); err != nil {
		return err
	}
	if archive {
		if err := s.repo.ArchiveTeam(txCtx, team.ID); err != nil {
			return err
		}
		if err := s.repo.InsertTeamEvent(txCtx, PlayTeamEvent{
			TeamID:        team.ID,
			ActorUserID:   userID,
			SubjectUserID: userID,
			Type:          PlayTeamEventArchived,
			Detail:        map[string]any{"reason_code": "last_member_left"},
		}); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit leave team tx: %w", err)
	}
	return nil
}

func (s *PlayService) TransferTeamCaptain(ctx context.Context, actorUserID, targetUserID int64) error {
	rt := s.GetRuntime(ctx)
	if !rt.AgentTeamEnabled {
		return ErrPlayFeatureDisabled
	}
	if actorUserID == targetUserID {
		return ErrPlayTeamCaptainTransferSelf
	}
	if s.entClient == nil {
		return fmt.Errorf("play service: ent client missing")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin transfer team captain tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	currentTeam, err := s.repo.GetUserTeam(txCtx, actorUserID)
	if err != nil {
		return err
	}
	if currentTeam == nil {
		return ErrPlayTeamNotMember
	}
	team, err := s.repo.LockTeam(txCtx, currentTeam.ID)
	if err != nil {
		return err
	}
	if team == nil {
		return ErrPlayTeamNotFound
	}
	actorMembership, err := s.repo.LockActiveTeamMembership(txCtx, actorUserID)
	if err != nil {
		return err
	}
	if actorMembership == nil || actorMembership.TeamID != team.ID {
		return ErrPlayTeamNotMember
	}
	if team.CaptainUserID != actorUserID {
		return ErrPlayTeamCaptainRequired
	}
	targetMembership, err := s.repo.LockActiveTeamMembership(txCtx, targetUserID)
	if err != nil {
		return err
	}
	if targetMembership == nil || targetMembership.TeamID != team.ID {
		return ErrPlayTeamMemberNotFound
	}
	if err := s.repo.TransferTeamCaptain(txCtx, team.ID, targetUserID); err != nil {
		return err
	}
	if err := s.repo.InsertTeamEvent(txCtx, PlayTeamEvent{
		TeamID:        team.ID,
		ActorUserID:   actorUserID,
		SubjectUserID: targetUserID,
		Type:          PlayTeamEventCaptainTransferred,
		Detail:        map[string]any{"previous_captain_user_id": actorUserID},
	}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transfer team captain tx: %w", err)
	}
	return nil
}

func (s *PlayService) RemoveTeamMember(ctx context.Context, actorUserID, targetUserID int64) error {
	rt := s.GetRuntime(ctx)
	if !rt.AgentTeamEnabled {
		return ErrPlayFeatureDisabled
	}
	if actorUserID == targetUserID {
		return ErrPlayTeamCaptainCannotRemoveSelf
	}
	if s.entClient == nil {
		return fmt.Errorf("play service: ent client missing")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin remove team member tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	currentTeam, err := s.repo.GetUserTeam(txCtx, actorUserID)
	if err != nil {
		return err
	}
	if currentTeam == nil {
		return ErrPlayTeamNotMember
	}
	team, err := s.repo.LockTeam(txCtx, currentTeam.ID)
	if err != nil {
		return err
	}
	if team == nil {
		return ErrPlayTeamNotFound
	}
	actorMembership, err := s.repo.LockActiveTeamMembership(txCtx, actorUserID)
	if err != nil {
		return err
	}
	if actorMembership == nil || actorMembership.TeamID != team.ID {
		return ErrPlayTeamNotMember
	}
	if team.CaptainUserID != actorUserID {
		return ErrPlayTeamCaptainRequired
	}
	targetMembership, err := s.repo.LockActiveTeamMembership(txCtx, targetUserID)
	if err != nil {
		return err
	}
	if targetMembership == nil || targetMembership.TeamID != team.ID {
		return ErrPlayTeamMemberNotFound
	}
	if err := s.repo.RemoveTeamMember(txCtx, team.ID, targetUserID); err != nil {
		return err
	}
	if err := s.repo.InsertTeamEvent(txCtx, PlayTeamEvent{
		TeamID:        team.ID,
		ActorUserID:   actorUserID,
		SubjectUserID: targetUserID,
		Type:          PlayTeamEventMemberRemoved,
		Detail:        map[string]any{},
	}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit remove team member tx: %w", err)
	}
	return nil
}

func (s *PlayService) buildTeamSummary(ctx context.Context, userID int64) (*PlayTeamSummary, error) {
	team, err := s.repo.GetUserTeam(ctx, userID)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, nil
	}
	summary, err := s.buildTeamSummaryByID(ctx, team.ID)
	if err != nil || summary == nil {
		return summary, err
	}
	if summary.CaptainID != userID {
		summary.InviteCode = ""
	}
	return summary, nil
}

func (s *PlayService) buildTeamSummaryByID(ctx context.Context, teamID int64) (*PlayTeamSummary, error) {
	teamDB, err := s.repo.GetTeamByID(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if teamDB == nil {
		return nil, nil
	}
	recruiting := true
	if statusRepo, ok := s.repo.(PlayTeamCompetitionStatusRepository); ok {
		value, statusErr := statusRepo.GetTeamRecruiting(ctx, teamID)
		if statusErr != nil {
			return nil, statusErr
		}
		recruiting = value
	}
	members, err := s.repo.ListTeamMembers(ctx, teamID)
	if err != nil {
		return nil, err
	}
	userIDs := make([]int64, 0, len(members))
	for _, m := range members {
		userIDs = append(userIDs, m.UserID)
	}
	now := s.serverNow()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, 0)
	tokenSum, err := s.repo.SumTeamTokenUsage(ctx, userIDs, start, end)
	if err != nil {
		return nil, err
	}
	usageByUser, err := s.repo.ListTeamMemberTokenUsage(ctx, userIDs, start, end)
	if err != nil {
		return nil, err
	}
	for i := range members {
		members[i].TokenSum = usageByUser[members[i].UserID]
		if tokenSum > 0 {
			members[i].TokenPct = int(members[i].TokenSum * 100 / tokenSum)
		}
	}
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return nil, fmt.Errorf("load team summary timezone: %w", err)
	}
	localNow := now.In(shanghai)
	rewardStart := time.Date(localNow.Year(), localNow.Month(), 1, 0, 0, 0, 0, shanghai)
	rewardEnd := rewardStart.AddDate(0, 1, 0)
	contributions, err := s.repo.ListTeamRewardContributions(ctx, teamID, rewardStart, rewardEnd)
	if err != nil {
		return nil, err
	}
	contributions = normalizeTeamContributions(contributions)
	teamSpend := sumTeamContributions(contributions).Round(teamRewardAmountScale)
	spendByUser := make(map[int64]decimal.Decimal, len(contributions))
	for _, contribution := range contributions {
		spendByUser[contribution.UserID] = contribution.Amount
	}
	for i := range members {
		members[i].Spend = spendByUser[members[i].UserID].Round(teamRewardAmountScale)
		if teamSpend.IsPositive() {
			members[i].SpendPct = int(members[i].Spend.Mul(decimal.NewFromInt(100)).Div(teamSpend).IntPart())
		}
	}
	cfg := s.currentCompetitionRewardConfig(ctx)
	reachedThreshold, rewardRate := reachedTeamRewardTier(teamSpend, cfg.Tiers)
	nextThreshold := decimal.Zero
	for _, tier := range cfg.Tiers {
		if tier.Threshold.GreaterThan(teamSpend) {
			nextThreshold = tier.Threshold
			break
		}
	}
	estimatedPool := resolveTeamRewardPool(teamSpend, cfg)
	if estimatedPool.IsPositive() && teamSpend.IsPositive() {
		for i := range members {
			if members[i].Spend.IsPositive() {
				members[i].EstimatedReward = members[i].Spend.Div(teamSpend).Mul(estimatedPool).Round(teamRewardAmountScale)
			}
		}
	}
	summary := &PlayTeamSummary{
		ID:               teamDB.ID,
		Name:             teamDB.Name,
		InviteCode:       teamDB.InviteCode,
		CaptainID:        teamDB.CaptainUserID,
		Recruiting:       recruiting,
		MemberCount:      len(members),
		TokenSum:         tokenSum,
		Members:          members,
		CurrentMonth:     rewardStart.Format("2006-01"),
		TeamSpend:        teamSpend,
		ReachedThreshold: reachedThreshold,
		RewardRate:       rewardRate,
		NextThreshold:    nextThreshold,
		EstimatedPool:    estimatedPool,
		RewardCap:        cfg.Cap,
		RewardTiers:      cfg.Tiers,
	}
	return summary, nil
}
