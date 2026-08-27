package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const canvasPromptMirrorSourceKey = service.CanvasPromptMirrorSourceKey

type CanvasPromptMirrorRepository struct {
	db *sql.DB
}

func NewCanvasPromptMirrorRepository(db *sql.DB) *CanvasPromptMirrorRepository {
	return &CanvasPromptMirrorRepository{db: db}
}

func (r *CanvasPromptMirrorRepository) Refresh(ctx context.Context, snapshot service.CanvasPromptMirrorSnapshot) (*service.CanvasPromptMirrorManifest, error) {
	if r == nil || r.db == nil {
		return nil, service.ErrCanvasPromptMirrorUnavailable
	}
	if len(snapshot.Items) > 5000 {
		return nil, service.ErrCanvasPromptMirrorLimit
	}
	categories := append([]string(nil), snapshot.Categories...)
	sort.Strings(categories)
	categoryJSON, err := json.Marshal(categories)
	if err != nil {
		return nil, err
	}
	now := snapshot.RefreshedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin canvas prompt mirror refresh: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, item := range snapshot.Items {
		tags, err := json.Marshal(item.Tags)
		if err != nil {
			return nil, err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO canvas_prompt_mirror_items
				(source_key, external_id, title, prompt_text, cover_url, source_url, preview, category, tags,
				 content_hash, source_created_at, source_updated_at, created_at, updated_at, deleted_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10, $11, $12, $13, $13, NULL)
			ON CONFLICT (source_key, external_id) DO UPDATE SET
				title = EXCLUDED.title,
				prompt_text = EXCLUDED.prompt_text,
				cover_url = EXCLUDED.cover_url,
				source_url = EXCLUDED.source_url,
				preview = EXCLUDED.preview,
				category = EXCLUDED.category,
				tags = EXCLUDED.tags,
				content_hash = EXCLUDED.content_hash,
				source_created_at = EXCLUDED.source_created_at,
				source_updated_at = EXCLUDED.source_updated_at,
				updated_at = CASE WHEN canvas_prompt_mirror_items.content_hash IS DISTINCT FROM EXCLUDED.content_hash
					OR canvas_prompt_mirror_items.deleted_at IS NOT NULL THEN EXCLUDED.updated_at
					ELSE canvas_prompt_mirror_items.updated_at END,
				deleted_at = NULL`,
			canvasPromptMirrorSourceKey, item.ExternalID, item.Title, item.Prompt, item.CoverURL,
			item.SourceURL, item.Preview, item.Category, string(tags), item.ContentHash,
			item.SourceCreatedAt, item.SourceUpdatedAt, now)
		if err != nil {
			return nil, fmt.Errorf("upsert canvas prompt %s: %w", item.ExternalID, err)
		}
	}
	ids := make([]string, 0, len(snapshot.Items))
	for _, item := range snapshot.Items {
		ids = append(ids, item.ExternalID)
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE canvas_prompt_mirror_items
		SET deleted_at = $3, updated_at = $3
		WHERE source_key = $1 AND deleted_at IS NULL
		  AND NOT (external_id = ANY($2::text[]))`, canvasPromptMirrorSourceKey, pq.Array(ids), now)
	if err != nil {
		return nil, fmt.Errorf("tombstone canvas prompts: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO canvas_prompt_mirror_state
			(source_key, revision, total, categories, categories_hash, updated_at, categories_updated_at, refreshed_at)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6, $6, $6)
		ON CONFLICT (source_key) DO UPDATE SET
			revision = EXCLUDED.revision,
			total = EXCLUDED.total,
			categories = EXCLUDED.categories,
			categories_hash = EXCLUDED.categories_hash,
			updated_at = CASE WHEN canvas_prompt_mirror_state.revision IS DISTINCT FROM EXCLUDED.revision
				THEN EXCLUDED.updated_at ELSE canvas_prompt_mirror_state.updated_at END,
			categories_updated_at = CASE WHEN canvas_prompt_mirror_state.categories_hash IS DISTINCT FROM EXCLUDED.categories_hash
				THEN EXCLUDED.categories_updated_at ELSE canvas_prompt_mirror_state.categories_updated_at END,
			refreshed_at = EXCLUDED.refreshed_at`,
		canvasPromptMirrorSourceKey, snapshot.Revision, len(snapshot.Items), string(categoryJSON),
		snapshot.CategoriesHash, now)
	if err != nil {
		return nil, fmt.Errorf("update canvas prompt mirror state: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit canvas prompt mirror refresh: %w", err)
	}
	return r.Manifest(ctx)
}

func (r *CanvasPromptMirrorRepository) Manifest(ctx context.Context) (*service.CanvasPromptMirrorManifest, error) {
	if r == nil || r.db == nil {
		return nil, service.ErrCanvasPromptMirrorUnavailable
	}
	var revision string
	var total int64
	var rawCategories []byte
	var updatedAt time.Time
	if err := r.db.QueryRowContext(ctx, `SELECT revision, total, categories, updated_at FROM canvas_prompt_mirror_state WHERE source_key = $1`, canvasPromptMirrorSourceKey).
		Scan(&revision, &total, &rawCategories, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &service.CanvasPromptMirrorManifest{MediaType: "image", Revision: emptyCanvasPromptMirrorRevision(), Categories: []string{}, UpdatedAt: time.Unix(0, 0).UTC()}, nil
		}
		return nil, fmt.Errorf("read canvas prompt mirror manifest: %w", err)
	}
	var categories []string
	if err := json.Unmarshal(rawCategories, &categories); err != nil {
		return nil, fmt.Errorf("decode canvas prompt mirror categories: %w", err)
	}
	return &service.CanvasPromptMirrorManifest{MediaType: "image", Revision: revision, Total: total, Categories: categories, UpdatedAt: updatedAt.UTC()}, nil
}

func (r *CanvasPromptMirrorRepository) Catalog(ctx context.Context, page, pageSize int) (*service.CanvasPromptMirrorCatalog, error) {
	if r == nil || r.db == nil {
		return nil, service.ErrCanvasPromptMirrorUnavailable
	}
	manifest, err := r.Manifest(ctx)
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 100
	}
	pages := int(math.Ceil(float64(manifest.Total) / float64(pageSize)))
	if pages < 1 {
		pages = 1
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT external_id, title, prompt_text, cover_url, source_url, preview, category, tags, source_created_at, updated_at
		FROM canvas_prompt_mirror_items
		WHERE source_key = $1 AND deleted_at IS NULL
		ORDER BY external_id ASC
		LIMIT $2 OFFSET $3`, canvasPromptMirrorSourceKey, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, fmt.Errorf("list canvas prompt mirror catalog: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.CanvasPromptMirrorCatalogItem, 0, pageSize)
	for rows.Next() {
		item, err := scanCanvasPromptMirrorItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &service.CanvasPromptMirrorCatalog{Items: items, Total: manifest.Total, Page: page, PageSize: pageSize, Pages: pages, Revision: manifest.Revision, Categories: manifest.Categories}, nil
}

func (r *CanvasPromptMirrorRepository) Delta(ctx context.Context, since time.Time) (*service.CanvasPromptMirrorDelta, error) {
	if r == nil || r.db == nil {
		return nil, service.ErrCanvasPromptMirrorUnavailable
	}
	manifest, err := r.Manifest(ctx)
	if err != nil {
		return nil, err
	}
	var categoriesUpdatedAt time.Time
	if err := r.db.QueryRowContext(ctx, `SELECT categories_updated_at FROM canvas_prompt_mirror_state WHERE source_key = $1`, canvasPromptMirrorSourceKey).Scan(&categoriesUpdatedAt); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	boundary := since.UTC().Add(-time.Microsecond)
	rows, err := r.db.QueryContext(ctx, `
		SELECT external_id, title, prompt_text, cover_url, source_url, preview, category, tags, source_created_at, updated_at
		FROM canvas_prompt_mirror_items
		WHERE source_key = $1 AND updated_at > $2
		  AND deleted_at IS NULL
		ORDER BY updated_at ASC, external_id ASC`, canvasPromptMirrorSourceKey, boundary)
	if err != nil {
		return nil, fmt.Errorf("list canvas prompt mirror delta: %w", err)
	}
	items := make([]service.CanvasPromptMirrorCatalogItem, 0)
	for rows.Next() {
		item, err := scanCanvasPromptMirrorItem(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	deletedRows, err := r.db.QueryContext(ctx, `
		SELECT external_id FROM canvas_prompt_mirror_items
		WHERE source_key = $1 AND updated_at > $2 AND deleted_at IS NOT NULL
		ORDER BY updated_at ASC, external_id ASC`, canvasPromptMirrorSourceKey, boundary)
	if err != nil {
		return nil, err
	}
	defer func() { _ = deletedRows.Close() }()
	deletedIDs := make([]string, 0)
	for deletedRows.Next() {
		var id string
		if err := deletedRows.Scan(&id); err != nil {
			return nil, err
		}
		deletedIDs = append(deletedIDs, id)
	}
	if err := deletedRows.Err(); err != nil {
		return nil, err
	}
	cursor := manifest.UpdatedAt
	if categoriesUpdatedAt.After(cursor) {
		cursor = categoriesUpdatedAt
	}
	var categories *[]string
	if categoriesUpdatedAt.After(boundary) {
		copyCategories := append([]string(nil), manifest.Categories...)
		categories = &copyCategories
	}
	return &service.CanvasPromptMirrorDelta{Cursor: cursor.UTC().Format(time.RFC3339Nano), Revision: manifest.Revision, Version: manifest.Revision, ETag: manifest.Revision, Items: items, DeletedIDs: deletedIDs, Categories: categories}, nil
}

func (r *CanvasPromptMirrorRepository) Get(ctx context.Context, id string) (*service.CanvasPromptMirrorCatalogItem, error) {
	if r == nil || r.db == nil {
		return nil, service.ErrCanvasPromptMirrorUnavailable
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT external_id, title, prompt_text, cover_url, source_url, preview, category, tags, source_created_at, updated_at
		FROM canvas_prompt_mirror_items
		WHERE source_key = $1 AND external_id = $2 AND deleted_at IS NULL`, canvasPromptMirrorSourceKey, id)
	item, err := scanCanvasPromptMirrorItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get canvas prompt mirror item: %w", err)
	}
	return &item, nil
}

func scanCanvasPromptMirrorItem(rows interface{ Scan(...any) error }) (service.CanvasPromptMirrorCatalogItem, error) {
	var item service.CanvasPromptMirrorCatalogItem
	var rawTags []byte
	var createdAt *time.Time
	if err := rows.Scan(&item.ID, &item.Title, &item.Prompt, &item.CoverURL, &item.SourceURL, &item.Preview, &item.Category, &rawTags, &createdAt, &item.UpdatedAt); err != nil {
		return item, err
	}
	if err := json.Unmarshal(rawTags, &item.Tags); err != nil {
		return item, err
	}
	if item.Tags == nil {
		item.Tags = []string{}
	}
	item.MediaType = "image"
	item.PromptText = item.Prompt
	item.CoverSourceURL = item.CoverURL
	if item.Category != "" {
		item.Categories = []string{item.Category}
	}
	item.CoverURL = canvasPromptMirrorCoverPath(item.ID)
	item.Media = []service.PromptMedia{{
		MediaType: "image",
		URL:       item.CoverURL,
		AltZH:     item.Title,
		SortOrder: 0,
	}}
	item.Version = 1
	if createdAt != nil {
		item.CreatedAt = createdAt.UTC()
	}
	item.UpdatedAt = item.UpdatedAt.UTC()
	return item, nil
}

func canvasPromptMirrorCoverPath(id string) string {
	return "/api/v1/mobile/canvas-prompts/" + url.PathEscape(id) + "/cover"
}

func emptyCanvasPromptMirrorRevision() string {
	sum := sha256.Sum256([]byte(`{"Items":null,"Categories":[]}`))
	return hex.EncodeToString(sum[:])
}

var _ service.CanvasPromptMirrorRepository = (*CanvasPromptMirrorRepository)(nil)
