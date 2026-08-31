package service

import (
	"context"
	"fmt"
	"time"
)

const (
	publicStatusDataDelay   = 90 * time.Minute
	publicStatusUnavailable = 6 * time.Hour
	publicStatusFutureLimit = 3 * time.Minute
)

type PublicStatusFreshness string

const (
	PublicStatusFreshnessFresh       PublicStatusFreshness = "fresh"
	PublicStatusFreshnessDelayed     PublicStatusFreshness = "delayed"
	PublicStatusFreshnessUnavailable PublicStatusFreshness = "unavailable"
)

// PublicStatusSummaryRaw is the persisted public snapshot. The repository must
// return the latest completed window only; it must not calculate live percentiles.
type PublicStatusSummaryRaw struct {
	HasSnapshot             bool
	TotalRequests           int64
	AvailabilityPct         *float64
	AvailabilitySampleCount int64
	TTFTP50Ms               *float64
	TTFTP95Ms               *float64
	TTFTSampleCount         int64
	WindowEnd               time.Time
	DataThrough             *time.Time
	ComputedAt              time.Time
}

type PublicStatusSummaryRepository interface {
	GetPublicStatusSummary(ctx context.Context) (PublicStatusSummaryRaw, error)
	// ListPublicStatusSnapshotWindowEnds is intentionally bounded by the
	// worker's recent catch-up horizon. It never exposes raw operational data
	// and it is not used by the public HTTP request path.
	ListPublicStatusSnapshotWindowEnds(ctx context.Context, from, through time.Time) ([]time.Time, error)
	// IsPublicStatusSnapshotWindowReady verifies the durable hourly aggregation
	// completion watermark before the worker attempts an immutable insert.
	IsPublicStatusSnapshotWindowReady(ctx context.Context, windowEnd time.Time) (bool, error)
	RefreshPublicStatusSnapshot(ctx context.Context, windowEnd time.Time) error
}

type PublicStatusAvailability struct {
	ValuePct    *float64   `json:"value_pct"`
	SampleCount int64      `json:"sample_count"`
	WindowStart *time.Time `json:"window_start"`
	WindowEnd   *time.Time `json:"window_end"`
}

type PublicStatusTTFT struct {
	P50Ms       *float64   `json:"p50_ms"`
	P95Ms       *float64   `json:"p95_ms"`
	SampleCount int64      `json:"sample_count"`
	WindowStart *time.Time `json:"window_start"`
	WindowEnd   *time.Time `json:"window_end"`
}

// PublicStatusSummary is safe to expose publicly. It contains fixed windows,
// sample sizes, and the business data watermark instead of a request-time view.
type PublicStatusSummary struct {
	TotalRequests int64                    `json:"total_requests"`
	Availability  PublicStatusAvailability `json:"availability"`
	TTFT          PublicStatusTTFT         `json:"ttft"`
	DataThrough   *time.Time               `json:"data_through"`
	ComputedAt    time.Time                `json:"computed_at"`
	Freshness     PublicStatusFreshness    `json:"freshness"`
}

type PublicStatusSummaryService struct {
	repo PublicStatusSummaryRepository
	now  func() time.Time
}

func NewPublicStatusSummaryService(repo PublicStatusSummaryRepository) *PublicStatusSummaryService {
	return &PublicStatusSummaryService{repo: repo, now: time.Now}
}

func (s *PublicStatusSummaryService) Get(ctx context.Context) (*PublicStatusSummary, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("public status summary repository is unavailable")
	}
	now := s.now().UTC()
	raw, err := s.repo.GetPublicStatusSummary(ctx)
	if err != nil {
		return nil, fmt.Errorf("get public status summary: %w", err)
	}

	summary := &PublicStatusSummary{
		TotalRequests: raw.TotalRequests,
		Availability: PublicStatusAvailability{
			ValuePct:    raw.AvailabilityPct,
			SampleCount: raw.AvailabilitySampleCount,
		},
		TTFT: PublicStatusTTFT{
			P50Ms:       raw.TTFTP50Ms,
			P95Ms:       raw.TTFTP95Ms,
			SampleCount: raw.TTFTSampleCount,
		},
		ComputedAt: raw.ComputedAt,
		Freshness:  PublicStatusFreshnessUnavailable,
	}
	if summary.ComputedAt.IsZero() {
		summary.ComputedAt = now
	} else {
		summary.ComputedAt = summary.ComputedAt.UTC()
	}
	if raw.DataThrough != nil {
		value := raw.DataThrough.UTC()
		summary.DataThrough = &value
	}
	if raw.HasSnapshot && !raw.WindowEnd.IsZero() {
		windowEnd := raw.WindowEnd.UTC()
		availabilityStart := windowEnd.Add(-30 * 24 * time.Hour)
		ttftStart := windowEnd.Add(-24 * time.Hour)
		summary.Availability.WindowStart = &availabilityStart
		summary.Availability.WindowEnd = &windowEnd
		summary.TTFT.WindowStart = &ttftStart
		summary.TTFT.WindowEnd = &windowEnd
	}

	s.applyFreshness(summary, now)
	return summary, nil
}

// RecomputeFreshness updates a cached snapshot against the current business
// clock. Handlers use this when the repository is temporarily unavailable so
// a stale cached response cannot remain green past its data watermark.
func (s *PublicStatusSummaryService) RecomputeFreshness(summary *PublicStatusSummary) *PublicStatusSummary {
	return s.RecomputeFreshnessAt(summary, s.now().UTC())
}

// RecomputeFreshnessAt is the clock-injectable form used by HTTP caches and
// deterministic tests.
func (s *PublicStatusSummaryService) RecomputeFreshnessAt(summary *PublicStatusSummary, now time.Time) *PublicStatusSummary {
	if summary == nil {
		return nil
	}
	s.applyFreshness(summary, now.UTC())
	return summary
}

func (s *PublicStatusSummaryService) applyFreshness(summary *PublicStatusSummary, now time.Time) {
	summary.Freshness = PublicStatusFreshnessUnavailable
	if summary.DataThrough == nil || summary.Availability.SampleCount == 0 || summary.TTFT.SampleCount == 0 ||
		summary.Availability.WindowStart == nil || summary.Availability.WindowEnd == nil ||
		summary.TTFT.WindowStart == nil || summary.TTFT.WindowEnd == nil {
		return
	}

	age := now.Sub(*summary.DataThrough)
	if summary.DataThrough.After(now.Add(publicStatusFutureLimit)) || age > publicStatusUnavailable {
		return
	}
	if age > publicStatusDataDelay {
		summary.Freshness = PublicStatusFreshnessDelayed
		return
	}
	summary.Freshness = PublicStatusFreshnessFresh
}
