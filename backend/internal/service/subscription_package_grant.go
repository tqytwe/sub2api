package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrPackagePlanNotFound   = infraerrors.NotFound("PACKAGE_PLAN_NOT_FOUND", "subscription package plan not found")
	ErrPackagePlanHasNoQuota = infraerrors.BadRequest("PACKAGE_PLAN_HAS_NO_QUOTA", "subscription plan has no package quota")
	ErrPackageGrantInvalid   = infraerrors.BadRequest("PACKAGE_GRANT_INVALID", "invalid package grant request")
	ErrPackageGrantConflict  = infraerrors.Conflict("PACKAGE_GRANT_CONFLICT", "Idempotency-Key was already used for a different package grant")
)

type PackagePlanGrantInput struct {
	UserID         int64
	PlanID         int64
	GrantedBy      int64
	Notes          string
	IdempotencyKey string
}

type BulkPackagePlanGrantInput struct {
	UserIDs        []int64
	PlanID         int64
	GrantedBy      int64
	Notes          string
	IdempotencyKey string
}

type BulkPackagePlanGrantResult struct {
	SuccessCount  int
	CreatedCount  int
	ReusedCount   int
	FailedCount   int
	Subscriptions []UserSubscription
	Errors        []string
	Statuses      map[int64]string
}

func (s *SubscriptionService) GrantPackagePlan(ctx context.Context, input *PackagePlanGrantInput) (*UserSubscription, bool, error) {
	plan, err := s.loadPackagePlan(ctx, input)
	if err != nil {
		return nil, false, err
	}
	return s.grantPackagePlan(ctx, input, plan)
}

func (s *SubscriptionService) BulkGrantPackagePlan(ctx context.Context, input *BulkPackagePlanGrantInput) (*BulkPackagePlanGrantResult, error) {
	if input == nil || len(input.UserIDs) == 0 || len(input.UserIDs) > 100 || input.PlanID <= 0 || input.GrantedBy <= 0 || strings.TrimSpace(input.IdempotencyKey) == "" {
		return nil, ErrPackageGrantInvalid
	}
	plan, err := s.loadPackagePlan(ctx, &PackagePlanGrantInput{PlanID: input.PlanID, UserID: input.UserIDs[0], GrantedBy: input.GrantedBy, IdempotencyKey: input.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	result := &BulkPackagePlanGrantResult{
		Subscriptions: make([]UserSubscription, 0, len(input.UserIDs)),
		Errors:        make([]string, 0),
		Statuses:      make(map[int64]string, len(input.UserIDs)),
	}
	seen := make(map[int64]struct{}, len(input.UserIDs))
	for _, userID := range input.UserIDs {
		if userID <= 0 {
			result.FailedCount++
			result.Errors = append(result.Errors, fmt.Sprintf("user %d: invalid user ID", userID))
			result.Statuses[userID] = "failed"
			continue
		}
		if _, duplicate := seen[userID]; duplicate {
			continue
		}
		seen[userID] = struct{}{}
		subscription, replayed, grantErr := s.grantPackagePlan(ctx, &PackagePlanGrantInput{
			UserID: userID, PlanID: input.PlanID, GrantedBy: input.GrantedBy,
			Notes: input.Notes, IdempotencyKey: input.IdempotencyKey,
		}, plan)
		if grantErr != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, fmt.Sprintf("user %d: %v", userID, grantErr))
			result.Statuses[userID] = "failed"
			continue
		}
		result.SuccessCount++
		result.Subscriptions = append(result.Subscriptions, *subscription)
		if replayed {
			result.ReusedCount++
			result.Statuses[userID] = "reused"
		} else {
			result.CreatedCount++
			result.Statuses[userID] = "created"
		}
	}
	return result, nil
}

func (s *SubscriptionService) loadPackagePlan(ctx context.Context, input *PackagePlanGrantInput) (*dbent.SubscriptionPlan, error) {
	if input == nil || input.UserID <= 0 || input.PlanID <= 0 || input.GrantedBy <= 0 || strings.TrimSpace(input.IdempotencyKey) == "" {
		return nil, ErrPackageGrantInvalid
	}
	if s == nil || s.entClient == nil {
		return nil, infraerrors.ServiceUnavailable("PACKAGE_PLAN_SERVICE_UNAVAILABLE", "subscription package plan service is unavailable")
	}
	plan, err := s.entClient.SubscriptionPlan.Get(ctx, input.PlanID)
	if dbent.IsNotFound(err) {
		return nil, ErrPackagePlanNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load subscription package plan: %w", err)
	}
	if _, _, _, ok := packageQuotaSnapshot(subscriptionPlanSnapshot(plan)); !ok {
		return nil, ErrPackagePlanHasNoQuota
	}
	return plan, nil
}

func (s *SubscriptionService) grantPackagePlan(ctx context.Context, input *PackagePlanGrantInput, plan *dbent.SubscriptionPlan) (*UserSubscription, bool, error) {
	if input == nil || plan == nil || input.UserID <= 0 || input.PlanID <= 0 || input.GrantedBy <= 0 || strings.TrimSpace(input.IdempotencyKey) == "" || plan.ID != input.PlanID {
		return nil, false, ErrPackageGrantInvalid
	}
	snapshot := subscriptionPlanSnapshot(plan)
	if _, _, _, ok := packageQuotaSnapshot(snapshot); !ok {
		return nil, false, ErrPackagePlanHasNoQuota
	}
	validityDays := psComputeValidityDays(plan.ValidityDays, plan.ValidityUnit)
	if validityDays <= 0 || validityDays > MaxValidityDays {
		return nil, false, ErrPackageGrantInvalid
	}
	grantKey := packageGrantKey(input.GrantedBy, input.IdempotencyKey, input.UserID)
	var subscription *UserSubscription
	var replayed bool
	err := s.withSubscriptionUpdateTx(ctx, func(txCtx context.Context) error {
		existing, findErr := s.packageEntitlementByGrantKey(txCtx, grantKey)
		if findErr != nil {
			return findErr
		}
		if existing != nil {
			if !packageGrantMatches(existing, input, plan) {
				return ErrPackageGrantConflict
			}
			subscription, findErr = s.userSubRepo.GetByUserIDAndGroupID(txCtx, input.UserID, plan.GroupID)
			if findErr != nil {
				return findErr
			}
			subscription.PackageEntitlement = existing
			replayed = true
			return nil
		}

		subscription, _, findErr = s.assignOrExtendSubscription(txCtx, &AssignSubscriptionInput{
			UserID: input.UserID, GroupID: plan.GroupID, ValidityDays: validityDays,
			AssignedBy: input.GrantedBy, Notes: input.Notes,
		}, true)
		if findErr != nil {
			return findErr
		}
		entitlement, createErr := s.createAdminPackageEntitlement(txCtx, input, plan, grantKey, snapshot)
		if createErr != nil {
			return createErr
		}
		subscription.PackageEntitlement = entitlement
		return nil
	})
	if err != nil {
		existing, recoveryErr := s.packageEntitlementByGrantKey(ctx, grantKey)
		if recoveryErr == nil && existing != nil && packageGrantMatches(existing, input, plan) {
			subscription, recoveryErr = s.userSubRepo.GetByUserIDAndGroupID(ctx, input.UserID, plan.GroupID)
			if recoveryErr == nil {
				subscription.PackageEntitlement = existing
				return subscription, true, nil
			}
		}
		return nil, false, err
	}
	if !replayed {
		s.maybeInvalidateAssignmentCaches(input.UserID, plan.GroupID, false)
	}
	return subscription, replayed, nil
}

func packageGrantKey(grantedBy int64, idempotencyKey string, userID int64) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d:%s:%d", grantedBy, strings.TrimSpace(idempotencyKey), userID)))
	return "admin:" + hex.EncodeToString(hash[:])
}

func packageGrantMatches(entitlement *PackageEntitlement, input *PackagePlanGrantInput, plan *dbent.SubscriptionPlan) bool {
	return entitlement != nil && entitlement.SourceType == PackageEntitlementSourceAdminGrant &&
		entitlement.UserID == input.UserID && entitlement.GroupID == plan.GroupID &&
		entitlement.GrantedBy != nil && *entitlement.GrantedBy == input.GrantedBy &&
		entitlement.PlanID != nil && *entitlement.PlanID == plan.ID
}

func (s *SubscriptionService) packageEntitlementByGrantKey(ctx context.Context, grantKey string) (*PackageEntitlement, error) {
	rows, err := s.packageQuotaRunner(ctx).QueryContext(ctx, `
		SELECT id, payment_order_id, source_type, granted_by,
		       NULLIF(plan_snapshot->>'plan_id', '')::bigint,
		       user_id, group_id, starts_at, expires_at, status, COALESCE(exhausted_reason, ''),
		       request_limit, request_used, amount_limit_usd, amount_used_usd, token_limit, token_used
		FROM subscription_package_entitlements
		WHERE grant_key = $1`, grantKey)
	if err != nil {
		return nil, fmt.Errorf("query package grant: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate package grant: %w", err)
		}
		return nil, nil
	}
	entitlement, err := scanPackageEntitlement(rows)
	if err != nil {
		return nil, err
	}
	status, reason := PackageQuotaState(entitlement, s.now())
	entitlement.Status = status
	entitlement.ExhaustedReason = reason
	return entitlement, nil
}

func (s *SubscriptionService) createAdminPackageEntitlement(ctx context.Context, input *PackagePlanGrantInput, plan *dbent.SubscriptionPlan, grantKey string, snapshot map[string]any) (*PackageEntitlement, error) {
	requestLimit, amountLimit, tokenLimit, ok := packageQuotaSnapshot(snapshot)
	if !ok {
		return nil, ErrPackagePlanHasNoQuota
	}
	startsAt := s.now()
	expiresAt, err := s.nextPackageEntitlementExpiry(ctx, input.UserID, plan.GroupID, startsAt, psComputeValidityDays(plan.ValidityDays, plan.ValidityUnit))
	if err != nil {
		return nil, err
	}
	planSnapshot, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal package grant snapshot: %w", err)
	}
	_, err = s.packageQuotaRunner(ctx).ExecContext(ctx, `
		INSERT INTO subscription_package_entitlements (
			user_id, group_id, starts_at, expires_at, request_limit, amount_limit_usd,
			token_limit, plan_snapshot, source_type, granted_by, grant_key
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, 'admin_grant', $9, $10)`,
		input.UserID, plan.GroupID, startsAt, expiresAt, requestLimit, amountLimit, tokenLimit, string(planSnapshot), input.GrantedBy, grantKey)
	if err != nil {
		return nil, fmt.Errorf("create admin package entitlement: %w", err)
	}
	entitlement, err := s.packageEntitlementByGrantKey(ctx, grantKey)
	if err != nil {
		return nil, err
	}
	if entitlement == nil {
		return nil, sql.ErrNoRows
	}
	return entitlement, nil
}
