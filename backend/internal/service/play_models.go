package service

import (
	"context"
	"time"
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
	ID      int64
	Name    string
	StartAt time.Time
	EndAt   time.Time
	Status  string
}

type PlayArenaScoreRow struct {
	Rank        int
	UserID      int64
	DisplayName string
	AvatarURL   string
	TokenSum    int64
}

type PlayRewardLedgerEntry struct {
	UserID         int64
	Source         string
	Amount         float64
	IdempotencyKey string
	Detail         map[string]any
}

type PlayCheckinStatus struct {
	Enabled                bool
	CheckedInToday         bool
	RewardAmount           float64
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
	RewardAmount   float64
	BalanceAdded   float64
	ServerDate     string
	StreakCount    int
	MilestoneBonus float64
}

type PlayBlindboxStatus struct {
	Enabled             bool
	CostAmount          float64
	DailyLimit          int
	EffectiveLimit      int
	OpensToday          int
	CanOpen             bool
	ServerDate          string
	RechargeBoostActive bool
	CampaignActive      bool
	PaidEnabled         bool
	RegionEnabled       bool
	Pool                PlayBlindboxPool
	TicketBalance       int
}

type PlayBlindboxOpenResult struct {
	CostAmount   float64
	RewardAmount float64
	NetAmount    float64
	OpensToday   int
	ServerDate   string
	PoolVersion  string
	OpenSource   string
}

type PlayBlindboxTier struct {
	Amount float64 `json:"amount"`
	Weight int64   `json:"weight"`
}

type PlayBlindboxPool struct {
	Version string             `json:"version"`
	Cost    float64            `json:"cost"`
	RTPCap  float64            `json:"rtp_cap"`
	Tiers   []PlayBlindboxTier `json:"tiers"`
}

type PublicMetricSnapshot struct {
	SnapshotID     string    `json:"snapshot_id"`
	UpdatedAt      time.Time `json:"updated_at"`
	Source         string    `json:"source"`
	Requests24h    int64     `json:"requests_24h"`
	RequestsTotal  int64     `json:"requests_total"`
	ActiveUsers7d  int64     `json:"active_users_7d"`
	TokensTotal    int64     `json:"tokens_total"`
	SuccessRate30d *float64  `json:"success_rate_30d"`
	P50TTFTMs      *int64    `json:"p50_ttft_ms"`
	P95TTFTMs      *int64    `json:"p95_ttft_ms"`
}

type PlayPublicActivity struct {
	ID        int64          `json:"id"`
	EventType string         `json:"event_type"`
	Actor     string         `json:"actor"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}

type PlayActivityRepository interface {
	InsertPlayActivity(ctx context.Context, eventKey, eventType string, userID int64, subjectType string, subjectID int64, payload map[string]any, createdAt time.Time) error
}

type PlayArenaSummary struct {
	Enabled                bool             `json:"enabled"`
	Period                 *PlayArenaPeriod `json:"period,omitempty"`
	Participants           int              `json:"participants"`
	MyScore                int64            `json:"my_score"`
	MyRank                 int              `json:"my_rank"`
	ScoreMultiplierApplied float64          `json:"score_multiplier_applied"`
	TokensToPreviousRank   int64            `json:"tokens_to_previous_rank"`
	NewcomerRank           int              `json:"newcomer_rank"`
}

// PlayBlindboxRecentWin is a privacy-masked public feed row for recent opens.
type PlayBlindboxRecentWin struct {
	UserLabel    string
	RewardAmount float64
	CreatedAt    time.Time
}

type BlindboxTicketRepository interface {
	GetBlindboxTicketBalance(ctx context.Context, userID int64) (int, error)
	ConsumeBlindboxTicket(ctx context.Context, userID int64, idempotencyKey string) error
	InsertBlindboxTicket(ctx context.Context, userID int64, source string, quantity int, idempotencyKey string, detail map[string]any) error
	InsertBlindboxOpenAudit(ctx context.Context, userID int64, source, poolVersion, idempotencyKey string, cost, reward float64, detail map[string]any) error
	InsertBlindboxOpenV2(ctx context.Context, userID int64, date time.Time, source, poolVersion string, cost, reward float64, idempotencyKey string) error
}

type PlayQuizQuestion struct {
	ID      int64
	Prompt  string
	Options []string
}

type PlayQuizToday struct {
	Enabled          bool
	Questions        []PlayQuizQuestion
	AlreadySubmitted bool
	PreviousScore    int
	PreviousTotal    int
	PreviousReward   float64
	RewardPerCorrect float64
	ServerDate       string
}

type PlayQuizAnswer struct {
	QuestionID  int64
	ChoiceIndex int
}

type PlayQuizSubmitResult struct {
	Score        int
	Total        int
	RewardAmount float64
	ServerDate   string
}

type PlayTeamMember struct {
	UserID       int64
	DisplayName  string
	AvatarURL    string
	JoinedAt     time.Time
	TokenSum     int64
	TokenPct     int
	RequestCount int64
	ActiveDays   int
}

type PlayTeamSummary struct {
	ID           int64
	Name         string
	InviteCode   string
	CaptainID    int64
	MemberCount  int
	TokenSum     int64
	Members      []PlayTeamMember
	Affiliate    *PlayTeamAffiliateInfo
	RequestCount int64
	ActiveDays   int
	Level        int
	MaxMembers   int
	IsPublic     bool
	Weekly       *PlayTeamWeeklyProgress
}

type PlayTeamWeeklyProgress struct {
	WeekStart     string `json:"week_start"`
	TokenTarget   int64  `json:"token_target"`
	RequestTarget int64  `json:"request_target"`
	TokenSum      int64  `json:"token_sum"`
	RequestCount  int64  `json:"request_count"`
	ActiveDays    int    `json:"active_days"`
	Completed     bool   `json:"completed"`
}

type PlayTeamDiscovery struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	MemberCount  int    `json:"member_count"`
	MaxMembers   int    `json:"max_members"`
	Level        int    `json:"level"`
	TokenSum     int64  `json:"token_sum"`
	RequestCount int64  `json:"request_count"`
}

type PlayTeamJoinRequest struct {
	ID          int64     `json:"id"`
	TeamID      int64     `json:"team_id"`
	UserID      int64     `json:"user_id"`
	DisplayName string    `json:"display_name"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type PlayTeamEngagement struct {
	RequestCount int64
	TokenSum     int64
	ActiveDays   int
	Level        int
	MaxMembers   int
	IsPublic     bool
	Weekly       *PlayTeamWeeklyProgress
}

type PlayMemberEngagement struct {
	RequestCount int64
	TokenSum     int64
	ActiveDays   int
}

type AdvancedTeamRepository interface {
	SetTeamMaxMembers(ctx context.Context, teamID int64, maxMembers int) error
	GetTeamMemberCount(ctx context.Context, teamID int64) (int, error)
	GetTeamEngagement(ctx context.Context, teamID int64, monthStart, weekStart time.Time) (*PlayTeamEngagement, error)
	ListTeamMemberEngagement(ctx context.Context, userIDs []int64, start, end time.Time) (map[int64]PlayMemberEngagement, error)
	ListDiscoverableTeams(ctx context.Context, monthStart time.Time, limit int) ([]PlayTeamDiscovery, error)
	CreateTeamJoinRequest(ctx context.Context, teamID, userID int64) error
	ListTeamJoinRequests(ctx context.Context, teamID int64) ([]PlayTeamJoinRequest, error)
	ApproveTeamJoinRequest(ctx context.Context, requestID, captainID int64) error
	RejectTeamJoinRequest(ctx context.Context, requestID, captainID int64) error
	LeaveTeam(ctx context.Context, teamID, userID int64, deleteTeam bool) error
	TransferTeamCaptain(ctx context.Context, teamID, captainID, nextCaptainID int64) error
	RemoveTeamMember(ctx context.Context, teamID, captainID, memberID int64) error
	ListTeamActivity(ctx context.Context, teamID int64, limit int) ([]PlayPublicActivity, error)
}

type PlayTeamMe struct {
	Enabled bool
	Team    *PlayTeamSummary
}

type PlayArenaCurrent struct {
	Enabled              bool
	Period               *PlayArenaPeriod
	TokenSum             int64
	DisplayTokenSum      int64
	Rank                 int
	TokensToPrevRank     int64
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

type PlayRepository interface {
	HasCheckin(ctx context.Context, userID int64, date time.Time) (bool, error)
	InsertCheckin(ctx context.Context, userID int64, date time.Time, reward float64, streakCount int) error
	GetCheckinStreakOnDate(ctx context.Context, userID int64, date time.Time) (streak int, found bool, err error)
	InsertRewardLedger(ctx context.Context, entry PlayRewardLedgerEntry) error
	GetActiveArenaPeriod(ctx context.Context, now time.Time) (*PlayArenaPeriod, error)
	EnsureMonthlyArenaPeriod(ctx context.Context, now time.Time) (*PlayArenaPeriod, error)
	GetArenaPeriodByID(ctx context.Context, periodID int64) (*PlayArenaPeriod, error)
	MarkArenaPeriodSettled(ctx context.Context, periodID int64) error
	ListArenaLeaderboard(ctx context.Context, start, end time.Time, limit int) ([]PlayArenaScoreRow, error)
	GetUserArenaScore(ctx context.Context, userID int64, start, end time.Time) (tokenSum int64, rank int, err error)
	GetArenaTokensToPrevRank(ctx context.Context, userID int64, start, end time.Time, rank int, tokenSum int64) (int64, error)
	CountBlindboxOpens(ctx context.Context, userID int64, date time.Time) (int, error)
	InsertBlindboxOpen(ctx context.Context, userID int64, date time.Time, cost, reward float64, idempotencyKey string) error
	ListRecentBlindboxWins(ctx context.Context, limit int) ([]PlayBlindboxRecentWin, error)
	ListQuizQuestions(ctx context.Context, language string) ([]PlayQuizQuestionDB, error)
	GetQuizAttempt(ctx context.Context, userID int64, date time.Time) (*PlayQuizAttemptDB, error)
	InsertQuizAttempt(ctx context.Context, userID int64, date time.Time, score, total int, reward float64, answers map[string]any) error
	GetUserTeam(ctx context.Context, userID int64) (*PlayTeamDB, error)
	CreateTeam(ctx context.Context, name string, captainUserID int64, inviteCode string) (*PlayTeamDB, error)
	JoinTeam(ctx context.Context, teamID, userID int64) error
	GetTeamByInviteCode(ctx context.Context, inviteCode string) (*PlayTeamDB, error)
	GetTeamByID(ctx context.Context, teamID int64) (*PlayTeamDB, error)
	ListTeamMembers(ctx context.Context, teamID int64) ([]PlayTeamMember, error)
	SumTeamTokenUsage(ctx context.Context, userIDs []int64, start, end time.Time) (int64, error)
	ListTeamMemberTokenUsage(ctx context.Context, userIDs []int64, start, end time.Time) (map[int64]int64, error)
	ListActiveCampaigns(ctx context.Context, now time.Time) ([]PlayCampaign, error)
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
}

type PlayQuizQuestionDB struct {
	ID           int64
	Language     string
	Prompt       string
	OptionsJSON  string
	CorrectIndex int
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
	BlindboxDailyLimit          int
	BlindboxPool                PlayBlindboxPool
	BlindboxPaidEnabled         bool
	BlindboxRegionEnabled       bool
	BlindboxFirstRequestTickets int
	BlindboxTeamWeeklyTickets   int
	QuizEnabled                 bool
	QuizRewardPerCorrect        float64
	QuizQuestionsPerDay         int
	AgentTeamEnabled            bool
	TeamMaxMembers              int
	TeamWeeklyTokenTarget       int64
	TeamWeeklyRequestTarget     int64
	PublicActivityMinCount      int
	FounderSeasonJSON           string
	GrowthExperimentJSON        string
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
	CampaignsEnabled            bool
	ImageStudioEnabled          bool
	DailyQuestsEnabled          bool
	DailyArenaEnabled           bool
	DailyQuests                 []PlayDailyQuestDef
	DailyArenaTopRewards        []PlayArenaSettlementTier
}
