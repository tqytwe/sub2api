package service

import infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"

var (
	ErrPlayCheckinAlreadyDone          = infraerrors.Conflict("PLAY_CHECKIN_ALREADY_DONE", "already checked in today")
	ErrPlayRewardDuplicate             = infraerrors.Conflict("PLAY_REWARD_DUPLICATE", "reward already granted")
	ErrPlayFeatureDisabled             = infraerrors.BadRequest("PLAY_FEATURE_DISABLED", "play feature is disabled")
	ErrPlayInsufficientBalance         = infraerrors.BadRequest("INSUFFICIENT_BALANCE", "insufficient balance")
	ErrPlayBlindboxDailyLimit          = infraerrors.Conflict("PLAY_BLINDBOX_DAILY_LIMIT", "daily blindbox limit reached")
	ErrPlayQuizAlreadyDone             = infraerrors.Conflict("PLAY_QUIZ_ALREADY_DONE", "quiz already submitted today")
	ErrPlayQuizInvalidAnswer           = infraerrors.BadRequest("PLAY_QUIZ_INVALID_ANSWER", "invalid quiz answer")
	ErrPlayTeamAlreadyJoined           = infraerrors.Conflict("PLAY_TEAM_ALREADY_JOINED", "already in a squad")
	ErrPlayTeamNotFound                = infraerrors.NotFound("PLAY_TEAM_NOT_FOUND", "squad not found")
	ErrPlayTeamNameRequired            = infraerrors.BadRequest("PLAY_TEAM_NAME_REQUIRED", "squad name is required")
	ErrPlayTeamNotMember               = infraerrors.NotFound("PLAY_TEAM_NOT_MEMBER", "active squad membership not found")
	ErrPlayTeamMemberNotFound          = infraerrors.NotFound("PLAY_TEAM_MEMBER_NOT_FOUND", "active squad member not found")
	ErrPlayTeamCaptainRequired         = infraerrors.Forbidden("PLAY_TEAM_CAPTAIN_REQUIRED", "only the active squad captain may perform this action")
	ErrPlayTeamCaptainMustTransfer     = infraerrors.Conflict("PLAY_TEAM_CAPTAIN_MUST_TRANSFER", "transfer captaincy before leaving the squad")
	ErrPlayTeamCaptainTransferSelf     = infraerrors.BadRequest("PLAY_TEAM_CAPTAIN_TRANSFER_SELF", "captaincy must be transferred to another active member")
	ErrPlayTeamCaptainCannotRemoveSelf = infraerrors.BadRequest(
		"PLAY_TEAM_CAPTAIN_CANNOT_REMOVE_SELF",
		"the captain cannot remove themselves",
	)
	ErrPlayAdminTeamInvalidOperation = infraerrors.BadRequest(
		"PLAY_TEAM_ADMIN_OPERATION_INVALID",
		"team member repair operation must be add or move",
	)
	ErrPlayAdminTeamReasonInvalid = infraerrors.BadRequest(
		"PLAY_TEAM_ADMIN_REASON_INVALID",
		"team member repair reason must contain 10 to 500 characters",
	)
	ErrPlayAdminTeamSourceConflict = infraerrors.Conflict(
		"PLAY_TEAM_SOURCE_CONFLICT",
		"team membership changed after the administrator previewed it",
	)
	ErrPlayAdminTeamMoveRequired = infraerrors.Conflict(
		"PLAY_TEAM_MEMBER_MOVE_REQUIRED",
		"user already belongs to another active team; use move",
	)
	ErrPlayAdminTeamMoveSourceRequired = infraerrors.BadRequest(
		"PLAY_TEAM_MEMBER_SOURCE_REQUIRED",
		"expected source team is required for move",
	)
	ErrPlayAdminTeamNoSource = infraerrors.Conflict(
		"PLAY_TEAM_MEMBER_NO_SOURCE",
		"user does not have an active source team membership",
	)
	ErrPlayAdminTeamCaptainTransferRequired = infraerrors.Conflict(
		"PLAY_TEAM_CAPTAIN_TRANSFER_REQUIRED",
		"transfer captaincy before moving a captain who still has other members",
	)
	ErrPlayAdminTeamEffectiveAtOutsideMonth = infraerrors.BadRequest(
		"PLAY_TEAM_EFFECTIVE_AT_OUTSIDE_CURRENT_MONTH",
		"effective time must be within the current Asia/Shanghai calendar month",
	)
	ErrPlayAdminTeamEffectiveAtFuture = infraerrors.BadRequest(
		"PLAY_TEAM_EFFECTIVE_AT_FUTURE",
		"effective time cannot be in the future",
	)
	ErrPlayAdminTeamEffectiveBeforeSourceJoined = infraerrors.Conflict(
		"PLAY_TEAM_EFFECTIVE_BEFORE_SOURCE_JOINED",
		"effective time cannot be before the source membership began",
	)
	ErrPlayAdminTeamEffectiveBeforeTargetCreated = infraerrors.Conflict(
		"PLAY_TEAM_EFFECTIVE_BEFORE_TARGET_CREATED",
		"effective time cannot be before the target team was created",
	)
	ErrPlayAdminTeamEffectiveBeforeUserCreated = infraerrors.Conflict(
		"PLAY_TEAM_EFFECTIVE_BEFORE_USER_CREATED",
		"effective time cannot be before the user was created",
	)
	ErrPlayAdminTeamSourceHistoryConflict = infraerrors.Conflict(
		"PLAY_TEAM_SOURCE_HISTORY_CONFLICT",
		"source team captain or membership history changed after the requested effective time",
	)
	ErrPlayAdminTeamSettlementSnapshotExists = infraerrors.Conflict(
		"PLAY_TEAM_SETTLEMENT_SNAPSHOT_EXISTS",
		"an immutable team settlement snapshot already covers this effective time",
	)
	ErrPlayAdminTeamMembershipOverlap = infraerrors.Conflict(
		"PLAY_TEAM_MEMBERSHIP_OVERLAP",
		"effective time overlaps another historical team membership",
	)
	ErrPlayAdminTeamUserInactive = infraerrors.Conflict(
		"PLAY_TEAM_MEMBER_USER_INACTIVE",
		"disabled users cannot be added to a team",
	)
	ErrPlayCheckinMakeupUnavailable = infraerrors.BadRequest("PLAY_CHECKIN_MAKEUP_UNAVAILABLE", "makeup is not available")
	ErrPlayCheckinMakeupAlreadyDone = infraerrors.Conflict("PLAY_CHECKIN_MAKEUP_ALREADY_DONE", "makeup already used for this date")
	ErrPlayArenaPeriodNotSettleable = infraerrors.BadRequest("PLAY_ARENA_PERIOD_NOT_SETTLEABLE", "arena period cannot be settled")
	ErrPlayArenaNoPeriod            = infraerrors.NotFound("PLAY_ARENA_NO_PERIOD", "no arena period to settle")
)
