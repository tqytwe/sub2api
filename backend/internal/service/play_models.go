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

type PlayBlindboxOpenRecord struct {
	UserID         int64
	Date           time.Time
	Cost           float64
	Reward         float64
	IdempotencyKey string
	PoolVersion    string
	OpenSource     string
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
	BlindboxPool        PlayBlindboxPool
	DailyLimit          int
	EffectiveLimit      int
	OpensToday          int
	CanOpen             bool
	ServerDate          string
	RechargeBoostActive bool
	CampaignActive      bool
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

// PlayBlindboxRecentWin is a privacy-masked public feed row for recent opens.
type PlayBlindboxRecentWin struct {
	UserLabel    string
	RewardAmount float64
	CreatedAt    time.Time
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
	UserID      int64
	DisplayName string
	AvatarURL   string
	JoinedAt    time.Time
	TokenSum    int64
	TokenPct    int
}

type PlayTeamSummary struct {
	ID          int64
	Name        string
	InviteCode  string
	CaptainID   int64
	MemberCount int
	TokenSum    int64
	Members     []PlayTeamMember
	Affiliate   *PlayTeamAffiliateInfo
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
	LockBlindboxOpenUser(ctx context.Context, userID int64) (balance float64, err error)
	UpdatePlayBalance(ctx context.Context, userID int64, amount float64) error
	CountBlindboxOpens(ctx context.Context, userID int64, date time.Time) (int, error)
	InsertBlindboxOpen(ctx context.Context, userID int64, date time.Time, cost, reward float64, idempotencyKey string) error
	InsertBlindboxOpenRecord(ctx context.Context, record PlayBlindboxOpenRecord) error
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
