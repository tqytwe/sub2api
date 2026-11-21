package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const providerProfitLinkSelect = `SELECT l.cost_mode,p.id,p.name,p.site_type,c.id,c.name,l.revision,p.observed_at,p.status,
 CASE WHEN jsonb_typeof(p.metrics->'today_spend')='number' THEN p.metrics->>'today_spend' END,
 CASE WHEN jsonb_typeof(p.metrics->'total_spend')='number' THEN p.metrics->>'total_spend' END,
 CASE WHEN p.metrics->>'currency' IN ('USD','CNY','quota_units') THEN p.metrics->>'currency' ELSE '' END
 FROM provider_profit_links l
 JOIN mature_providers p ON p.id=l.provider_id
 JOIN upstream_channels c ON c.id=l.channel_id`

func scanProviderProfitLink(row interface{ Scan(...any) error }) (*service.ProviderProfitLink, error) {
	var value service.ProviderProfitLink
	err := row.Scan(
		&value.CostMode, &value.ProviderID, &value.ProviderName, &value.SiteType,
		&value.ChannelID, &value.ChannelName, &value.Revision, &value.ObservedAt,
		&value.Status, &value.Metrics.TodaySpend, &value.Metrics.TotalSpend,
		&value.Metrics.Currency,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrProviderProfitLinkNotFound
	}
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func (r *channelRepository) ListProviderProfitLinks(ctx context.Context, page, size int) (*service.ProviderProfitLinkPage, error) {
	if err := service.ValidateProviderProfitLinkPage(page, size); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	out := &service.ProviderProfitLinkPage{Items: []service.ProviderProfitLink{}, Page: page, PageSize: size}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM provider_profit_links`).Scan(&out.Total); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, providerProfitLinkSelect+` ORDER BY p.id LIMIT $1 OFFSET $2`, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		value, scanErr := scanProviderProfitLink(rows)
		if scanErr != nil {
			_ = rows.Close()
			return nil, scanErr
		}
		out.Items = append(out.Items, *value)
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func isProviderProfitForeignKeyViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && (pqErr.Code == "23503" || pqErr.Code == "23001") {
		return true
	}
	return err != nil && errors.Is(err, sql.ErrNoRows)
}

func (r *channelRepository) SetProviderProfitLink(ctx context.Context, id int64, mutation service.ProviderProfitLinkMutation) (*service.ProviderProfitLink, error) {
	if err := mutation.Validate(id); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	var out *service.ProviderProfitLink
	err := r.runInTx(ctx, func(tx *sql.Tx) error {
		var currentChannel, revision int64
		var costMode string
		err := tx.QueryRowContext(ctx, `SELECT channel_id,revision,cost_mode FROM provider_profit_links WHERE provider_id=$1 FOR UPDATE`, id).Scan(&currentChannel, &revision, &costMode)
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrProviderProfitLinkNotFound
		}
		if err != nil {
			return err
		}
		if mutation.Revision != revision {
			return service.ErrProviderProfitLinkConflict
		}
		if mutation.CostMode != nil {
			costMode = *mutation.CostMode
		}
		if mutation.ChannelID != currentChannel || mutation.CostMode != nil {
			_, err = tx.ExecContext(ctx, `UPDATE provider_profit_links SET channel_id=$2,cost_mode=$3,revision=revision+1 WHERE provider_id=$1`, id, mutation.ChannelID, costMode)
			if isUniqueViolation(err) {
				return service.ErrProviderProfitLinkConflict
			}
			if isProviderProfitForeignKeyViolation(err) {
				return service.ErrProviderProfitLinkNotFound
			}
			if err != nil {
				return err
			}
		}
		out, err = scanProviderProfitLink(tx.QueryRowContext(ctx, providerProfitLinkSelect+` WHERE p.id=$1`, id))
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

var _ service.ProviderProfitLinkRepository = (*channelRepository)(nil)
