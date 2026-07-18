package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
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

func (r *playRepository) ListAdminCampaigns(ctx context.Context) ([]service.PlayCampaign, error) {
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT id, name, start_at, end_at, rules_json::text, enabled, created_at
		FROM play_campaigns
		ORDER BY enabled DESC, start_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list admin play campaigns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.PlayCampaign, 0)
	for rows.Next() {
		item, err := scanPlayCampaign(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list admin play campaigns rows: %w", err)
	}
	return out, nil
}

func (r *playRepository) CreateAdminCampaign(ctx context.Context, campaign service.PlayCampaign) (*service.PlayCampaign, error) {
	rulesJSON, err := json.Marshal(campaign.Rules)
	if err != nil {
		return nil, fmt.Errorf("marshal play campaign rules: %w", err)
	}

	var item service.PlayCampaign
	var rulesRaw string
	err = scanSingleRow(ctx, r.sqlExec(ctx), `
		INSERT INTO play_campaigns (name, start_at, end_at, rules_json, enabled)
		VALUES ($1, $2, $3, $4::jsonb, $5)
		RETURNING id, name, start_at, end_at, rules_json::text, enabled, created_at`,
		[]any{campaign.Name, campaign.StartAt, campaign.EndAt, string(rulesJSON), campaign.Enabled},
		&item.ID, &item.Name, &item.StartAt, &item.EndAt, &rulesRaw, &item.Enabled, &item.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create admin play campaign: %w", err)
	}
	item.Rules = service.ParsePlayCampaignRules(rulesRaw)
	return &item, nil
}

func (r *playRepository) UpdateAdminCampaign(ctx context.Context, campaign service.PlayCampaign) (*service.PlayCampaign, error) {
	rulesJSON, err := json.Marshal(campaign.Rules)
	if err != nil {
		return nil, fmt.Errorf("marshal play campaign rules: %w", err)
	}

	var item service.PlayCampaign
	var rulesRaw string
	err = scanSingleRow(ctx, r.sqlExec(ctx), `
		UPDATE play_campaigns
		SET name = $2, start_at = $3, end_at = $4, rules_json = $5::jsonb, enabled = $6
		WHERE id = $1
		RETURNING id, name, start_at, end_at, rules_json::text, enabled, created_at`,
		[]any{campaign.ID, campaign.Name, campaign.StartAt, campaign.EndAt, string(rulesJSON), campaign.Enabled},
		&item.ID, &item.Name, &item.StartAt, &item.EndAt, &rulesRaw, &item.Enabled, &item.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("update admin play campaign: %w", err)
	}
	item.Rules = service.ParsePlayCampaignRules(rulesRaw)
	return &item, nil
}

func (r *playRepository) DeleteAdminCampaign(ctx context.Context, id int64) error {
	res, err := r.sqlExec(ctx).ExecContext(ctx, `DELETE FROM play_campaigns WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete admin play campaign: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete admin play campaign rows affected: %w", err)
	}
	if n == 0 {
		return infraerrors.NotFound("PLAY_CAMPAIGN_NOT_FOUND", "campaign not found")
	}
	return nil
}

type campaignRowScanner interface {
	Scan(dest ...any) error
}

func scanPlayCampaign(row campaignRowScanner) (service.PlayCampaign, error) {
	var item service.PlayCampaign
	var rulesRaw string
	if err := row.Scan(&item.ID, &item.Name, &item.StartAt, &item.EndAt, &rulesRaw, &item.Enabled, &item.CreatedAt); err != nil {
		return item, fmt.Errorf("scan play campaign: %w", err)
	}
	item.Rules = service.ParsePlayCampaignRules(rulesRaw)
	return item, nil
}
