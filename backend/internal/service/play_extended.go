package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

func (s *PlayService) GetBlindboxStatus(ctx context.Context, userID int64) (*PlayBlindboxStatus, error) {
	rt := s.GetRuntime(ctx)
	now := s.serverNow()
	date := s.serverDate(now)
	out := &PlayBlindboxStatus{
		Enabled:    rt.BlindboxEnabled,
		CostAmount: rt.BlindboxCost,
		DailyLimit: rt.BlindboxDailyLimit,
		ServerDate: date.Format("2006-01-02"),
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
	if rt.BlindboxCost <= 0 {
		return nil, fmt.Errorf("blindbox cost not configured")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("blindbox:%d:%d", userID, time.Now().UnixNano())
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Balance < rt.BlindboxCost {
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

	reward := pickBlindboxReward()
	net := reward - rt.BlindboxCost

	if err := s.grantBalance(ctx, userID, net, PlayRewardSourceBlindbox, idempotencyKey, map[string]any{
		"open_date":      dateKey,
		"cost_amount":    rt.BlindboxCost,
		"reward_amount":  reward,
		"net_amount":     net,
	}, func(txCtx context.Context) error {
		return s.repo.InsertBlindboxOpen(txCtx, userID, date, rt.BlindboxCost, reward, idempotencyKey)
	}); err != nil {
		if errors.Is(err, ErrPlayRewardDuplicate) {
			return nil, ErrPlayRewardDuplicate
		}
		return nil, err
	}

	return &PlayBlindboxOpenResult{
		CostAmount:   rt.BlindboxCost,
		RewardAmount: reward,
		NetAmount:    net,
		OpensToday:   opens + 1,
		ServerDate:   dateKey,
	}, nil
}

func pickBlindboxReward() float64 {
	tiers := []struct {
		amount float64
		weight int64
	}{
		{0.05, 40},
		{0.2, 30},
		{0.5, 20},
		{1.0, 8},
		{2.0, 2},
	}
	var total int64
	for _, t := range tiers {
		total += t.weight
	}
	n, err := rand.Int(rand.Reader, big.NewInt(total))
	if err != nil {
		return 0.2
	}
	var acc int64
	for _, t := range tiers {
		acc += t.weight
		if n.Int64() < acc {
			return t.amount
		}
	}
	return tiers[0].amount
}

func (s *PlayService) GetQuizToday(ctx context.Context, userID int64) (*PlayQuizToday, error) {
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

	questions, err := s.repo.ListQuizQuestions(ctx, rt.QuizQuestionsPerDay)
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

func (s *PlayService) SubmitQuiz(ctx context.Context, userID int64, answers []PlayQuizAnswer) (*PlayQuizSubmitResult, error) {
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

	questions, err := s.repo.ListQuizQuestions(ctx, rt.QuizQuestionsPerDay)
	if err != nil {
		return nil, err
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
	if err := s.repo.JoinTeam(ctx, team.ID, userID); err != nil {
		return nil, err
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
	tokenSum, err := s.repo.SumTeamTokenUsage(ctx, userIDs, start, end)
	if err != nil {
		return nil, err
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
	}
	affiliateInfo, err := s.enrichTeamAffiliate(ctx, teamDB.ID, teamDB.CaptainUserID, tokenSum, rt)
	if err != nil {
		return nil, err
	}
	summary.Affiliate = affiliateInfo
	return summary, nil
}

func generateTeamInviteCode() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate invite code: %w", err)
	}
	return strings.ToUpper(hex.EncodeToString(b)), nil
}
