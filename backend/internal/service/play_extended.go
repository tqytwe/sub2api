package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (s *PlayService) GetBlindboxStatus(ctx context.Context, userID int64) (*PlayBlindboxStatus, error) {
	rt := s.GetRuntime(ctx)
	now := s.serverNow()
	date := s.serverDate(now)
	out := &PlayBlindboxStatus{
		Enabled:       rt.BlindboxEnabled,
		CostAmount:    rt.BlindboxPool.Cost,
		DailyLimit:    rt.BlindboxDailyLimit,
		ServerDate:    date.Format("2006-01-02"),
		PaidEnabled:   rt.BlindboxPaidEnabled,
		RegionEnabled: rt.BlindboxRegionEnabled,
		Pool:          rt.BlindboxPool,
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
	if ticketRepo, ok := s.repo.(BlindboxTicketRepository); ok {
		balance, err := ticketRepo.GetBlindboxTicketBalance(ctx, userID)
		if err != nil {
			return nil, err
		}
		out.TicketBalance = balance
	}
	out.CanOpen = opens < out.EffectiveLimit && (out.TicketBalance > 0 || (rt.BlindboxPaidEnabled && rt.BlindboxRegionEnabled))
	return out, nil
}

func (s *PlayService) OpenBlindbox(ctx context.Context, userID int64, idempotencyKey string) (*PlayBlindboxOpenResult, error) {
	rt := s.GetRuntime(ctx)
	if !rt.BlindboxEnabled {
		return nil, ErrPlayFeatureDisabled
	}
	pool := rt.BlindboxPool
	if err := ValidateBlindboxPool(pool); err != nil {
		return nil, fmt.Errorf("blindbox cost not configured")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("blindbox:%d:%d", userID, time.Now().UnixNano())
	}

	openSource := "paid"
	cost := pool.Cost
	if ticketRepo, ok := s.repo.(BlindboxTicketRepository); ok {
		balance, err := ticketRepo.GetBlindboxTicketBalance(ctx, userID)
		if err != nil {
			return nil, err
		}
		if balance > 0 {
			openSource = "ticket"
			cost = 0
		}
	}
	if openSource == "paid" && (!rt.BlindboxPaidEnabled || !rt.BlindboxRegionEnabled) {
		return nil, ErrPlayBlindboxPaidDisabled
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Balance < cost {
		return nil, ErrPlayInsufficientBalance
	}

	now := s.serverNow()
	date := s.serverDate(now)
	dateKey := date.Format("2006-01-02")
	opens, err := s.repo.CountBlindboxOpens(ctx, userID, date)
	if err != nil {
		return nil, err
	}
	mods, err := s.resolvePlayEffectModifiers(ctx, userID, rt)
	if err != nil {
		return nil, err
	}
	effectiveLimit := rt.BlindboxDailyLimit + mods.BlindboxExtraOpens
	if opens >= effectiveLimit {
		return nil, ErrPlayBlindboxDailyLimit
	}

	reward := pickBlindboxReward(pool)
	net := reward - cost

	if err := s.grantBalance(ctx, userID, net, PlayRewardSourceBlindbox, idempotencyKey, map[string]any{
		"open_date":     dateKey,
		"cost_amount":   cost,
		"reward_amount": reward,
		"net_amount":    net,
		"pool_version":  pool.Version,
		"open_source":   openSource,
	}, func(txCtx context.Context) error {
		if ticketRepo, ok := s.repo.(BlindboxTicketRepository); ok {
			if openSource == "ticket" {
				if err := ticketRepo.ConsumeBlindboxTicket(txCtx, userID, "ticket-consume:"+idempotencyKey); err != nil {
					return err
				}
			}
			if err := ticketRepo.InsertBlindboxOpenV2(txCtx, userID, date, openSource, pool.Version, cost, reward, idempotencyKey); err != nil {
				return err
			}
			return ticketRepo.InsertBlindboxOpenAudit(txCtx, userID, openSource, pool.Version, idempotencyKey, cost, reward, map[string]any{
				"open_date":  dateKey,
				"net_amount": net,
			})
		}
		return s.repo.InsertBlindboxOpen(txCtx, userID, date, cost, reward, idempotencyKey)
	}); err != nil {
		if errors.Is(err, ErrPlayRewardDuplicate) {
			return nil, ErrPlayRewardDuplicate
		}
		return nil, err
	}
	if activityRepo, ok := s.repo.(PlayActivityRepository); ok {
		_ = activityRepo.InsertPlayActivity(ctx, "blindbox:"+idempotencyKey, "blindbox_opened", userID, "user", userID, map[string]any{
			"reward":       reward,
			"pool_version": pool.Version,
			"open_source":  openSource,
		}, now)
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

func pickBlindboxReward(pool PlayBlindboxPool) float64 {
	n, err := rand.Int(rand.Reader, big.NewInt(blindboxWeightTotal))
	if err != nil {
		return pool.Tiers[0].Amount
	}
	var acc int64
	for _, t := range pool.Tiers {
		acc += t.Weight
		if n.Int64() < acc {
			return t.Amount
		}
	}
	return pool.Tiers[0].Amount
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
	if existing, err := s.repo.GetUserTeam(ctx, userID); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, ErrPlayTeamAlreadyJoined
	}

	code, err := generateTeamInviteCode()
	if err != nil {
		return nil, err
	}
	team, err := s.repo.CreateTeam(ctx, name, userID, code)
	if err != nil {
		return nil, err
	}
	if err := s.repo.JoinTeam(ctx, team.ID, userID); err != nil {
		return nil, err
	}
	if advanced, ok := s.repo.(AdvancedTeamRepository); ok {
		_ = advanced.SetTeamMaxMembers(ctx, team.ID, rt.TeamMaxMembers)
	}
	if activityRepo, ok := s.repo.(PlayActivityRepository); ok {
		_ = activityRepo.InsertPlayActivity(ctx, fmt.Sprintf("team_created:%d", team.ID), "team_created", userID, "team", team.ID, map[string]any{"team_name": team.Name}, s.serverNow())
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
	if existing, err := s.repo.GetUserTeam(ctx, userID); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, ErrPlayTeamAlreadyJoined
	}
	team, err := s.repo.GetTeamByInviteCode(ctx, inviteCode)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, ErrPlayTeamNotFound
	}
	if advanced, ok := s.repo.(AdvancedTeamRepository); ok {
		count, err := advanced.GetTeamMemberCount(ctx, team.ID)
		if err != nil {
			return nil, err
		}
		if count >= rt.TeamMaxMembers {
			return nil, ErrPlayTeamFull
		}
	}
	if err := s.repo.JoinTeam(ctx, team.ID, userID); err != nil {
		return nil, err
	}
	if activityRepo, ok := s.repo.(PlayActivityRepository); ok {
		_ = activityRepo.InsertPlayActivity(ctx, fmt.Sprintf("team_joined:%d:%d", team.ID, userID), "team_joined", userID, "team", team.ID, map[string]any{}, s.serverNow())
	}
	return s.buildTeamSummaryByID(ctx, team.ID)
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
	weekStart := s.serverDate(now).AddDate(0, 0, -((int(now.Weekday()) + 6) % 7))
	_ = s.RefreshGrowthWorld(ctx, now)
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
	rt := s.GetRuntime(ctx)
	summary := &PlayTeamSummary{
		ID:          teamDB.ID,
		Name:        teamDB.Name,
		InviteCode:  teamDB.InviteCode,
		CaptainID:   teamDB.CaptainUserID,
		MemberCount: len(members),
		TokenSum:    tokenSum,
		Members:     members,
		MaxMembers:  rt.TeamMaxMembers,
		IsPublic:    true,
		Level:       1,
	}
	if advanced, ok := s.repo.(AdvancedTeamRepository); ok {
		if engagement, err := advanced.GetTeamEngagement(ctx, teamID, start, weekStart); err != nil {
			return nil, err
		} else if engagement != nil {
			summary.RequestCount = engagement.RequestCount
			summary.ActiveDays = engagement.ActiveDays
			summary.TokenSum = engagement.TokenSum
			summary.Level = engagement.Level
			summary.MaxMembers = engagement.MaxMembers
			summary.IsPublic = engagement.IsPublic
			summary.Weekly = engagement.Weekly
		}
		memberStats, err := advanced.ListTeamMemberEngagement(ctx, userIDs, start, end)
		if err != nil {
			return nil, err
		}
		for i := range members {
			stats := memberStats[members[i].UserID]
			members[i].TokenSum = stats.TokenSum
			members[i].RequestCount = stats.RequestCount
			members[i].ActiveDays = stats.ActiveDays
			if summary.TokenSum > 0 {
				members[i].TokenPct = int(members[i].TokenSum * 100 / summary.TokenSum)
			}
		}
		summary.Members = members
	}
	affiliateInfo, err := s.enrichTeamAffiliate(ctx, teamDB.ID, teamDB.CaptainUserID, summary.TokenSum, rt)
	if err != nil {
		return nil, err
	}
	summary.Affiliate = affiliateInfo
	return summary, nil
}

func (s *PlayService) DiscoverTeams(ctx context.Context, limit int) ([]PlayTeamDiscovery, error) {
	if !s.GetRuntime(ctx).AgentTeamEnabled {
		return nil, ErrPlayFeatureDisabled
	}
	advanced, ok := s.repo.(AdvancedTeamRepository)
	if !ok {
		return []PlayTeamDiscovery{}, nil
	}
	now := s.serverNow()
	_ = s.RefreshGrowthWorld(ctx, now)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	return advanced.ListDiscoverableTeams(ctx, monthStart, limit)
}

func (s *PlayService) RequestTeamJoin(ctx context.Context, userID, teamID int64) error {
	if existing, err := s.repo.GetUserTeam(ctx, userID); err != nil {
		return err
	} else if existing != nil {
		return ErrPlayTeamAlreadyJoined
	}
	advanced, ok := s.repo.(AdvancedTeamRepository)
	if !ok {
		return ErrPlayTeamNotFound
	}
	count, err := advanced.GetTeamMemberCount(ctx, teamID)
	if err != nil {
		return err
	}
	if count >= s.GetRuntime(ctx).TeamMaxMembers {
		return ErrPlayTeamFull
	}
	return advanced.CreateTeamJoinRequest(ctx, teamID, userID)
}

func (s *PlayService) ListTeamJoinRequests(ctx context.Context, captainID int64) ([]PlayTeamJoinRequest, error) {
	team, err := s.repo.GetUserTeam(ctx, captainID)
	if err != nil || team == nil {
		return nil, ErrPlayTeamNotFound
	}
	if team.CaptainUserID != captainID {
		return nil, ErrPlayTeamNotCaptain
	}
	advanced, ok := s.repo.(AdvancedTeamRepository)
	if !ok {
		return []PlayTeamJoinRequest{}, nil
	}
	return advanced.ListTeamJoinRequests(ctx, team.ID)
}

func (s *PlayService) ReviewTeamJoinRequest(ctx context.Context, captainID, requestID int64, approve bool) error {
	advanced, ok := s.repo.(AdvancedTeamRepository)
	if !ok {
		return ErrPlayTeamJoinRequestNotFound
	}
	if approve {
		return advanced.ApproveTeamJoinRequest(ctx, requestID, captainID)
	}
	return advanced.RejectTeamJoinRequest(ctx, requestID, captainID)
}

func (s *PlayService) LeaveTeam(ctx context.Context, userID int64) error {
	team, err := s.repo.GetUserTeam(ctx, userID)
	if err != nil || team == nil {
		return ErrPlayTeamNotFound
	}
	advanced, ok := s.repo.(AdvancedTeamRepository)
	if !ok {
		return ErrPlayTeamNotFound
	}
	count, err := advanced.GetTeamMemberCount(ctx, team.ID)
	if err != nil {
		return err
	}
	if team.CaptainUserID == userID && count > 1 {
		return ErrPlayTeamTransferRequired
	}
	return advanced.LeaveTeam(ctx, team.ID, userID, team.CaptainUserID == userID)
}

func (s *PlayService) TransferTeamCaptain(ctx context.Context, captainID, nextCaptainID int64) error {
	team, err := s.repo.GetUserTeam(ctx, captainID)
	if err != nil || team == nil {
		return ErrPlayTeamNotFound
	}
	advanced, ok := s.repo.(AdvancedTeamRepository)
	if !ok {
		return ErrPlayTeamNotCaptain
	}
	return advanced.TransferTeamCaptain(ctx, team.ID, captainID, nextCaptainID)
}

func (s *PlayService) RemoveTeamMember(ctx context.Context, captainID, memberID int64) error {
	team, err := s.repo.GetUserTeam(ctx, captainID)
	if err != nil || team == nil {
		return ErrPlayTeamNotFound
	}
	advanced, ok := s.repo.(AdvancedTeamRepository)
	if !ok {
		return ErrPlayTeamNotCaptain
	}
	return advanced.RemoveTeamMember(ctx, team.ID, captainID, memberID)
}

func (s *PlayService) ListMyTeamActivity(ctx context.Context, userID, limit int64) ([]PlayPublicActivity, error) {
	team, err := s.repo.GetUserTeam(ctx, userID)
	if err != nil || team == nil {
		return []PlayPublicActivity{}, nil
	}
	advanced, ok := s.repo.(AdvancedTeamRepository)
	if !ok {
		return []PlayPublicActivity{}, nil
	}
	return advanced.ListTeamActivity(ctx, team.ID, int(limit))
}

func generateTeamInviteCode() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate invite code: %w", err)
	}
	return strings.ToUpper(hex.EncodeToString(b)), nil
}
