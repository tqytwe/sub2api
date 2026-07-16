package service

import (
	"context"
	"fmt"
	"time"
)

type PublicHomeStatsRaw struct {
	TotalRequests      int64
	Success30d         int64
	ErrorSLA30d        int64
	TTFTWeightedSum24h float64
	TTFTSamples24h     int64
	OpsDataThrough     *time.Time
}

type PublicHomeStats struct {
	TotalRequests   int64      `json:"total_requests"`
	AvailabilityPct *float64   `json:"availability_pct"`
	AvgTTFTMs       *float64   `json:"avg_ttft_ms"`
	OpsDataThrough  *time.Time `json:"ops_data_through"`
	ComputedAt      time.Time  `json:"computed_at"`
}

type PublicHomeStatsRepository interface {
	GetPublicHomeStats(ctx context.Context, now time.Time) (PublicHomeStatsRaw, error)
}

type PublicHomeStatsService struct {
	repo PublicHomeStatsRepository
	now  func() time.Time
}

func NewPublicHomeStatsService(repo PublicHomeStatsRepository) *PublicHomeStatsService {
	return &PublicHomeStatsService{
		repo: repo,
		now:  time.Now,
	}
}

func (s *PublicHomeStatsService) Get(ctx context.Context) (*PublicHomeStats, error) {
	computedAt := s.now().UTC()
	raw, err := s.repo.GetPublicHomeStats(ctx, computedAt)
	if err != nil {
		return nil, fmt.Errorf("get public home stats: %w", err)
	}

	stats := &PublicHomeStats{
		TotalRequests: raw.TotalRequests,
		ComputedAt:    computedAt,
	}
	if raw.OpsDataThrough != nil {
		value := raw.OpsDataThrough.UTC()
		stats.OpsDataThrough = &value
	}
	if denominator := raw.Success30d + raw.ErrorSLA30d; denominator > 0 {
		value := float64(raw.Success30d) / float64(denominator) * 100
		stats.AvailabilityPct = &value
	}
	if raw.TTFTSamples24h > 0 {
		value := raw.TTFTWeightedSum24h / float64(raw.TTFTSamples24h)
		stats.AvgTTFTMs = &value
	}

	return stats, nil
}
