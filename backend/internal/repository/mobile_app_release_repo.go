package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type mobileAppReleaseRepository struct{ db *sql.DB }

func NewMobileAppReleaseRepository(db *sql.DB) service.MobileAppReleaseRepository {
	return &mobileAppReleaseRepository{db: db}
}

const mobileReleaseColumns = `id, distribution, package_name, artifact_type, version, version_code,
storage_key, download_url, bytes, sha256, signing_certificate_sha256, min_supported_version_code,
notes, notes_i18n, manifest, status, rollout_percent, created_by, created_at, updated_at, published_at`

func (r *mobileAppReleaseRepository) Create(ctx context.Context, release service.MobileAppRelease) (*service.MobileAppRelease, error) {
	notes, err := json.Marshal(release.Notes)
	if err != nil {
		return nil, err
	}
	notesI18n, err := json.Marshal(release.NotesI18n)
	if err != nil {
		return nil, err
	}
	manifest, err := json.Marshal(release.VersionManifest)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `INSERT INTO mobile_app_releases
        (distribution, package_name, artifact_type, version, version_code, storage_key, download_url, bytes,
         sha256, signing_certificate_sha256, min_supported_version_code, notes, notes_i18n, manifest,
         status, rollout_percent, created_by)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,$13::jsonb,$14::jsonb,$15,$16,$17)
        RETURNING `+mobileReleaseColumns,
		release.Distribution, release.PackageName, release.ArtifactType, release.Version, release.VersionCode,
		release.StorageKey, release.DownloadURL, release.Bytes, release.SHA256, release.SigningCertificateSHA256,
		release.MinSupportedVersionCode, notes, notesI18n, manifest, release.Status, release.RolloutPercent, release.CreatedBy)
	return scanMobileAppRelease(row)
}

func (r *mobileAppReleaseRepository) List(ctx context.Context, distribution string) ([]service.MobileAppRelease, error) {
	query := `SELECT ` + mobileReleaseColumns + ` FROM mobile_app_releases`
	args := []any{}
	if distribution != "" {
		query += ` WHERE distribution = $1`
		args = append(args, distribution)
	}
	query += ` ORDER BY version_code DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.MobileAppRelease, 0)
	for rows.Next() {
		item, err := scanMobileAppRelease(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *mobileAppReleaseRepository) Get(ctx context.Context, id int64) (*service.MobileAppRelease, error) {
	return scanMobileAppRelease(r.db.QueryRowContext(ctx, `SELECT `+mobileReleaseColumns+` FROM mobile_app_releases WHERE id = $1`, id))
}

func (r *mobileAppReleaseRepository) HighestVersionCode(ctx context.Context, distribution string) (int64, error) {
	var code sql.NullInt64
	err := r.db.QueryRowContext(ctx, `SELECT MAX(version_code) FROM mobile_app_releases WHERE distribution = $1`, distribution).Scan(&code)
	if err != nil {
		return 0, err
	}
	if !code.Valid {
		return 0, nil
	}
	return code.Int64, nil
}

func (r *mobileAppReleaseRepository) Published(ctx context.Context, distribution string) (*service.MobileAppRelease, error) {
	return scanMobileAppRelease(r.db.QueryRowContext(ctx, `SELECT `+mobileReleaseColumns+`
        FROM mobile_app_releases WHERE distribution=$1 AND status='published'
        ORDER BY version_code DESC LIMIT 1`, distribution))
}

func (r *mobileAppReleaseRepository) Publish(ctx context.Context, id int64) (*service.MobileAppRelease, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var distribution string
	if err := tx.QueryRowContext(ctx, `SELECT distribution FROM mobile_app_releases WHERE id = $1`, id).Scan(&distribution); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrMobileReleaseNotFound
		}
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mobile_app_releases SET status='paused', updated_at=NOW() WHERE distribution=$1 AND status='published' AND id<>$2`, distribution, id); err != nil {
		return nil, err
	}
	row := tx.QueryRowContext(ctx, `UPDATE mobile_app_releases SET status='published', published_at=NOW(), updated_at=NOW()
        WHERE id=$1 AND status IN ('ready','paused','published') RETURNING `+mobileReleaseColumns, id)
	release, err := scanMobileAppRelease(row)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return release, nil
}

func (r *mobileAppReleaseRepository) SetStatus(ctx context.Context, id int64, status string) (*service.MobileAppRelease, error) {
	row := r.db.QueryRowContext(ctx, `UPDATE mobile_app_releases SET status=$2, updated_at=NOW() WHERE id=$1 RETURNING `+mobileReleaseColumns, id, status)
	return scanMobileAppRelease(row)
}

type mobileReleaseRow interface{ Scan(...any) error }

func scanMobileAppRelease(row mobileReleaseRow) (*service.MobileAppRelease, error) {
	var release service.MobileAppRelease
	var notesJSON, notesI18nJSON, manifestJSON []byte
	var publishedAt sql.NullTime
	var createdBy sql.NullInt64
	if err := row.Scan(&release.ID, &release.Distribution, &release.PackageName, &release.ArtifactType,
		&release.Version, &release.VersionCode, &release.StorageKey, &release.DownloadURL, &release.Bytes,
		&release.SHA256, &release.SigningCertificateSHA256, &release.MinSupportedVersionCode,
		&notesJSON, &notesI18nJSON, &manifestJSON, &release.Status, &release.RolloutPercent, &createdBy,
		&release.CreatedAt, &release.UpdatedAt, &publishedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrMobileReleaseNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(notesJSON, &release.Notes); err != nil {
		return nil, fmt.Errorf("decode release notes: %w", err)
	}
	if err := json.Unmarshal(notesI18nJSON, &release.NotesI18n); err != nil {
		return nil, fmt.Errorf("decode localized release notes: %w", err)
	}
	if err := json.Unmarshal(manifestJSON, &release.VersionManifest); err != nil {
		return nil, fmt.Errorf("decode release manifest: %w", err)
	}
	if createdBy.Valid {
		release.CreatedBy = createdBy.Int64
	}
	if publishedAt.Valid {
		value := publishedAt.Time
		release.PublishedAt = &value
	}
	return &release, nil
}

var _ service.MobileAppReleaseRepository = (*mobileAppReleaseRepository)(nil)
