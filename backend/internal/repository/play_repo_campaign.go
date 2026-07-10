package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *playRepository) ListActiveCampaigns(ctx context.Context, now time.Time) ([]service.PlayCampaign, error) {
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT id, name, start_at, end_at, rules_json::text, enabled, created_at
		FROM play_campaigns
		WHERE enabled = TRUE AND start_at <= $1 AND end_at > $1
		ORDER BY start_at ASC, id ASC`, now)
	if err != nil {
		return nil, fmt.Errorf("list active play campaigns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.PlayCampaign, 0)
	for rows.Next() {
		var item service.PlayCampaign
		var rulesRaw string
		if err := rows.Scan(&item.ID, &item.Name, &item.StartAt, &item.EndAt, &rulesRaw, &item.Enabled, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan play campaign: %w", err)
		}
		item.Rules = service.ParsePlayCampaignRules(rulesRaw)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list active play campaigns rows: %w", err)
	}
	return out, nil
}
