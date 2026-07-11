package service

import (
	"encoding/json"
	"math"
	"time"
)

const (
	PlayQuestKeyCheckin       = "checkin"
	PlayQuestKeyImageGenerate = "image_generate"
	PlayQuestKeyAPICall       = "api_call"
)

type PlayDailyQuestDef struct {
	Key       string `json:"key"`
	Energy    int    `json:"energy"`
	Auto      bool   `json:"auto,omitempty"`
	MinCount  int    `json:"min_count,omitempty"`
	MinTokens int64  `json:"min_tokens,omitempty"`
	CTARoute  string `json:"cta_route,omitempty"`
}

type PlayQuestTask struct {
	Key       string `json:"key"`
	Label     string `json:"label,omitempty"`
	Completed bool   `json:"completed"`
	Energy    int    `json:"energy"`
	CTARoute  string `json:"cta_route,omitempty"`
}

type PlayQuestToday struct {
	Enabled           bool            `json:"enabled"`
	Energy            int             `json:"energy"`
	Level             int             `json:"level"`
	EnergyToNextLevel int             `json:"energy_to_next_level"`
	Tasks             []PlayQuestTask `json:"tasks"`
	ServerDate        string          `json:"server_date"`
}

type PlayQuestProgressRow struct {
	QuestKey      string
	Completed     bool
	CompletedAt   *time.Time
	RewardClaimed bool
}

func parsePlayDailyQuests(raw string) []PlayDailyQuestDef {
	if raw == "" {
		return defaultPlayDailyQuests()
	}
	var defs []PlayDailyQuestDef
	if err := json.Unmarshal([]byte(raw), &defs); err != nil || len(defs) == 0 {
		return defaultPlayDailyQuests()
	}
	return defs
}

func defaultPlayDailyQuests() []PlayDailyQuestDef {
	return []PlayDailyQuestDef{
		{Key: PlayQuestKeyCheckin, Energy: 10, Auto: true, CTARoute: "/check-in"},
		{Key: PlayQuestKeyImageGenerate, Energy: 20, MinCount: 1, CTARoute: "/image-studio"},
		{Key: PlayQuestKeyAPICall, Energy: 15, MinTokens: 100, CTARoute: "/keys"},
	}
}

func questCTARoute(key string) string {
	switch key {
	case PlayQuestKeyCheckin:
		return "/check-in"
	case PlayQuestKeyImageGenerate:
		return "/image-studio"
	case PlayQuestKeyAPICall:
		return "/keys"
	default:
		return ""
	}
}

func computeQuestLevel(energy int) (level int, energyToNext int) {
	level = int(math.Floor(math.Sqrt(float64(energy)/100))) + 1
	if level < 1 {
		level = 1
	}
	nextThreshold := level * level * 100
	energyToNext = nextThreshold - energy
	if energyToNext < 0 {
		energyToNext = 0
	}
	return level, energyToNext
}

func parsePlayDailyArenaRewards(raw string) []PlayArenaSettlementTier {
	if raw == "" {
		return []PlayArenaSettlementTier{
			{RankMax: 1, Amount: 0.5},
			{RankMax: 3, Amount: 0.2},
			{RankMax: 10, Amount: 0.1},
		}
	}
	var tiers []PlayArenaSettlementTier
	if err := json.Unmarshal([]byte(raw), &tiers); err != nil {
		return nil
	}
	return tiers
}
