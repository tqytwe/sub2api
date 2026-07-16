package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// GetPlayBlindboxPool returns the effective configured pool.
func (s *SettingService) GetPlayBlindboxPool(ctx context.Context) (PlayBlindboxPool, error) {
	if s == nil || s.settingRepo == nil {
		return PlayBlindboxPool{}, fmt.Errorf("blindbox pool settings repository is not configured")
	}

	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyPlayBlindboxPoolJSON})
	if err != nil {
		return PlayBlindboxPool{}, fmt.Errorf("read blindbox pool: %w", err)
	}
	pool, _ := parseBlindboxPool(values[SettingKeyPlayBlindboxPoolJSON])
	return pool, nil
}

// SetPlayBlindboxPool validates and atomically replaces the stored pool.
func (s *SettingService) SetPlayBlindboxPool(ctx context.Context, pool PlayBlindboxPool) (PlayBlindboxPool, error) {
	if s == nil || s.settingRepo == nil {
		return PlayBlindboxPool{}, fmt.Errorf("blindbox pool settings repository is not configured")
	}

	pool.Version = strings.TrimSpace(pool.Version)
	if err := ValidateBlindboxPool(pool); err != nil {
		return PlayBlindboxPool{}, infraerrors.BadRequest("PLAY_BLINDBOX_POOL_INVALID", err.Error())
	}

	raw, err := json.Marshal(pool)
	if err != nil {
		return PlayBlindboxPool{}, fmt.Errorf("marshal blindbox pool: %w", err)
	}
	if err := s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyPlayBlindboxPoolJSON: string(raw),
	}); err != nil {
		return PlayBlindboxPool{}, fmt.Errorf("persist blindbox pool: %w", err)
	}

	if s.onUpdate != nil {
		s.onUpdate()
	}
	return pool, nil
}

func (s *PlayService) GetBlindboxPoolConfig(ctx context.Context) (PlayBlindboxPool, error) {
	if s == nil || s.settingService == nil {
		return PlayBlindboxPool{}, fmt.Errorf("play settings service is not configured")
	}
	return s.settingService.GetPlayBlindboxPool(ctx)
}

func (s *PlayService) UpdateBlindboxPoolConfig(ctx context.Context, pool PlayBlindboxPool) (PlayBlindboxPool, error) {
	if s == nil || s.settingService == nil {
		return PlayBlindboxPool{}, fmt.Errorf("play settings service is not configured")
	}
	return s.settingService.SetPlayBlindboxPool(ctx, pool)
}
