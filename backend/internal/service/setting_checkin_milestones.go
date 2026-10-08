package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const maxPlayCheckinMilestones = 32

// PlayCheckinMilestoneSettings captures both the milestone rewards configuration
// and the makeup check-in feature toggle.
type PlayCheckinMilestoneSettings struct {
	Milestones    []PlayStreakMilestone `json:"milestones"`
	MakeupEnabled bool                  `json:"makeup_enabled"`
}

// GetPlayCheckinMilestoneSettings returns the effective checkin milestone
// configuration and makeup toggle used by the checkin runtime.
func (s *SettingService) GetPlayCheckinMilestoneSettings(ctx context.Context) (PlayCheckinMilestoneSettings, error) {
	if s == nil || s.settingRepo == nil {
		return PlayCheckinMilestoneSettings{}, fmt.Errorf("checkin milestone settings repository is not configured")
	}

	runtime := s.GetPlayRuntime(ctx)
	return PlayCheckinMilestoneSettings{
		Milestones:    append([]PlayStreakMilestone(nil), runtime.StreakMilestones...),
		MakeupEnabled: runtime.CheckinMakeupEnabled,
	}, nil
}

// SetPlayCheckinMilestoneSettings validates and atomically replaces checkin
// streak milestones and the makeup toggle. It does not affect historical
// checkin records or ledger entries.
func (s *SettingService) SetPlayCheckinMilestoneSettings(ctx context.Context, settings PlayCheckinMilestoneSettings) (PlayCheckinMilestoneSettings, error) {
	if s == nil || s.settingRepo == nil {
		return PlayCheckinMilestoneSettings{}, fmt.Errorf("checkin milestone settings repository is not configured")
	}

	if err := validatePlayCheckinMilestones(settings.Milestones); err != nil {
		return PlayCheckinMilestoneSettings{}, infraerrors.BadRequest("PLAY_CHECKIN_MILESTONES_INVALID", err.Error())
	}

	milestones, err := json.Marshal(settings.Milestones)
	if err != nil {
		return PlayCheckinMilestoneSettings{}, fmt.Errorf("marshal checkin milestones: %w", err)
	}

	makeupValue := "true"
	if !settings.MakeupEnabled {
		makeupValue = "false"
	}

	if err := s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyPlayCheckinStreakMilestones: string(milestones),
		SettingKeyPlayCheckinMakeupEnabled:    makeupValue,
	}); err != nil {
		return PlayCheckinMilestoneSettings{}, fmt.Errorf("persist checkin milestone settings: %w", err)
	}

	if s.onUpdate != nil {
		s.onUpdate()
	}

	return settings, nil
}

func validatePlayCheckinMilestones(milestones []PlayStreakMilestone) error {
	if len(milestones) == 0 {
		return fmt.Errorf("checkin milestones are required (at least one milestone)")
	}
	if len(milestones) > maxPlayCheckinMilestones {
		return fmt.Errorf("checkin milestones must contain at most %d entries", maxPlayCheckinMilestones)
	}

	previousDays := 0
	for i, milestone := range milestones {
		if milestone.Days <= previousDays {
			return fmt.Errorf("checkin milestone %d days must be strictly increasing (got %d after %d)", i+1, milestone.Days, previousDays)
		}
		if math.IsNaN(milestone.Bonus) || math.IsInf(milestone.Bonus, 0) || milestone.Bonus <= 0 {
			return fmt.Errorf("checkin milestone %d bonus must be positive", i+1)
		}
		previousDays = milestone.Days
	}
	return nil
}

func (s *PlayService) GetCheckinMilestoneSettings(ctx context.Context) (PlayCheckinMilestoneSettings, error) {
	if s == nil || s.settingService == nil {
		return PlayCheckinMilestoneSettings{}, fmt.Errorf("play settings service is not configured")
	}
	return s.settingService.GetPlayCheckinMilestoneSettings(ctx)
}

func (s *PlayService) UpdateCheckinMilestoneSettings(ctx context.Context, settings PlayCheckinMilestoneSettings) (PlayCheckinMilestoneSettings, error) {
	if s == nil || s.settingService == nil {
		return PlayCheckinMilestoneSettings{}, fmt.Errorf("play settings service is not configured")
	}
	return s.settingService.SetPlayCheckinMilestoneSettings(ctx, settings)
}
