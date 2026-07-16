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
	ErrPlayCheckinMakeupUnavailable = infraerrors.BadRequest("PLAY_CHECKIN_MAKEUP_UNAVAILABLE", "makeup is not available")
	ErrPlayCheckinMakeupAlreadyDone = infraerrors.Conflict("PLAY_CHECKIN_MAKEUP_ALREADY_DONE", "makeup already used for this date")
	ErrPlayArenaPeriodNotSettleable = infraerrors.BadRequest("PLAY_ARENA_PERIOD_NOT_SETTLEABLE", "arena period cannot be settled")
	ErrPlayArenaNoPeriod            = infraerrors.NotFound("PLAY_ARENA_NO_PERIOD", "no arena period to settle")
)
