package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type modelCatalogRepository struct {
	db *sql.DB
}

const catalogSelectColumns = `id, model_name, platform, display_name, use_case, sort_order,
	visible_public, visible_auth, featured, group_ids,
	official_input_price, official_output_price, official_cache_read_price, official_cache_write_price,
	official_source, official_updated_at, price_multiplier,
	input_price, output_price, cache_read_price, cache_write_price,
	billing_mode, source, source_updated_at, created_at, updated_at`

// NewModelCatalogRepository creates a model catalog repository.
func NewModelCatalogRepository(db *sql.DB) service.ModelCatalogRepository {
	return &modelCatalogRepository{db: db}
}

func (r *modelCatalogRepository) ListCatalog(ctx context.Context, filter service.CatalogListFilter) ([]service.SiteModelCatalogEntry, error) {
	var sb strings.Builder
	args := make([]any, 0, 8)
	_, _ = sb.WriteString("SELECT " + catalogSelectColumns + " FROM site_model_catalog WHERE 1=1")

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
	_, _ = sb.WriteString(" ORDER BY sort_order ASC, model_name ASC")
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
	row := r.db.QueryRowContext(ctx, "SELECT "+catalogSelectColumns+" FROM site_model_catalog WHERE id = $1", id)
	entry, err := scanCatalogEntry(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return entry, err
}

func (r *modelCatalogRepository) GetCatalogPricing(ctx context.Context, modelName string) (*service.SiteModelCatalogEntry, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+catalogSelectColumns+`
		FROM site_model_catalog
		WHERE LOWER(model_name) = LOWER($1)
		ORDER BY
			CASE WHEN input_price IS NOT NULL OR output_price IS NOT NULL THEN 0 ELSE 1 END,
			visible_auth DESC,
			id ASC
		LIMIT 1`, strings.TrimSpace(modelName))
	entry, err := scanCatalogEntry(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get catalog pricing: %w", err)
	}
	return entry, nil
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
			visible_public, visible_auth, featured, group_ids,
			official_input_price, official_output_price, official_cache_read_price, official_cache_write_price,
			official_source, official_updated_at, price_multiplier,
			input_price, output_price, cache_read_price, cache_write_price,
			billing_mode, source, source_updated_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,NOW())
		ON CONFLICT (model_name, platform) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			use_case = EXCLUDED.use_case,
			sort_order = EXCLUDED.sort_order,
			visible_public = EXCLUDED.visible_public,
			visible_auth = EXCLUDED.visible_auth,
			featured = EXCLUDED.featured,
			group_ids = EXCLUDED.group_ids,
			official_input_price = COALESCE(EXCLUDED.official_input_price, site_model_catalog.official_input_price),
			official_output_price = COALESCE(EXCLUDED.official_output_price, site_model_catalog.official_output_price),
			official_cache_read_price = COALESCE(EXCLUDED.official_cache_read_price, site_model_catalog.official_cache_read_price),
			official_cache_write_price = COALESCE(EXCLUDED.official_cache_write_price, site_model_catalog.official_cache_write_price),
			official_source = COALESCE(EXCLUDED.official_source, site_model_catalog.official_source),
			official_updated_at = COALESCE(EXCLUDED.official_updated_at, site_model_catalog.official_updated_at),
			price_multiplier = EXCLUDED.price_multiplier,
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
		entry.VisiblePublic, entry.VisibleAuth, entry.Featured, catalogGroupIDsValue(entry.GroupIDs),
		entry.OfficialInputPrice, entry.OfficialOutputPrice, entry.OfficialCacheReadPrice, entry.OfficialCacheWritePrice,
		catalogNullString(entry.OfficialSource), entry.OfficialUpdatedAt, entry.PriceMultiplier,
		entry.InputPrice, entry.OutputPrice, entry.CacheReadPrice, entry.CacheWritePrice,
		billingMode, source, entry.SourceUpdatedAt,
	).Scan(&entry.ID, &entry.CreatedAt, &entry.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert site model catalog: %w", err)
	}
	return nil
}

// UpsertDiscoveryCatalogEntry inserts or updates catalog rows from the discovery pool.
// On conflict it preserves visibility/sort and only refreshes prices and metadata.
func (r *modelCatalogRepository) UpsertDiscoveryCatalogEntry(ctx context.Context, entry *service.SiteModelCatalogEntry) error {
	billingMode := entry.BillingMode
	if billingMode == "" {
		billingMode = string(service.BillingModeToken)
	}
	source := entry.Source
	if source == "" {
		source = "discovery"
	}
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO site_model_catalog (
			model_name, platform, display_name, use_case, sort_order,
			visible_public, visible_auth, featured, group_ids,
			official_input_price, official_output_price, official_cache_read_price, official_cache_write_price,
			official_source, official_updated_at, price_multiplier,
			input_price, output_price, cache_read_price, cache_write_price,
			billing_mode, source, source_updated_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,NOW())
		ON CONFLICT (model_name, platform) DO UPDATE SET
			use_case = COALESCE(EXCLUDED.use_case, site_model_catalog.use_case),
			group_ids = COALESCE(EXCLUDED.group_ids, site_model_catalog.group_ids),
			official_input_price = EXCLUDED.official_input_price,
			official_output_price = EXCLUDED.official_output_price,
			official_cache_read_price = EXCLUDED.official_cache_read_price,
			official_cache_write_price = EXCLUDED.official_cache_write_price,
			official_source = EXCLUDED.official_source,
			official_updated_at = EXCLUDED.official_updated_at,
			price_multiplier = COALESCE(EXCLUDED.price_multiplier, site_model_catalog.price_multiplier),
			input_price = COALESCE(EXCLUDED.input_price, site_model_catalog.input_price),
			output_price = COALESCE(EXCLUDED.output_price, site_model_catalog.output_price),
			cache_read_price = COALESCE(EXCLUDED.cache_read_price, site_model_catalog.cache_read_price),
			cache_write_price = COALESCE(EXCLUDED.cache_write_price, site_model_catalog.cache_write_price),
			source = EXCLUDED.source,
			source_updated_at = EXCLUDED.source_updated_at,
			updated_at = NOW()
		RETURNING id, created_at, updated_at`,
		entry.ModelName, entry.Platform, entry.DisplayName, entry.UseCase, entry.SortOrder,
		entry.VisiblePublic, entry.VisibleAuth, entry.Featured, catalogGroupIDsValue(entry.GroupIDs),
		entry.OfficialInputPrice, entry.OfficialOutputPrice, entry.OfficialCacheReadPrice, entry.OfficialCacheWritePrice,
		catalogNullString(entry.OfficialSource), entry.OfficialUpdatedAt, entry.PriceMultiplier,
		entry.InputPrice, entry.OutputPrice, entry.CacheReadPrice, entry.CacheWritePrice,
		billingMode, source, entry.SourceUpdatedAt,
	).Scan(&entry.ID, &entry.CreatedAt, &entry.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert discovery catalog: %w", err)
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
			visible_public = $6, visible_auth = $7, featured = $8, group_ids = $9,
			input_price = $10, output_price = $11, cache_read_price = $12, cache_write_price = $13,
			price_multiplier = $14, billing_mode = $15, source = $16, source_updated_at = $17, updated_at = NOW()
		 WHERE id = $18`,
		entry.ModelName, entry.Platform, entry.DisplayName, entry.UseCase, entry.SortOrder,
		entry.VisiblePublic, entry.VisibleAuth, entry.Featured, catalogGroupIDsValue(entry.GroupIDs),
		entry.InputPrice, entry.OutputPrice, entry.CacheReadPrice, entry.CacheWritePrice, entry.PriceMultiplier,
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
		args = append(args, *multiplier)
		pos := len(args)
		sets = append(sets,
			fmt.Sprintf("input_price = CASE WHEN official_input_price IS NOT NULL THEN official_input_price * $%d ELSE input_price END", pos),
			fmt.Sprintf("output_price = CASE WHEN official_output_price IS NOT NULL THEN official_output_price * $%d ELSE output_price END", pos),
			fmt.Sprintf("cache_read_price = CASE WHEN official_cache_read_price IS NOT NULL THEN official_cache_read_price * $%d ELSE cache_read_price END", pos),
			fmt.Sprintf("cache_write_price = CASE WHEN official_cache_write_price IS NOT NULL THEN official_cache_write_price * $%d ELSE cache_write_price END", pos),
			fmt.Sprintf("price_multiplier = $%d", pos),
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
	if absoluteInput != nil || absoluteOutput != nil {
		sets = append(sets, "price_multiplier = NULL")
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

func (r *modelCatalogRepository) BatchUpdateGroups(ctx context.Context, ids []int64, groupIDs []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE site_model_catalog
		SET group_ids = $1, updated_at = NOW()
		WHERE id = ANY($2)
	`, catalogGroupIDsValue(groupIDs), pq.Array(ids))
	if err != nil {
		return 0, fmt.Errorf("batch update catalog groups: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (r *modelCatalogRepository) UpdateCatalogOfficialPrices(
	ctx context.Context,
	modelName, platform, source string,
	input, output, cacheRead, cacheWrite *float64,
	updatedAt time.Time,
) (int, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE site_model_catalog SET
			official_input_price = $1::numeric,
			official_output_price = $2::numeric,
			official_cache_read_price = $3::numeric,
			official_cache_write_price = $4::numeric,
			official_source = $5,
			official_updated_at = $6,
			input_price = CASE WHEN price_multiplier IS NOT NULL AND $1::numeric IS NOT NULL THEN $1::numeric * price_multiplier ELSE input_price END,
			output_price = CASE WHEN price_multiplier IS NOT NULL AND $2::numeric IS NOT NULL THEN $2::numeric * price_multiplier ELSE output_price END,
			cache_read_price = CASE WHEN price_multiplier IS NOT NULL AND $3::numeric IS NOT NULL THEN $3::numeric * price_multiplier ELSE cache_read_price END,
			cache_write_price = CASE WHEN price_multiplier IS NOT NULL AND $4::numeric IS NOT NULL THEN $4::numeric * price_multiplier ELSE cache_write_price END,
			updated_at = NOW()
		WHERE LOWER(model_name) = LOWER($7)
			AND LOWER(platform) = LOWER($8)
	`, input, output, cacheRead, cacheWrite, source, updatedAt, modelName, platform)
	if err != nil {
		return 0, fmt.Errorf("update catalog official prices: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (r *modelCatalogRepository) ListDiscoveries(ctx context.Context, filter service.DiscoveryListFilter) (service.DiscoveryListResult, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	where := "WHERE 1=1"
	args := make([]any, 0, 4)
	if filter.Status != "" {
		args = append(args, filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if q := strings.TrimSpace(filter.Search); q != "" {
		args = append(args, "%"+strings.ToLower(q)+"%")
		where += fmt.Sprintf(" AND (LOWER(model_name) LIKE $%d OR LOWER(platform) LIKE $%d)", len(args), len(args))
	}

	countQuery := "SELECT COUNT(*) FROM model_discoveries " + where
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return service.DiscoveryListResult{}, err
	}

	args = append(args, limit, offset)
	query := fmt.Sprintf(`SELECT id, model_name, platform, source, payload, status, discovered_at
		FROM model_discoveries %s ORDER BY discovered_at DESC LIMIT $%d OFFSET $%d`,
		where, len(args)-1, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return service.DiscoveryListResult{}, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.ModelDiscovery, 0)
	for rows.Next() {
		var d service.ModelDiscovery
		var payload []byte
		if err := rows.Scan(&d.ID, &d.ModelName, &d.Platform, &d.Source, &payload, &d.Status, &d.DiscoveredAt); err != nil {
			return service.DiscoveryListResult{}, err
		}
		_ = json.Unmarshal(payload, &d.Payload)
		if d.Payload == nil {
			d.Payload = map[string]any{}
		}
		out = append(out, d)
	}
	return service.DiscoveryListResult{Items: out, Total: total}, rows.Err()
}

func (r *modelCatalogRepository) ListDiscoveriesByIDs(ctx context.Context, ids []int64) ([]service.ModelDiscovery, error) {
	if len(ids) == 0 {
		return []service.ModelDiscovery{}, nil
	}
	query := `SELECT id, model_name, platform, source, payload, status, discovered_at
		FROM model_discoveries WHERE id = ANY($1) AND status = 'new'`
	rows, err := r.db.QueryContext(ctx, query, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.ModelDiscovery, 0, len(ids))
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
	var resultJSON any
	if job.Result != nil {
		encoded, err := json.Marshal(job.Result)
		if err != nil {
			return fmt.Errorf("marshal model sync job result: %w", err)
		}
		resultJSON = string(encoded)
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO model_sync_jobs (id, kind, status, result, error, started_at, completed_at)
		 VALUES ($1,$2,$3,$4::jsonb,$5,$6,$7)`,
		job.ID, job.Kind, job.Status, resultJSON, catalogNullString(job.Error), job.StartedAt, job.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("create model sync job: %w", err)
	}
	return nil
}

func (r *modelCatalogRepository) UpdateSyncJob(ctx context.Context, job *service.ModelSyncJob) error {
	var resultJSON any
	if job.Result != nil {
		encoded, err := json.Marshal(job.Result)
		if err != nil {
			return fmt.Errorf("marshal model sync job result: %w", err)
		}
		resultJSON = string(encoded)
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE model_sync_jobs SET status = $2, result = $3::jsonb, error = $4, completed_at = $5 WHERE id = $1`,
		job.ID, job.Status, resultJSON, catalogNullString(job.Error), job.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("update model sync job: %w", err)
	}
	return nil
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
		return nil, fmt.Errorf("get model sync job: %w", err)
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
	var displayName, useCase, officialSource sql.NullString
	var sourceUpdated, officialUpdated sql.NullTime
	var groupIDs pq.Int64Array
	err := row.Scan(
		&e.ID, &e.ModelName, &e.Platform, &displayName, &useCase, &e.SortOrder,
		&e.VisiblePublic, &e.VisibleAuth, &e.Featured, &groupIDs,
		&e.OfficialInputPrice, &e.OfficialOutputPrice, &e.OfficialCacheReadPrice, &e.OfficialCacheWritePrice,
		&officialSource, &officialUpdated, &e.PriceMultiplier,
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
	if groupIDs != nil {
		e.GroupIDs = make([]int64, len(groupIDs))
		copy(e.GroupIDs, groupIDs)
	}
	if officialSource.Valid {
		e.OfficialSource = officialSource.String
	}
	if officialUpdated.Valid {
		t := officialUpdated.Time
		e.OfficialUpdatedAt = &t
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

func catalogGroupIDsValue(groupIDs []int64) any {
	if groupIDs == nil {
		return nil
	}
	return pq.Array(groupIDs)
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
