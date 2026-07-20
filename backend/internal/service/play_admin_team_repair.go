package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/shopspring/decimal"
)

const adminTeamMemberCandidateLimit = 20

func (s *PlayService) ListAdminTeamMemberCandidates(
	ctx context.Context,
	query AdminTeamMemberCandidateQuery,
) (*PlayAdminTeamMemberCandidateList, error) {
	if query.TargetTeamID <= 0 {
		return nil, ErrPlayTeamNotFound
	}
	query.Operation = strings.ToLower(strings.TrimSpace(query.Operation))
	if query.Operation == "" {
		query.Operation = AdminTeamMemberOperationAdd
	}
	if query.Operation != AdminTeamMemberOperationAdd && query.Operation != AdminTeamMemberOperationMove {
		return nil, ErrPlayAdminTeamInvalidOperation
	}
	now := s.serverNow()
	effectiveAt, err := resolveAdminTeamEffectiveAt(query.EffectiveAt, now)
	if err != nil {
		return nil, err
	}
	targetTeam, err := s.repo.GetAdminTeamMeta(ctx, query.TargetTeamID)
	if err != nil {
		return nil, err
	}
	if targetTeam == nil || targetTeam.ArchivedAt != nil {
		return nil, ErrPlayTeamNotFound
	}
	search := strings.TrimSpace(query.Query)
	if search == "" {
		return &PlayAdminTeamMemberCandidateList{
			Items:       []PlayAdminTeamMemberCandidate{},
			EffectiveAt: effectiveAt,
		}, nil
	}
	limit := query.Limit
	if limit <= 0 || limit > adminTeamMemberCandidateLimit {
		limit = adminTeamMemberCandidateLimit
	}
	items, err := s.repo.ListAdminTeamMemberCandidates(ctx, query.TargetTeamID, search, limit)
	if err != nil {
		return nil, err
	}

	monthStart, monthEnd, err := currentTeamRewardWindow(now)
	if err != nil {
		return nil, err
	}
	targetSpend, err := s.repo.GetAdminTeamSpend(ctx, query.TargetTeamID, monthStart, monthEnd)
	if err != nil {
		return nil, err
	}
	cfg := s.currentTeamRewardConfig(ctx)
	if len(cfg.Tiers) == 0 || cfg.Cap.LessThanOrEqual(decimal.Zero) {
		cfg = defaultTeamRewardConfig()
	}

	for i := range items {
		item := &items[i]
		sourceCaptainHasOtherMembers := false
		item.Blockers = []string{}
		item.Warnings = []string{}
		if item.Status != StatusActive {
			item.Blockers = appendCode(item.Blockers, PlayAdminTeamBlockerUserInactive)
		}

		userSpend, loadErr := s.repo.GetUserActualCost(ctx, item.UserID, effectiveAt, monthEnd)
		if loadErr != nil {
			return nil, loadErr
		}
		item.Impact = PlayAdminTeamMemberImpact{
			EffectiveAt:       effectiveAt,
			UserSpend:         userSpend,
			TargetSpendBefore: targetSpend,
			TargetSpendAfter:  targetSpend.Add(userSpend),
			TargetPoolBefore:  resolveTeamRewardPool(targetSpend, cfg),
			TargetPoolAfter:   resolveTeamRewardPool(targetSpend.Add(userSpend), cfg),
		}

		affectedTeamIDs := []int64{query.TargetTeamID}
		excludeMembershipID := int64(0)
		alreadyInTarget := false
		if item.CurrentTeam != nil {
			excludeMembershipID = item.CurrentMembershipID
			if item.CurrentTeam.ID == query.TargetTeamID {
				alreadyInTarget = true
				item.Warnings = appendCode(item.Warnings, PlayAdminTeamWarningAlreadyInTarget)
				item.Impact.TargetSpendAfter = targetSpend
				item.Impact.TargetPoolAfter = item.Impact.TargetPoolBefore
			} else {
				affectedTeamIDs = append(affectedTeamIDs, item.CurrentTeam.ID)
				sourceSpend, loadErr := s.repo.GetAdminTeamSpend(ctx, item.CurrentTeam.ID, monthStart, monthEnd)
				if loadErr != nil {
					return nil, loadErr
				}
				sourceAfter := sourceSpend.Sub(userSpend)
				if sourceAfter.IsNegative() {
					sourceAfter = decimal.Zero
				}
				item.Impact.SourceSpendBefore = sourceSpend
				item.Impact.SourceSpendAfter = sourceAfter
				item.Impact.SourcePoolBefore = resolveTeamRewardPool(sourceSpend, cfg)
				item.Impact.SourcePoolAfter = resolveTeamRewardPool(sourceAfter, cfg)
				if query.Operation == AdminTeamMemberOperationAdd {
					item.Blockers = appendCode(item.Blockers, PlayAdminTeamBlockerMoveRequired)
				}
				if item.CurrentTeam.ArchivedAt != nil {
					item.Warnings = appendCode(item.Warnings, PlayAdminTeamWarningArchivedMembershipRepair)
				}
				if item.IsCaptain {
					memberCount, countErr := s.repo.CountActiveTeamMembers(ctx, item.CurrentTeam.ID)
					if countErr != nil {
						return nil, countErr
					}
					if memberCount > 1 {
						sourceCaptainHasOtherMembers = true
						item.Blockers = appendCode(item.Blockers, PlayAdminTeamBlockerCaptainTransferRequired)
					} else if item.CurrentTeam.ArchivedAt == nil {
						item.Warnings = appendCode(item.Warnings, PlayAdminTeamWarningSourceWillArchive)
					}
				}
				if item.CurrentJoinedAt != nil && effectiveAt.Before(*item.CurrentJoinedAt) {
					item.Blockers = appendCode(item.Blockers, PlayAdminTeamBlockerEffectiveBeforeJoined)
				}
			}
		} else if query.Operation == AdminTeamMemberOperationMove {
			item.Blockers = appendCode(item.Blockers, PlayAdminTeamBlockerNoSource)
		}
		if alreadyInTarget {
			continue
		}
		if !targetTeam.CreatedAt.IsZero() && effectiveAt.Before(targetTeam.CreatedAt) {
			item.Blockers = appendCode(item.Blockers, PlayAdminTeamBlockerEffectiveBeforeTargetCreated)
		}
		if !item.CreatedAt.IsZero() && effectiveAt.Before(item.CreatedAt) {
			item.Blockers = appendCode(item.Blockers, PlayAdminTeamBlockerEffectiveBeforeUserCreated)
		}
		if query.Operation == AdminTeamMemberOperationMove &&
			item.CurrentTeam != nil &&
			item.CurrentTeam.ID != query.TargetTeamID &&
			(!item.IsCaptain || !sourceCaptainHasOtherMembers) {
			captainChanged, historyErr := s.repo.HasTeamCaptainChangeAfter(ctx, item.CurrentTeam.ID, effectiveAt)
			if historyErr != nil {
				return nil, historyErr
			}
			sourceHistoryConflict := captainChanged
			if item.IsCaptain && !sourceCaptainHasOtherMembers {
				otherMembership, membershipErr := s.repo.HasOtherTeamMembershipAfter(
					ctx,
					item.CurrentTeam.ID,
					item.CurrentMembershipID,
					effectiveAt,
				)
				if membershipErr != nil {
					return nil, membershipErr
				}
				sourceHistoryConflict = sourceHistoryConflict || otherMembership
			}
			if sourceHistoryConflict {
				item.Blockers = appendCode(item.Blockers, PlayAdminTeamBlockerSourceHistoryConflict)
				item.Warnings = removeCode(item.Warnings, PlayAdminTeamWarningSourceWillArchive)
			}
		}

		snapshot, snapshotErr := s.repo.HasTeamRewardSnapshotAt(ctx, affectedTeamIDs, effectiveAt)
		if snapshotErr != nil {
			return nil, snapshotErr
		}
		if snapshot {
			item.Blockers = appendCode(item.Blockers, PlayAdminTeamBlockerSettlementSnapshot)
		}
		overlap, overlapErr := s.repo.HasTeamMembershipOverlap(ctx, item.UserID, effectiveAt, excludeMembershipID)
		if overlapErr != nil {
			return nil, overlapErr
		}
		if overlap {
			item.Blockers = appendCode(item.Blockers, PlayAdminTeamBlockerMembershipOverlap)
		}
	}

	return &PlayAdminTeamMemberCandidateList{Items: items, EffectiveAt: effectiveAt}, nil
}

func (s *PlayService) ListAdminTeamEvents(
	ctx context.Context,
	teamID int64,
	limit int,
) ([]PlayAdminTeamEventRecord, error) {
	if teamID <= 0 {
		return nil, ErrPlayTeamNotFound
	}
	team, err := s.repo.GetAdminTeamMeta(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, ErrPlayTeamNotFound
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return s.repo.ListAdminTeamEvents(ctx, teamID, limit)
}

func (s *PlayService) RepairAdminTeamMember(
	ctx context.Context,
	input AdminTeamMemberRepairInput,
) (*AdminTeamMemberRepairResult, error) {
	input.Operation = strings.ToLower(strings.TrimSpace(input.Operation))
	input.Reason = strings.TrimSpace(input.Reason)
	if input.Operation != AdminTeamMemberOperationAdd && input.Operation != AdminTeamMemberOperationMove {
		return nil, ErrPlayAdminTeamInvalidOperation
	}
	if utf8.RuneCountInString(input.Reason) < 10 || utf8.RuneCountInString(input.Reason) > 500 {
		return nil, ErrPlayAdminTeamReasonInvalid
	}
	if input.Operation == AdminTeamMemberOperationMove &&
		(input.ExpectedSourceTeamID == nil || *input.ExpectedSourceTeamID <= 0) {
		return nil, ErrPlayAdminTeamMoveSourceRequired
	}
	effectiveAt, err := s.resolveAdminTeamEffectiveAt(input.EffectiveAt)
	if err != nil {
		return nil, err
	}
	if s.entClient == nil {
		return nil, fmt.Errorf("play service: ent client missing")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin admin team member repair tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	candidate, err := s.repo.LockAdminTeamCandidateUser(txCtx, input.UserID)
	if err != nil {
		return nil, err
	}
	if candidate == nil {
		return nil, ErrUserNotFound
	}
	if candidate.Status != StatusActive {
		return nil, ErrPlayAdminTeamUserInactive
	}
	observedMembership, err := s.repo.GetActiveTeamMembership(txCtx, input.UserID)
	if err != nil {
		return nil, err
	}

	teamIDs := []int64{input.TargetTeamID}
	if observedMembership != nil &&
		observedMembership.TeamID > 0 &&
		observedMembership.TeamID != input.TargetTeamID {
		teamIDs = append(teamIDs, observedMembership.TeamID)
	}
	sort.Slice(teamIDs, func(i, j int) bool { return teamIDs[i] < teamIDs[j] })
	lockedTeams := make(map[int64]*PlayTeamDB, len(teamIDs))
	for _, teamID := range teamIDs {
		team, lockErr := s.repo.LockTeamForAdmin(txCtx, teamID)
		if lockErr != nil {
			return nil, lockErr
		}
		if team == nil {
			return nil, ErrPlayTeamNotFound
		}
		lockedTeams[teamID] = team
	}
	membership, err := s.repo.LockActiveTeamMembership(txCtx, input.UserID)
	if err != nil {
		return nil, err
	}
	if !sameActiveTeamMembership(observedMembership, membership) {
		return nil, ErrPlayAdminTeamSourceConflict
	}

	alreadyInTarget := membership != nil && membership.TeamID == input.TargetTeamID
	var sourceTeamID int64
	if membership != nil {
		sourceTeamID = membership.TeamID
	}
	if !alreadyInTarget {
		switch input.Operation {
		case AdminTeamMemberOperationAdd:
			if membership != nil {
				return nil, ErrPlayAdminTeamMoveRequired
			}
		case AdminTeamMemberOperationMove:
			if membership == nil {
				return nil, ErrPlayAdminTeamNoSource
			}
			if *input.ExpectedSourceTeamID != sourceTeamID {
				return nil, ErrPlayAdminTeamSourceConflict
			}
		}
	}

	targetTeam := lockedTeams[input.TargetTeamID]
	if targetTeam == nil || targetTeam.ArchivedAt != nil {
		return nil, ErrPlayTeamNotFound
	}
	if alreadyInTarget {
		if input.Operation == AdminTeamMemberOperationMove {
			if *input.ExpectedSourceTeamID != input.TargetTeamID {
				return nil, ErrPlayAdminTeamSourceConflict
			}
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit admin team member no-op tx: %w", err)
		}
		return &AdminTeamMemberRepairResult{
			Status:      AdminTeamMemberRepairStatusNoOp,
			TeamID:      input.TargetTeamID,
			UserID:      input.UserID,
			EffectiveAt: membership.JoinedAt,
			Warnings:    []string{PlayAdminTeamWarningAlreadyInTarget},
		}, nil
	}
	if !targetTeam.CreatedAt.IsZero() && effectiveAt.Before(targetTeam.CreatedAt) {
		return nil, ErrPlayAdminTeamEffectiveBeforeTargetCreated
	}
	if !candidate.CreatedAt.IsZero() && effectiveAt.Before(candidate.CreatedAt) {
		return nil, ErrPlayAdminTeamEffectiveBeforeUserCreated
	}

	excludeMembershipID := int64(0)
	if membership != nil {
		excludeMembershipID = membership.ID
		if effectiveAt.Before(membership.JoinedAt) {
			return nil, ErrPlayAdminTeamEffectiveBeforeSourceJoined
		}
	}
	overlap, err := s.repo.HasTeamMembershipOverlap(txCtx, input.UserID, effectiveAt, excludeMembershipID)
	if err != nil {
		return nil, err
	}
	if overlap {
		return nil, ErrPlayAdminTeamMembershipOverlap
	}
	snapshot, err := s.repo.HasTeamRewardSnapshotAt(txCtx, teamIDs, effectiveAt)
	if err != nil {
		return nil, err
	}
	if snapshot {
		return nil, ErrPlayAdminTeamSettlementSnapshotExists
	}

	teamInviteCodes := make([]string, 0, len(lockedTeams))
	for _, team := range lockedTeams {
		if team != nil && strings.TrimSpace(team.InviteCode) != "" {
			teamInviteCodes = append(teamInviteCodes, team.InviteCode)
		}
	}
	safeReason := redactAdminTeamRepairReasonWithSecrets(input.Reason, teamInviteCodes...)

	warnings := []string{}
	if membership != nil {
		sourceTeam := lockedTeams[sourceTeamID]
		if sourceTeam == nil {
			return nil, ErrPlayTeamNotFound
		}
		sourceWillArchive := false
		sourceCaptainIsSoleMember := false
		if sourceTeam.CaptainUserID == input.UserID {
			memberCount, countErr := s.repo.CountActiveTeamMembers(txCtx, sourceTeamID)
			if countErr != nil {
				return nil, countErr
			}
			if memberCount > 1 {
				return nil, ErrPlayAdminTeamCaptainTransferRequired
			}
			sourceCaptainIsSoleMember = true
			sourceWillArchive = sourceTeam.ArchivedAt == nil
		}
		captainChanged, historyErr := s.repo.HasTeamCaptainChangeAfter(txCtx, sourceTeamID, effectiveAt)
		if historyErr != nil {
			return nil, historyErr
		}
		if captainChanged {
			return nil, ErrPlayAdminTeamSourceHistoryConflict
		}
		if sourceCaptainIsSoleMember {
			otherMembership, membershipErr := s.repo.HasOtherTeamMembershipAfter(
				txCtx,
				sourceTeamID,
				membership.ID,
				effectiveAt,
			)
			if membershipErr != nil {
				return nil, membershipErr
			}
			if otherMembership {
				return nil, ErrPlayAdminTeamSourceHistoryConflict
			}
		}
		if sourceTeam.ArchivedAt != nil {
			warnings = appendCode(warnings, PlayAdminTeamWarningArchivedMembershipRepair)
		}
		if err := s.repo.CloseTeamMembershipAt(txCtx, membership.ID, effectiveAt); err != nil {
			return nil, err
		}
		if err := s.repo.InsertTeamEvent(txCtx, PlayTeamEvent{
			TeamID:        sourceTeamID,
			ActorUserID:   input.ActorUserID,
			SubjectUserID: input.UserID,
			Type:          PlayTeamEventAdminMemberMoved,
			Detail: adminTeamMoveEventDetail(
				"source",
				sourceTeamID,
				input.TargetTeamID,
				effectiveAt,
				safeReason,
			),
		}); err != nil {
			return nil, err
		}
		if sourceWillArchive {
			if err := s.repo.ArchiveTeamAt(txCtx, sourceTeamID, effectiveAt); err != nil {
				return nil, err
			}
			if err := s.repo.InsertTeamEvent(txCtx, PlayTeamEvent{
				TeamID:        sourceTeamID,
				ActorUserID:   input.ActorUserID,
				SubjectUserID: input.UserID,
				Type:          PlayTeamEventArchived,
				Detail: map[string]any{
					"reason_code":    "admin_moved_last_captain",
					"reason":         safeReason,
					"target_team_id": input.TargetTeamID,
					"effective_at":   effectiveAt.Format(time.RFC3339Nano),
				},
			}); err != nil {
				return nil, err
			}
			warnings = appendCode(warnings, PlayAdminTeamWarningSourceWillArchive)
		}
	}

	if err := s.repo.JoinTeamAt(txCtx, input.TargetTeamID, input.UserID, effectiveAt); err != nil {
		if errors.Is(err, ErrPlayTeamAlreadyJoined) {
			return nil, ErrPlayAdminTeamSourceConflict
		}
		return nil, err
	}
	eventType := PlayTeamEventAdminMemberAdded
	status := AdminTeamMemberRepairStatusAdded
	eventDetail := map[string]any{
		"operation":    AdminTeamMemberOperationAdd,
		"reason_code":  PlayTeamEventReasonAdminManualMembershipRepair,
		"reason":       safeReason,
		"effective_at": effectiveAt.Format(time.RFC3339Nano),
	}
	if membership != nil {
		eventType = PlayTeamEventAdminMemberMoved
		status = AdminTeamMemberRepairStatusMoved
		eventDetail = adminTeamMoveEventDetail(
			"target",
			sourceTeamID,
			input.TargetTeamID,
			effectiveAt,
			safeReason,
		)
	}
	if err := s.repo.InsertTeamEvent(txCtx, PlayTeamEvent{
		TeamID:        input.TargetTeamID,
		ActorUserID:   input.ActorUserID,
		SubjectUserID: input.UserID,
		Type:          eventType,
		Detail:        eventDetail,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		if isPlayTeamUniqueViolation(err) {
			return nil, ErrPlayAdminTeamSourceConflict
		}
		return nil, fmt.Errorf("commit admin team member repair tx: %w", err)
	}

	result := &AdminTeamMemberRepairResult{
		Status:      status,
		TeamID:      input.TargetTeamID,
		UserID:      input.UserID,
		EffectiveAt: effectiveAt,
		Warnings:    warnings,
	}
	if sourceTeamID > 0 {
		sourceCopy := sourceTeamID
		result.SourceTeamID = &sourceCopy
	}
	return result, nil
}

func sameActiveTeamMembership(observed, locked *PlayTeamMembershipDB) bool {
	if observed == nil || locked == nil {
		return observed == nil && locked == nil
	}
	return observed.ID == locked.ID && observed.TeamID == locked.TeamID
}

func (s *PlayService) resolveAdminTeamEffectiveAt(requested *time.Time) (time.Time, error) {
	return resolveAdminTeamEffectiveAt(requested, s.serverNow())
}

func resolveAdminTeamEffectiveAt(requested *time.Time, now time.Time) (time.Time, error) {
	effectiveAt := now
	if requested != nil {
		effectiveAt = *requested
	}
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Time{}, fmt.Errorf("load admin team repair timezone: %w", err)
	}
	localNow := now.In(shanghai)
	localEffective := effectiveAt.In(shanghai)
	monthStart := time.Date(localNow.Year(), localNow.Month(), 1, 0, 0, 0, 0, shanghai)
	if localEffective.Before(monthStart) {
		return time.Time{}, ErrPlayAdminTeamEffectiveAtOutsideMonth
	}
	if effectiveAt.After(now) {
		return time.Time{}, ErrPlayAdminTeamEffectiveAtFuture
	}
	return effectiveAt, nil
}

func adminTeamMoveEventDetail(
	side string,
	sourceTeamID int64,
	targetTeamID int64,
	effectiveAt time.Time,
	reason string,
) map[string]any {
	return map[string]any{
		"operation":      AdminTeamMemberOperationMove,
		"side":           side,
		"source_team_id": sourceTeamID,
		"target_team_id": targetTeamID,
		"effective_at":   effectiveAt.Format(time.RFC3339Nano),
		"reason_code":    PlayTeamEventReasonAdminManualMembershipRepair,
		"reason":         reason,
	}
}

var adminTeamRepairNarrativeSecretPattern = regexp.MustCompile(
	`(?i)\b((?:api[_-]?key|apikey|access[_-]?token|refresh[_-]?token|id[_-]?token|session[_-]?token|token|authorization|bearer|password|passwd|pwd|secret|client[_-]?secret|private[_-]?key|invite[_-]?code)(?:\s*[:=]\s*|\s+(?:is|是)\s+))(?:["']?)[^"'\s,;，。；、]{4,}(?:["']?)`,
)

var adminTeamGeneratedInviteCodePattern = regexp.MustCompile(`(?i)\b[0-9a-f]{8}\b`)

// RedactAdminTeamRepairReason preserves the operational explanation while
// removing credential-like material before it reaches events or audit logs.
func RedactAdminTeamRepairReason(reason string) string {
	redacted := redactContentModerationSecrets(reason)
	redacted = strings.ReplaceAll(redacted, "[已脱敏]", "***")
	redacted = adminTeamRepairNarrativeSecretPattern.ReplaceAllString(redacted, `${1}***`)
	redacted = adminTeamGeneratedInviteCodePattern.ReplaceAllString(redacted, "***")
	return strings.TrimSpace(redacted)
}

func redactAdminTeamRepairReasonWithSecrets(reason string, secrets ...string) string {
	redacted := RedactAdminTeamRepairReason(reason)
	for _, secret := range secrets {
		secret = strings.TrimSpace(secret)
		if len(secret) < 4 {
			continue
		}
		pattern, err := regexp.Compile(`(?i)` + regexp.QuoteMeta(secret))
		if err != nil {
			continue
		}
		redacted = pattern.ReplaceAllString(redacted, "***")
	}
	return strings.TrimSpace(redacted)
}

func appendCode(items []string, code string) []string {
	for _, existing := range items {
		if existing == code {
			return items
		}
	}
	return append(items, code)
}

func removeCode(items []string, code string) []string {
	out := items[:0]
	for _, existing := range items {
		if existing != code {
			out = append(out, existing)
		}
	}
	return out
}
