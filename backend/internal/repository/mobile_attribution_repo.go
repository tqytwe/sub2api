package repository

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type MobileAttributionRepository struct{ db *sql.DB }

func NewMobileAttributionRepository(db *sql.DB) service.MobileAttributionRepository {
	return &MobileAttributionRepository{db: db}
}

func (r *MobileAttributionRepository) BindInstallation(ctx context.Context, binding service.MobileAttributionBinding) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var existingUser sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT user_id FROM mobile_installations WHERE installation_id = $1::uuid FOR UPDATE`, binding.InstallationID).Scan(&existingUser)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if binding.RequireExisting && (errors.Is(err, sql.ErrNoRows) || !existingUser.Valid) {
		return service.ErrMobileAttributionInstallationUnbound
	}
	if err == nil && existingUser.Valid && existingUser.Int64 != binding.UserID {
		return service.ErrMobileAttributionInstallationConflict
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mobile_installations
			(installation_id, user_id, platform, app_version, locale, campaign_id, referral_campaign_id, referrer_user_id, attribution_digest, attribution_verified_at, first_seen_at, last_seen_at, updated_at)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, CASE WHEN $9 IS NULL THEN NULL ELSE $10 END, $10, $10, $10)
		ON CONFLICT (installation_id) DO UPDATE SET
			user_id = CASE WHEN mobile_installations.user_id IS NULL THEN EXCLUDED.user_id ELSE mobile_installations.user_id END,
			platform = EXCLUDED.platform, app_version = EXCLUDED.app_version, locale = EXCLUDED.locale,
			campaign_id = COALESCE(mobile_installations.campaign_id, EXCLUDED.campaign_id),
			referral_campaign_id = COALESCE(mobile_installations.referral_campaign_id, EXCLUDED.referral_campaign_id),
			referrer_user_id = COALESCE(mobile_installations.referrer_user_id, EXCLUDED.referrer_user_id),
			attribution_digest = COALESCE(mobile_installations.attribution_digest, EXCLUDED.attribution_digest),
			attribution_verified_at = COALESCE(mobile_installations.attribution_verified_at, EXCLUDED.attribution_verified_at),
			last_seen_at = EXCLUDED.last_seen_at, updated_at = EXCLUDED.updated_at`,
		binding.InstallationID, binding.UserID, binding.Platform, binding.AppVersion, binding.Locale,
		binding.CampaignID, binding.ReferralCampaignID, binding.ReferrerUserID, mobileAttributionNullableBytes(binding.AttributionDigest), binding.BoundAt)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE mobile_attribution_events SET user_id = $2 WHERE installation_id = $1::uuid AND user_id IS NULL`, binding.InstallationID, binding.UserID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *MobileAttributionRepository) RecordEvent(ctx context.Context, write service.MobileAttributionEventWrite) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	metadata := encodeAttributionMetadata(write.Metadata)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mobile_installations (installation_id, user_id, platform, app_version, locale, first_seen_at, last_seen_at, updated_at)
		VALUES ($1::uuid, NULLIF($2, 0), $3, $4, $5, $6, $6, $6)
		ON CONFLICT (installation_id) DO UPDATE SET
			platform = EXCLUDED.platform, app_version = EXCLUDED.app_version, locale = EXCLUDED.locale,
			last_seen_at = EXCLUDED.last_seen_at, updated_at = EXCLUDED.updated_at`,
		write.InstallationID, write.UserID, write.Platform, write.AppVersion, write.Locale, write.OccurredAt)
	if err != nil {
		return false, err
	}
	var id int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO mobile_attribution_events
			(installation_id, user_id, event_type, idempotency_key_digest, attribution_digest, campaign_id, referral_campaign_id, referrer_user_id, platform, app_version, occurred_at, verified, metadata)
		VALUES ($1::uuid, NULLIF($2, 0), $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb)
		ON CONFLICT (idempotency_key_digest) DO NOTHING RETURNING id`,
		write.InstallationID, write.UserID, write.EventType, write.IdempotencyKeyDigest, mobileAttributionNullableBytes(write.AttributionDigest), write.CampaignID, write.ReferralCampaignID, write.ReferrerUserID,
		write.Platform, write.AppVersion, write.OccurredAt, write.Verified, metadata).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		if err = tx.Commit(); err != nil {
			return false, err
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(write.AttributionDigest) > 0 {
		_, err = tx.ExecContext(ctx, `
			UPDATE mobile_installations SET
				campaign_id = COALESCE(campaign_id, $2), referral_campaign_id = COALESCE(referral_campaign_id, $3), referrer_user_id = COALESCE(referrer_user_id, $4),
				attribution_digest = COALESCE(attribution_digest, $5),
				attribution_verified_at = COALESCE(attribution_verified_at, $6), updated_at = $6
			WHERE installation_id = $1::uuid`, write.InstallationID, write.CampaignID, write.ReferralCampaignID, write.ReferrerUserID,
			mobileAttributionNullableBytes(write.AttributionDigest), write.OccurredAt)
		if err != nil {
			return false, err
		}
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return id > 0, nil
}

func (r *MobileAttributionRepository) ResolveReference(ctx context.Context, campaignID *int64, referralCode string) (service.MobileAttributionReference, error) {
	result := service.MobileAttributionReference{CampaignID: campaignID}
	if campaignID != nil {
		var exists bool
		if err := r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM play_campaigns WHERE id = $1)`, *campaignID).Scan(&exists); err != nil {
			return result, err
		}
		if !exists {
			return result, service.ErrMobileAttributionInvalidToken
		}
	}
	if strings.TrimSpace(referralCode) != "" {
		var id int64
		if err := r.db.QueryRowContext(ctx, `SELECT user_id FROM user_affiliates WHERE aff_code = $1`, strings.TrimSpace(referralCode)).Scan(&id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return result, service.ErrMobileAttributionInvalidToken
			}
			return result, err
		}
		result.ReferrerUserID = &id
	}
	return result, nil
}

// ResolveReferralToken verifies the referral campaign token issued by the
// campaign service. Only the resolved IDs are returned to the attribution
// service; the raw token is never written to storage.
func (r *MobileAttributionRepository) ResolveReferralToken(ctx context.Context, token string, now time.Time) (service.MobileAttributionReference, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != "v1" {
		return service.MobileAttributionReference{}, service.ErrMobileAttributionInvalidToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(raw) > 4096 {
		return service.MobileAttributionReference{}, service.ErrMobileAttributionInvalidToken
	}
	var hint struct {
		CampaignID int64     `json:"campaign_id"`
		InviterID  int64     `json:"inviter_id"`
		Nonce      string    `json:"nonce"`
		ExpiresAt  time.Time `json:"expires_at"`
	}
	if json.Unmarshal(raw, &hint) != nil || hint.CampaignID <= 0 || hint.InviterID <= 0 || hint.Nonce == "" || !now.Before(hint.ExpiresAt) {
		return service.MobileAttributionReference{}, service.ErrMobileAttributionInvalidToken
	}
	var secret []byte
	if err := r.db.QueryRowContext(ctx, `SELECT signing_secret FROM referral_campaigns WHERE id = $1`, hint.CampaignID).Scan(&secret); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.MobileAttributionReference{}, service.ErrMobileAttributionInvalidToken
		}
		return service.MobileAttributionReference{}, err
	}
	if len(secret) < 16 {
		return service.MobileAttributionReference{}, service.ErrMobileAttributionInvalidToken
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[1]))
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		return service.MobileAttributionReference{}, service.ErrMobileAttributionInvalidToken
	}
	campaignID, referrerID := hint.CampaignID, hint.InviterID
	return service.MobileAttributionReference{ReferralCampaignID: &campaignID, ReferrerUserID: &referrerID}, nil
}

func (r *MobileAttributionRepository) ListInstallations(ctx context.Context, filter service.MobileAttributionAdminFilter) ([]service.MobileAttributionInstallationRow, int64, error) {
	eventType := filter.EventType
	filter.EventType = ""
	where, args := mobileAttributionWhere(filter, "i")
	if eventType != "" {
		args = append(args, eventType)
		where += ` AND EXISTS (SELECT 1 FROM mobile_attribution_events filtered_event WHERE filtered_event.installation_id=i.installation_id AND filtered_event.event_type=$` + fmt.Sprint(len(args)) + `)`
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mobile_installations i WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (filter.Page - 1) * filter.PageSize
	args = append(args, filter.PageSize, offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT i.installation_id::text, i.user_id, i.platform, i.app_version, i.campaign_id, i.referral_campaign_id, i.referrer_user_id,
		       i.first_seen_at, i.last_seen_at, COUNT(e.id)
		FROM mobile_installations i LEFT JOIN mobile_attribution_events e ON e.installation_id = i.installation_id
		WHERE `+where+` GROUP BY i.installation_id ORDER BY i.last_seen_at DESC, i.installation_id LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.MobileAttributionInstallationRow, 0)
	for rows.Next() {
		var item service.MobileAttributionInstallationRow
		if err := rows.Scan(&item.InstallationID, &item.UserID, &item.Platform, &item.AppVersion, &item.CampaignID, &item.ReferralCampaignID, &item.ReferrerUserID, &item.FirstSeenAt, &item.LastSeenAt, &item.EventCount); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *MobileAttributionRepository) Funnel(ctx context.Context, filter service.MobileAttributionAdminFilter) ([]service.MobileAttributionFunnelStep, error) {
	where, args := mobileAttributionWhere(filter, "e")
	rows, err := r.db.QueryContext(ctx, `
		WITH event_types(event_type, position) AS (VALUES ('download', 1), ('click', 2), ('open', 3), ('register', 4), ('login', 5), ('share', 6))
		SELECT t.event_type, COUNT(DISTINCT e.installation_id) FROM event_types t
		LEFT JOIN mobile_attribution_events e ON e.event_type = t.event_type AND `+where+`
		GROUP BY t.event_type, t.position ORDER BY t.position`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]service.MobileAttributionFunnelStep, 0, 6)
	for rows.Next() {
		var item service.MobileAttributionFunnelStep
		if err := rows.Scan(&item.EventType, &item.Installs); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func mobileAttributionWhere(filter service.MobileAttributionAdminFilter, alias string) (string, []any) {
	parts := []string{"1=1"}
	args := make([]any, 0, 5)
	if filter.EventType != "" {
		args = append(args, filter.EventType)
		parts = append(parts, alias+".event_type = $"+fmt.Sprint(len(args)))
	}
	if filter.Platform != "" {
		args = append(args, filter.Platform)
		parts = append(parts, alias+".platform = $"+fmt.Sprint(len(args)))
	}
	if filter.CampaignID != nil {
		args = append(args, *filter.CampaignID)
		parts = append(parts, alias+".campaign_id = $"+fmt.Sprint(len(args)))
	}
	if filter.UserID != nil {
		args = append(args, *filter.UserID)
		parts = append(parts, alias+".user_id = $"+fmt.Sprint(len(args)))
	}
	if filter.From != nil {
		args = append(args, *filter.From)
		column := "first_seen_at"
		if alias == "e" {
			column = "occurred_at"
		}
		parts = append(parts, alias+"."+column+" >= $"+fmt.Sprint(len(args)))
	}
	if filter.To != nil {
		args = append(args, *filter.To)
		column := "first_seen_at"
		if alias == "e" {
			column = "occurred_at"
		}
		parts = append(parts, alias+"."+column+" < $"+fmt.Sprint(len(args)))
	}
	return strings.Join(parts, " AND "), args
}

func mobileAttributionNullableBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}
func encodeAttributionMetadata(value map[string]string) string {
	if value == nil {
		return "{}"
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

var _ = time.Time{}
