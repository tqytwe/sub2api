package service

import infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"

var (
	ErrPlayCheckinAlreadyDone       = infraerrors.Conflict("PLAY_CHECKIN_ALREADY_DONE", "already checked in today")
	ErrPlayRewardDuplicate          = infraerrors.Conflict("PLAY_REWARD_DUPLICATE", "reward already granted")
	ErrPlayFeatureDisabled          = infraerrors.BadRequest("PLAY_FEATURE_DISABLED", "play feature is disabled")
	ErrPlayInsufficientBalance      = infraerrors.BadRequest("INSUFFICIENT_BALANCE", "insufficient balance")
	ErrPlayBlindboxDailyLimit       = infraerrors.Conflict("PLAY_BLINDBOX_DAILY_LIMIT", "daily blindbox limit reached")
	ErrPlayBlindboxPaidDisabled     = infraerrors.Forbidden("PLAY_BLINDBOX_PAID_DISABLED", "paid blindbox opens are disabled in this region")
	ErrPlayQuizAlreadyDone          = infraerrors.Conflict("PLAY_QUIZ_ALREADY_DONE", "quiz already submitted today")
	ErrPlayQuizInvalidAnswer        = infraerrors.BadRequest("PLAY_QUIZ_INVALID_ANSWER", "invalid quiz answer")
	ErrPlayTeamAlreadyJoined        = infraerrors.Conflict("PLAY_TEAM_ALREADY_JOINED", "already in a squad")
	ErrPlayTeamNotFound             = infraerrors.NotFound("PLAY_TEAM_NOT_FOUND", "squad not found")
	ErrPlayTeamNameRequired         = infraerrors.BadRequest("PLAY_TEAM_NAME_REQUIRED", "squad name is required")
	ErrPlayTeamFull                 = infraerrors.Conflict("PLAY_TEAM_FULL", "team has reached its member limit")
	ErrPlayTeamNotCaptain           = infraerrors.Forbidden("PLAY_TEAM_NOT_CAPTAIN", "only the team captain can perform this action")
	ErrPlayTeamTransferRequired     = infraerrors.Conflict("PLAY_TEAM_TRANSFER_REQUIRED", "transfer captain ownership before leaving")
	ErrPlayTeamJoinRequestNotFound  = infraerrors.NotFound("PLAY_TEAM_JOIN_REQUEST_NOT_FOUND", "team join request not found")
	ErrPlayCheckinMakeupUnavailable = infraerrors.BadRequest("PLAY_CHECKIN_MAKEUP_UNAVAILABLE", "makeup is not available")
	ErrPlayCheckinMakeupAlreadyDone = infraerrors.Conflict("PLAY_CHECKIN_MAKEUP_ALREADY_DONE", "makeup already used for this date")
	ErrPlayArenaPeriodNotSettleable = infraerrors.BadRequest("PLAY_ARENA_PERIOD_NOT_SETTLEABLE", "arena period cannot be settled")
	ErrPlayArenaNoPeriod            = infraerrors.NotFound("PLAY_ARENA_NO_PERIOD", "no arena period to settle")
)
