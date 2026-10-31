package service

import "context"

// ListQualityHistory loads a bounded, cursor-paginated view of quality runs.
// The repository groups persisted probe rows into rounds; this layer adjusts
// expected totals for multi-model plans before exposing the page to admins.
func (s *ScheduledTestService) ListQualityHistory(ctx context.Context, beforeID int64) (*QualityHistoryPage, error) {
	if beforeID < 0 {
		beforeID = 0
	}
	items, err := s.resultRepo.ListQualityHistory(ctx, beforeID, 101)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		expected := item.TotalCount
		if cfg := item.PelicanConfig; cfg != nil && len(cfg.ModelIDs) > 1 {
			planned := len(cfg.ModelIDs) * cfg.ParallelCount
			if planned > expected {
				expected = planned
			}
		}
		item.TotalCount = expected - item.SkippedCount
		if item.TotalCount <= 0 {
			item.TotalCount = 0
			item.Status = "skipped"
		} else if item.PassedCount == item.TotalCount {
			item.Status = "success"
		} else if item.Status != "failed" {
			item.Status = "inconclusive"
		}
	}
	page := &QualityHistoryPage{Items: items}
	if len(items) > 100 {
		page.Items = items[:100]
		page.NextCursor = items[99].ID
	}
	return page, nil
}
