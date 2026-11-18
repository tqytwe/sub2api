package service

import (
	"context"
	"errors"
	"time"
)

type PriorityLatencySeriesQuery struct {
	AccountIDs    []int64
	Model         string
	GroupID       *int64
	Since         time.Time
	BucketMinutes int
	SlowTTFTMs    int
}

type PriorityLatencyBucket struct {
	AccountID    int64     `json:"account_id"`
	Bucket       time.Time `json:"bucket"`
	Samples      int       `json:"samples"`
	P50TTFTMs    float64   `json:"p50_ttft_ms"`
	P90TTFTMs    float64   `json:"p90_ttft_ms"`
	SlowSamples  int       `json:"slow_samples"`
	ErrorSamples *int      `json:"error_samples,omitempty"`
}

type PriorityLatencySeries struct {
	At            time.Time                `json:"at"`
	Model         string                   `json:"model"`
	GroupID       *int64                   `json:"group_id,omitempty"`
	BucketMinutes int                      `json:"bucket_minutes"`
	Accounts      []PriorityLatencyAccount `json:"accounts"`
}

type PriorityLatencyAccount struct {
	AccountID           int64                   `json:"account_id"`
	AccountName         string                  `json:"account_name"`
	Samples             int                     `json:"samples"`
	PeakBucketP90TTFTMs float64                 `json:"peak_bucket_p90_ttft_ms"`
	SlowSamples         int                     `json:"slow_samples"`
	ErrorSamples        *int                    `json:"error_samples,omitempty"`
	Buckets             []PriorityLatencyBucket `json:"buckets"`
}

type PriorityLatencySeriesReader interface {
	ReadPriorityLatencySeries(context.Context, PriorityLatencySeriesQuery) ([]PriorityLatencyBucket, error)
}

var ErrPriorityLatencySeriesUnavailable = errors.New("priority latency series unavailable")

func (s *OpenAIGatewayService) PriorityLatencySeries(ctx context.Context, minutes, bucketMinutes int) (*PriorityLatencySeries, error) {
	if s == nil || s.usageLogRepo == nil || minutes < 5 || minutes > 1440 || bucketMinutes < 1 || bucketMinutes > 60 {
		return nil, ErrPriorityLatencySeriesUnavailable
	}
	reader, ok := s.usageLogRepo.(PriorityLatencySeriesReader)
	if !ok {
		return nil, ErrPriorityLatencySeriesUnavailable
	}
	snapshot := s.PrioritySchedulingSnapshot()
	if snapshot == nil || len(snapshot.Candidates) == 0 {
		return nil, ErrPriorityLatencySeriesUnavailable
	}
	ids := make([]int64, 0, len(snapshot.Candidates))
	names := make(map[int64]string, len(snapshot.Candidates))
	for _, candidate := range snapshot.Candidates {
		ids = append(ids, candidate.AccountID)
		names[candidate.AccountID] = candidate.AccountName
	}
	rows, err := reader.ReadPriorityLatencySeries(ctx, PriorityLatencySeriesQuery{AccountIDs: ids, Model: snapshot.Model, GroupID: snapshot.GroupID, Since: time.Now().Add(-time.Duration(minutes) * time.Minute), BucketMinutes: bucketMinutes, SlowTTFTMs: s.prioritySchedulingRuntimeConfig().TargetTTFTMs})
	if err != nil {
		return nil, err
	}
	return buildPriorityLatencySeries(snapshot.At, snapshot.Model, snapshot.GroupID, bucketMinutes, ids, names, rows), nil
}

func buildPriorityLatencySeries(at time.Time, model string, groupID *int64, bucketMinutes int, ids []int64, names map[int64]string, rows []PriorityLatencyBucket) *PriorityLatencySeries {
	byAccount := make(map[int64][]PriorityLatencyBucket)
	for _, row := range rows {
		byAccount[row.AccountID] = append(byAccount[row.AccountID], row)
	}
	out := &PriorityLatencySeries{At: at, Model: model, GroupID: clonePriorityGroupID(groupID), BucketMinutes: bucketMinutes, Accounts: make([]PriorityLatencyAccount, 0, len(ids))}
	for _, id := range ids {
		var summary PriorityLatencyAccount
		summary.AccountID, summary.AccountName, summary.Buckets = id, names[id], byAccount[id]
		for _, bucket := range summary.Buckets {
			summary.Samples += bucket.Samples
			if bucket.P90TTFTMs > summary.PeakBucketP90TTFTMs {
				summary.PeakBucketP90TTFTMs = bucket.P90TTFTMs
			}
			summary.SlowSamples += bucket.SlowSamples
			if bucket.ErrorSamples != nil {
				if summary.ErrorSamples == nil {
					zero := 0
					summary.ErrorSamples = &zero
				}
				*summary.ErrorSamples += *bucket.ErrorSamples
			}
		}
		out.Accounts = append(out.Accounts, summary)
	}
	return out
}
