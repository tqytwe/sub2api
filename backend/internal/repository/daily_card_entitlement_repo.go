package repository

import (
	"context"
	"database/sql"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionentitlement"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionplan"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type dailyCardEntitlementRepository struct {
	client *dbent.Client
}

func (r *dailyCardEntitlementRepository) ReserveRequest(ctx context.Context, input service.DailyCardRequestHoldInput) error {
	return r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		var ownerID int64
		var quotaLimit, quotaUsed, quotaReserved float64
		var status string
		var expiresAt sql.NullTime
		rows, err := client.QueryContext(txCtx, `
			SELECT user_id, quota_limit_usd, quota_used_usd, quota_reserved_usd, status, expires_at
			FROM subscription_entitlements
			WHERE id = $1
			FOR UPDATE
		`, input.EntitlementID)
		if err != nil {
			return err
		}
		if !rows.Next() {
			_ = rows.Close()
			return service.ErrDailyCardUnavailable
		}
		err = rows.Scan(&ownerID, &quotaLimit, &quotaUsed, &quotaReserved, &status, &expiresAt)
		_ = rows.Close()
		if err != nil {
			return err
		}
		if ownerID != input.UserID || status != service.DailyCardStatusActive || !expiresAt.Valid || !input.ReservedAt.Before(expiresAt.Time) {
			return service.ErrDailyCardUnavailable
		}

		var existingFingerprint, existingStatus string
		var existingExpiresAt time.Time
		existingRows, err := client.QueryContext(txCtx, `
			SELECT request_fingerprint, status, expires_at
			FROM subscription_entitlement_holds
			WHERE entitlement_id = $1 AND request_id = $2
			FOR UPDATE
		`, input.EntitlementID, input.RequestID)
		if err != nil {
			return err
		}
		if existingRows.Next() {
			err = existingRows.Scan(&existingFingerprint, &existingStatus, &existingExpiresAt)
			_ = existingRows.Close()
			if err != nil {
				return err
			}
			if existingFingerprint != input.RequestFingerprint {
				return service.ErrDailyCardRequestConflict
			}
			if existingStatus == "reserved" && input.ReservedAt.Before(existingExpiresAt) {
				return nil
			}
			return service.ErrDailyCardRequestConflict
		}
		_ = existingRows.Close()

		if _, err := client.ExecContext(txCtx, `
			UPDATE subscription_entitlement_holds
			SET status = 'released', released_at = $2, updated_at = $2
			WHERE entitlement_id = $1 AND status = 'reserved' AND expires_at <= $2
		`, input.EntitlementID, input.ReservedAt); err != nil {
			return err
		}

		if quotaReserved != 0 {
			quotaReserved = 0
			if _, err := client.ExecContext(txCtx, `
				UPDATE subscription_entitlements
				SET quota_reserved_usd = 0, updated_at = $2
				WHERE id = $1
			`, input.EntitlementID, input.ReservedAt); err != nil {
				return err
			}
		}

		if quotaLimit-quotaUsed <= 0.0000000001 {
			return service.ErrDailyCardUnavailable
		}
		holdExpiresAt := expiresAt.Time
		if _, err := client.ExecContext(txCtx, `
			INSERT INTO subscription_entitlement_holds (
				entitlement_id, request_id, request_fingerprint, reserved_usd,
				captured_usd, status, expires_at, created_at, updated_at
			) VALUES ($1, $2, $3, $4, 0, 'reserved', $5, $6, $6)
		`, input.EntitlementID, input.RequestID, input.RequestFingerprint, 0, holdExpiresAt, input.ReservedAt); err != nil {
			return err
		}
		return nil
	})
}

func (r *dailyCardEntitlementRepository) ReleaseRequest(ctx context.Context, entitlementID, userID int64, requestID string, releasedAt time.Time) error {
	return r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		var ownerID int64
		var quotaReserved float64
		entitlementRows, err := client.QueryContext(txCtx, `
			SELECT user_id, quota_reserved_usd
			FROM subscription_entitlements
			WHERE id = $1
			FOR UPDATE
		`, entitlementID)
		if err != nil {
			return err
		}
		if !entitlementRows.Next() {
			_ = entitlementRows.Close()
			return service.ErrDailyCardUnavailable
		}
		err = entitlementRows.Scan(&ownerID, &quotaReserved)
		_ = entitlementRows.Close()
		if err != nil {
			return err
		}
		if ownerID != userID {
			return service.ErrDailyCardUnavailable
		}

		var reservedUSD float64
		var holdStatus string
		holdRows, err := client.QueryContext(txCtx, `
			SELECT reserved_usd, status
			FROM subscription_entitlement_holds
			WHERE entitlement_id = $1 AND request_id = $2
			FOR UPDATE
		`, entitlementID, requestID)
		if err != nil {
			return err
		}
		if !holdRows.Next() {
			_ = holdRows.Close()
			return nil
		}
		err = holdRows.Scan(&reservedUSD, &holdStatus)
		_ = holdRows.Close()
		if err != nil {
			return err
		}
		if holdStatus != "reserved" {
			return nil
		}
		if _, err := client.ExecContext(txCtx, `
			UPDATE subscription_entitlement_holds
			SET status = 'released', released_at = $3, updated_at = $3
			WHERE entitlement_id = $1 AND request_id = $2 AND status = 'reserved'
		`, entitlementID, requestID, releasedAt); err != nil {
			return err
		}
		remainingReserved := quotaReserved - reservedUSD
		if remainingReserved < 0 {
			remainingReserved = 0
		}
		_, err = client.ExecContext(txCtx, `
			UPDATE subscription_entitlements
			SET quota_reserved_usd = $2, updated_at = $3
			WHERE id = $1
		`, entitlementID, remainingReserved, releasedAt)
		return err
	})
}

func NewDailyCardEntitlementRepository(client *dbent.Client) service.DailyCardEntitlementRepository {
	return &dailyCardEntitlementRepository{client: client}
}

func (r *dailyCardEntitlementRepository) IsOneTimeGroup(ctx context.Context, groupID int64) (bool, error) {
	return clientFromContext(ctx, r.client).SubscriptionPlan.Query().
		Where(
			subscriptionplan.GroupIDEQ(groupID),
			subscriptionplan.QuotaModeEQ(service.DailyCardQuotaModeOneTime),
		).
		Exist(ctx)
}

func (r *dailyCardEntitlementRepository) IssuePaidCard(ctx context.Context, input service.IssueDailyCardInput) (*service.DailyCardEntitlement, bool, error) {
	var issued *service.DailyCardEntitlement
	created := false
	err := r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		if err := lockDailyCardQueue(txCtx, client, input.UserID, input.GroupID); err != nil {
			return err
		}
		existing, err := client.SubscriptionEntitlement.Query().
			Where(subscriptionentitlement.PaymentOrderIDEQ(input.PaymentOrderID)).
			Only(txCtx)
		if err == nil {
			issued = dailyCardEntitlementFromEntity(existing)
			return nil
		}
		if !dbent.IsNotFound(err) {
			return err
		}

		if _, err := reconcileDailyCardQueue(txCtx, client, input.UserID, input.GroupID, input.IssuedAt); err != nil {
			return err
		}
		_, err = client.SubscriptionEntitlement.Query().
			Where(
				subscriptionentitlement.UserIDEQ(input.UserID),
				subscriptionentitlement.GroupIDEQ(input.GroupID),
				subscriptionentitlement.StatusEQ(service.DailyCardStatusActive),
			).
			Only(txCtx)
		status := service.DailyCardStatusPending
		if dbent.IsNotFound(err) {
			status = service.DailyCardStatusActive
		} else if err != nil {
			return err
		}

		builder := client.SubscriptionEntitlement.Create().
			SetUserID(input.UserID).
			SetGroupID(input.GroupID).
			SetPlanID(input.PlanID).
			SetPaymentOrderID(input.PaymentOrderID).
			SetQuotaMode(service.DailyCardQuotaModeOneTime).
			SetQuotaLimitUsd(input.QuotaLimitUSD).
			SetDurationHours(input.DurationHours).
			SetStatus(status).
			SetCreatedAt(input.IssuedAt).
			SetUpdatedAt(input.IssuedAt)
		if status == service.DailyCardStatusActive {
			expiresAt := input.IssuedAt.Add(time.Duration(input.DurationHours) * time.Hour)
			builder.SetStartsAt(input.IssuedAt).
				SetActivatedAt(input.IssuedAt).
				SetExpiresAt(expiresAt)
		}
		entity, err := builder.Save(txCtx)
		if err != nil {
			return err
		}
		issued = dailyCardEntitlementFromEntity(entity)
		created = true
		return nil
	})
	return issued, created, err
}

func (r *dailyCardEntitlementRepository) GetActive(ctx context.Context, userID, groupID int64) (*service.DailyCardEntitlement, error) {
	entity, err := clientFromContext(ctx, r.client).SubscriptionEntitlement.Query().
		Where(
			subscriptionentitlement.UserIDEQ(userID),
			subscriptionentitlement.GroupIDEQ(groupID),
			subscriptionentitlement.StatusEQ(service.DailyCardStatusActive),
		).
		Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, service.ErrDailyCardEntitlementNotFound
	}
	if err != nil {
		return nil, err
	}
	return dailyCardEntitlementFromEntity(entity), nil
}

func (r *dailyCardEntitlementRepository) GetByPaymentOrder(ctx context.Context, paymentOrderID int64) (*service.DailyCardEntitlement, error) {
	entity, err := clientFromContext(ctx, r.client).SubscriptionEntitlement.Query().
		Where(subscriptionentitlement.PaymentOrderIDEQ(paymentOrderID)).
		Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, service.ErrDailyCardEntitlementNotFound
	}
	if err != nil {
		return nil, err
	}
	return dailyCardEntitlementFromEntity(entity), nil
}

func (r *dailyCardEntitlementRepository) ReconcileAndGetActive(ctx context.Context, userID, groupID int64, now time.Time) (*service.DailyCardEntitlement, error) {
	var active *service.DailyCardEntitlement
	err := r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		if err := lockDailyCardQueue(txCtx, client, userID, groupID); err != nil {
			return err
		}
		entity, err := reconcileDailyCardQueue(txCtx, client, userID, groupID, now)
		if err != nil {
			return err
		}
		if entity != nil {
			active = dailyCardEntitlementFromEntity(entity)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if active == nil {
		return nil, service.ErrDailyCardEntitlementNotFound
	}
	return active, nil
}

func (r *dailyCardEntitlementRepository) ListByUser(ctx context.Context, userID int64) ([]service.DailyCardEntitlement, error) {
	entities, err := clientFromContext(ctx, r.client).SubscriptionEntitlement.Query().
		Where(subscriptionentitlement.UserIDEQ(userID)).
		Order(dbent.Asc(subscriptionentitlement.FieldCreatedAt), dbent.Asc(subscriptionentitlement.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]service.DailyCardEntitlement, 0, len(entities))
	for _, entity := range entities {
		result = append(result, *dailyCardEntitlementFromEntity(entity))
	}
	return result, nil
}

func (r *dailyCardEntitlementRepository) HasRecurringOrderAfter(ctx context.Context, userID, groupID int64, after time.Time) (bool, error) {
	var exists bool
	rows, err := clientFromContext(ctx, r.client).QueryContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM payment_orders
			WHERE user_id = $1
			  AND subscription_group_id = $2
			  AND order_type = 'subscription'
			  AND status = 'COMPLETED'
			  AND COALESCE(paid_at, completed_at, created_at) > $3
			  AND LOWER(COALESCE(NULLIF(subscription_snapshot->>'quota_mode', ''), 'recurring')) = 'recurring'
		)
	`, userID, groupID, after)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return false, rows.Err()
	}
	if scanErr := rows.Scan(&exists); scanErr != nil {
		return false, scanErr
	}
	return exists, nil
}

func (r *dailyCardEntitlementRepository) withTx(ctx context.Context, fn func(context.Context, *dbent.Client) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func lockDailyCardQueue(ctx context.Context, client *dbent.Client, userID, groupID int64) error {
	_, err := client.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1, $2)", userID, groupID)
	return err
}

func reconcileDailyCardQueue(ctx context.Context, client *dbent.Client, userID, groupID int64, now time.Time) (*dbent.SubscriptionEntitlement, error) {
	activationAt := now
	for {
		active, err := client.SubscriptionEntitlement.Query().
			Where(
				subscriptionentitlement.UserIDEQ(userID),
				subscriptionentitlement.GroupIDEQ(groupID),
				subscriptionentitlement.StatusEQ(service.DailyCardStatusActive),
			).
			Only(ctx)
		if err == nil {
			if active.ExpiresAt == nil || now.Before(*active.ExpiresAt) {
				return active, nil
			}
			activationAt = *active.ExpiresAt
			if _, err := active.Update().
				SetStatus(service.DailyCardStatusExpired).
				SetEndedAt(activationAt).
				SetUpdatedAt(now).
				Save(ctx); err != nil {
				return nil, err
			}
		} else if !dbent.IsNotFound(err) {
			return nil, err
		}

		pending, err := client.SubscriptionEntitlement.Query().
			Where(
				subscriptionentitlement.UserIDEQ(userID),
				subscriptionentitlement.GroupIDEQ(groupID),
				subscriptionentitlement.StatusEQ(service.DailyCardStatusPending),
			).
			Order(dbent.Asc(subscriptionentitlement.FieldCreatedAt), dbent.Asc(subscriptionentitlement.FieldID)).
			First(ctx)
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		expiresAt := activationAt.Add(time.Duration(pending.DurationHours) * time.Hour)
		activated, err := pending.Update().
			SetStatus(service.DailyCardStatusActive).
			SetStartsAt(activationAt).
			SetActivatedAt(activationAt).
			SetExpiresAt(expiresAt).
			SetUpdatedAt(now).
			Save(ctx)
		if err != nil {
			return nil, err
		}
		if now.Before(expiresAt) {
			return activated, nil
		}
	}
}

func dailyCardEntitlementFromEntity(entity *dbent.SubscriptionEntitlement) *service.DailyCardEntitlement {
	if entity == nil {
		return nil
	}
	return &service.DailyCardEntitlement{
		ID: entity.ID, UserID: entity.UserID, GroupID: entity.GroupID,
		PlanID: entity.PlanID, PaymentOrderID: entity.PaymentOrderID,
		QuotaMode: entity.QuotaMode, QuotaLimitUSD: entity.QuotaLimitUsd,
		QuotaUsedUSD: entity.QuotaUsedUsd, QuotaReservedUSD: entity.QuotaReservedUsd,
		DurationHours: entity.DurationHours, Status: entity.Status,
		StartsAt: entity.StartsAt, ExpiresAt: entity.ExpiresAt, ActivatedAt: entity.ActivatedAt,
		ExhaustedAt: entity.ExhaustedAt, EndedAt: entity.EndedAt,
		CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

var _ service.DailyCardEntitlementRepository = (*dailyCardEntitlementRepository)(nil)
