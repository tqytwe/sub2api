package service

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

const (
	PlayRewardSourceCheckin          = "checkin"
	PlayRewardSourceCheckinMilestone = "checkin_milestone"
	PlayRewardSourceCheckinMakeup    = "checkin_makeup"
	PlayRewardSourceBlindbox         = "blindbox"
	PlayRewardSourceQuiz             = "quiz"
	PlayRewardSourceArenaSettlement  = "arena_settlement"
	PlayRewardSourceArenaDaily       = "arena_daily_settlement"
)

type PlayRewardType string

const (
	PlayRewardTypeNone    PlayRewardType = "none"
	PlayRewardTypeBalance PlayRewardType = "balance"
	PlayRewardTypeCoupon  PlayRewardType = "coupon"
	PlayRewardTypeRedeem  PlayRewardType = "redeem_code"
)

// PlayCouponRewardSummary is the user-facing portion of a coupon issuance.
// It intentionally excludes lock, usage, and idempotency metadata.
type PlayCouponRewardSummary struct {
	UserCouponID       int64             `json:"user_coupon_id"`
	TemplateID         int64             `json:"template_id"`
	Name               string            `json:"name"`
	BenefitType        CouponBenefitType `json:"benefit_type"`
	BenefitValue       float64           `json:"benefit_value"`
	MaxDiscountAmount  *float64          `json:"max_discount_amount,omitempty"`
	Currency           string            `json:"currency"`
	ApplicableScopes   []CouponScope     `json:"applicable_scopes"`
	MinimumOrderAmount float64           `json:"minimum_order_amount"`
	ValidFrom          time.Time         `json:"valid_from"`
	ExpiresAt          time.Time         `json:"expires_at"`
}

type PlayRedeemCodeRewardSummary struct {
	ID                int64      `json:"id"`
	Code              string     `json:"code"`
	Type              string     `json:"type"`
	Value             float64    `json:"value"`
	Status            string     `json:"status"`
	BatchName         string     `json:"batch_name,omitempty"`
	IssuedAt          *time.Time `json:"issued_at,omitempty"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	RewardPoolVersion string     `json:"reward_pool_version,omitempty"`
}

type PlayStreakMilestone struct {
	Days  int     `json:"days"`
	Bonus float64 `json:"bonus"`
}

type PlayArenaSettlementTier struct {
	RankMax int     `json:"rank_max"`
	Amount  float64 `json:"amount"`
}

type PlayRechargeBoostStatus struct {
	Active             bool
	ExpiresAt          time.Time
	CheckinMultiplier  float64
	BlindboxExtraOpens int
	ArenaMultiplier    float64
}

type PlayArenaPeriod struct {
	ID         int64
	Name       string
	StartAt    time.Time
	EndAt      time.Time
	Status     string
	PeriodType string
	SettledAt  *time.Time
}

type PlayArenaScoreRow struct {
	Rank        int
	UserID      int64
	DisplayName string
	Email       string
	AvatarURL   string
	TokenSum    int64
}

type PlayArenaDailyRewardLedgerRow struct {
	UserID      int64
	DisplayName string
	AvatarURL   string
	Amount      float64
	Rank        int
	TokenSum    int64
	CreatedAt   time.Time
}

type PlayRewardLedgerEntry struct {
	UserID         int64
	Source         string
	Amount         float64
	IdempotencyKey string
	Detail         map[string]any
}

type PlayBlindboxOpenRecord struct {
	UserID         int64
	Date           time.Time
	Cost           float64
	Reward         float64
	IdempotencyKey string
	PoolVersion    string
	OpenSource     string
	// Replay-only fields come from the immutable reward-ledger detail. They
	// preserve the original result even when VIP or pool settings change later.
	VIPTierSnapshot *PlayVIPStatus
	ExpectedReward  *float64
	RTPCap          *float64
}

type PlayVIPBlindboxPool struct {
	Tier int              `json:"tier"`
	Pool PlayBlindboxPool `json:"pool"`
}

type PlayCheckinStatus struct {
	Enabled                bool
	Eligible               bool
	IneligibleReason       string
	CheckedInToday         bool
	RewardAmount           float64
	CouponPoolReady        bool
	CouponWeightBP         int
	RedeemCodeWeightBP     int
	BalanceWeightBP        int
	ServerDate             string
	StreakCount            int
	NextMilestoneDays      int
	NextMilestoneBonus     float64
	CanMakeup              bool
	MakeupDate             string
	RechargeBoostActive    bool
	BoostCheckinMultiplier float64
}

type PlayCheckinResult struct {
	RewardAmount      float64
	BalanceAdded      float64
	RewardType        PlayRewardType
	Coupon            *PlayCouponRewardSummary
	RedeemCode        *PlayRedeemCodeRewardSummary
	CouponPoolVersion string
	ServerDate        string
	StreakCount       int
	MilestoneBonus    float64
}

type PlayBlindboxStatus struct {
	Enabled             bool
	CouponPoolReady     bool
	CouponPrizes        []PlayCouponPrizePreview
	CouponWeightBP      int
	RedeemCodeWeightBP  int
	BalanceWeightBP     int
	CostAmount          float64
	BlindboxPool        PlayBlindboxPool
	CurrentPool         PlayBlindboxPool
	NextPool            *PlayBlindboxPool
	VIPTier             PlayVIPStatus
	ExpectedReward      float64
	NextExpectedReward  float64
	PoolVersion         string
	RTPCap              float64
	DailyLimit          int
	EffectiveLimit      int
	OpensToday          int
	CanOpen             bool
	ServerDate          string
	RechargeBoostActive bool
	CampaignActive      bool
}

type PlayCouponPrizePreview struct {
	TemplateID int64  `json:"template_id"`
	Name       string `json:"name"`
	WeightBP   int    `json:"weight_bp"`
	Tier       string `json:"tier"`
}

type PlayBlindboxOpenResult struct {
	CostAmount        float64
	RewardAmount      float64
	NetAmount         float64
	RewardType        PlayRewardType
	Coupon            *PlayCouponRewardSummary
	RedeemCode        *PlayRedeemCodeRewardSummary
	CouponPoolVersion string
	OpensToday        int
	ServerDate        string
	PoolVersion       string
	OpenSource        string
	VIPTier           PlayVIPStatus
	ExpectedReward    float64
	RTPCap            float64
}

// PlayBlindboxRecentWin is a privacy-masked public feed row for recent opens.
type PlayBlindboxRecentWin struct {
	UserLabel    string
	RewardAmount float64
	RewardType   PlayRewardType
	CouponName   string
	CreatedAt    time.Time
}

type PlayQuizQuestion struct {
	ID      int64
	Prompt  string
	Options []string
}

type PlayQuizToday struct {
	Enabled                   bool
	CouponPoolReady           bool
	Questions                 []PlayQuizQuestion
	AlreadySubmitted          bool
	PreviousScore             int
	PreviousTotal             int
	PreviousReward            float64
	PreviousRewardType        PlayRewardType
	PreviousCoupon            *PlayCouponRewardSummary
	PreviousRedeemCode        *PlayRedeemCodeRewardSummary
	PreviousCouponPoolVersion string
	RewardPerCorrect          float64
	ServerDate                string
}

type PlayQuizAnswer struct {
	QuestionID  int64
	ChoiceIndex int
}

type PlayQuizSubmitResult struct {
	Score             int
	Total             int
	RewardAmount      float64
	RewardType        PlayRewardType
	Coupon            *PlayCouponRewardSummary
	RedeemCode        *PlayRedeemCodeRewardSummary
	CouponPoolVersion string
	ServerDate        string
}

type PlayTeamMember struct {
	UserID                int64
	DisplayName           string
	Email                 string
	AvatarURL             string
	JoinedAt              time.Time
	TokenSum              int64
	TokenPct              int
	Spend                 decimal.Decimal
	SpendPct              int
	EstimatedReward       decimal.Decimal
	LatestSettlementMonth string
	LatestActualReward    decimal.Decimal
	LatestPayoutStatus    string
	LatestPaidAt          *time.Time
}

type PlayTeamSummary struct {
	ID               int64
	Name             string
	InviteCode       string
	CaptainID        int64
	MemberCount      int
	TokenSum         int64
	Members          []PlayTeamMember
	Affiliate        *PlayTeamAffiliateInfo
	CurrentMonth     string
	TeamSpend        decimal.Decimal
	ReachedThreshold decimal.Decimal
	RewardRate       decimal.Decimal
	NextThreshold    decimal.Decimal
	EstimatedPool    decimal.Decimal
	RewardCap        decimal.Decimal
	RewardTiers      []TeamRewardTier
}

type PlayAdminTeamListItem struct {
	ID                 int64
	Name               string
	InviteCode         string
	CaptainID          int64
	CaptainDisplayName string
	CaptainAvatarURL   string
	CaptainEmail       string
	MemberCount        int
	TokenSum           int64
	TeamSpend          decimal.Decimal
	EstimatedPool      decimal.Decimal
	CreatedAt          time.Time
	ArchivedAt         *time.Time
}

type PlayAdminTeamList struct {
	Items    []PlayAdminTeamListItem
	Total    int
	Page     int
	PageSize int
}

type PlayAdminOpsSummary struct {
	TotalTeams               int
	ActiveTeams              int
	MonthSpend               decimal.Decimal
	EstimatedSharedPool      decimal.Decimal
	PendingFailedSettlements int
	MonthlyArenaRewardBudget float64
	DailyArenaRewardBudget   float64
}

type PlayAdminTeamDetail struct {
	Team        *PlayTeamSummary
	CreatedAt   time.Time
	ArchivedAt  *time.Time
	Settlements []PlayTeamSettlementRecord
}

type PlayTeamMe struct {
	Enabled bool
	Team    *PlayTeamSummary
}

const (
	PlayTeamEventCreated            = "team_created"
	PlayTeamEventMemberJoined       = "member_joined"
	PlayTeamEventMemberLeft         = "member_left"
	PlayTeamEventCaptainTransferred = "captain_transferred"
	PlayTeamEventMemberRemoved      = "member_removed"
	PlayTeamEventArchived           = "team_archived"
	PlayTeamEventAdminMemberAdded   = "admin_member_added"
	PlayTeamEventAdminMemberMoved   = "admin_member_moved"

	PlayTeamEventReasonAdminManualMembershipRepair = "admin_manual_membership_repair"
)

type PlayTeamEvent struct {
	TeamID        int64
	ActorUserID   int64
	SubjectUserID int64
	Type          string
	Detail        map[string]any
}

const (
	AdminTeamMemberOperationAdd  = "add"
	AdminTeamMemberOperationMove = "move"

	AdminTeamMemberRepairStatusAdded = "added"
	AdminTeamMemberRepairStatusMoved = "moved"
	AdminTeamMemberRepairStatusNoOp  = "no_op"

	PlayAdminTeamWarningAlreadyInTarget          = "PLAY_TEAM_MEMBER_ALREADY_IN_TARGET"
	PlayAdminTeamWarningSourceWillArchive        = "PLAY_TEAM_SOURCE_WILL_ARCHIVE"
	PlayAdminTeamWarningArchivedMembershipRepair = "PLAY_TEAM_ARCHIVED_MEMBERSHIP_REPAIR"

	PlayAdminTeamBlockerMoveRequired                 = "PLAY_TEAM_MEMBER_MOVE_REQUIRED"
	PlayAdminTeamBlockerNoSource                     = "PLAY_TEAM_MEMBER_NO_SOURCE"
	PlayAdminTeamBlockerCaptainTransferRequired      = "PLAY_TEAM_CAPTAIN_TRANSFER_REQUIRED"
	PlayAdminTeamBlockerSettlementSnapshot           = "PLAY_TEAM_SETTLEMENT_SNAPSHOT_EXISTS"
	PlayAdminTeamBlockerMembershipOverlap            = "PLAY_TEAM_MEMBERSHIP_OVERLAP"
	PlayAdminTeamBlockerUserInactive                 = "PLAY_TEAM_MEMBER_USER_INACTIVE"
	PlayAdminTeamBlockerEffectiveBeforeJoined        = "PLAY_TEAM_EFFECTIVE_BEFORE_SOURCE_JOINED"
	PlayAdminTeamBlockerEffectiveBeforeTargetCreated = "PLAY_TEAM_EFFECTIVE_BEFORE_TARGET_CREATED"
	PlayAdminTeamBlockerEffectiveBeforeUserCreated   = "PLAY_TEAM_EFFECTIVE_BEFORE_USER_CREATED"
	PlayAdminTeamBlockerSourceHistoryConflict        = "PLAY_TEAM_SOURCE_HISTORY_CONFLICT"
)

// Backward-compatible alias kept for tests and callers that use the shorter name.
const PlayAdminTeamWarningSourceArchived = PlayAdminTeamWarningSourceWillArchive

type PlayAdminTeamReference struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
}

type PlayAdminAffiliateReference struct {
	InviterUserID      int64  `json:"inviter_user_id"`
	InviterDisplayName string `json:"inviter_display_name"`
}

type PlayAdminTeamMemberImpact struct {
	EffectiveAt       time.Time       `json:"effective_at"`
	UserSpend         decimal.Decimal `json:"user_spend"`
	SourceSpendBefore decimal.Decimal `json:"source_spend_before"`
	SourceSpendAfter  decimal.Decimal `json:"source_spend_after"`
	SourcePoolBefore  decimal.Decimal `json:"source_pool_before"`
	SourcePoolAfter   decimal.Decimal `json:"source_pool_after"`
	TargetSpendBefore decimal.Decimal `json:"target_spend_before"`
	TargetSpendAfter  decimal.Decimal `json:"target_spend_after"`
	TargetPoolBefore  decimal.Decimal `json:"target_pool_before"`
	TargetPoolAfter   decimal.Decimal `json:"target_pool_after"`
}

type PlayAdminTeamMemberCandidate struct {
	UserID              int64                        `json:"user_id"`
	Email               string                       `json:"email"`
	Username            string                       `json:"username"`
	DisplayName         string                       `json:"display_name"`
	Status              string                       `json:"status"`
	CreatedAt           time.Time                    `json:"-"`
	CurrentTeam         *PlayAdminTeamReference      `json:"current_team,omitempty"`
	CurrentMembershipID int64                        `json:"-"`
	CurrentJoinedAt     *time.Time                   `json:"current_joined_at,omitempty"`
	IsCaptain           bool                         `json:"is_captain"`
	Affiliate           *PlayAdminAffiliateReference `json:"affiliate,omitempty"`
	Impact              PlayAdminTeamMemberImpact    `json:"impact"`
	Blockers            []string                     `json:"blockers"`
	Warnings            []string                     `json:"warnings"`
}

type PlayAdminTeamMemberCandidateQuery struct {
	TargetTeamID int64
	Query        string
	Operation    string
	EffectiveAt  *time.Time
	Limit        int
}

type AdminTeamMemberCandidateQuery = PlayAdminTeamMemberCandidateQuery

type PlayAdminTeamMemberCandidateList struct {
	Items       []PlayAdminTeamMemberCandidate `json:"items"`
	EffectiveAt time.Time                      `json:"effective_at"`
}

type AdminTeamMemberRepairInput struct {
	TargetTeamID         int64
	UserID               int64
	ActorUserID          int64
	Operation            string
	EffectiveAt          *time.Time
	Reason               string
	ExpectedSourceTeamID *int64
}

type AdminTeamMemberRepairResult struct {
	Status       string    `json:"status"`
	TeamID       int64     `json:"team_id"`
	UserID       int64     `json:"user_id"`
	SourceTeamID *int64    `json:"source_team_id,omitempty"`
	EffectiveAt  time.Time `json:"effective_at"`
	Warnings     []string  `json:"warnings"`
}

type PlayAdminTeamEventRecord struct {
	ID                 int64          `json:"id"`
	TeamID             int64          `json:"team_id"`
	ActorUserID        int64          `json:"actor_user_id"`
	ActorDisplayName   string         `json:"actor_display_name"`
	SubjectUserID      *int64         `json:"subject_user_id,omitempty"`
	SubjectDisplayName string         `json:"subject_display_name,omitempty"`
	Type               string         `json:"event_type"`
	Detail             map[string]any `json:"detail"`
	CreatedAt          time.Time      `json:"created_at"`
}

type PlayArenaCurrent struct {
	Enabled              bool
	Period               *PlayArenaPeriod
	TokenSum             int64
	DisplayTokenSum      int64
	Rank                 int
	TokensToPrevRank     int64
	EstimatedReward      float64
	RechargeBoostActive  bool
	ArenaScoreMultiplier float64
	CampaignActive       bool
}

type PlayArenaSettlementResult struct {
	PeriodID     int64
	PeriodName   string
	WinnersCount int
	TotalAwarded float64
}

type PlayArenaDailyRewardSummary struct {
	Enabled bool
	Recent  *PlayArenaDailyRecentRewardSummary
	Current *PlayArenaDailyCurrentRewardEstimate
}

type PlayArenaDailyRecentRewardSummary struct {
	Period       *PlayArenaPeriod
	SettledAt    *time.Time
	PaidToday    bool
	WinnersCount int
	TotalAmount  float64
	Winners      []PlayArenaDailyRewardWinner
}

type PlayArenaDailyRewardWinner struct {
	Rank        int
	UserID      int64
	DisplayName string
	AvatarURL   string
	TokenSum    int64
	Amount      float64
}

type PlayArenaDailyCurrentRewardEstimate struct {
	Period *PlayArenaPeriod
	Rows   []PlayArenaDailyRewardEstimateRow
}

type PlayArenaDailyRewardEstimateRow struct {
	Rank            int
	UserID          int64
	DisplayName     string
	AvatarURL       string
	TokenSum        int64
	EstimatedReward float64
}

type PlayTeamRewardPublicWinner struct {
	SettlementID int64
	PeriodStart  time.Time
	TeamName     string
	DisplayName  string
	AvatarURL    string
	Amount       float64
	PaidAt       *time.Time
}

type PlayArenaRewardPublicWinner struct {
	Rank        int
	Period      *PlayArenaPeriod
	DisplayName string
	AvatarURL   string
	Amount      float64
	PaidAt      *time.Time
}

type PlayArenaMonthlyRewardSummary struct {
	Enabled      bool
	Period       *PlayArenaPeriod
	SettledAt    *time.Time
	WinnersCount int
	TotalAmount  float64
	Winners      []PlayArenaRewardPublicWinner
}

// Optional interfaces keep lightweight PlayRepository test doubles compatible.
type PlayPublicTeamRewardsRepository interface {
	ListPublicTeamRewardWinners(ctx context.Context, limit int) ([]PlayTeamRewardPublicWinner, error)
}

type PlayMonthlyArenaRewardsRepository interface {
	GetLatestSettledMonthlyArenaPeriod(ctx context.Context) (*PlayArenaPeriod, error)
	ListArenaMonthlyRewardLedger(ctx context.Context, periodID int64) ([]PlayArenaDailyRewardLedgerRow, error)
}

type PlayRepository interface {
	HasCheckin(ctx context.Context, userID int64, date time.Time) (bool, error)
	InsertCheckin(ctx context.Context, userID int64, date time.Time, reward float64, streakCount int) error
	GetCheckinStreakOnDate(ctx context.Context, userID int64, date time.Time) (streak int, found bool, err error)
	InsertRewardLedger(ctx context.Context, entry PlayRewardLedgerEntry) error
	GetActiveArenaPeriod(ctx context.Context, now time.Time) (*PlayArenaPeriod, error)
	EnsureMonthlyArenaPeriod(ctx context.Context, now time.Time) (*PlayArenaPeriod, error)
	GetArenaPeriodByID(ctx context.Context, periodID int64) (*PlayArenaPeriod, error)
	MarkArenaPeriodSettled(ctx context.Context, periodID int64) error
	GetLatestSettledDailyArenaPeriod(ctx context.Context) (*PlayArenaPeriod, error)
	ListArenaDailyRewardLedger(ctx context.Context, periodID int64) ([]PlayArenaDailyRewardLedgerRow, error)
	ListArenaLeaderboard(ctx context.Context, start, end time.Time, limit int) ([]PlayArenaScoreRow, error)
	GetUserArenaScore(ctx context.Context, userID int64, start, end time.Time) (tokenSum int64, rank int, err error)
	GetArenaTokensToPrevRank(ctx context.Context, userID int64, start, end time.Time, rank int, tokenSum int64) (int64, error)
	LockBlindboxOpenUser(ctx context.Context, userID int64) (balance float64, err error)
	UpdatePlayBalance(ctx context.Context, userID int64, amount float64) error
	CountBlindboxOpens(ctx context.Context, userID int64, date time.Time) (int, error)
	FindBlindboxOpenByIdempotency(ctx context.Context, userID int64, idempotencyKey string) (*PlayBlindboxOpenRecord, error)
	InsertBlindboxOpen(ctx context.Context, userID int64, date time.Time, cost, reward float64, idempotencyKey string) error
	InsertBlindboxOpenRecord(ctx context.Context, record PlayBlindboxOpenRecord) error
	ListRecentBlindboxWins(ctx context.Context, limit int) ([]PlayBlindboxRecentWin, error)
	ListQuizQuestions(ctx context.Context, language string) ([]PlayQuizQuestionDB, error)
	ListAdminQuizQuestions(ctx context.Context, filter PlayAdminQuizQuestionFilter) ([]PlayAdminQuizQuestion, int, PlayAdminQuizQuestionStats, error)
	CreateAdminQuizQuestion(ctx context.Context, input PlayAdminQuizQuestionInput) (*PlayAdminQuizQuestion, error)
	UpdateAdminQuizQuestion(ctx context.Context, id int64, input PlayAdminQuizQuestionInput) (*PlayAdminQuizQuestion, error)
	DeleteAdminQuizQuestion(ctx context.Context, id int64) error
	GetQuizAttempt(ctx context.Context, userID int64, date time.Time) (*PlayQuizAttemptDB, error)
	InsertQuizAttempt(ctx context.Context, userID int64, date time.Time, score, total int, reward float64, answers map[string]any) error
	GetUserTeam(ctx context.Context, userID int64) (*PlayTeamDB, error)
	LockAdminTeamCandidateUser(ctx context.Context, userID int64) (*PlayAdminTeamMemberCandidate, error)
	GetActiveTeamMembership(ctx context.Context, userID int64) (*PlayTeamMembershipDB, error)
	LockActiveTeamMembership(ctx context.Context, userID int64) (*PlayTeamMembershipDB, error)
	LockTeam(ctx context.Context, teamID int64) (*PlayTeamDB, error)
	LockTeamForAdmin(ctx context.Context, teamID int64) (*PlayTeamDB, error)
	CreateTeam(ctx context.Context, name string, captainUserID int64, inviteCode string) (*PlayTeamDB, error)
	JoinTeam(ctx context.Context, teamID, userID int64) error
	JoinTeamAt(ctx context.Context, teamID, userID int64, joinedAt time.Time) error
	CountActiveTeamMembers(ctx context.Context, teamID int64) (int, error)
	LeaveTeam(ctx context.Context, teamID, userID int64) error
	CloseTeamMembershipAt(ctx context.Context, membershipID int64, leftAt time.Time) error
	TransferTeamCaptain(ctx context.Context, teamID, captainUserID int64) error
	RemoveTeamMember(ctx context.Context, teamID, userID int64) error
	ArchiveTeam(ctx context.Context, teamID int64) error
	ArchiveTeamAt(ctx context.Context, teamID int64, archivedAt time.Time) error
	InsertTeamEvent(ctx context.Context, event PlayTeamEvent) error
	ListAdminTeamEvents(ctx context.Context, teamID int64, limit int) ([]PlayAdminTeamEventRecord, error)
	ListAdminTeamMemberCandidates(ctx context.Context, targetTeamID int64, query string, limit int) ([]PlayAdminTeamMemberCandidate, error)
	HasTeamRewardSnapshotAt(ctx context.Context, teamIDs []int64, effectiveAt time.Time) (bool, error)
	HasTeamMembershipOverlap(ctx context.Context, userID int64, effectiveAt time.Time, excludeMembershipID int64) (bool, error)
	HasTeamCaptainChangeAfter(ctx context.Context, teamID int64, effectiveAt time.Time) (bool, error)
	HasOtherTeamMembershipAfter(ctx context.Context, teamID, excludeMembershipID int64, effectiveAt time.Time) (bool, error)
	GetAdminTeamSpend(ctx context.Context, teamID int64, start, end time.Time) (decimal.Decimal, error)
	GetUserActualCost(ctx context.Context, userID int64, start, end time.Time) (decimal.Decimal, error)
	WithTeamRewardSnapshotLock(ctx context.Context, teamID int64, fn func(context.Context) error) error
	ListTeamRewardContributions(ctx context.Context, teamID int64, start, end time.Time) ([]TeamContribution, error)
	GetTeamRewardSettlementByTeamPeriod(ctx context.Context, teamID int64, periodStart time.Time) (*PlayTeamSettlement, error)
	CreateTeamRewardSnapshot(ctx context.Context, settlement PlayTeamSettlement, allocations []PlayTeamRewardAllocation) (*PlayTeamSettlement, bool, error)
	GetTeamRewardSettlement(ctx context.Context, settlementID int64) (*PlayTeamSettlement, error)
	ListUnpaidTeamRewardAllocations(ctx context.Context, settlementID int64) ([]PlayTeamRewardAllocation, error)
	MarkTeamRewardSettlementProcessing(ctx context.Context, settlementID int64) error
	ClaimTeamRewardAllocation(ctx context.Context, allocationID int64) (bool, error)
	MarkTeamRewardAllocationPaid(ctx context.Context, allocationID int64) error
	MarkTeamRewardAllocationFailed(ctx context.Context, allocationID int64, message string) error
	RefreshTeamRewardSettlementStatus(ctx context.Context, settlementID int64) (*PlayTeamSettlement, error)
	ListTeamIDsForRewardMonth(ctx context.Context, start, end time.Time) ([]int64, error)
	ListTeamRewardSettlementsByTeam(ctx context.Context, teamID int64, limit int) ([]PlayTeamSettlement, error)
	ListTeamRewardSettlements(ctx context.Context, limit int) ([]PlayTeamSettlement, error)
	ListTeamRewardAllocations(ctx context.Context, settlementID int64) ([]PlayTeamRewardAllocation, error)
	ListUserTeamRewardSettlements(ctx context.Context, userID int64, limit int) ([]PlayUserTeamSettlementRecord, error)
	GetTeamByInviteCode(ctx context.Context, inviteCode string) (*PlayTeamDB, error)
	GetTeamByID(ctx context.Context, teamID int64) (*PlayTeamDB, error)
	CountAdminTeams(ctx context.Context) (total int, active int, err error)
	CountTeamRewardSettlementsNeedingAttention(ctx context.Context) (int, error)
	ListAdminTeamMonthlySpends(ctx context.Context, start, end time.Time) ([]decimal.Decimal, error)
	GetAdminTeamMeta(ctx context.Context, teamID int64) (*PlayAdminTeamListItem, error)
	ListAdminTeams(ctx context.Context, status, query string, start, end time.Time, limit, offset int) ([]PlayAdminTeamListItem, int, error)
	ListTeamMembers(ctx context.Context, teamID int64) ([]PlayTeamMember, error)
	SumTeamTokenUsage(ctx context.Context, userIDs []int64, start, end time.Time) (int64, error)
	ListTeamMemberTokenUsage(ctx context.Context, userIDs []int64, start, end time.Time) (map[int64]int64, error)
	ListActiveCampaigns(ctx context.Context, now time.Time) ([]PlayCampaign, error)
	ListAdminCampaigns(ctx context.Context) ([]PlayCampaign, error)
	CreateAdminCampaign(ctx context.Context, campaign PlayCampaign) (*PlayCampaign, error)
	UpdateAdminCampaign(ctx context.Context, campaign PlayCampaign) (*PlayCampaign, error)
	DeleteAdminCampaign(ctx context.Context, id int64) error
	UpsertQuestProgress(ctx context.Context, userID int64, questDate time.Time, questKey string, completed bool) error
	ListQuestProgress(ctx context.Context, userID int64, questDate time.Time) ([]PlayQuestProgressRow, error)
	GetUserDailyTokenSum(ctx context.Context, userID int64, start, end time.Time) (int64, error)
	EnsureDailyArenaPeriod(ctx context.Context, now time.Time) (*PlayArenaPeriod, error)
	ListExpiredActiveDailyArenaPeriods(ctx context.Context, now time.Time) ([]PlayArenaPeriod, error)
	CountImageStudioJobsToday(ctx context.Context, userID int64, dayStart time.Time) (int, error)
	HasCompletedImageStudioJob(ctx context.Context, userID int64) (bool, error)
	UpsertRechargeBoost(ctx context.Context, userID int64, expiresAt time.Time) error
	GetActiveRechargeBoost(ctx context.Context, userID int64, now time.Time) (*time.Time, error)
	HasCompletedBalanceRechargeSince(ctx context.Context, userID int64, since time.Time) (bool, error)
	CreateMobileFeedback(ctx context.Context, record MobileFeedbackRecord) (*MobileFeedbackRecord, error)
	ListAdminMobileFeedback(ctx context.Context, filter MobileFeedbackListFilter) ([]MobileFeedbackRecord, int64, error)
	GetAdminMobileFeedback(ctx context.Context, id int64) (*MobileFeedbackRecord, error)
	UpdateAdminMobileFeedback(ctx context.Context, id int64, input MobileFeedbackStatusUpdate) (*MobileFeedbackRecord, error)
	ListUserMobileFeedback(ctx context.Context, userID int64, filter MobileFeedbackListFilter) ([]MobileFeedbackRecord, int64, error)
	GetUserMobileFeedback(ctx context.Context, userID, id int64) (*MobileFeedbackRecord, error)
	ListMobileFeedbackMessages(ctx context.Context, userID, feedbackID int64) ([]MobileFeedbackMessage, error)
	CreateUserMobileFeedbackMessage(ctx context.Context, userID, feedbackID int64, content string) (*MobileFeedbackMessage, error)
	CloseUserMobileFeedback(ctx context.Context, userID, id int64) (*MobileFeedbackRecord, error)
}

// PlayMembershipRepository is optional so lightweight PlayRepository test
// doubles and deployments without the new migration remain compatible.
type PlayMembershipRepository interface {
	GetMembershipPaidTotal(ctx context.Context, userID int64) (float64, error)
	SyncMembershipOrderContribution(ctx context.Context, orderID, userID int64, orderType string, paidAmount, refundAmount float64, paidAt *time.Time, status string) error
	ListTeamLeaderboardBase(ctx context.Context, start, end time.Time, limit int) ([]PlayTeamLeaderboardBase, error)
	GetTeamLeaderboardRank(ctx context.Context, teamID int64, start, end time.Time) (rank int, total int, previousSpend decimal.Decimal, err error)
}

type PlayMembershipAdminRow struct {
	UserID       int64
	Email        string
	Username     string
	NetPaid      decimal.Decimal
	RegisteredAt time.Time
	FirstPaidAt  *time.Time
	LastPaidAt   *time.Time
}

type PlayMembershipAdminRepository interface {
	MembershipAdminOverview(ctx context.Context, memberThreshold float64) (totalMembers int, netPaid decimal.Decimal, err error)
	ListMembershipAdminRows(ctx context.Context, query string, memberOnly *bool, memberThreshold float64, page, pageSize int) ([]PlayMembershipAdminRow, int, error)
	ListMembershipPaidTotals(ctx context.Context) (map[int64]decimal.Decimal, error)
	GetMembershipAdminRow(ctx context.Context, userID int64) (*PlayMembershipAdminRow, error)
	ListMembershipContributions(ctx context.Context, userID int64, limit int) ([]PlayMembershipContribution, error)
	ListMembershipTierHistory(ctx context.Context, userID int64, limit int) ([]PlayMembershipTierChange, error)
	CountRecentMembershipTierChanges(ctx context.Context, since time.Time) (upgrades, downgrades int, err error)
	RecordMembershipTierChange(ctx context.Context, change PlayMembershipTierChange) error
	GetVIPConfigVersion(ctx context.Context) (int64, error)
	PublishVIPConfig(ctx context.Context, expectedVersion, actorID int64, reason, tiersJSON string, affected, upgraded, downgraded int, changes []PlayMembershipTierChange) (int64, error)
}

type PlayMembershipContribution struct {
	OrderID      int64           `json:"order_id"`
	OrderType    string          `json:"order_type"`
	PaidAmount   decimal.Decimal `json:"paid_amount"`
	RefundAmount decimal.Decimal `json:"refund_amount"`
	NetAmount    decimal.Decimal `json:"net_amount"`
	PaidAt       *time.Time      `json:"paid_at,omitempty"`
	Status       string          `json:"status"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type PlayMembershipTierChange struct {
	UserID        int64           `json:"user_id,omitempty"`
	OrderID       *int64          `json:"order_id,omitempty"`
	FromTier      int             `json:"from_tier"`
	ToTier        int             `json:"to_tier"`
	NetPaidBefore decimal.Decimal `json:"net_paid_before"`
	NetPaidAfter  decimal.Decimal `json:"net_paid_after"`
	Reason        string          `json:"reason"`
	CreatedAt     time.Time       `json:"created_at"`
}

type PlayAppAnalyticsRepository interface {
	AppAnalytics(ctx context.Context, from, to time.Time, version, channel string) (scans, downloadRedirects, firstLaunches, registeredInstalls, activeUsers, dau, wau, mau int64, funnel []map[string]any, versions []map[string]any, err error)
}

type PlayTeamLeaderboardBase struct {
	TeamID      int64
	TeamName    string
	MemberCount int
	Spend       decimal.Decimal
}

type PlayCheckinEligibilityRepository interface {
	GetCheckinEligibility(ctx context.Context, userID int64, since time.Time, now time.Time) (eligible bool, reason string, err error)
}

type PlayQuizQuestionDB struct {
	ID           int64
	Language     string
	Prompt       string
	OptionsJSON  string
	CorrectIndex int
}

type PlayAdminQuizQuestionFilter struct {
	Language   string
	Active     *bool
	Category   string
	Difficulty string
	Query      string
	Page       int
	PageSize   int
}

type PlayAdminQuizQuestionInput struct {
	Language     string
	Prompt       string
	Options      []string
	CorrectIndex int
	Category     string
	Difficulty   string
	Explanation  string
	SortOrder    int
	Active       bool
}

type PlayAdminQuizQuestion struct {
	ID           int64
	Language     string
	Prompt       string
	Options      []string
	CorrectIndex int
	Category     string
	Difficulty   string
	Explanation  string
	SortOrder    int
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type PlayAdminQuizQuestionStats struct {
	Total        int
	Active       int
	Inactive     int
	ZhActive     int
	EnActive     int
	Categories   []string
	Difficulties []string
}

type PlayQuizAttemptDB struct {
	Score        int
	Total        int
	RewardAmount float64
}

type PlayTeamDB struct {
	ID            int64
	Name          string
	CaptainUserID int64
	InviteCode    string
	CreatedAt     time.Time
	ArchivedAt    *time.Time
}

type PlayTeamMembershipDB struct {
	ID       int64
	TeamID   int64
	UserID   int64
	JoinedAt time.Time
}

const (
	PlayTeamSettlementStatusPending    = "pending"
	PlayTeamSettlementStatusProcessing = "processing"
	PlayTeamSettlementStatusCompleted  = "completed"
	PlayTeamSettlementStatusPartial    = "partial"
	PlayTeamSettlementStatusFailed     = "failed"

	PlayTeamRewardAllocationStatusPending    = "pending"
	PlayTeamRewardAllocationStatusProcessing = "processing"
	PlayTeamRewardAllocationStatusPaid       = "paid"
	PlayTeamRewardAllocationStatusFailed     = "failed"
)

type PlayTeamSettlement struct {
	ID                  int64           `json:"id"`
	TeamID              int64           `json:"team_id"`
	PeriodStart         time.Time       `json:"period_start"`
	WindowStart         time.Time       `json:"window_start"`
	WindowEnd           time.Time       `json:"window_end"`
	TeamSpend           decimal.Decimal `json:"team_spend"`
	ReachedThreshold    decimal.Decimal `json:"reached_threshold"`
	RewardRate          decimal.Decimal `json:"reward_rate"`
	PoolAmount          decimal.Decimal `json:"pool_amount"`
	CapAmount           decimal.Decimal `json:"cap_amount"`
	Status              string          `json:"status"`
	LastError           string          `json:"last_error,omitempty"`
	ProcessingStartedAt *time.Time      `json:"processing_started_at,omitempty"`
	CompletedAt         *time.Time      `json:"completed_at,omitempty"`
}

type PlayTeamRewardAllocation struct {
	ID             int64           `json:"id"`
	SettlementID   int64           `json:"settlement_id"`
	UserID         int64           `json:"user_id"`
	DisplayName    string          `json:"display_name,omitempty"`
	AvatarURL      string          `json:"avatar_url,omitempty"`
	Email          string          `json:"-"`
	Contribution   decimal.Decimal `json:"contribution"`
	Ratio          decimal.Decimal `json:"ratio"`
	RewardAmount   decimal.Decimal `json:"reward_amount"`
	PayoutStatus   string          `json:"payout_status"`
	IdempotencyKey string          `json:"-"`
	PaidAt         *time.Time      `json:"paid_at,omitempty"`
	LastError      string          `json:"last_error,omitempty"`
}

type PlayTeamSettlementRecord struct {
	Settlement  PlayTeamSettlement         `json:"settlement"`
	Allocations []PlayTeamRewardAllocation `json:"allocations"`
}

type PlayUserTeamSettlementRecord struct {
	Settlement PlayTeamSettlement       `json:"settlement"`
	TeamName   string                   `json:"team_name"`
	Allocation PlayTeamRewardAllocation `json:"allocation"`
}

type PlayTeamRewardSettings struct {
	Enabled    bool             `json:"enabled"`
	Tiers      []TeamRewardTier `json:"tiers"`
	Cap        decimal.Decimal  `json:"cap"`
	StartMonth string           `json:"start_month"`
}

type PlayRuntime struct {
	CheckinEnabled              bool
	CheckinReward               float64
	CheckinMakeupEnabled        bool
	StreakMilestones            []PlayStreakMilestone
	ArenaEnabled                bool
	ArenaSettlementRewards      []PlayArenaSettlementTier
	BlindboxEnabled             bool
	BlindboxCost                float64
	BlindboxPool                PlayBlindboxPool
	BlindboxDailyLimit          int
	QuizEnabled                 bool
	QuizRewardPerCorrect        float64
	QuizQuestionsPerDay         int
	AgentTeamEnabled            bool
	PublicModelsEnabled         bool
	RechargeBoostEnabled        bool
	RechargeBoostDurationHours  int
	RechargeBoostCheckinMult    float64
	RechargeBoostBlindboxExtra  int
	RechargeBoostArenaMult      float64
	VIPTiers                    []PlayVIPTier
	TeamAffiliateEnabled        bool
	TeamAffiliateTokenThreshold int64
	TeamAffiliateCaptainBonus   float64
	TeamSharedRewardEnabled     bool
	TeamSharedRewardTiers       []TeamRewardTier
	TeamSharedRewardCap         decimal.Decimal
	TeamSharedRewardStartMonth  string
	CampaignsEnabled            bool
	ImageStudioEnabled          bool
	DailyQuestsEnabled          bool
	DailyArenaEnabled           bool
	DailyQuests                 []PlayDailyQuestDef
	DailyArenaTopRewards        []PlayArenaSettlementTier
}
