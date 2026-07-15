package service

import (
	"context"
	"errors"
	"sync"
	"time"
)

const publicMetricSnapshotTTL = 30 * time.Second

type PublicMetricsRepository interface {
	GetLatestPublicMetricSnapshot(ctx context.Context) (*PublicMetricSnapshot, error)
	RefreshPublicMetricSnapshot(ctx context.Context, bucket time.Time) (*PublicMetricSnapshot, error)
	ListPublicActivity(ctx context.Context, limit, minCount int) ([]PlayPublicActivity, error)
	RefreshPlayUsageAggregates(ctx context.Context, now time.Time, rechargeMultiplier, campaignMultiplier float64, weeklyTokenTarget, weeklyRequestTarget int64, firstRequestTickets, teamWeeklyTickets int) error
}

type PublicMetricSnapshotService struct {
	repo PublicMetricsRepository
	mu   sync.Mutex
	last *PublicMetricSnapshot
}

func NewPublicMetricSnapshotService(repo PublicMetricsRepository) *PublicMetricSnapshotService {
	return &PublicMetricSnapshotService{repo: repo}
}

func (s *PublicMetricSnapshotService) Get(ctx context.Context) (*PublicMetricSnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("public metrics repository is unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last != nil && time.Since(s.last.UpdatedAt) <= publicMetricSnapshotTTL {
		copy := *s.last
		return &copy, nil
	}
	latest, latestErr := s.repo.GetLatestPublicMetricSnapshot(ctx)
	if latestErr == nil && latest != nil && time.Since(latest.UpdatedAt) <= publicMetricSnapshotTTL {
		s.last = latest
		copy := *latest
		return &copy, nil
	}
	now := time.Now().UTC()
	refreshed, err := s.refreshLocked(ctx, now)
	if err == nil {
		return refreshed, nil
	}
	if latestErr == nil && latest != nil {
		s.last = latest
		copy := *latest
		return &copy, nil
	}
	return unavailablePublicMetricSnapshot(now), nil
}

func (s *PublicMetricSnapshotService) Refresh(ctx context.Context, now time.Time) (*PublicMetricSnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("public metrics repository is unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshLocked(ctx, now)
}

func (s *PublicMetricSnapshotService) refreshLocked(ctx context.Context, now time.Time) (*PublicMetricSnapshot, error) {
	bucket := now.UTC().Truncate(time.Minute)
	snapshot, err := s.repo.RefreshPublicMetricSnapshot(ctx, bucket)
	if err != nil {
		return nil, err
	}
	s.last = snapshot
	copy := *snapshot
	return &copy, nil
}

func unavailablePublicMetricSnapshot(now time.Time) *PublicMetricSnapshot {
	bucket := now.UTC().Truncate(time.Minute)
	return &PublicMetricSnapshot{
		SnapshotID: bucket.Format(time.RFC3339) + ":unavailable",
		UpdatedAt:  bucket,
		Source:     "estimated",
	}
}

func (s *PlayService) GetPublicMetricSnapshot(ctx context.Context) (*PublicMetricSnapshot, error) {
	if s == nil || s.publicMetrics == nil {
		return nil, errors.New("public metrics service is unavailable")
	}
	return s.publicMetrics.Get(ctx)
}

func (s *PlayService) ListPublicActivity(ctx context.Context, limit int) ([]PlayPublicActivity, error) {
	metricsRepo, ok := s.repo.(PublicMetricsRepository)
	if !ok {
		return []PlayPublicActivity{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	rt := s.GetRuntime(ctx)
	return metricsRepo.ListPublicActivity(ctx, limit, rt.PublicActivityMinCount)
}

func (s *PlayService) RefreshGrowthWorld(ctx context.Context, now time.Time) error {
	metricsRepo, ok := s.repo.(PublicMetricsRepository)
	if !ok {
		return nil
	}
	rt := s.GetRuntime(ctx)
	campaignMultiplier := 1.0
	if rt.CampaignsEnabled {
		campaigns, err := s.repo.ListActiveCampaigns(ctx, now)
		if err != nil {
			return err
		}
		if rules := aggregateCampaignRules(campaigns); rules.ArenaScoreMultiplier > 1 {
			campaignMultiplier = rules.ArenaScoreMultiplier
		}
	}
	rechargeMultiplier := 1.0
	if rt.RechargeBoostEnabled && rt.RechargeBoostArenaMult > 1 {
		rechargeMultiplier = rt.RechargeBoostArenaMult
	}
	if err := metricsRepo.RefreshPlayUsageAggregates(ctx, now, rechargeMultiplier, campaignMultiplier, rt.TeamWeeklyTokenTarget, rt.TeamWeeklyRequestTarget, rt.BlindboxFirstRequestTickets, rt.BlindboxTeamWeeklyTickets); err != nil {
		return err
	}
	if s.publicMetrics != nil {
		_, err := s.publicMetrics.Refresh(ctx, now)
		return err
	}
	return nil
}
