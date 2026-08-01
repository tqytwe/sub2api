package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

const (
	playTeamPublicListDefaultLimit = 20
	playTeamPublicListMaxLimit     = 50
)

func (s *PlayService) PublicTeamDirectory(ctx context.Context, limit int) (*PlayTeamDirectory, error) {
	start, end, err := currentTeamCompetitionWindow(s.serverNow())
	if err != nil {
		return nil, err
	}
	repo, ok := s.repo.(PlayTeamCompetitionReadRepository)
	if !ok {
		return &PlayTeamDirectory{Month: start.Format("2006-01"), Rows: []PlayTeamDirectoryEntry{}}, nil
	}
	rows, err := repo.ListPublicTeamDirectory(ctx, start, end, normalizeTeamCompetitionListLimit(limit, playTeamPublicListDefaultLimit))
	if err != nil {
		return nil, err
	}
	cfg := s.currentCompetitionRewardConfig(ctx)
	out := &PlayTeamDirectory{Month: start.Format("2006-01"), Rows: make([]PlayTeamDirectoryEntry, 0, len(rows))}
	for _, row := range rows {
		memberCount := row.MemberCount
		if memberCount < 0 {
			memberCount = 0
		}
		out.Rows = append(out.Rows, PlayTeamDirectoryEntry{
			TeamID:                row.TeamID,
			TeamName:              row.TeamName,
			MemberCount:           memberCount,
			MemberCapacity:        PlayTeamMaxMembers,
			MonthlySpend:          row.Spend.Round(teamRewardAmountScale),
			EstimatedPool:         resolveTeamRewardPool(row.Spend, cfg),
			AcceptingApplications: row.Recruiting && memberCount < PlayTeamMaxMembers,
		})
	}
	return out, nil
}

func (s *PlayService) PublicTeamLeaderboard(ctx context.Context, limit int) (*PlayTeamPublicLeaderboard, error) {
	start, end, err := currentTeamCompetitionWindow(s.serverNow())
	if err != nil {
		return nil, err
	}
	repo, ok := s.repo.(PlayTeamCompetitionReadRepository)
	if !ok {
		return &PlayTeamPublicLeaderboard{Month: start.Format("2006-01"), Rows: []PlayTeamPublicLeaderboardEntry{}}, nil
	}
	rows, total, err := repo.ListPublicTeamLeaderboard(ctx, start, end, normalizeTeamCompetitionListLimit(limit, playTeamPublicListMaxLimit))
	if err != nil {
		return nil, err
	}
	cfg := s.currentCompetitionRewardConfig(ctx)
	out := &PlayTeamPublicLeaderboard{
		Month:      start.Format("2006-01"),
		TotalTeams: total,
		Rows:       make([]PlayTeamPublicLeaderboardEntry, 0, len(rows)),
	}
	for _, row := range rows {
		gap := row.GapToPrev
		if gap.IsNegative() {
			gap = decimal.Zero
		}
		out.Rows = append(out.Rows, PlayTeamPublicLeaderboardEntry{
			Rank:          row.Rank,
			TeamID:        row.TeamID,
			TeamName:      row.TeamName,
			MemberCount:   row.MemberCount,
			Spend:         row.Spend.Round(teamRewardAmountScale),
			EstimatedPool: resolveTeamRewardPool(row.Spend, cfg),
			GapToPrevious: gap.Round(teamRewardAmountScale),
		})
	}
	return out, nil
}

func (s *PlayService) ListPublicTeamSeasons(ctx context.Context, limit int) ([]PlayTeamSeason, error) {
	repo, ok := s.repo.(PlayTeamCompetitionReadRepository)
	if !ok {
		return []PlayTeamSeason{}, nil
	}
	rows, err := repo.ListPublicTeamSeasons(ctx, normalizeTeamCompetitionListLimit(limit, 12))
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *PlayService) GetPublicTeamSeason(ctx context.Context, month string, limit int) (*PlayTeamSeasonDetail, error) {
	monthStart, err := parseTeamCompetitionMonth(month)
	if err != nil {
		return nil, err
	}
	repo, ok := s.repo.(PlayTeamCompetitionReadRepository)
	if !ok {
		return nil, ErrPlayTeamSeasonNotFound
	}
	season, rows, total, err := repo.GetPublicTeamSeason(ctx, monthStart, normalizeTeamCompetitionSeasonRankingLimit(limit))
	if err != nil {
		return nil, err
	}
	if season == nil {
		return nil, ErrPlayTeamSeasonNotFound
	}
	return &PlayTeamSeasonDetail{Season: *season, Rows: rows, TotalTeams: total}, nil
}

func currentTeamCompetitionWindow(now time.Time) (time.Time, time.Time, error) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("load team competition timezone: %w", err)
	}
	local := now.In(location)
	start := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location)
	return start, start.AddDate(0, 1, 0), nil
}

func parseTeamCompetitionMonth(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	parsed, err := time.ParseInLocation("2006-01", value, time.UTC)
	if err != nil || parsed.Format("2006-01") != value {
		return time.Time{}, infraerrors.BadRequest("PLAY_TEAM_SEASON_MONTH_INVALID", "team season month must use YYYY-MM")
	}
	return parsed, nil
}

func normalizeTeamCompetitionListLimit(limit, defaultLimit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > playTeamPublicListMaxLimit {
		return playTeamPublicListMaxLimit
	}
	return limit
}

func normalizeTeamCompetitionSeasonRankingLimit(limit int) int {
	if limit <= 0 || limit > playTeamSeasonRankingLimit {
		return playTeamSeasonRankingLimit
	}
	return limit
}

func teamRewardEligibleAt(joinedAt time.Time) time.Time {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return joinedAt
	}
	local := joinedAt.In(location)
	if local.Day() <= 25 {
		return joinedAt
	}
	return time.Date(local.Year(), local.Month()+1, 1, 0, 0, 0, 0, location)
}

func generateTeamInviteCode() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate team invite code: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func isHighEntropyTeamInviteCode(value string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	return err == nil && len(decoded) == 32 && base64.RawURLEncoding.EncodeToString(decoded) == value
}

// PlayTeamJoinApplicationInput deliberately accepts only a target team and a
// short message. Membership, reward eligibility, and all timestamps are owned
// by the backend.
type PlayTeamJoinApplicationInput struct {
	TeamID  int64
	Message string
}

func (s *PlayService) GetTeamAdmissionEligibility(ctx context.Context, userID int64) (*PlayTeamAdmissionEligibility, error) {
	out := &PlayTeamAdmissionEligibility{
		Enabled:         s.GetRuntime(ctx).AgentTeamEnabled,
		MemberCapacity:  PlayTeamMaxMembers,
		ApplicationTTL:  int(playTeamJoinApplicationTTL.Hours()),
		CaptainSLAHours: int(playTeamJoinApplicationSLA.Hours()),
	}
	if !out.Enabled {
		return out, nil
	}
	repo, err := s.teamCompetitionLifecycleRepository()
	if err != nil {
		return nil, err
	}
	team, err := s.repo.GetUserTeam(ctx, userID)
	if err != nil {
		return nil, err
	}
	if team != nil {
		teamID := team.ID
		out.CurrentTeamID = &teamID
		return out, nil
	}
	leftAt, err := repo.GetLatestTeamLeftAt(ctx, userID)
	if err != nil {
		return nil, err
	}
	if leftAt != nil {
		cooldownEndsAt := leftAt.Add(playTeamJoinCooldown)
		if cooldownEndsAt.After(s.serverNow()) {
			out.CooldownActive = true
			out.CooldownEndsAt = &cooldownEndsAt
			return out, nil
		}
	}
	out.CanApplyOrJoin = true
	return out, nil
}

func (s *PlayService) createTeamWithCompetitionAdmission(ctx context.Context, userID int64, name string) (*PlayTeamSummary, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrPlayTeamNameRequired
	}
	if err := s.checkTeamAdmissionRisk(ctx, PlayTeamAdmissionActionCreate, userID, 0); err != nil {
		return nil, err
	}
	repo, err := s.teamCompetitionLifecycleRepository()
	if err != nil {
		return nil, err
	}
	now := s.serverNow()
	inviteCode, err := generateTeamInviteCode()
	if err != nil {
		return nil, err
	}
	invite := PlayTeamInvite{
		Code:      inviteCode,
		RotatedAt: now,
		ExpiresAt: now.Add(playTeamInviteTTL),
	}
	var team *PlayTeamDB
	if err := repo.WithTeamCompetitionAdmissionTx(ctx, func(txCtx context.Context) error {
		if err := s.ensureTeamAdmissionEligibility(txCtx, repo, userID, now); err != nil {
			return err
		}
		created, createErr := repo.CreateTeamWithInvite(txCtx, name, userID, invite)
		if createErr != nil {
			return createErr
		}
		if joinErr := repo.JoinTeamWithEligibility(txCtx, created.ID, userID, now, teamRewardEligibleAt(now)); joinErr != nil {
			if isPlayTeamUniqueViolation(joinErr) {
				return ErrPlayTeamAlreadyJoined
			}
			return joinErr
		}
		if eventErr := s.repo.InsertTeamEvent(txCtx, PlayTeamEvent{
			TeamID:        created.ID,
			ActorUserID:   userID,
			SubjectUserID: userID,
			Type:          PlayTeamEventCreated,
			Detail: map[string]any{
				"reward_eligible_at": teamRewardEligibleAt(now).Format(time.RFC3339),
			},
		}); eventErr != nil {
			return eventErr
		}
		team = created
		return nil
	}); err != nil {
		return nil, err
	}
	if team == nil {
		return nil, fmt.Errorf("create team competition: no team created")
	}
	return s.buildTeamSummary(ctx, userID)
}

func (s *PlayService) joinTeamWithCompetitionInvite(ctx context.Context, userID int64, inviteCode string) (*PlayTeamSummary, error) {
	// Base64URL is case-sensitive. Do not normalize it to uppercase.
	inviteCode = strings.TrimSpace(inviteCode)
	if inviteCode == "" {
		return nil, ErrPlayTeamNotFound
	}
	if err := s.checkTeamAdmissionRisk(ctx, PlayTeamAdmissionActionInviteJoin, userID, 0); err != nil {
		return nil, err
	}
	repo, err := s.teamCompetitionLifecycleRepository()
	if err != nil {
		return nil, err
	}
	now := s.serverNow()
	var teamID int64
	if err := repo.WithTeamCompetitionAdmissionTx(ctx, func(txCtx context.Context) error {
		if err := s.ensureTeamAdmissionEligibility(txCtx, repo, userID, now); err != nil {
			return err
		}
		team, lookupErr := repo.LockTeamByInviteCode(txCtx, inviteCode)
		if lookupErr != nil {
			return lookupErr
		}
		if team == nil {
			return ErrPlayTeamNotFound
		}
		if !team.Recruiting {
			return ErrPlayTeamRecruitmentClosed
		}
		if team.InviteCodeExpiresAt == nil || !team.InviteCodeExpiresAt.After(now) {
			return ErrPlayTeamInviteExpired
		}
		if !isHighEntropyTeamInviteCode(team.InviteCode) {
			// Legacy short codes remain visible to their captain for rotation, but
			// they are no longer valid admission credentials.
			return ErrPlayTeamInviteExpired
		}
		if err := s.ensureTeamCapacity(txCtx, team.ID); err != nil {
			return err
		}
		eligibleAt := teamRewardEligibleAt(now)
		if joinErr := repo.JoinTeamWithEligibility(txCtx, team.ID, userID, now, eligibleAt); joinErr != nil {
			if isPlayTeamUniqueViolation(joinErr) {
				return ErrPlayTeamAlreadyJoined
			}
			return joinErr
		}
		if eventErr := s.repo.InsertTeamEvent(txCtx, PlayTeamEvent{
			TeamID:        team.ID,
			ActorUserID:   userID,
			SubjectUserID: userID,
			Type:          PlayTeamEventMemberJoined,
			Detail: map[string]any{
				"source":             "invite",
				"reward_eligible_at": eligibleAt.Format(time.RFC3339),
			},
		}); eventErr != nil {
			return eventErr
		}
		teamID = team.ID
		return nil
	}); err != nil {
		return nil, err
	}
	if teamID <= 0 {
		return nil, fmt.Errorf("join team competition: no team joined")
	}
	return s.buildTeamSummary(ctx, userID)
}

func (s *PlayService) ApplyToTeam(ctx context.Context, userID int64, input PlayTeamJoinApplicationInput) (*PlayTeamJoinApplication, error) {
	if !s.GetRuntime(ctx).AgentTeamEnabled {
		return nil, ErrPlayFeatureDisabled
	}
	if input.TeamID <= 0 {
		return nil, ErrPlayTeamNotFound
	}
	input.Message = strings.TrimSpace(input.Message)
	if len([]rune(input.Message)) > 300 {
		return nil, infraerrors.BadRequest("PLAY_TEAM_APPLICATION_MESSAGE_INVALID", "team join application message must be at most 300 characters")
	}
	if err := s.checkTeamAdmissionRisk(ctx, PlayTeamAdmissionActionApplication, userID, input.TeamID); err != nil {
		return nil, err
	}
	repo, err := s.teamCompetitionLifecycleRepository()
	if err != nil {
		return nil, err
	}
	now := s.serverNow()
	var application *PlayTeamJoinApplication
	if err := repo.WithTeamCompetitionAdmissionTx(ctx, func(txCtx context.Context) error {
		if err := s.expireUserTeamJoinApplications(txCtx, repo, userID, now); err != nil {
			return err
		}
		if err := s.ensureTeamAdmissionEligibility(txCtx, repo, userID, now); err != nil {
			return err
		}
		team, lookupErr := repo.LockTeamCompetition(txCtx, input.TeamID)
		if lookupErr != nil {
			return lookupErr
		}
		if team == nil {
			return ErrPlayTeamNotFound
		}
		if !team.Recruiting {
			return ErrPlayTeamRecruitmentClosed
		}
		if err := s.ensureTeamCapacity(txCtx, team.ID); err != nil {
			return err
		}
		created, createErr := repo.CreateTeamJoinApplication(txCtx, PlayTeamJoinApplication{
			TeamID:      team.ID,
			ApplicantID: userID,
			Status:      PlayTeamJoinApplicationPending,
			Message:     input.Message,
			RequestedAt: now,
			SLADueAt:    now.Add(playTeamJoinApplicationSLA),
			ExpiresAt:   now.Add(playTeamJoinApplicationTTL),
		})
		if createErr != nil {
			if isPlayTeamUniqueViolation(createErr) {
				return ErrPlayTeamApplicationNotPending
			}
			return createErr
		}
		actor := userID
		if eventErr := repo.RecordTeamJoinApplicationEvent(txCtx, created.ID, team.ID, &actor, "submitted", "", PlayTeamJoinApplicationPending, map[string]any{}); eventErr != nil {
			return eventErr
		}
		application = created
		return nil
	}); err != nil {
		return nil, err
	}
	return application, nil
}

func (s *PlayService) ListMyTeamJoinApplications(ctx context.Context, userID int64, limit int) ([]PlayTeamJoinApplication, error) {
	if !s.GetRuntime(ctx).AgentTeamEnabled {
		return nil, ErrPlayFeatureDisabled
	}
	repo, err := s.teamCompetitionLifecycleRepository()
	if err != nil {
		return nil, err
	}
	now := s.serverNow()
	if err := repo.WithTeamCompetitionAdmissionTx(ctx, func(txCtx context.Context) error {
		return s.expireUserTeamJoinApplications(txCtx, repo, userID, now)
	}); err != nil {
		return nil, err
	}
	return repo.ListUserTeamJoinApplications(ctx, userID, normalizeTeamCompetitionListLimit(limit, 20))
}

// WithdrawTeamJoinApplication lets an applicant cancel only their own pending
// request. The application row is locked so withdrawal cannot race approval.
func (s *PlayService) WithdrawTeamJoinApplication(ctx context.Context, actorUserID, applicationID int64) (*PlayTeamJoinApplication, error) {
	if !s.GetRuntime(ctx).AgentTeamEnabled {
		return nil, ErrPlayFeatureDisabled
	}
	if applicationID <= 0 {
		return nil, ErrPlayTeamApplicationNotFound
	}
	repo, err := s.teamCompetitionLifecycleRepository()
	if err != nil {
		return nil, err
	}
	now := s.serverNow()
	var updated *PlayTeamJoinApplication
	expired := false
	if err := repo.WithTeamCompetitionAdmissionTx(ctx, func(txCtx context.Context) error {
		application, lookupErr := repo.LockTeamJoinApplication(txCtx, applicationID)
		if lookupErr != nil {
			return lookupErr
		}
		if application == nil || application.ApplicantID != actorUserID {
			// Do not reveal another user's application state or existence.
			return ErrPlayTeamApplicationNotFound
		}
		if application.Status == PlayTeamJoinApplicationExpired {
			return ErrPlayTeamApplicationExpired
		}
		if application.Status != PlayTeamJoinApplicationPending {
			return ErrPlayTeamApplicationNotPending
		}
		fromStatus := application.Status
		if !application.ExpiresAt.After(now) {
			application.Status = PlayTeamJoinApplicationExpired
			expired = true
		} else {
			application.Status = PlayTeamJoinApplicationWithdrawn
		}
		application.HandledAt = &now
		application.HandledByID = &actorUserID
		if updateErr := repo.UpdateTeamJoinApplication(txCtx, *application); updateErr != nil {
			return updateErr
		}
		eventType := "withdrawn"
		if expired {
			eventType = "expired"
		}
		if eventErr := repo.RecordTeamJoinApplicationEvent(
			txCtx,
			application.ID,
			application.TeamID,
			&actorUserID,
			eventType,
			fromStatus,
			application.Status,
			map[string]any{},
		); eventErr != nil {
			return eventErr
		}
		updated = application
		return nil
	}); err != nil {
		return nil, err
	}
	if expired {
		return updated, ErrPlayTeamApplicationExpired
	}
	return updated, nil
}

func (s *PlayService) ListCaptainTeamJoinApplications(ctx context.Context, actorUserID int64, limit int) ([]PlayTeamJoinApplication, error) {
	if !s.GetRuntime(ctx).AgentTeamEnabled {
		return nil, ErrPlayFeatureDisabled
	}
	repo, err := s.teamCompetitionLifecycleRepository()
	if err != nil {
		return nil, err
	}
	team, err := s.repo.GetUserTeam(ctx, actorUserID)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, ErrPlayTeamNotMember
	}
	if team.CaptainUserID != actorUserID {
		return nil, ErrPlayTeamCaptainRequired
	}
	return repo.ListCaptainTeamJoinApplications(ctx, team.ID, normalizeTeamCompetitionListLimit(limit, 50))
}

func (s *PlayService) DecideTeamJoinApplication(ctx context.Context, actorUserID, applicationID int64, decision, note string) (*PlayTeamJoinApplication, error) {
	if !s.GetRuntime(ctx).AgentTeamEnabled {
		return nil, ErrPlayFeatureDisabled
	}
	if applicationID <= 0 {
		return nil, ErrPlayTeamApplicationNotFound
	}
	decision = strings.ToLower(strings.TrimSpace(decision))
	if decision != "approve" && decision != "reject" {
		return nil, ErrPlayTeamApplicationDecisionInvalid
	}
	note = strings.TrimSpace(note)
	if len([]rune(note)) > 300 {
		return nil, infraerrors.BadRequest("PLAY_TEAM_APPLICATION_NOTE_INVALID", "team join application decision note must be at most 300 characters")
	}
	repo, err := s.teamCompetitionLifecycleRepository()
	if err != nil {
		return nil, err
	}
	now := s.serverNow()
	var updated *PlayTeamJoinApplication
	expired := false
	if err := repo.WithTeamCompetitionAdmissionTx(ctx, func(txCtx context.Context) error {
		application, lookupErr := repo.LockTeamJoinApplication(txCtx, applicationID)
		if lookupErr != nil {
			return lookupErr
		}
		if application == nil {
			return ErrPlayTeamApplicationNotFound
		}
		team, teamErr := repo.LockTeamCompetition(txCtx, application.TeamID)
		if teamErr != nil {
			return teamErr
		}
		if team == nil {
			return ErrPlayTeamNotFound
		}
		if team.CaptainUserID != actorUserID {
			return ErrPlayTeamCaptainRequired
		}
		if application.Status != PlayTeamJoinApplicationPending {
			return ErrPlayTeamApplicationNotPending
		}
		if !application.ExpiresAt.After(now) {
			application.Status = PlayTeamJoinApplicationExpired
			application.HandledAt = &now
			application.HandledByID = &actorUserID
			if updateErr := repo.UpdateTeamJoinApplication(txCtx, *application); updateErr != nil {
				return updateErr
			}
			if eventErr := repo.RecordTeamJoinApplicationEvent(txCtx, application.ID, team.ID, &actorUserID, "expired", PlayTeamJoinApplicationPending, PlayTeamJoinApplicationExpired, map[string]any{}); eventErr != nil {
				return eventErr
			}
			updated = application
			expired = true
			return nil
		}

		fromStatus := application.Status
		if decision == "approve" {
			if err := s.checkTeamAdmissionRisk(ctx, PlayTeamAdmissionActionApprove, application.ApplicantID, team.ID); err != nil {
				return err
			}
			if err := s.ensureTeamAdmissionEligibility(txCtx, repo, application.ApplicantID, now); err != nil {
				return err
			}
			if !team.Recruiting {
				return ErrPlayTeamRecruitmentClosed
			}
			if err := s.ensureTeamCapacity(txCtx, team.ID); err != nil {
				return err
			}
			eligibleAt := teamRewardEligibleAt(now)
			if joinErr := repo.JoinTeamWithEligibility(txCtx, team.ID, application.ApplicantID, now, eligibleAt); joinErr != nil {
				if isPlayTeamUniqueViolation(joinErr) {
					return ErrPlayTeamAlreadyJoined
				}
				return joinErr
			}
			application.Status = PlayTeamJoinApplicationApproved
			application.HandledAt = &now
			application.HandledByID = &actorUserID
			application.DecisionNote = note
			if updateErr := repo.UpdateTeamJoinApplication(txCtx, *application); updateErr != nil {
				return updateErr
			}
			if eventErr := s.repo.InsertTeamEvent(txCtx, PlayTeamEvent{
				TeamID:        team.ID,
				ActorUserID:   actorUserID,
				SubjectUserID: application.ApplicantID,
				Type:          PlayTeamEventMemberJoined,
				Detail: map[string]any{
					"source":             "application",
					"application_id":     application.ID,
					"reward_eligible_at": eligibleAt.Format(time.RFC3339),
				},
			}); eventErr != nil {
				return eventErr
			}
		} else {
			application.Status = PlayTeamJoinApplicationRejected
			application.HandledAt = &now
			application.HandledByID = &actorUserID
			application.DecisionNote = note
			if updateErr := repo.UpdateTeamJoinApplication(txCtx, *application); updateErr != nil {
				return updateErr
			}
		}
		eventType := "approved"
		if decision == "reject" {
			eventType = "rejected"
		}
		if eventErr := repo.RecordTeamJoinApplicationEvent(txCtx, application.ID, team.ID, &actorUserID, eventType, fromStatus, application.Status, map[string]any{}); eventErr != nil {
			return eventErr
		}
		updated = application
		return nil
	}); err != nil {
		return nil, err
	}
	if expired {
		return updated, ErrPlayTeamApplicationExpired
	}
	return updated, nil
}

func (s *PlayService) RotateTeamInvite(ctx context.Context, actorUserID int64) (*PlayTeamInvite, error) {
	if !s.GetRuntime(ctx).AgentTeamEnabled {
		return nil, ErrPlayFeatureDisabled
	}
	repo, err := s.teamCompetitionLifecycleRepository()
	if err != nil {
		return nil, err
	}
	now := s.serverNow()
	code, err := generateTeamInviteCode()
	if err != nil {
		return nil, err
	}
	invite := PlayTeamInvite{Code: code, RotatedAt: now, ExpiresAt: now.Add(playTeamInviteTTL)}
	if err := repo.WithTeamCompetitionAdmissionTx(ctx, func(txCtx context.Context) error {
		team, err := s.repo.GetUserTeam(txCtx, actorUserID)
		if err != nil {
			return err
		}
		if team == nil {
			return ErrPlayTeamNotMember
		}
		locked, err := repo.LockTeamCompetition(txCtx, team.ID)
		if err != nil {
			return err
		}
		if locked == nil {
			return ErrPlayTeamNotFound
		}
		if locked.CaptainUserID != actorUserID {
			return ErrPlayTeamCaptainRequired
		}
		if err := repo.UpdateTeamInvite(txCtx, locked.ID, invite); err != nil {
			return err
		}
		return s.repo.InsertTeamEvent(txCtx, PlayTeamEvent{
			TeamID:      locked.ID,
			ActorUserID: actorUserID,
			Type:        "invite_rotated",
			Detail: map[string]any{
				"expires_at": invite.ExpiresAt.Format(time.RFC3339),
			},
		})
	}); err != nil {
		return nil, err
	}
	return &invite, nil
}

func (s *PlayService) SetTeamRecruiting(ctx context.Context, actorUserID int64, recruiting bool) error {
	if !s.GetRuntime(ctx).AgentTeamEnabled {
		return ErrPlayFeatureDisabled
	}
	repo, err := s.teamCompetitionLifecycleRepository()
	if err != nil {
		return err
	}
	return repo.WithTeamCompetitionAdmissionTx(ctx, func(txCtx context.Context) error {
		team, err := s.repo.GetUserTeam(txCtx, actorUserID)
		if err != nil {
			return err
		}
		if team == nil {
			return ErrPlayTeamNotMember
		}
		locked, err := repo.LockTeamCompetition(txCtx, team.ID)
		if err != nil {
			return err
		}
		if locked == nil {
			return ErrPlayTeamNotFound
		}
		if locked.CaptainUserID != actorUserID {
			return ErrPlayTeamCaptainRequired
		}
		if err := repo.UpdateTeamRecruiting(txCtx, locked.ID, recruiting); err != nil {
			return err
		}
		return s.repo.InsertTeamEvent(txCtx, PlayTeamEvent{
			TeamID:      locked.ID,
			ActorUserID: actorUserID,
			Type:        "recruitment_updated",
			Detail:      map[string]any{"recruiting": recruiting},
		})
	})
}

func (s *PlayService) ensureTeamAdmissionEligibility(ctx context.Context, repo PlayTeamCompetitionLifecycleRepository, userID int64, now time.Time) error {
	if userID <= 0 {
		return ErrUserNotFound
	}
	active, err := s.repo.LockActiveTeamMembership(ctx, userID)
	if err != nil {
		return err
	}
	if active != nil {
		return ErrPlayTeamAlreadyJoined
	}
	leftAt, err := repo.GetLatestTeamLeftAt(ctx, userID)
	if err != nil {
		return err
	}
	if leftAt != nil && leftAt.Add(playTeamJoinCooldown).After(now) {
		return ErrPlayTeamJoinCooldown
	}
	return nil
}

func (s *PlayService) ensureTeamCapacity(ctx context.Context, teamID int64) error {
	memberCount, err := s.repo.CountActiveTeamMembers(ctx, teamID)
	if err != nil {
		return err
	}
	if memberCount >= PlayTeamMaxMembers {
		return ErrPlayTeamFull
	}
	return nil
}

func (s *PlayService) expireUserTeamJoinApplications(ctx context.Context, repo PlayTeamCompetitionLifecycleRepository, userID int64, now time.Time) error {
	expired, err := repo.ExpirePendingTeamJoinApplications(ctx, userID, now)
	if err != nil {
		return err
	}
	for _, application := range expired {
		if err := repo.RecordTeamJoinApplicationEvent(ctx, application.ID, application.TeamID, nil, "expired", PlayTeamJoinApplicationPending, PlayTeamJoinApplicationExpired, map[string]any{}); err != nil {
			return err
		}
	}
	return nil
}

func (s *PlayService) checkTeamAdmissionRisk(ctx context.Context, action PlayTeamAdmissionAction, userID, teamID int64) error {
	if s == nil || s.teamAdmissionRisk == nil {
		return nil
	}
	if err := s.teamAdmissionRisk.CheckTeamAdmission(ctx, action, userID, teamID); err != nil {
		return ErrPlayTeamAdmissionRiskRejected.WithCause(err)
	}
	return nil
}

func (s *PlayService) teamCompetitionLifecycleRepository() (PlayTeamCompetitionLifecycleRepository, error) {
	repo, ok := s.repo.(PlayTeamCompetitionLifecycleRepository)
	if !ok {
		return nil, infraerrors.ServiceUnavailable("PLAY_TEAM_COMPETITION_UNAVAILABLE", "team competition admission is unavailable")
	}
	return repo, nil
}
