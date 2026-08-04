package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// AdmitRequest is the durable replay gate. It does not reserve quota: cost is
// settled only after upstream usage is known.
func (r *dailyCardEntitlementRepository) AdmitRequest(ctx context.Context, input service.DailyCardRequestAdmissionInput) error {
	return r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		var ownerID, groupID int64
		var quotaLimit, quotaUsed float64
		var status string
		var expiresAt sql.NullTime
		entitlementRows, err := client.QueryContext(txCtx, `
			SELECT user_id, group_id, quota_limit_usd, quota_used_usd, status, expires_at
			FROM subscription_entitlements WHERE id = $1 FOR UPDATE
		`, input.EntitlementID)
		if err != nil {
			return err
		}
		if !entitlementRows.Next() {
			_ = entitlementRows.Close()
			return service.ErrDailyCardUnavailable
		}
		err = entitlementRows.Scan(&ownerID, &groupID, &quotaLimit, &quotaUsed, &status, &expiresAt)
		_ = entitlementRows.Close()
		if err != nil {
			return err
		}
		if ownerID != input.UserID || status != service.DailyCardStatusActive || !expiresAt.Valid || !input.AdmittedAt.Before(expiresAt.Time) || quotaLimit-quotaUsed <= 0.0000000001 {
			return service.ErrDailyCardUnavailable
		}

		var state string
		replayRows, err := client.QueryContext(txCtx, `
			SELECT state FROM daily_card_request_replays
			WHERE user_id = $1 AND group_id = $2 AND client_request_id = $3 FOR UPDATE
		`, input.UserID, groupID, input.ClientRequestID)
		if err != nil {
			return err
		}
		if replayRows.Next() {
			err = replayRows.Scan(&state)
			_ = replayRows.Close()
			if err != nil {
				return err
			}
			switch state {
			case "completed":
				return service.ErrDailyCardDuplicateRequest
			case "retryable":
				_, err = client.ExecContext(txCtx, `
					UPDATE daily_card_request_replays
					SET entitlement_id = $4, settlement_request_id = $5, request_fingerprint = $6, request_path = $7,
					    state = 'forwarding', upstream_account_id = NULL, upstream_request_id = NULL,
					    dispatched_at = $8, completed_at = NULL, updated_at = $8
					WHERE user_id = $1 AND group_id = $2 AND client_request_id = $3
				`, input.UserID, groupID, input.ClientRequestID, input.EntitlementID, input.SettlementRequestID, input.RequestFingerprint, input.RequestPath, input.AdmittedAt)
				return err
			default:
				return service.ErrDailyCardRequestPendingConfirmation
			}
		}
		_ = replayRows.Close()
		_, err = client.ExecContext(txCtx, `
			INSERT INTO daily_card_request_replays (
				entitlement_id, user_id, group_id, client_request_id, settlement_request_id, request_fingerprint,
				request_path, state, dispatched_at, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, 'forwarding', $8, $8, $8)
		`, input.EntitlementID, input.UserID, groupID, input.ClientRequestID, input.SettlementRequestID, input.RequestFingerprint, input.RequestPath, input.AdmittedAt)
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "unique") {
			return service.ErrDailyCardRequestPendingConfirmation
		}
		return err
	})
}

func (r *dailyCardEntitlementRepository) MarkRequestRetryable(ctx context.Context, entitlementID int64, settlementRequestID string, updatedAt time.Time) error {
	_, err := r.client.ExecContext(ctx, `
		UPDATE daily_card_request_replays
		SET state = 'retryable', updated_at = $3
		WHERE entitlement_id = $1 AND settlement_request_id = $2 AND state = 'forwarding'
	`, entitlementID, settlementRequestID, updatedAt)
	return err
}

func (r *dailyCardEntitlementRepository) GetRequestReplay(ctx context.Context, entitlementID int64, clientRequestID string) (*service.DailyCardRequestReplay, error) {
	rows, err := r.client.QueryContext(ctx, `
		SELECT entitlement_id, client_request_id, settlement_request_id, state, request_path,
		       dispatched_at, completed_at, reconciled_at, reconciled_by, reconciliation_evidence
		FROM daily_card_request_replays WHERE entitlement_id = $1 AND client_request_id = $2
	`, entitlementID, clientRequestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, service.ErrDailyCardEntitlementNotFound
	}
	var out service.DailyCardRequestReplay
	if err := rows.Scan(&out.EntitlementID, &out.ClientRequestID, &out.SettlementRequestID, &out.State, &out.RequestPath, &out.DispatchedAt, &out.CompletedAt, &out.ReconciledAt, &out.ReconciledBy, &out.Evidence); err != nil {
		return nil, err
	}
	return &out, rows.Err()
}

func (r *dailyCardEntitlementRepository) ReconcileRequest(ctx context.Context, input service.DailyCardRequestReconciliationInput) (*service.DailyCardRequestReplay, error) {
	if input.Action != "mark_completed" && input.Action != "allow_retry" {
		return nil, service.ErrDailyCardInvalidInput
	}
	err := r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		rows, err := client.QueryContext(txCtx, `
			SELECT settlement_request_id, state FROM daily_card_request_replays
			WHERE entitlement_id = $1 AND client_request_id = $2 FOR UPDATE
		`, input.EntitlementID, input.ClientRequestID)
		if err != nil {
			return err
		}
		if !rows.Next() {
			_ = rows.Close()
			return service.ErrDailyCardEntitlementNotFound
		}
		var settlementRequestID, state string
		err = rows.Scan(&settlementRequestID, &state)
		_ = rows.Close()
		if err != nil {
			return err
		}
		if state != "pending_confirmation" {
			return service.ErrDailyCardRequestPendingConfirmation
		}
		if input.Action == "mark_completed" {
			evidenceRows, err := client.QueryContext(txCtx, `SELECT 1 FROM usage_billing_dedup WHERE request_id = $1 LIMIT 1`, settlementRequestID)
			if err != nil {
				return err
			}
			hasEvidence := evidenceRows.Next()
			_ = evidenceRows.Close()
			if !hasEvidence {
				return service.ErrDailyCardRequestPendingConfirmation
			}
		}
		state = "completed"
		if input.Action == "allow_retry" {
			state = "retryable"
		}
		_, err = client.ExecContext(txCtx, `
			UPDATE daily_card_request_replays
			SET state = $3, reconciled_at = $4, reconciled_by = $5, reconciliation_evidence = $6, updated_at = $4
			WHERE entitlement_id = $1 AND client_request_id = $2
		`, input.EntitlementID, input.ClientRequestID, state, input.ReconciledAt, input.ActorID, input.Evidence)
		return err
	})
	if err != nil {
		return nil, err
	}
	return r.GetRequestReplay(ctx, input.EntitlementID, input.ClientRequestID)
}
