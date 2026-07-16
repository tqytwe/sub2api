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
)

func (s *PlayService) GetBlindboxStatus(ctx context.Context, userID int64) (*PlayBlindboxStatus, error) {
	rt := s.GetRuntime(ctx)
	now := s.serverNow()
	date := s.serverDate(now)
	out := &PlayBlindboxStatus{
		Enabled:      rt.BlindboxEnabled,
		CostAmount:   rt.BlindboxPool.Cost,
		BlindboxPool: rt.BlindboxPool,
		DailyLimit:   rt.BlindboxDailyLimit,
		ServerDate:   date.Format("2006-01-02"),
	}
	out.EffectiveLimit = rt.BlindboxDailyLimit
	if !rt.BlindboxEnabled || userID <= 0 {
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
	out.CanOpen = opens < out.EffectiveLimit
	return out, nil
}

func (s *PlayService) OpenBlindbox(ctx context.Context, userID int64, idempotencyKey string) (*PlayBlindboxOpenResult, error) {
	rt := s.GetRuntime(ctx)
	if !rt.BlindboxEnabled {
		return nil, ErrPlayFeatureDisabled
	}
	pool := rt.BlindboxPool
	if err := ValidateBlindboxPool(pool); err != nil {
		return nil, fmt.Errorf("blindbox pool not configured: %w", err)
	}
	cost := pool.Cost
	idempotencyKey, err := scopeBlindboxIdempotencyKey(userID, idempotencyKey)
	if err != nil {
		return nil, err
	}

	now := s.serverNow()
	date := s.serverDate(now)
	dateKey := date.Format("2006-01-02")
	mods, err := s.resolvePlayEffectModifiers(ctx, userID, rt)
	if err != nil {
		return nil, err
	}
	effectiveLimit := rt.BlindboxDailyLimit + mods.BlindboxExtraOpens

	reward, err := s.pickBlindboxReward(pool)
	if err != nil {
		return nil, err
	}
	const openSource = "paid"
	net := reward - cost

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

	if err := s.grantBalanceInTx(txCtx, userID, net, PlayRewardSourceBlindbox, idempotencyKey, map[string]any{
		"open_date":     dateKey,
		"cost_amount":   cost,
		"reward_amount": reward,
		"net_amount":    net,
		"pool_version":  pool.Version,
		"open_source":   openSource,
	}, func(txCtx context.Context) error {
		return s.repo.InsertBlindboxOpenRecord(txCtx, PlayBlindboxOpenRecord{
			UserID:         userID,
			Date:           date,
			Cost:           cost,
			Reward:         reward,
			IdempotencyKey: idempotencyKey,
			PoolVersion:    pool.Version,
			OpenSource:     openSource,
		})
	}); err != nil {
		if errors.Is(err, ErrPlayRewardDuplicate) {
			return nil, ErrPlayRewardDuplicate
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit blindbox open tx: %w", err)
	}

	return &PlayBlindboxOpenResult{
		CostAmount:   cost,
		RewardAmount: reward,
		NetAmount:    net,
		OpensToday:   opens + 1,
		ServerDate:   dateKey,
		PoolVersion:  pool.Version,
		OpenSource:   openSource,
	}, nil
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
		Enabled:          rt.QuizEnabled,
		RewardPerCorrect: rt.QuizRewardPerCorrect,
		ServerDate:       dateKey,
	}
	if !rt.QuizEnabled {
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

	if userID <= 0 {
		return out, nil
	}
	attempt, err := s.repo.GetQuizAttempt(ctx, userID, date)
	if err != nil {
		return nil, err
	}
	if attempt != nil {
		out.AlreadySubmitted = true
		out.PreviousScore = attempt.Score
		out.PreviousTotal = attempt.Total
		out.PreviousReward = attempt.RewardAmount
	}
	return out, nil
}

func (s *PlayService) SubmitQuiz(ctx context.Context, userID int64, language string, answers []PlayQuizAnswer) (*PlayQuizSubmitResult, error) {
	rt := s.GetRuntime(ctx)
	if !rt.QuizEnabled {
		return nil, ErrPlayFeatureDisabled
	}
	if len(answers) == 0 {
		return nil, ErrPlayQuizInvalidAnswer
	}

	now := s.serverNow()
	date := s.serverDate(now)
	dateKey := date.Format("2006-01-02")
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
	for _, ans := range answers {
		q, ok := byID[ans.QuestionID]
		if !ok {
			return nil, ErrPlayQuizInvalidAnswer
		}
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

	reward := float64(score) * rt.QuizRewardPerCorrect
	idempotencyKey := fmt.Sprintf("quiz:%d:%s", userID, dateKey)

	if reward > 0 {
		if err := s.grantBalance(ctx, userID, reward, PlayRewardSourceQuiz, idempotencyKey, map[string]any{
			"attempt_date": dateKey,
			"score":        score,
			"total":        total,
		}, func(txCtx context.Context) error {
			return s.repo.InsertQuizAttempt(txCtx, userID, date, score, total, reward, answerDetail)
		}); err != nil {
			if errors.Is(err, ErrPlayQuizAlreadyDone) || errors.Is(err, ErrPlayRewardDuplicate) {
				return nil, ErrPlayQuizAlreadyDone
			}
			return nil, err
		}
	} else if err := s.repo.InsertQuizAttempt(ctx, userID, date, score, total, 0, answerDetail); err != nil {
		if errors.Is(err, ErrPlayQuizAlreadyDone) {
			return nil, ErrPlayQuizAlreadyDone
		}
		return nil, err
	}

	return &PlayQuizSubmitResult{
		Score:        score,
		Total:        total,
		RewardAmount: reward,
		ServerDate:   dateKey,
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
	return s.buildTeamSummaryByID(ctx, team.ID)
}

func (s *PlayService) JoinTeam(ctx context.Context, userID int64, inviteCode string) (*PlayTeamSummary, error) {
	rt := s.GetRuntime(ctx)
	if !rt.AgentTeamEnabled {
		return nil, ErrPlayFeatureDisabled
	}
	inviteCode = strings.ToUpper(strings.TrimSpace(inviteCode))
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
	return s.buildTeamSummaryByID(ctx, team.ID)
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
			Detail:        map[string]any{"reason": "last_member_left"},
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
	return s.buildTeamSummaryByID(ctx, team.ID)
}

func (s *PlayService) buildTeamSummaryByID(ctx context.Context, teamID int64) (*PlayTeamSummary, error) {
	teamDB, err := s.repo.GetTeamByID(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if teamDB == nil {
		return nil, nil
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
	summary := &PlayTeamSummary{
		ID:          teamDB.ID,
		Name:        teamDB.Name,
		InviteCode:  teamDB.InviteCode,
		CaptainID:   teamDB.CaptainUserID,
		MemberCount: len(members),
		TokenSum:    tokenSum,
		Members:     members,
	}
	return summary, nil
}

func generateTeamInviteCode() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate invite code: %w", err)
	}
	return strings.ToUpper(hex.EncodeToString(b)), nil
}
