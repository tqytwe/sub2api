package service

import (
	"context"
	"fmt"
)

// GetAccountConcurrencySnapshot reads account-level slots in bounded batches.
// Missing counts are unknown, not zero. Batches are successive observations,
// not an atomic snapshot across Redis keys or separate page requests.
func (s *ConcurrencyService) GetAccountConcurrencySnapshot(ctx context.Context, ids []int64) (map[int64]int, error) {
	const batchSize = 250
	counts := make(map[int64]int, len(ids))
	if len(ids) == 0 {
		return counts, nil
	}
	if s == nil || s.cache == nil {
		return nil, fmt.Errorf("account concurrency cache unavailable")
	}
	for start := 0; start < len(ids); start += batchSize {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		batch := ids[start:min(start+batchSize, len(ids))]
		values, err := s.cache.GetAccountConcurrencyBatch(ctx, batch)
		if err != nil {
			return nil, err
		}
		for _, id := range batch {
			value, ok := values[id]
			if !ok || value < 0 {
				return nil, fmt.Errorf("account concurrency snapshot incomplete")
			}
			counts[id] = value
		}
	}
	return counts, nil
}
