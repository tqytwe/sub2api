package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type modelCatalogRepository struct {
	db *sql.DB
}

// NewModelCatalogRepository creates a model catalog repository.
func NewModelCatalogRepository(db *sql.DB) service.ModelCatalogRepository {
	return &modelCatalogRepository{db: db}
}

func (r *modelCatalogRepository) ListCatalog(ctx context.Context, filter service.CatalogListFilter) ([]service.SiteModelCatalogEntry, error) {
	var sb strings.Builder
	args := make([]any, 0, 8)
	sb.WriteString(`SELECT id, model_name, platform, display_name, use_case, sort_order,
		visible_public, visible_auth, featured,
		input_price, output_price, cache_read_price, cache_write_price,
		billing_mode, source, source_updated_at, created_at, updated_at
		FROM site_model_catalog WHERE 1=1`)

	if p := strings.TrimSpace(filter.Platform); p != "" {
		args = append(args, p)
		fmt.Fprintf(&sb, " AND platform = $%d", len(args))
	}
	if filter.VisiblePublic != nil {
		args = append(args, *filter.VisiblePublic)
		fmt.Fprintf(&sb, " AND visible_public = $%d", len(args))
	}
	if filter.VisibleAuth != nil {
		args = append(args, *filter.VisibleAuth)
		fmt.Fprintf(&sb, " AND visible_auth = $%d", len(args))
	}
	if q := strings.TrimSpace(filter.Search); q != "" {
		args = append(args, "%"+strings.ToLower(q)+"%")
		fmt.Fprintf(&sb, " AND (LOWER(model_name) LIKE $%d OR LOWER(platform) LIKE $%d)", len(args), len(args))
	}
	sb.WriteString(" ORDER BY sort_order ASC, model_name ASC")
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		fmt.Fprintf(&sb, " LIMIT $%d", len(args))
	}
	if filter.Offset > 0 {
		args = append(args, filter.Offset)
		fmt.Fprintf(&sb, " OFFSET $%d", len(args))
	}

	rows, err := r.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list site model catalog: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.SiteModelCatalogEntry, 0)
	for rows.Next() {
		entry, err := scanCatalogEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *entry)
	}
	return out, rows.Err()
}

func (r *modelCatalogRepository) GetCatalogEntry(ctx context.Context, id int64) (*service.SiteModelCatalogEntry, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, model_name, platform, display_name, use_case, sort_order,
			visible_public, visible_auth, featured,
			input_price, output_price, cache_read_price, cache_write_price,
			billing_mode, source, source_updated_at, created_at, updated_at
		 FROM site_model_catalog WHERE id = $1`, id)
	entry, err := scanCatalogEntry(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return entry, err
}

func (r *modelCatalogRepository) UpsertCatalogEntry(ctx context.Context, entry *service.SiteModelCatalogEntry) error {
	billingMode := entry.BillingMode
	if billingMode == "" {
		billingMode = string(service.BillingModeToken)
	}
	source := entry.Source
	if source == "" {
		source = "manual"
	}
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO site_model_catalog (
			model_name, platform, display_name, use_case, sort_order,
			visible_public, visible_auth, featured,
			input_price, output_price, cache_read_price, cache_write_price,
			billing_mode, source, source_updated_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,NOW())
		ON CONFLICT (model_name, platform) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			use_case = EXCLUDED.use_case,
			sort_order = EXCLUDED.sort_order,
			visible_public = EXCLUDED.visible_public,
			visible_auth = EXCLUDED.visible_auth,
			featured = EXCLUDED.featured,
			input_price = EXCLUDED.input_price,
			output_price = EXCLUDED.output_price,
			cache_read_price = EXCLUDED.cache_read_price,
			cache_write_price = EXCLUDED.cache_write_price,
			billing_mode = EXCLUDED.billing_mode,
			source = EXCLUDED.source,
			source_updated_at = EXCLUDED.source_updated_at,
			updated_at = NOW()
		RETURNING id, created_at, updated_at`,
		entry.ModelName, entry.Platform, entry.DisplayName, entry.UseCase, entry.SortOrder,
		entry.VisiblePublic, entry.VisibleAuth, entry.Featured,
		entry.InputPrice, entry.OutputPrice, entry.CacheReadPrice, entry.CacheWritePrice,
		billingMode, source, entry.SourceUpdatedAt,
	).Scan(&entry.ID, &entry.CreatedAt, &entry.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert site model catalog: %w", err)
	}
	return nil
}

func (r *modelCatalogRepository) UpdateCatalogEntry(ctx context.Context, entry *service.SiteModelCatalogEntry) error {
	billingMode := entry.BillingMode
	if billingMode == "" {
		billingMode = string(service.BillingModeToken)
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE site_model_catalog SET
			model_name = $1, platform = $2, display_name = $3, use_case = $4, sort_order = $5,
			visible_public = $6, visible_auth = $7, featured = $8,
			input_price = $9, output_price = $10, cache_read_price = $11, cache_write_price = $12,
			billing_mode = $13, source = $14, source_updated_at = $15, updated_at = NOW()
		 WHERE id = $16`,
		entry.ModelName, entry.Platform, entry.DisplayName, entry.UseCase, entry.SortOrder,
		entry.VisiblePublic, entry.VisibleAuth, entry.Featured,
		entry.InputPrice, entry.OutputPrice, entry.CacheReadPrice, entry.CacheWritePrice,
		billingMode, entry.Source, entry.SourceUpdatedAt, entry.ID,
	)
	if err != nil {
		return fmt.Errorf("update site model catalog: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("catalog entry not found: %d", entry.ID)
	}
	return nil
}

func (r *modelCatalogRepository) DeleteCatalogEntry(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM site_model_catalog WHERE id = $1`, id)
	return err
}

func (r *modelCatalogRepository) BatchUpdateVisibility(ctx context.Context, ids []int64, visiblePublic, visibleAuth *bool) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var sets []string
	args := make([]any, 0, len(ids)+2)
	if visiblePublic != nil {
		args = append(args, *visiblePublic)
		sets = append(sets, fmt.Sprintf("visible_public = $%d", len(args)))
	}
	if visibleAuth != nil {
		args = append(args, *visibleAuth)
		sets = append(sets, fmt.Sprintf("visible_auth = $%d", len(args)))
	}
	if len(sets) == 0 {
		return 0, nil
	}
	sets = append(sets, "updated_at = NOW()")
	args = append(args, pq.Array(ids))
	query := fmt.Sprintf("UPDATE site_model_catalog SET %s WHERE id = ANY($%d)", strings.Join(sets, ", "), len(args))
	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (r *modelCatalogRepository) BatchUpdatePrices(ctx context.Context, ids []int64, multiplier *float64, absoluteInput, absoluteOutput *float64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var sets []string
	args := make([]any, 0, len(ids)+3)
	if multiplier != nil && *multiplier > 0 {
		m := *multiplier
		sets = append(sets,
			fmt.Sprintf("input_price = CASE WHEN input_price IS NOT NULL THEN input_price * %f ELSE input_price END", m),
			fmt.Sprintf("output_price = CASE WHEN output_price IS NOT NULL THEN output_price * %f ELSE output_price END", m),
		)
	}
	if absoluteInput != nil {
		args = append(args, *absoluteInput)
		sets = append(sets, fmt.Sprintf("input_price = $%d", len(args)))
	}
	if absoluteOutput != nil {
		args = append(args, *absoluteOutput)
		sets = append(sets, fmt.Sprintf("output_price = $%d", len(args)))
	}
	if len(sets) == 0 {
		return 0, nil
	}
	sets = append(sets, "updated_at = NOW()")
	args = append(args, pq.Array(ids))
	query := fmt.Sprintf("UPDATE site_model_catalog SET %s WHERE id = ANY($%d)", strings.Join(sets, ", "), len(args))
	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (r *modelCatalogRepository) ListDiscoveries(ctx context.Context, status string, limit int) ([]service.ModelDiscovery, error) {
	if limit <= 0 {
		limit = 200
	}
	query := `SELECT id, model_name, platform, source, payload, status, discovered_at
		FROM model_discoveries WHERE 1=1`
	args := make([]any, 0, 2)
	if status != "" {
		args = append(args, status)
		query += fmt.Sprintf(" AND status = $%d", len(args))
	}
	args = append(args, limit)
	query += fmt.Sprintf(" ORDER BY discovered_at DESC LIMIT $%d", len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.ModelDiscovery, 0)
	for rows.Next() {
		var d service.ModelDiscovery
		var payload []byte
		if err := rows.Scan(&d.ID, &d.ModelName, &d.Platform, &d.Source, &payload, &d.Status, &d.DiscoveredAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(payload, &d.Payload)
		if d.Payload == nil {
			d.Payload = map[string]any{}
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *modelCatalogRepository) UpsertDiscovery(ctx context.Context, d *service.ModelDiscovery) error {
	payload, err := json.Marshal(d.Payload)
	if err != nil {
		return err
	}
	status := d.Status
	if status == "" {
		status = "new"
	}
	return r.db.QueryRowContext(ctx,
		`INSERT INTO model_discoveries (model_name, platform, source, payload, status)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (model_name, platform, source) DO UPDATE SET
			payload = EXCLUDED.payload,
			status = CASE WHEN model_discoveries.status = 'imported' THEN model_discoveries.status ELSE EXCLUDED.status END,
			discovered_at = NOW()
		 RETURNING id, discovered_at`,
		d.ModelName, d.Platform, d.Source, payload, status,
	).Scan(&d.ID, &d.DiscoveredAt)
}

func (r *modelCatalogRepository) UpdateDiscoveryStatus(ctx context.Context, ids []int64, status string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE model_discoveries SET status = $1 WHERE id = ANY($2)`, status, pq.Array(ids))
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (r *modelCatalogRepository) CreateSyncJob(ctx context.Context, job *service.ModelSyncJob) error {
	var resultJSON []byte
	if job.Result != nil {
		var err error
		resultJSON, err = json.Marshal(job.Result)
		if err != nil {
			return err
		}
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO model_sync_jobs (id, kind, status, result, error, started_at, completed_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		job.ID, job.Kind, job.Status, resultJSON, catalogNullString(job.Error), job.StartedAt, job.CompletedAt,
	)
	return err
}

func (r *modelCatalogRepository) UpdateSyncJob(ctx context.Context, job *service.ModelSyncJob) error {
	var resultJSON []byte
	if job.Result != nil {
		var err error
		resultJSON, err = json.Marshal(job.Result)
		if err != nil {
			return err
		}
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE model_sync_jobs SET status = $2, result = $3, error = $4, completed_at = $5 WHERE id = $1`,
		job.ID, job.Status, resultJSON, catalogNullString(job.Error), job.CompletedAt,
	)
	return err
}

func (r *modelCatalogRepository) GetSyncJob(ctx context.Context, id string) (*service.ModelSyncJob, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, kind, status, result, error, started_at, completed_at FROM model_sync_jobs WHERE id = $1`, id)
	var job service.ModelSyncJob
	var resultJSON []byte
	var errText sql.NullString
	var completedAt sql.NullTime
	if err := row.Scan(&job.ID, &job.Kind, &job.Status, &resultJSON, &errText, &job.StartedAt, &completedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if len(resultJSON) > 0 {
		_ = json.Unmarshal(resultJSON, &job.Result)
	}
	if errText.Valid {
		job.Error = errText.String
	}
	if completedAt.Valid {
		t := completedAt.Time
		job.CompletedAt = &t
	}
	return &job, nil
}

type catalogScanner interface {
	Scan(dest ...any) error
}

func scanCatalogEntry(row catalogScanner) (*service.SiteModelCatalogEntry, error) {
	var e service.SiteModelCatalogEntry
	var displayName, useCase sql.NullString
	var sourceUpdated sql.NullTime
	err := row.Scan(
		&e.ID, &e.ModelName, &e.Platform, &displayName, &useCase, &e.SortOrder,
		&e.VisiblePublic, &e.VisibleAuth, &e.Featured,
		&e.InputPrice, &e.OutputPrice, &e.CacheReadPrice, &e.CacheWritePrice,
		&e.BillingMode, &e.Source, &sourceUpdated, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if displayName.Valid {
		e.DisplayName = &displayName.String
	}
	if useCase.Valid {
		e.UseCase = &useCase.String
	}
	if sourceUpdated.Valid {
		t := sourceUpdated.Time
		e.SourceUpdatedAt = &t
	}
	return &e, nil
}

func catalogNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// ListAllModelPricingEntries returns every channel_model_pricing row (for sync).
func (r *channelRepository) ListAllModelPricingEntries(ctx context.Context) ([]service.ChannelModelPricing, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, channel_id, platform, models, billing_mode, input_price, output_price,
			cache_write_price, cache_read_price, image_output_price, per_request_price, created_at, updated_at
		 FROM channel_model_pricing ORDER BY channel_id, id`)
	if err != nil {
		return nil, fmt.Errorf("list all model pricing: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result, pricingIDs, err := scanModelPricingRows(rows)
	if err != nil {
		return nil, err
	}
	if len(pricingIDs) > 0 {
		intervalMap, err := r.batchLoadIntervals(ctx, pricingIDs)
		if err != nil {
			return nil, err
		}
		for i := range result {
			result[i].Intervals = intervalMap[result[i].ID]
		}
	}
	return result, nil
}

// UpdateModelPricingPrices updates only price columns on a pricing entry.
func (r *channelRepository) UpdateModelPricingPrices(ctx context.Context, pricing *service.ChannelModelPricing) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE channel_model_pricing SET
			input_price = $1, output_price = $2, cache_write_price = $3, cache_read_price = $4, updated_at = NOW()
		 WHERE id = $5`,
		pricing.InputPrice, pricing.OutputPrice, pricing.CacheWritePrice, pricing.CacheReadPrice, pricing.ID,
	)
	return err
}

// ListCatalogModelKeys returns lower(model_name)::platform keys already in catalog.
func (r *modelCatalogRepository) ListCatalogModelKeys(ctx context.Context) (map[string]struct{}, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT model_name, platform FROM site_model_catalog`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[string]struct{})
	for rows.Next() {
		var name, platform string
		if err := rows.Scan(&name, &platform); err != nil {
			return nil, err
		}
		out[catalogKey(name, platform)] = struct{}{}
	}
	return out, rows.Err()
}

func catalogKey(name, platform string) string {
	return strings.ToLower(strings.TrimSpace(name)) + "::" + strings.ToLower(strings.TrimSpace(platform))
}
