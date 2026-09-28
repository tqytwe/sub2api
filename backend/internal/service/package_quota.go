package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

const (
	PackageEntitlementActive    = "active"
	PackageEntitlementExhausted = "exhausted"
	PackageEntitlementExpired   = "expired"

	PackageExhaustedByRequest = "request"
	PackageExhaustedByAmount  = "amount"
	PackageExhaustedByToken   = "token"

	PackageEntitlementSourcePayment    = "payment"
	PackageEntitlementSourceAdminGrant = "admin_grant"
)

// PackageEntitlement is the immutable quota snapshot created from a paid
// package order. Nil limits mean that dimension does not constrain the plan.
type PackageEntitlement struct {
	ID              int64
	PaymentOrderID  *int64
	SourceType      string
	GrantedBy       *int64
	PlanID          *int64
	UserID          int64
	GroupID         int64
	StartsAt        time.Time
	ExpiresAt       time.Time
	Status          string
	ExhaustedReason string
	RequestLimit    *int64
	RequestUsed     int64
	AmountLimitUSD  *float64
	AmountUsedUSD   float64
	TokenLimit      *int64
	TokenUsed       int64
}

type packageEntitlementKey struct {
	userID  int64
	groupID int64
}

func subscriptionPlanSnapshot(plan *dbent.SubscriptionPlan) map[string]any {
	if plan == nil {
		return nil
	}
	snapshot := map[string]any{
		"plan_id":       plan.ID,
		"group_id":      plan.GroupID,
		"validity_days": psComputeValidityDays(plan.ValidityDays, plan.ValidityUnit),
	}
	if plan.RequestLimit != nil {
		snapshot["request_limit"] = *plan.RequestLimit
	}
	if plan.AmountLimitUsd != nil {
		snapshot["amount_limit_usd"] = *plan.AmountLimitUsd
	}
	if plan.TokenLimit != nil {
		snapshot["token_limit"] = *plan.TokenLimit
	}
	return snapshot
}

// PackageQuotaState derives the authoritative state from time and counters.
// Persisted status is only an optimization and must never keep an expired or
// exhausted package usable.
func PackageQuotaState(e *PackageEntitlement, now time.Time) (status, reason string) {
	if e == nil || !e.ExpiresAt.After(now) {
		return PackageEntitlementExpired, ""
	}
	if e.RequestLimit != nil && e.RequestUsed >= *e.RequestLimit {
		return PackageEntitlementExhausted, PackageExhaustedByRequest
	}
	if e.AmountLimitUSD != nil && e.AmountUsedUSD >= *e.AmountLimitUSD {
		return PackageEntitlementExhausted, PackageExhaustedByAmount
	}
	if e.TokenLimit != nil && e.TokenUsed >= *e.TokenLimit {
		return PackageEntitlementExhausted, PackageExhaustedByToken
	}
	return PackageEntitlementActive, ""
}

func (e *PackageEntitlement) IsActiveAt(now time.Time) bool {
	status, _ := PackageQuotaState(e, now)
	return status == PackageEntitlementActive
}

// attachPackageEntitlements attaches one authoritative package snapshot to
// each subscription returned by a list endpoint. It uses one query for the page
// rather than querying package data once per row.
func (s *SubscriptionService) attachPackageEntitlements(ctx context.Context, subs []UserSubscription) error {
	if s == nil || s.entClient == nil || len(subs) == 0 {
		return nil
	}

	keys := make([]packageEntitlementKey, 0, len(subs))
	seen := make(map[packageEntitlementKey]struct{}, len(subs))
	for i := range subs {
		key := packageEntitlementKey{userID: subs[i].UserID, groupID: subs[i].GroupID}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}

	entitlements, err := s.packageEntitlementsForKeys(ctx, keys)
	if err != nil {
		return err
	}
	for i := range subs {
		subs[i].PackageEntitlement = entitlements[packageEntitlementKey{userID: subs[i].UserID, groupID: subs[i].GroupID}]
	}
	return nil
}

// packageEntitlementForAdmin returns the package currently relevant to an
// admin action. An exhausted package is intentionally returned so an explicit
// admin reset can restore only that package, never a later renewal.
func (s *SubscriptionService) packageEntitlementForAdmin(ctx context.Context, userID, groupID int64) (*PackageEntitlement, bool, error) {
	if s == nil || s.entClient == nil {
		return nil, false, nil
	}
	key := packageEntitlementKey{userID: userID, groupID: groupID}
	entitlements, err := s.packageEntitlementsForKeys(ctx, []packageEntitlementKey{key})
	if err != nil {
		return nil, false, err
	}
	entitlement, managed := entitlements[key]
	return entitlement, managed, nil
}

func (s *SubscriptionService) packageEntitlementsForKeys(ctx context.Context, keys []packageEntitlementKey) (map[packageEntitlementKey]*PackageEntitlement, error) {
	result := make(map[packageEntitlementKey]*PackageEntitlement, len(keys))
	if len(keys) == 0 {
		return result, nil
	}

	args := make([]any, 0, len(keys)*2)
	values := make([]string, 0, len(keys))
	for i, key := range keys {
		first := i*2 + 1
		values = append(values, fmt.Sprintf("($%d::bigint, $%d::bigint)", first, first+1))
		args = append(args, key.userID, key.groupID)
	}
	rows, err := s.packageQuotaRunner(ctx).QueryContext(ctx, fmt.Sprintf(`
		WITH selected_subscriptions (user_id, group_id) AS (VALUES %s)
		SELECT e.id, e.payment_order_id, e.source_type, e.granted_by,
		       NULLIF(e.plan_snapshot->>'plan_id', '')::bigint,
		       e.user_id, e.group_id, e.starts_at, e.expires_at,
		       e.status, COALESCE(e.exhausted_reason, ''), e.request_limit, e.request_used,
		       e.amount_limit_usd, e.amount_used_usd, e.token_limit, e.token_used
		FROM subscription_package_entitlements e
		JOIN selected_subscriptions s ON s.user_id = e.user_id AND s.group_id = e.group_id
		WHERE e.status <> 'revoked'
		ORDER BY e.user_id ASC, e.group_id ASC, e.expires_at ASC, e.id ASC`, strings.Join(values, ", ")), args...)
	if err != nil {
		return nil, fmt.Errorf("query package entitlements for subscriptions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	now := s.now()
	for rows.Next() {
		entitlement, err := scanPackageEntitlement(rows)
		if err != nil {
			return nil, err
		}
		status, reason := PackageQuotaState(entitlement, now)
		entitlement.Status = status
		entitlement.ExhaustedReason = reason
		key := packageEntitlementKey{userID: entitlement.UserID, groupID: entitlement.GroupID}
		if current, ok := result[key]; !ok || packageEntitlementDisplayPriority(entitlement.Status) < packageEntitlementDisplayPriority(current.Status) {
			result[key] = entitlement
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate package entitlements for subscriptions: %w", err)
	}
	return result, nil
}

func scanPackageEntitlement(scanner interface{ Scan(...any) error }) (*PackageEntitlement, error) {
	var entitlement PackageEntitlement
	var paymentOrderID, grantedBy, planID, requestLimit, tokenLimit sql.NullInt64
	var amountLimit sql.NullFloat64
	var exhaustedReason sql.NullString
	if err := scanner.Scan(&entitlement.ID, &paymentOrderID, &entitlement.SourceType, &grantedBy, &planID, &entitlement.UserID, &entitlement.GroupID,
		&entitlement.StartsAt, &entitlement.ExpiresAt, &entitlement.Status, &exhaustedReason,
		&requestLimit, &entitlement.RequestUsed, &amountLimit, &entitlement.AmountUsedUSD, &tokenLimit, &entitlement.TokenUsed); err != nil {
		return nil, fmt.Errorf("scan package entitlement: %w", err)
	}
	if paymentOrderID.Valid {
		value := paymentOrderID.Int64
		entitlement.PaymentOrderID = &value
	}
	if grantedBy.Valid {
		value := grantedBy.Int64
		entitlement.GrantedBy = &value
	}
	if planID.Valid {
		value := planID.Int64
		entitlement.PlanID = &value
	}
	if exhaustedReason.Valid {
		entitlement.ExhaustedReason = exhaustedReason.String
	}
	if requestLimit.Valid {
		value := requestLimit.Int64
		entitlement.RequestLimit = &value
	}
	if amountLimit.Valid {
		value := amountLimit.Float64
		entitlement.AmountLimitUSD = &value
	}
	if tokenLimit.Valid {
		value := tokenLimit.Int64
		entitlement.TokenLimit = &value
	}
	return &entitlement, nil
}

func packageEntitlementDisplayPriority(status string) int {
	switch status {
	case PackageEntitlementActive:
		return 0
	case PackageEntitlementExhausted:
		return 1
	default:
		return 2
	}
}

type packageQuotaRunner interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (s *SubscriptionService) packageQuotaRunner(ctx context.Context) packageQuotaRunner {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx
	}
	return s.entClient
}

// alignPackageEntitlementsToSubscriptionExpiry keeps the package chain within
// a manually adjusted subscription term without changing any quota counters.
func (s *SubscriptionService) alignPackageEntitlementsToSubscriptionExpiry(ctx context.Context, userID, groupID int64, newExpiresAt time.Time) error {
	if s == nil || s.entClient == nil {
		return nil
	}

	runner := s.packageQuotaRunner(ctx)
	rows, err := runner.QueryContext(ctx, `
		SELECT id, payment_order_id, source_type, granted_by,
		       NULLIF(plan_snapshot->>'plan_id', '')::bigint,
		       user_id, group_id, starts_at, expires_at,
		       status, COALESCE(exhausted_reason, ''), request_limit, request_used,
		       amount_limit_usd, amount_used_usd, token_limit, token_used
		FROM subscription_package_entitlements
		WHERE user_id = $1 AND group_id = $2 AND status <> 'revoked'
		ORDER BY expires_at ASC, id ASC
		FOR UPDATE`, userID, groupID)
	if err != nil {
		return fmt.Errorf("lock package entitlements for term adjustment: %w", err)
	}

	entitlements := make([]*PackageEntitlement, 0, 1)
	for rows.Next() {
		entitlement, scanErr := scanPackageEntitlement(rows)
		if scanErr != nil {
			_ = rows.Close()
			return scanErr
		}
		entitlements = append(entitlements, entitlement)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate package entitlements for term adjustment: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close package entitlement term rows: %w", err)
	}
	if len(entitlements) == 0 {
		return nil
	}

	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	terminalIndex := len(entitlements) - 1
	for index, entitlement := range entitlements {
		if index != terminalIndex && !entitlement.ExpiresAt.After(newExpiresAt) {
			continue
		}

		adjusted := *entitlement
		adjusted.ExpiresAt = newExpiresAt
		status, reason := PackageQuotaState(&adjusted, now)
		if entitlement.ExpiresAt.Equal(newExpiresAt) && entitlement.Status == status && entitlement.ExhaustedReason == reason {
			continue
		}

		result, err := runner.ExecContext(ctx, `
			UPDATE subscription_package_entitlements
			SET expires_at = $2, status = $3, exhausted_reason = NULLIF($4, ''), updated_at = NOW()
			WHERE id = $1 AND status <> 'revoked'`, entitlement.ID, newExpiresAt, status, reason)
		if err != nil {
			return fmt.Errorf("align package entitlement expiry: %w", err)
		}
		updated, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("inspect package entitlement expiry update: %w", err)
		}
		if updated != 1 {
			return fmt.Errorf("align package entitlement expiry: %w", sql.ErrNoRows)
		}
	}
	return nil
}

func (s *SubscriptionService) packageEntitlementForUserGroup(ctx context.Context, userID, groupID int64) (*PackageEntitlement, bool, error) {
	if s == nil || s.entClient == nil {
		return nil, false, nil
	}
	rows, err := s.packageQuotaRunner(ctx).QueryContext(ctx, `
		SELECT id, payment_order_id, source_type, granted_by,
		       NULLIF(plan_snapshot->>'plan_id', '')::bigint,
		       user_id, group_id, starts_at, expires_at,
		       status, COALESCE(exhausted_reason, ''), request_limit, request_used,
		       amount_limit_usd, amount_used_usd, token_limit, token_used
		FROM subscription_package_entitlements
		WHERE user_id = $1 AND group_id = $2 AND status <> 'revoked'
		ORDER BY expires_at ASC, id ASC`, userID, groupID)
	if err != nil {
		return nil, false, fmt.Errorf("query package entitlements: %w", err)
	}
	defer func() { _ = rows.Close() }()
	managed := false
	allExpired := true
	now := s.now()
	for rows.Next() {
		managed = true
		e, err := scanPackageEntitlement(rows)
		if err != nil {
			return nil, false, err
		}
		status, reason := PackageQuotaState(e, now)
		if status != PackageEntitlementExpired {
			allExpired = false
		}
		if status != e.Status || reason != e.ExhaustedReason {
			if _, err := s.packageQuotaRunner(ctx).ExecContext(ctx, `UPDATE subscription_package_entitlements SET status = $2, exhausted_reason = NULLIF($3, ''), updated_at = NOW() WHERE id = $1`, e.ID, status, reason); err != nil {
				return nil, false, fmt.Errorf("synchronize package entitlement state: %w", err)
			}
			e.Status, e.ExhaustedReason = status, reason
		}
		if status == PackageEntitlementActive {
			return e, true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate package entitlements: %w", err)
	}
	if managed && allExpired {
		return nil, true, ErrPackageEntitlementExpired
	}
	return nil, managed, nil
}

// CreatePackageEntitlementFromOrder is idempotent by payment_order_id and
// receives limits only from the immutable order snapshot.
func (s *SubscriptionService) CreatePackageEntitlementFromOrder(ctx context.Context, orderID, userID, groupID int64, startsAt time.Time, validityDays int, snapshot map[string]any) error {
	requestLimit, amountLimit, tokenLimit, ok := packageQuotaSnapshot(snapshot)
	if !ok {
		return nil
	}
	if validityDays <= 0 {
		return fmt.Errorf("invalid package entitlement validity days")
	}
	expiresAt, err := s.nextPackageEntitlementExpiry(ctx, userID, groupID, startsAt, validityDays)
	if err != nil {
		return err
	}
	planSnapshot, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal package entitlement snapshot: %w", err)
	}
	_, err = s.packageQuotaRunner(ctx).ExecContext(ctx, `
		INSERT INTO subscription_package_entitlements (
			payment_order_id, user_id, group_id, starts_at, expires_at,
			request_limit, amount_limit_usd, token_limit, plan_snapshot
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)
		ON CONFLICT (payment_order_id) DO NOTHING`, orderID, userID, groupID, startsAt,
		expiresAt, requestLimit, amountLimit, tokenLimit, string(planSnapshot))
	if err != nil {
		return fmt.Errorf("create package entitlement: %w", err)
	}
	return nil
}

// nextPackageEntitlementExpiry preserves the subscription extension contract
// for repeat purchases. The new quota can be used as soon as an older quota is
// exhausted, but it must not lose its own validity window by expiring alongside
// the older entitlement.
func (s *SubscriptionService) nextPackageEntitlementExpiry(ctx context.Context, userID, groupID int64, startsAt time.Time, validityDays int) (time.Time, error) {
	rows, err := s.packageQuotaRunner(ctx).QueryContext(ctx, `
		SELECT MAX(expires_at)
		FROM subscription_package_entitlements
		WHERE user_id = $1 AND group_id = $2 AND status <> 'revoked'`, userID, groupID)
	if err != nil {
		return time.Time{}, fmt.Errorf("query existing package entitlement expiry: %w", err)
	}
	defer func() { _ = rows.Close() }()

	base := startsAt
	if rows.Next() {
		var latest sql.NullTime
		if err := rows.Scan(&latest); err != nil {
			return time.Time{}, fmt.Errorf("scan existing package entitlement expiry: %w", err)
		}
		if latest.Valid && latest.Time.After(base) {
			base = latest.Time
		}
	}
	if err := rows.Err(); err != nil {
		return time.Time{}, fmt.Errorf("iterate existing package entitlement expiry: %w", err)
	}
	return packageEntitlementExpiry(base, validityDays), nil
}

func packageEntitlementExpiry(base time.Time, validityDays int) time.Time {
	expiresAt := base.AddDate(0, 0, validityDays)
	if expiresAt.After(MaxExpiresAt) {
		return MaxExpiresAt
	}
	return expiresAt
}

func packageQuotaSnapshot(snapshot map[string]any) (requestLimit *int64, amountLimit *float64, tokenLimit *int64, ok bool) {
	if snapshot == nil {
		return nil, nil, nil, false
	}
	if v, valid := snapshotInt64(snapshot["request_limit"]); valid && v > 0 {
		requestLimit = &v
	}
	if v, valid := snapshotFloat64(snapshot["amount_limit_usd"]); valid && v > 0 {
		amountLimit = &v
	}
	if v, valid := snapshotInt64(snapshot["token_limit"]); valid && v > 0 {
		tokenLimit = &v
	}
	return requestLimit, amountLimit, tokenLimit, requestLimit != nil || amountLimit != nil || tokenLimit != nil
}

func snapshotInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case *int64:
		if v == nil {
			return 0, false
		}
		return *v, true
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), v == float64(int64(v))
	case json.Number:
		n, err := v.Int64()
		return n, err == nil
	default:
		return 0, false
	}
}

func snapshotFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case *float64:
		if v == nil {
			return 0, false
		}
		return *v, true
	case float64:
		return v, true
	case int64:
		return float64(v), true
	case int:
		return float64(v), true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	default:
		return 0, false
	}
}
