package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const couponTemplateColumns = `
	id, template_key, version, name, description, status,
	benefit_type, benefit_value, max_discount_amount, currency,
	applicable_scopes, minimum_order_amount, eligible_plan_ids,
	validity_mode, validity_days, fixed_expires_at, valid_from,
	total_issue_limit, issued_count, rules, created_by, updated_by,
	created_at, updated_at`

const couponPoolColumns = `
	id, activity, version, status, coupon_weight_bp, redeem_code_weight_bp, balance_weight_bp, reward_config,
	fallback_template_id, created_by, updated_by, published_at, created_at, updated_at`

type couponRepository struct {
	db *sql.DB
}

func NewCouponRepository(db *sql.DB) service.CouponRepository {
	return &couponRepository{db: db}
}

func (r *couponRepository) exec(ctx context.Context) (sqlQueryExecutor, error) {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		if executor := sqlExecutorFromEntClient(tx.Client()); executor != nil {
			return executor, nil
		}
	}
	if r == nil || r.db == nil {
		return nil, service.ErrCouponRepositoryUnavailable
	}
	return r.db, nil
}

func (r *couponRepository) withTx(ctx context.Context, fn func(sqlQueryExecutor) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		if executor := sqlExecutorFromEntClient(tx.Client()); executor != nil {
			return fn(executor)
		}
	}
	if r == nil || r.db == nil {
		return service.ErrCouponRepositoryUnavailable
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin coupon transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit coupon transaction: %w", err)
	}
	return nil
}

func (r *couponRepository) CreateCouponTemplate(ctx context.Context, template service.CouponTemplate) (*service.CouponTemplate, error) {
	exec, err := r.exec(ctx)
	if err != nil {
		return nil, err
	}
	scopes, planIDs, rules, err := marshalCouponTemplateJSON(template)
	if err != nil {
		return nil, err
	}
	var saved couponTemplateRow
	err = scanSingleRow(ctx, exec, `
		INSERT INTO coupon_templates (
			template_key, version, name, description, status,
			benefit_type, benefit_value, max_discount_amount, currency,
			applicable_scopes, minimum_order_amount, eligible_plan_ids,
			validity_mode, validity_days, fixed_expires_at, valid_from,
			total_issue_limit, rules, created_by, updated_by
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20
		) RETURNING `+couponTemplateColumns,
		[]any{
			template.Key, template.Version, template.Name, template.Description, template.Status,
			template.BenefitType, template.BenefitValue, template.MaxDiscountAmount, template.Currency,
			scopes, template.MinimumOrderAmount, planIDs,
			template.ValidityMode, template.ValidityDays, template.FixedExpiresAt, template.ValidFrom,
			template.TotalIssueLimit, rules, template.CreatedBy, template.UpdatedBy,
		}, saved.scanDest()...)
	if err != nil {
		if isUniqueConstraintViolation(err) {
			return nil, infraerrors.Conflict("COUPON_TEMPLATE_KEY_EXISTS", "coupon template key already exists")
		}
		return nil, fmt.Errorf("create coupon template: %w", err)
	}
	return saved.value()
}

func (r *couponRepository) GetCouponTemplate(ctx context.Context, id int64) (*service.CouponTemplate, error) {
	exec, err := r.exec(ctx)
	if err != nil {
		return nil, err
	}
	return getCouponTemplate(ctx, exec, id, false)
}

func getCouponTemplate(ctx context.Context, exec sqlQueryExecutor, id int64, forUpdate bool) (*service.CouponTemplate, error) {
	query := `SELECT ` + couponTemplateColumns + ` FROM coupon_templates WHERE id = $1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var template couponTemplateRow
	err := scanSingleRow(ctx, exec, query, []any{id}, template.scanDest()...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get coupon template: %w", err)
	}
	return template.value()
}

func (r *couponRepository) UpdateCouponTemplate(ctx context.Context, template service.CouponTemplate) (*service.CouponTemplate, error) {
	scopes, planIDs, rules, err := marshalCouponTemplateJSON(template)
	if err != nil {
		return nil, err
	}
	var saved *service.CouponTemplate
	err = r.withTx(ctx, func(exec sqlQueryExecutor) error {
		// Publishers lock every referenced template before making a pool live.
		// Taking the same lock here makes the published-fallback guard safe when
		// an administrator updates a template while another publishes a pool.
		locked, lockErr := getCouponTemplate(ctx, exec, template.ID, true)
		if lockErr != nil {
			return lockErr
		}
		if locked == nil {
			return infraerrors.NotFound("COUPON_TEMPLATE_NOT_FOUND", "coupon template not found")
		}
		// The service validates against a non-locking read. A coupon may have
		// been issued after that read but before this transaction obtained the
		// template lock, so enforce the immutable-terms and stock invariants
		// from the locked row as the final authority.
		if template.TotalIssueLimit != nil && *template.TotalIssueLimit < locked.IssuedCount {
			return infraerrors.BadRequest("COUPON_ISSUE_LIMIT_BELOW_ISSUED", "coupon issue limit cannot be lower than already issued coupons")
		}
		if locked.IssuedCount > 0 && !service.CouponTemplateFinancialTermsEqual(*locked, template) {
			return infraerrors.Conflict("COUPON_TEMPLATE_TERMS_LOCKED", "issued coupon terms cannot be changed; create a new template version")
		}
		isFallback, fallbackErr := hasPublishedCouponRewardFallback(ctx, exec, template.ID)
		if fallbackErr != nil {
			return fallbackErr
		}
		if isFallback {
			if validateErr := service.ValidateCouponRewardFallbackTemplate(&template, time.Now().UTC()); validateErr != nil {
				return infraerrors.Conflict(
					"COUPON_POOL_FALLBACK_TEMPLATE_LOCKED",
					fmt.Sprintf("published coupon pool fallback templates must remain active and unbounded (%v); create a new template and pool version", validateErr),
				)
			}
		}

		var row couponTemplateRow
		updateErr := scanSingleRow(ctx, exec, `
			UPDATE coupon_templates SET
				template_key = $2,
				version = $3,
				name = $4,
				description = $5,
				status = $6,
				benefit_type = $7,
				benefit_value = $8,
				max_discount_amount = $9,
				currency = $10,
				applicable_scopes = $11,
				minimum_order_amount = $12,
				eligible_plan_ids = $13,
				validity_mode = $14,
				validity_days = $15,
				fixed_expires_at = $16,
				valid_from = $17,
				total_issue_limit = $18,
				rules = $19,
				updated_by = $20,
				updated_at = NOW()
			WHERE id = $1
			RETURNING `+couponTemplateColumns,
			[]any{
				template.ID, template.Key, template.Version, template.Name, template.Description, template.Status,
				template.BenefitType, template.BenefitValue, template.MaxDiscountAmount, template.Currency,
				scopes, template.MinimumOrderAmount, planIDs,
				template.ValidityMode, template.ValidityDays, template.FixedExpiresAt, template.ValidFrom,
				template.TotalIssueLimit, rules, template.UpdatedBy,
			}, row.scanDest()...)
		if updateErr != nil {
			if isUniqueConstraintViolation(updateErr) {
				return infraerrors.Conflict("COUPON_TEMPLATE_KEY_EXISTS", "coupon template key already exists")
			}
			return fmt.Errorf("update coupon template: %w", updateErr)
		}
		value, valueErr := row.value()
		if valueErr != nil {
			return valueErr
		}
		saved = value
		return nil
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}

// hasPublishedCouponRewardFallback intentionally runs while the caller holds
// the template row lock. PublishCouponRewardPool takes that same template lock
// before it changes status, which serializes a publish against any attempt to
// make its fallback template bounded or inactive.
func hasPublishedCouponRewardFallback(ctx context.Context, exec sqlQueryExecutor, templateID int64) (bool, error) {
	var exists bool
	err := scanSingleRow(ctx, exec, `
		SELECT EXISTS(
			SELECT 1 FROM coupon_reward_pool_versions
			WHERE status = 'published' AND fallback_template_id = $1
		)`, []any{templateID}, &exists)
	if err != nil {
		return false, fmt.Errorf("check published coupon fallback template: %w", err)
	}
	return exists, nil
}

func (r *couponRepository) DeleteCouponTemplate(ctx context.Context, id int64) error {
	exec, err := r.exec(ctx)
	if err != nil {
		return err
	}
	result, err := exec.ExecContext(ctx, `DELETE FROM coupon_templates WHERE id = $1 AND issued_count = 0`, id)
	if err != nil {
		return fmt.Errorf("delete coupon template: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete coupon template rows affected: %w", err)
	}
	if affected == 0 {
		return infraerrors.Conflict("COUPON_TEMPLATE_DELETE_REJECTED", "coupon template does not exist or has issued coupons")
	}
	return nil
}

func (r *couponRepository) ListCouponTemplates(ctx context.Context, filter service.CouponTemplateListFilter) ([]service.CouponTemplate, int64, error) {
	exec, err := r.exec(ctx)
	if err != nil {
		return nil, 0, err
	}
	where := make([]string, 0, 2)
	args := make([]any, 0, 4)
	if filter.Status != "" {
		args = append(args, filter.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.Search != "" {
		args = append(args, "%"+strings.ToLower(filter.Search)+"%")
		where = append(where, fmt.Sprintf("(LOWER(template_key) LIKE $%d OR LOWER(name) LIKE $%d)", len(args), len(args)))
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}
	var total int64
	if err := scanSingleRow(ctx, exec, "SELECT COUNT(*) FROM coupon_templates"+clause, args, &total); err != nil {
		return nil, 0, fmt.Errorf("count coupon templates: %w", err)
	}
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := exec.QueryContext(ctx, "SELECT "+couponTemplateColumns+" FROM coupon_templates"+clause+
		fmt.Sprintf(" ORDER BY updated_at DESC, id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list coupon templates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.CouponTemplate, 0, filter.PageSize)
	for rows.Next() {
		var template couponTemplateRow
		if err := rows.Scan(template.scanDest()...); err != nil {
			return nil, 0, fmt.Errorf("scan coupon template: %w", err)
		}
		value, err := template.value()
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *value)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate coupon templates: %w", err)
	}
	return out, total, nil
}

type couponTemplateRaw struct {
	scopes      []byte
	planIDs     []byte
	rules       []byte
	maxDiscount sql.NullFloat64
	fixedExpiry sql.NullTime
	validFrom   sql.NullTime
	totalLimit  sql.NullInt64
	createdBy   sql.NullInt64
	updatedBy   sql.NullInt64
}

type couponTemplateRow struct {
	template service.CouponTemplate
	raw      couponTemplateRaw
}

func (row *couponTemplateRow) scanDest() []any {
	template := &row.template
	raw := &row.raw
	return []any{
		&template.ID, &template.Key, &template.Version, &template.Name, &template.Description, &template.Status,
		&template.BenefitType, &template.BenefitValue, &raw.maxDiscount, &template.Currency,
		&raw.scopes, &template.MinimumOrderAmount, &raw.planIDs,
		&template.ValidityMode, &template.ValidityDays, &raw.fixedExpiry, &raw.validFrom,
		&raw.totalLimit, &template.IssuedCount, &raw.rules, &raw.createdBy, &raw.updatedBy,
		&template.CreatedAt, &template.UpdatedAt,
	}
}

func (row *couponTemplateRow) value() (*service.CouponTemplate, error) {
	template := &row.template
	raw := &row.raw
	if err := json.Unmarshal(raw.scopes, &template.ApplicableScopes); err != nil {
		return nil, fmt.Errorf("decode coupon template scopes: %w", err)
	}
	if err := json.Unmarshal(raw.planIDs, &template.EligiblePlanIDs); err != nil {
		return nil, fmt.Errorf("decode coupon template plan ids: %w", err)
	}
	if err := json.Unmarshal(raw.rules, &template.Rules); err != nil {
		return nil, fmt.Errorf("decode coupon template rules: %w", err)
	}
	if raw.maxDiscount.Valid {
		value := raw.maxDiscount.Float64
		template.MaxDiscountAmount = &value
	}
	if raw.fixedExpiry.Valid {
		value := raw.fixedExpiry.Time
		template.FixedExpiresAt = &value
	}
	if raw.validFrom.Valid {
		value := raw.validFrom.Time
		template.ValidFrom = &value
	}
	if raw.totalLimit.Valid {
		value := raw.totalLimit.Int64
		template.TotalIssueLimit = &value
	}
	if raw.createdBy.Valid {
		value := raw.createdBy.Int64
		template.CreatedBy = &value
	}
	if raw.updatedBy.Valid {
		value := raw.updatedBy.Int64
		template.UpdatedBy = &value
	}
	if template.ApplicableScopes == nil {
		template.ApplicableScopes = []service.CouponScope{}
	}
	if template.EligiblePlanIDs == nil {
		template.EligiblePlanIDs = []int64{}
	}
	if template.Rules == nil {
		template.Rules = map[string]any{}
	}
	return template, nil
}

func marshalCouponTemplateJSON(template service.CouponTemplate) ([]byte, []byte, []byte, error) {
	scopes, err := json.Marshal(template.ApplicableScopes)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("encode coupon scopes: %w", err)
	}
	planIDs, err := json.Marshal(template.EligiblePlanIDs)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("encode coupon plan ids: %w", err)
	}
	rules, err := json.Marshal(template.Rules)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("encode coupon rules: %w", err)
	}
	return scopes, planIDs, rules, nil
}

const couponUserCouponBaseSelectColumns = `
	uc.id, uc.template_id, COALESCE(ct.name, ''), uc.user_id, uc.status,
	uc.terms_snapshot, uc.source, uc.source_ref, uc.issue_batch_id,
	uc.idempotency_key, uc.issued_at, uc.valid_from, uc.expires_at,
	uc.locked_order_id, uc.locked_at, uc.used_order_id, uc.used_at,
	uc.voided_at, uc.void_reason, uc.created_at, uc.updated_at`

const couponUserCouponAdminSelectColumns = `
	uc.id, uc.template_id, COALESCE(ct.name, ''), uc.user_id, uc.status,
	uc.terms_snapshot, uc.source, uc.source_ref, uc.issue_batch_id,
	uc.idempotency_key, uc.issued_at, uc.valid_from, uc.expires_at,
	uc.locked_order_id, uc.locked_at, uc.used_order_id, uc.used_at,
	uc.voided_at, uc.void_reason, uc.created_at, uc.updated_at,
	COALESCE(u.email, ''), COALESCE(u.username, ''),
	COALESCE(po.out_trade_no, ''), COALESCE(po.order_type, ''), COALESCE(po.status, ''),
	COALESCE(po.amount, 0), COALESCE(po.pay_amount, 0), COALESCE(po.discount_amount, 0),
	COALESCE(po.payment_currency, '')`

func (r *couponRepository) IssueCoupon(ctx context.Context, request service.CouponIssueInput, issuedAt time.Time) (*service.UserCoupon, error) {
	var issued *service.UserCoupon
	err := r.withTx(ctx, func(exec sqlQueryExecutor) error {
		existing, err := getUserCouponByIdempotency(ctx, exec, request.IdempotencyKey, true)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.TemplateID != request.TemplateID || existing.UserID != request.UserID || existing.Source != request.Source {
				return infraerrors.Conflict("COUPON_IDEMPOTENCY_CONFLICT", "coupon idempotency key belongs to a different issue request")
			}
			issued = existing
			return nil
		}
		template, err := getCouponTemplate(ctx, exec, request.TemplateID, true)
		if err != nil {
			return err
		}
		if template == nil {
			return infraerrors.NotFound("COUPON_TEMPLATE_NOT_FOUND", "coupon template not found")
		}
		issued, err = issueCouponWithTemplate(ctx, exec, template, request, issuedAt)
		return err
	})
	if err != nil {
		return nil, err
	}
	return issued, nil
}

func issueCouponWithTemplate(
	ctx context.Context,
	exec sqlQueryExecutor,
	template *service.CouponTemplate,
	request service.CouponIssueInput,
	issuedAt time.Time,
) (*service.UserCoupon, error) {
	if template == nil || template.Status != service.CouponTemplateStatusActive {
		return nil, infraerrors.Conflict("COUPON_TEMPLATE_INACTIVE", "coupon template is not active")
	}
	if template.TotalIssueLimit != nil && template.IssuedCount >= *template.TotalIssueLimit {
		return nil, infraerrors.Conflict("COUPON_TEMPLATE_STOCK_EXHAUSTED", "coupon template issue limit is exhausted")
	}
	validFrom, expiresAt, err := service.CouponExpiryForIssue(*template, issuedAt)
	if err != nil {
		return nil, infraerrors.Conflict("COUPON_TEMPLATE_NOT_ISSUABLE", err.Error())
	}
	if !expiresAt.After(issuedAt) {
		return nil, infraerrors.Conflict("COUPON_TEMPLATE_EXPIRED", "coupon template has already expired")
	}
	termsJSON, err := json.Marshal(service.CouponTermsFromTemplate(*template))
	if err != nil {
		return nil, fmt.Errorf("encode coupon terms snapshot: %w", err)
	}
	var row couponUserCouponRow
	err = scanSingleRow(ctx, exec, `
		INSERT INTO user_coupons (
			template_id, user_id, status, terms_snapshot, source, source_ref,
			issue_batch_id, idempotency_key, issued_at, valid_from, expires_at
		) VALUES ($1,$2,'available',$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, template_id, ''::text, user_id, status,
			terms_snapshot, source, source_ref, issue_batch_id,
			idempotency_key, issued_at, valid_from, expires_at,
			locked_order_id, locked_at, used_order_id, used_at,
			voided_at, void_reason, created_at, updated_at`,
		[]any{
			template.ID, request.UserID, termsJSON, request.Source, strings.TrimSpace(request.SourceRef),
			request.IssueBatchID, strings.TrimSpace(request.IdempotencyKey), issuedAt, validFrom, expiresAt,
		}, row.scanBaseDest()...)
	if err != nil {
		if isUniqueConstraintViolation(err) {
			return getUserCouponByIdempotency(ctx, exec, request.IdempotencyKey, true)
		}
		return nil, fmt.Errorf("insert user coupon: %w", err)
	}
	issued, err := row.value()
	if err != nil {
		return nil, err
	}
	result, err := exec.ExecContext(ctx, `
		UPDATE coupon_templates
		SET issued_count = issued_count + 1, updated_at = NOW()
		WHERE id = $1
		  AND (total_issue_limit IS NULL OR issued_count < total_issue_limit)`, template.ID)
	if err != nil {
		return nil, fmt.Errorf("increment coupon template issued count: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("read coupon template issued count result: %w", err)
	}
	if affected != 1 {
		return nil, infraerrors.Conflict("COUPON_TEMPLATE_STOCK_EXHAUSTED", "coupon template issue limit is exhausted")
	}
	template.IssuedCount++
	if err := insertCouponEvent(ctx, exec, couponEventInput{
		CouponID:     &issued.ID,
		TemplateID:   &template.ID,
		IssueBatchID: request.IssueBatchID,
		EventType:    "issued",
		ActorType:    couponActorType(request.ActorID),
		ActorUserID:  couponActorID(request.ActorID),
		Metadata: map[string]any{
			"source":          request.Source,
			"source_ref":      request.SourceRef,
			"idempotency_key": request.IdempotencyKey,
		},
	}); err != nil {
		return nil, err
	}
	return issued, nil
}

func (r *couponRepository) IssueCouponBatch(ctx context.Context, request service.CouponBatchIssueInput, issuedAt time.Time) (*service.CouponIssueBatch, error) {
	inputSnapshot, inputSnapshotJSON, err := couponBatchRequestSnapshot(request)
	if err != nil {
		return nil, err
	}
	var batch *service.CouponIssueBatch
	err = r.withTx(ctx, func(exec sqlQueryExecutor) error {
		existing, err := getCouponIssueBatchByIdempotency(ctx, exec, request.IdempotencyKey, true)
		if err != nil {
			return err
		}
		if existing != nil {
			if !couponBatchRequestMatches(existing, request, inputSnapshot) {
				return infraerrors.Conflict("COUPON_BATCH_IDEMPOTENCY_CONFLICT", "coupon batch idempotency key belongs to a different issue request")
			}
			batch = existing
			return nil
		}
		template, err := getCouponTemplate(ctx, exec, request.TemplateID, true)
		if err != nil {
			return err
		}
		if template == nil {
			return infraerrors.NotFound("COUPON_TEMPLATE_NOT_FOUND", "coupon template not found")
		}
		if template.Status != service.CouponTemplateStatusActive {
			return infraerrors.Conflict("COUPON_TEMPLATE_INACTIVE", "coupon template is not active")
		}
		if template.TotalIssueLimit != nil && *template.TotalIssueLimit-template.IssuedCount < int64(len(request.UserIDs)) {
			return infraerrors.Conflict("COUPON_TEMPLATE_STOCK_EXHAUSTED", "coupon template issue limit cannot satisfy this batch")
		}
		var created couponIssueBatchRow
		err = scanSingleRow(ctx, exec, `
			INSERT INTO coupon_issue_batches (
				template_id, source, requested_count, issued_count, failed_count,
				status, idempotency_key, input_snapshot, created_by, completed_at
			) VALUES ($1,$2,$3,0,0,'completed',$4,$5,$6,$7)
			RETURNING `+couponIssueBatchColumns,
			[]any{template.ID, request.Source, len(request.UserIDs), request.IdempotencyKey, inputSnapshotJSON, couponActorID(request.ActorID), issuedAt},
			created.scanDest()...)
		if err != nil {
			return fmt.Errorf("create coupon issue batch: %w", err)
		}
		batch, err = created.value()
		if err != nil {
			return err
		}
		for _, userID := range request.UserIDs {
			batchID := batch.ID
			_, err := issueCouponWithTemplate(ctx, exec, template, service.CouponIssueInput{
				TemplateID:     template.ID,
				UserID:         userID,
				Source:         request.Source,
				SourceRef:      fmt.Sprintf("batch:%d", batch.ID),
				IssueBatchID:   &batchID,
				IdempotencyKey: fmt.Sprintf("coupon-batch:%d:user:%d", batch.ID, userID),
				ActorID:        request.ActorID,
			}, issuedAt)
			if err != nil {
				return err
			}
		}
		batch.IssuedCount = len(request.UserIDs)
		if _, err := exec.ExecContext(ctx, `UPDATE coupon_issue_batches SET issued_count = $2 WHERE id = $1`, batch.ID, batch.IssuedCount); err != nil {
			return fmt.Errorf("finalize coupon issue batch: %w", err)
		}
		return insertCouponEvent(ctx, exec, couponEventInput{
			TemplateID:   &template.ID,
			IssueBatchID: &batch.ID,
			EventType:    "batch_completed",
			ActorType:    couponActorType(request.ActorID),
			ActorUserID:  couponActorID(request.ActorID),
			Metadata: map[string]any{
				"requested_count": len(request.UserIDs),
			},
		})
	})
	if err != nil {
		return nil, err
	}
	return batch, nil
}

// couponBatchRequestSnapshot serializes then decodes the incoming payload so
// idempotency checks compare JSON values with the same number representation
// PostgreSQL returns from the saved JSONB snapshot.
func couponBatchRequestSnapshot(request service.CouponBatchIssueInput) (map[string]any, []byte, error) {
	raw, err := json.Marshal(map[string]any{
		"user_ids": request.UserIDs,
		"metadata": request.Metadata,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("encode coupon batch input snapshot: %w", err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, nil, fmt.Errorf("normalize coupon batch input snapshot: %w", err)
	}
	return snapshot, raw, nil
}

func couponBatchRequestMatches(existing *service.CouponIssueBatch, request service.CouponBatchIssueInput, snapshot map[string]any) bool {
	return existing != nil &&
		existing.TemplateID == request.TemplateID &&
		existing.Source == request.Source &&
		existing.RequestedCount == len(request.UserIDs) &&
		reflect.DeepEqual(existing.InputSnapshot, snapshot)
}

func (r *couponRepository) GetUserCoupon(ctx context.Context, id int64) (*service.UserCoupon, error) {
	exec, err := r.exec(ctx)
	if err != nil {
		return nil, err
	}
	return getUserCoupon(ctx, exec, id, false)
}

func getUserCoupon(ctx context.Context, exec sqlQueryExecutor, id int64, forUpdate bool) (*service.UserCoupon, error) {
	query := `SELECT ` + couponUserCouponBaseSelectColumns + `
		FROM user_coupons uc
		JOIN coupon_templates ct ON ct.id = uc.template_id
		WHERE uc.id = $1`
	if forUpdate {
		query += ` FOR UPDATE OF uc`
	}
	var row couponUserCouponRow
	err := scanSingleRow(ctx, exec, query, []any{id}, row.scanBaseDest()...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user coupon: %w", err)
	}
	return row.value()
}

func getUserCouponByIdempotency(ctx context.Context, exec sqlQueryExecutor, key string, forUpdate bool) (*service.UserCoupon, error) {
	query := `SELECT ` + couponUserCouponBaseSelectColumns + `
		FROM user_coupons uc
		JOIN coupon_templates ct ON ct.id = uc.template_id
		WHERE uc.idempotency_key = $1`
	if forUpdate {
		query += ` FOR UPDATE OF uc`
	}
	var row couponUserCouponRow
	err := scanSingleRow(ctx, exec, query, []any{key}, row.scanBaseDest()...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user coupon by idempotency: %w", err)
	}
	return row.value()
}

func (r *couponRepository) ListUserCoupons(ctx context.Context, filter service.UserCouponListFilter) ([]service.UserCoupon, int64, error) {
	exec, err := r.exec(ctx)
	if err != nil {
		return nil, 0, err
	}
	where := make([]string, 0, 3)
	args := make([]any, 0, 5)
	if filter.UserID > 0 {
		args = append(args, filter.UserID)
		where = append(where, fmt.Sprintf("uc.user_id = $%d", len(args)))
	}
	if query := strings.TrimSpace(filter.UserQuery); query != "" {
		args = append(args, "%"+query+"%")
		placeholder := fmt.Sprintf("$%d", len(args))
		where = append(where, "(u.email ILIKE "+placeholder+" OR u.username ILIKE "+placeholder+" OR uc.user_id::text = "+placeholder+")")
	}
	if filter.TemplateID > 0 {
		args = append(args, filter.TemplateID)
		where = append(where, fmt.Sprintf("uc.template_id = $%d", len(args)))
	}
	if filter.Source != "" {
		args = append(args, filter.Source)
		where = append(where, fmt.Sprintf("uc.source = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where = append(where, fmt.Sprintf("uc.status = $%d", len(args)))
	}
	if filter.IssuedFrom != nil {
		args = append(args, *filter.IssuedFrom)
		where = append(where, fmt.Sprintf("uc.issued_at >= $%d", len(args)))
	}
	if filter.IssuedTo != nil {
		args = append(args, *filter.IssuedTo)
		where = append(where, fmt.Sprintf("uc.issued_at < $%d", len(args)))
	}
	// The service persists expiry before listing. Keep this read-side guard as
	// well so a coupon that expires between that sweep and this query cannot
	// briefly reappear in an available wallet response.
	where = append(where, "(uc.status <> 'available' OR uc.expires_at > NOW())")
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}
	var total int64
	if err := scanSingleRow(ctx, exec, `SELECT COUNT(*) FROM user_coupons uc LEFT JOIN users u ON u.id = uc.user_id`+clause, args, &total); err != nil {
		return nil, 0, fmt.Errorf("count user coupons: %w", err)
	}
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := exec.QueryContext(ctx, `SELECT `+couponUserCouponAdminSelectColumns+`
		FROM user_coupons uc
		JOIN coupon_templates ct ON ct.id = uc.template_id
		LEFT JOIN users u ON u.id = uc.user_id
		LEFT JOIN payment_orders po ON po.id = uc.used_order_id`+clause+
		fmt.Sprintf(" ORDER BY uc.created_at DESC, uc.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list user coupons: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.UserCoupon, 0, filter.PageSize)
	for rows.Next() {
		var row couponUserCouponRow
		if err := rows.Scan(row.scanAdminDest()...); err != nil {
			return nil, 0, fmt.Errorf("scan user coupon: %w", err)
		}
		coupon, err := row.value()
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *coupon)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate user coupons: %w", err)
	}
	return out, total, nil
}

// ExpireAvailableUserCoupons is intentionally lazy. Locked coupons remain
// reserved for their in-flight payment and are converted to expired only when
// that order later releases the lock.
func (r *couponRepository) ExpireAvailableUserCoupons(ctx context.Context, userID int64, at time.Time) error {
	if at.IsZero() {
		at = time.Now()
	}
	return r.withTx(ctx, func(exec sqlQueryExecutor) error {
		where := []string{"status = 'available'", "expires_at <= $1"}
		args := []any{at}
		if userID > 0 {
			args = append(args, userID)
			where = append(where, "user_id = $2")
		}
		if _, err := exec.ExecContext(ctx, `UPDATE user_coupons
			SET status = 'expired', updated_at = $1
			WHERE `+strings.Join(where, " AND "), args...); err != nil {
			return fmt.Errorf("persist expired user coupons: %w", err)
		}
		return nil
	})
}

func (r *couponRepository) VoidUserCoupon(ctx context.Context, couponID int64, reason string, actorID int64, at time.Time) (*service.UserCoupon, error) {
	var saved *service.UserCoupon
	err := r.withTx(ctx, func(exec sqlQueryExecutor) error {
		coupon, err := getUserCoupon(ctx, exec, couponID, true)
		if err != nil {
			return err
		}
		if coupon == nil {
			return infraerrors.NotFound("COUPON_NOT_FOUND", "coupon not found")
		}
		switch coupon.Status {
		case service.UserCouponStatusVoided:
			saved = coupon
			return nil
		case service.UserCouponStatusUsed:
			return infraerrors.Conflict("COUPON_ALREADY_USED", "used coupons cannot be voided")
		case service.UserCouponStatusLocked:
			return infraerrors.Conflict("COUPON_LOCKED", "locked coupons cannot be voided")
		}
		var row couponUserCouponRow
		err = scanSingleRow(ctx, exec, `
			UPDATE user_coupons SET
				status = 'voided', voided_at = $2, void_reason = $3, updated_at = NOW()
			WHERE id = $1
			RETURNING id, template_id, ''::text, user_id, status,
				terms_snapshot, source, source_ref, issue_batch_id,
				idempotency_key, issued_at, valid_from, expires_at,
				locked_order_id, locked_at, used_order_id, used_at,
				voided_at, void_reason, created_at, updated_at`,
			[]any{couponID, at, reason}, row.scanBaseDest()...)
		if err != nil {
			return fmt.Errorf("void user coupon: %w", err)
		}
		saved, err = row.value()
		if err != nil {
			return err
		}
		return insertCouponEvent(ctx, exec, couponEventInput{
			CouponID:    &saved.ID,
			TemplateID:  &saved.TemplateID,
			EventType:   "voided",
			ActorType:   couponActorType(actorID),
			ActorUserID: couponActorID(actorID),
			Metadata:    map[string]any{"reason": reason},
		})
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}

type couponUserCouponRaw struct {
	terms                   []byte
	issueBatch              sql.NullInt64
	lockedOrder             sql.NullInt64
	lockedAt                sql.NullTime
	usedOrder               sql.NullInt64
	usedAt                  sql.NullTime
	voidedAt                sql.NullTime
	usedOrderNo             sql.NullString
	usedOrderType           sql.NullString
	usedOrderStatus         sql.NullString
	usedOrderAmount         sql.NullFloat64
	usedOrderPayAmount      sql.NullFloat64
	usedOrderDiscountAmount sql.NullFloat64
	usedOrderCurrency       sql.NullString
}

type couponUserCouponRow struct {
	coupon service.UserCoupon
	raw    couponUserCouponRaw
}

func (row *couponUserCouponRow) scanBaseDest() []any {
	coupon := &row.coupon
	raw := &row.raw
	return []any{
		&coupon.ID, &coupon.TemplateID, &coupon.TemplateName, &coupon.UserID, &coupon.Status,
		&raw.terms, &coupon.Source, &coupon.SourceRef, &raw.issueBatch,
		&coupon.IdempotencyKey, &coupon.IssuedAt, &coupon.ValidFrom, &coupon.ExpiresAt,
		&raw.lockedOrder, &raw.lockedAt, &raw.usedOrder, &raw.usedAt,
		&raw.voidedAt, &coupon.VoidReason, &coupon.CreatedAt, &coupon.UpdatedAt,
	}
}

func (row *couponUserCouponRow) scanAdminDest() []any {
	coupon := &row.coupon
	raw := &row.raw
	return append(row.scanBaseDest(),
		&coupon.UserEmail, &coupon.UserName,
		&raw.usedOrderNo, &raw.usedOrderType, &raw.usedOrderStatus,
		&raw.usedOrderAmount, &raw.usedOrderPayAmount, &raw.usedOrderDiscountAmount,
		&raw.usedOrderCurrency,
	)
}

func (row *couponUserCouponRow) value() (*service.UserCoupon, error) {
	coupon := &row.coupon
	raw := &row.raw
	if err := json.Unmarshal(raw.terms, &coupon.TermsSnapshot); err != nil {
		return nil, fmt.Errorf("decode user coupon terms snapshot: %w", err)
	}
	if raw.issueBatch.Valid {
		value := raw.issueBatch.Int64
		coupon.IssueBatchID = &value
	}
	if raw.lockedOrder.Valid {
		value := raw.lockedOrder.Int64
		coupon.LockedOrderID = &value
	}
	if raw.lockedAt.Valid {
		value := raw.lockedAt.Time
		coupon.LockedAt = &value
	}
	if raw.usedOrder.Valid {
		value := raw.usedOrder.Int64
		coupon.UsedOrderID = &value
	}
	if raw.usedAt.Valid {
		value := raw.usedAt.Time
		coupon.UsedAt = &value
	}
	if raw.usedOrderNo.Valid {
		coupon.UsedOrderNo = raw.usedOrderNo.String
	}
	if raw.usedOrderType.Valid {
		coupon.UsedOrderType = raw.usedOrderType.String
	}
	if raw.usedOrderStatus.Valid {
		coupon.UsedOrderStatus = raw.usedOrderStatus.String
	}
	if raw.usedOrderAmount.Valid {
		coupon.UsedOrderAmount = raw.usedOrderAmount.Float64
	}
	if raw.usedOrderPayAmount.Valid {
		coupon.UsedOrderPayAmount = raw.usedOrderPayAmount.Float64
	}
	if raw.usedOrderDiscountAmount.Valid {
		coupon.UsedOrderDiscountAmount = raw.usedOrderDiscountAmount.Float64
	}
	if raw.usedOrderCurrency.Valid {
		coupon.UsedOrderCurrency = raw.usedOrderCurrency.String
	}
	if raw.voidedAt.Valid {
		value := raw.voidedAt.Time
		coupon.VoidedAt = &value
	}
	if coupon.TermsSnapshot.ApplicableScopes == nil {
		coupon.TermsSnapshot.ApplicableScopes = []service.CouponScope{}
	}
	if coupon.TermsSnapshot.EligiblePlanIDs == nil {
		coupon.TermsSnapshot.EligiblePlanIDs = []int64{}
	}
	if coupon.TermsSnapshot.Rules == nil {
		coupon.TermsSnapshot.Rules = map[string]any{}
	}
	return coupon, nil
}

const couponIssueBatchColumns = `
	id, template_id, ''::text, source, requested_count, issued_count,
	failed_count, status, idempotency_key, input_snapshot, created_by,
	completed_at, created_at`

func getCouponIssueBatchByIdempotency(ctx context.Context, exec sqlQueryExecutor, key string, forUpdate bool) (*service.CouponIssueBatch, error) {
	query := `SELECT b.id, b.template_id, COALESCE(t.name, ''), b.source, b.requested_count, b.issued_count,
		b.failed_count, b.status, b.idempotency_key, b.input_snapshot, b.created_by,
		b.completed_at, b.created_at
		FROM coupon_issue_batches b
		JOIN coupon_templates t ON t.id = b.template_id
		WHERE b.idempotency_key = $1`
	if forUpdate {
		query += ` FOR UPDATE OF b`
	}
	var row couponIssueBatchRow
	err := scanSingleRow(ctx, exec, query, []any{key}, row.scanDest()...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get coupon issue batch by idempotency: %w", err)
	}
	return row.value()
}

func (r *couponRepository) ListCouponIssueBatches(ctx context.Context, filter service.CouponIssueBatchListFilter) ([]service.CouponIssueBatch, int64, error) {
	exec, err := r.exec(ctx)
	if err != nil {
		return nil, 0, err
	}
	where := ""
	args := make([]any, 0, 3)
	if filter.TemplateID > 0 {
		args = append(args, filter.TemplateID)
		where = " WHERE b.template_id = $1"
	}
	var total int64
	if err := scanSingleRow(ctx, exec, "SELECT COUNT(*) FROM coupon_issue_batches b"+where, args, &total); err != nil {
		return nil, 0, fmt.Errorf("count coupon issue batches: %w", err)
	}
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := exec.QueryContext(ctx, `SELECT b.id, b.template_id, COALESCE(t.name, ''), b.source, b.requested_count, b.issued_count,
		b.failed_count, b.status, b.idempotency_key, b.input_snapshot, b.created_by,
		b.completed_at, b.created_at
		FROM coupon_issue_batches b
		JOIN coupon_templates t ON t.id = b.template_id`+where+
		fmt.Sprintf(" ORDER BY b.created_at DESC, b.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list coupon issue batches: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.CouponIssueBatch, 0, filter.PageSize)
	for rows.Next() {
		var row couponIssueBatchRow
		if err := rows.Scan(row.scanDest()...); err != nil {
			return nil, 0, fmt.Errorf("scan coupon issue batch: %w", err)
		}
		batch, err := row.value()
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *batch)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate coupon issue batches: %w", err)
	}
	return out, total, nil
}

type couponIssueBatchRaw struct {
	inputSnapshot []byte
	createdBy     sql.NullInt64
	completedAt   sql.NullTime
}

type couponIssueBatchRow struct {
	batch service.CouponIssueBatch
	raw   couponIssueBatchRaw
}

func (row *couponIssueBatchRow) scanDest() []any {
	batch := &row.batch
	raw := &row.raw
	return []any{
		&batch.ID, &batch.TemplateID, &batch.TemplateName, &batch.Source, &batch.RequestedCount, &batch.IssuedCount,
		&batch.FailedCount, &batch.Status, &batch.IdempotencyKey, &raw.inputSnapshot, &raw.createdBy,
		&raw.completedAt, &batch.CreatedAt,
	}
}

func (row *couponIssueBatchRow) value() (*service.CouponIssueBatch, error) {
	batch := &row.batch
	raw := &row.raw
	if err := json.Unmarshal(raw.inputSnapshot, &batch.InputSnapshot); err != nil {
		return nil, fmt.Errorf("decode coupon issue batch input snapshot: %w", err)
	}
	if raw.createdBy.Valid {
		value := raw.createdBy.Int64
		batch.CreatedBy = &value
	}
	if raw.completedAt.Valid {
		value := raw.completedAt.Time
		batch.CompletedAt = &value
	}
	if batch.InputSnapshot == nil {
		batch.InputSnapshot = map[string]any{}
	}
	return batch, nil
}

func (r *couponRepository) SaveCouponRewardPool(ctx context.Context, pool service.CouponRewardPoolVersion) (*service.CouponRewardPoolVersion, error) {
	var saved *service.CouponRewardPoolVersion
	err := r.withTx(ctx, func(exec sqlQueryExecutor) error {
		if pool.ID == 0 {
			rewardConfig, err := json.Marshal(pool.RewardConfig)
			if err != nil {
				return fmt.Errorf("encode coupon reward config: %w", err)
			}
			var row couponRewardPoolRow
			err = scanSingleRow(ctx, exec, `
				INSERT INTO coupon_reward_pool_versions (
					activity, version, status, coupon_weight_bp, redeem_code_weight_bp, balance_weight_bp, reward_config,
					fallback_template_id, created_by, updated_by
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
				RETURNING `+couponPoolColumns,
				[]any{
					pool.Activity, pool.Version, pool.Status, pool.CouponWeightBP, pool.RedeemCodeWeightBP, pool.BalanceWeightBP, rewardConfig,
					pool.FallbackTemplateID, pool.CreatedBy, pool.UpdatedBy,
				}, row.scanDest()...)
			if err != nil {
				if isUniqueConstraintViolation(err) {
					return infraerrors.Conflict("COUPON_POOL_VERSION_EXISTS", "coupon pool version already exists for this activity")
				}
				return fmt.Errorf("create coupon reward pool: %w", err)
			}
			saved, err = row.value()
			if err != nil {
				return err
			}
			if err := insertCouponRewardPoolEntries(ctx, exec, saved.ID, pool.Entries); err != nil {
				return err
			}
			return nil
		}

		existing, err := getCouponRewardPool(ctx, exec, pool.ID, true)
		if err != nil {
			return err
		}
		if existing == nil {
			return infraerrors.NotFound("COUPON_POOL_NOT_FOUND", "coupon pool not found")
		}
		if existing.Status != service.CouponRewardPoolStatusDraft {
			return infraerrors.Conflict("COUPON_POOL_NOT_DRAFT", "published or retired coupon pools cannot be changed")
		}
		rewardConfig, err := json.Marshal(pool.RewardConfig)
		if err != nil {
			return fmt.Errorf("encode coupon reward config: %w", err)
		}
		var row couponRewardPoolRow
		err = scanSingleRow(ctx, exec, `
			UPDATE coupon_reward_pool_versions SET
				activity = $2,
				version = $3,
				coupon_weight_bp = $4,
				redeem_code_weight_bp = $5,
				balance_weight_bp = $6,
				reward_config = $7,
				fallback_template_id = $8,
				updated_by = $9,
				updated_at = NOW()
			WHERE id = $1
			RETURNING `+couponPoolColumns,
			[]any{
				pool.ID, pool.Activity, pool.Version, pool.CouponWeightBP, pool.RedeemCodeWeightBP, pool.BalanceWeightBP, rewardConfig,
				pool.FallbackTemplateID, pool.UpdatedBy,
			}, row.scanDest()...)
		if err != nil {
			if isUniqueConstraintViolation(err) {
				return infraerrors.Conflict("COUPON_POOL_VERSION_EXISTS", "coupon pool version already exists for this activity")
			}
			return fmt.Errorf("update coupon reward pool: %w", err)
		}
		saved, err = row.value()
		if err != nil {
			return err
		}
		if _, err := exec.ExecContext(ctx, `DELETE FROM coupon_reward_pool_entries WHERE pool_version_id = $1`, pool.ID); err != nil {
			return fmt.Errorf("replace coupon reward pool entries: %w", err)
		}
		return insertCouponRewardPoolEntries(ctx, exec, pool.ID, pool.Entries)
	})
	if err != nil {
		return nil, err
	}
	return r.GetCouponRewardPool(ctx, saved.ID)
}

func (r *couponRepository) GetCouponRewardPool(ctx context.Context, id int64) (*service.CouponRewardPoolVersion, error) {
	exec, err := r.exec(ctx)
	if err != nil {
		return nil, err
	}
	return getCouponRewardPool(ctx, exec, id, false)
}

func getCouponRewardPool(ctx context.Context, exec sqlQueryExecutor, id int64, forUpdate bool) (*service.CouponRewardPoolVersion, error) {
	query := `SELECT ` + couponPoolColumns + ` FROM coupon_reward_pool_versions WHERE id = $1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var row couponRewardPoolRow
	err := scanSingleRow(ctx, exec, query, []any{id}, row.scanDest()...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get coupon reward pool: %w", err)
	}
	pool, err := row.value()
	if err != nil {
		return nil, err
	}
	entries, err := listCouponRewardPoolEntries(ctx, exec, pool.ID, forUpdate)
	if err != nil {
		return nil, err
	}
	pool.Entries = entries
	return pool, nil
}

func (r *couponRepository) ListCouponRewardPools(ctx context.Context, activity service.CouponRewardActivity) ([]service.CouponRewardPoolVersion, error) {
	exec, err := r.exec(ctx)
	if err != nil {
		return nil, err
	}
	query := `SELECT id FROM coupon_reward_pool_versions`
	args := []any(nil)
	if activity != "" {
		query += ` WHERE activity = $1`
		args = append(args, activity)
	}
	query += ` ORDER BY activity ASC, created_at DESC, id DESC`
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list coupon reward pool ids: %w", err)
	}
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan coupon reward pool id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate coupon reward pool ids: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close coupon reward pool ids: %w", err)
	}
	out := make([]service.CouponRewardPoolVersion, 0, len(ids))
	for _, id := range ids {
		pool, err := getCouponRewardPool(ctx, exec, id, false)
		if err != nil {
			return nil, err
		}
		if pool != nil {
			out = append(out, *pool)
		}
	}
	return out, nil
}

func (r *couponRepository) DeleteCouponRewardPool(ctx context.Context, id int64) error {
	exec, err := r.exec(ctx)
	if err != nil {
		return err
	}
	result, err := exec.ExecContext(ctx, `
		DELETE FROM coupon_reward_pool_versions
		WHERE id = $1 AND status = 'draft'`, id)
	if err != nil {
		return fmt.Errorf("delete coupon reward pool: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete coupon reward pool rows affected: %w", err)
	}
	if affected != 1 {
		return infraerrors.Conflict("COUPON_POOL_DELETE_REJECTED", "coupon reward pool does not exist or is not a draft")
	}
	return nil
}

func (r *couponRepository) PublishCouponRewardPool(ctx context.Context, id int64, actorID int64, at time.Time) (*service.CouponRewardPoolVersion, error) {
	var published *service.CouponRewardPoolVersion
	err := r.withTx(ctx, func(exec sqlQueryExecutor) error {
		pool, err := getCouponRewardPool(ctx, exec, id, true)
		if err != nil {
			return err
		}
		if pool == nil {
			return infraerrors.NotFound("COUPON_POOL_NOT_FOUND", "coupon pool not found")
		}
		if pool.Status != service.CouponRewardPoolStatusDraft {
			return infraerrors.Conflict("COUPON_POOL_NOT_DRAFT", "only draft coupon pools can be published")
		}
		if err := service.ValidateCouponRewardPool(*pool); err != nil {
			return infraerrors.BadRequest("INVALID_COUPON_CONFIGURATION", err.Error())
		}
		// Draws lock the current published version before its entries and
		// templates. Take that version lock before this publisher reaches a
		// template lock so a draw and a publish cannot wait on each other.
		if err := lockPublishedCouponRewardPoolVersion(ctx, exec, pool.Activity); err != nil {
			return err
		}
		var fallbackTemplate *service.CouponTemplate
		for _, templateID := range couponPoolTemplateIDs(*pool) {
			template, err := getCouponTemplate(ctx, exec, templateID, true)
			if err != nil {
				return err
			}
			if template == nil || template.Status != service.CouponTemplateStatusActive {
				return infraerrors.Conflict("COUPON_POOL_TEMPLATE_INACTIVE", "published coupon pool templates must be active")
			}
			if template.ID == pool.FallbackTemplateID {
				fallbackTemplate = template
			}
		}
		if err := service.ValidateCouponRewardFallbackTemplate(fallbackTemplate, at); err != nil {
			return infraerrors.Conflict(
				"COUPON_POOL_FALLBACK_TEMPLATE_INVALID",
				fmt.Sprintf("published coupon pool fallback template must remain active and unbounded (%v)", err),
			)
		}
		if _, err := exec.ExecContext(ctx, `
			UPDATE coupon_reward_pool_versions
			SET status = 'retired', updated_by = $2, updated_at = $3
			WHERE activity = $1 AND status = 'published'`, pool.Activity, couponActorID(actorID), at); err != nil {
			return fmt.Errorf("retire previous coupon reward pool: %w", err)
		}
		var row couponRewardPoolRow
		err = scanSingleRow(ctx, exec, `
			UPDATE coupon_reward_pool_versions
			SET status = 'published', updated_by = $2, published_at = $3, updated_at = $3
			WHERE id = $1
			RETURNING `+couponPoolColumns,
			[]any{id, couponActorID(actorID), at}, row.scanDest()...)
		if err != nil {
			return fmt.Errorf("publish coupon reward pool: %w", err)
		}
		published, err = row.value()
		if err != nil {
			return err
		}
		published.Entries = pool.Entries
		return nil
	})
	if err != nil {
		return nil, err
	}
	return published, nil
}

func (r *couponRepository) GetPublishedCouponRewardPool(ctx context.Context, activity service.CouponRewardActivity) (*service.CouponRewardPoolVersion, error) {
	exec, err := r.exec(ctx)
	if err != nil {
		return nil, err
	}
	var id int64
	err = scanSingleRow(ctx, exec, `
		SELECT id FROM coupon_reward_pool_versions
		WHERE activity = $1 AND status = 'published'
		ORDER BY published_at DESC NULLS LAST, id DESC LIMIT 1`, []any{activity}, &id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get published coupon reward pool id: %w", err)
	}
	return getCouponRewardPool(ctx, exec, id, false)
}

func lockPublishedCouponRewardPoolVersion(ctx context.Context, exec sqlQueryExecutor, activity service.CouponRewardActivity) error {
	var id int64
	err := scanSingleRow(ctx, exec, `
		SELECT id FROM coupon_reward_pool_versions
		WHERE activity = $1 AND status = 'published'
		ORDER BY published_at DESC NULLS LAST, id DESC
		LIMIT 1 FOR UPDATE`, []any{activity}, &id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lock current published coupon reward pool: %w", err)
	}
	return nil
}

// HasIssuableCouponRewardEntry is intentionally a global check. Per-user caps
// are enforced during the transactional draw; they do not make a pool globally
// unavailable while another eligible user can still receive the entry.
func (r *couponRepository) HasIssuableCouponRewardEntry(ctx context.Context, activity service.CouponRewardActivity, at time.Time) (bool, error) {
	if at.IsZero() {
		at = time.Now()
	}
	pool, err := r.GetPublishedCouponRewardPool(ctx, activity)
	if err != nil {
		return false, err
	}
	if pool == nil {
		return false, nil
	}
	if pool.RedeemCodeWeightBP > 0 && len(service.EnabledRedeemRewardEntriesForRepository(pool.RewardConfig.RedeemEntries)) > 0 {
		return true, nil
	}
	if pool.BalanceWeightBP > 0 && len(service.EnabledBalanceRewardEntriesForRepository(pool.RewardConfig.BalanceEntries)) > 0 {
		return true, nil
	}
	if pool.CouponWeightBP <= 0 {
		return false, nil
	}
	exec, err := r.exec(ctx)
	if err != nil {
		return false, err
	}
	for _, entry := range pool.Entries {
		if !couponRewardEntryWindowHasCapacity(entry, at) {
			continue
		}
		template, err := getCouponTemplate(ctx, exec, entry.TemplateID, false)
		if err != nil {
			return false, err
		}
		if couponRewardTemplateIssuable(template, at) {
			return true, nil
		}
	}
	return false, nil
}

func insertCouponRewardPoolEntries(ctx context.Context, exec sqlQueryExecutor, poolID int64, entries []service.CouponRewardPoolEntry) error {
	for i, entry := range entries {
		if _, err := exec.ExecContext(ctx, `
			INSERT INTO coupon_reward_pool_entries (
				pool_version_id, template_id, weight_bp, enabled,
				starts_at, ends_at, stock_cap, per_user_issue_limit, sort_order
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			poolID, entry.TemplateID, entry.WeightBP, entry.Enabled,
			entry.StartsAt, entry.EndsAt, entry.StockCap, entry.PerUserIssueLimit, i,
		); err != nil {
			return fmt.Errorf("insert coupon reward pool entry: %w", err)
		}
	}
	return nil
}

func listCouponRewardPoolEntries(ctx context.Context, exec sqlQueryExecutor, poolID int64, forUpdate bool) ([]service.CouponRewardPoolEntry, error) {
	query := `SELECT e.id, e.pool_version_id, e.template_id, COALESCE(t.name, ''),
		e.weight_bp, e.enabled, e.starts_at, e.ends_at, e.stock_cap,
		e.issued_count, e.per_user_issue_limit, e.sort_order
		FROM coupon_reward_pool_entries e
		JOIN coupon_templates t ON t.id = e.template_id
		WHERE e.pool_version_id = $1
		ORDER BY e.sort_order ASC, e.id ASC`
	if forUpdate {
		query += ` FOR UPDATE OF e`
	}
	rows, err := exec.QueryContext(ctx, query, poolID)
	if err != nil {
		return nil, fmt.Errorf("list coupon reward pool entries: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.CouponRewardPoolEntry, 0)
	for rows.Next() {
		var row couponRewardPoolEntryRow
		if err := rows.Scan(row.scanDest()...); err != nil {
			return nil, fmt.Errorf("scan coupon reward pool entry: %w", err)
		}
		out = append(out, row.value())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate coupon reward pool entries: %w", err)
	}
	return out, nil
}

type couponRewardPoolRaw struct {
	createdBy    sql.NullInt64
	updatedBy    sql.NullInt64
	publishedAt  sql.NullTime
	rewardConfig []byte
}

type couponRewardPoolRow struct {
	pool service.CouponRewardPoolVersion
	raw  couponRewardPoolRaw
}

func (row *couponRewardPoolRow) scanDest() []any {
	pool := &row.pool
	raw := &row.raw
	return []any{
		&pool.ID, &pool.Activity, &pool.Version, &pool.Status, &pool.CouponWeightBP, &pool.RedeemCodeWeightBP, &pool.BalanceWeightBP, &raw.rewardConfig,
		&pool.FallbackTemplateID, &raw.createdBy, &raw.updatedBy, &raw.publishedAt, &pool.CreatedAt, &pool.UpdatedAt,
	}
}

func (row *couponRewardPoolRow) value() (*service.CouponRewardPoolVersion, error) {
	pool := &row.pool
	raw := &row.raw
	if raw.createdBy.Valid {
		value := raw.createdBy.Int64
		pool.CreatedBy = &value
	}
	if raw.updatedBy.Valid {
		value := raw.updatedBy.Int64
		pool.UpdatedBy = &value
	}
	if raw.publishedAt.Valid {
		value := raw.publishedAt.Time
		pool.PublishedAt = &value
	}
	if len(raw.rewardConfig) > 0 {
		if err := json.Unmarshal(raw.rewardConfig, &pool.RewardConfig); err != nil {
			return nil, fmt.Errorf("decode coupon reward config: %w", err)
		}
	}
	pool.Entries = []service.CouponRewardPoolEntry{}
	return pool, nil
}

type couponRewardPoolEntryRaw struct {
	startsAt sql.NullTime
	endsAt   sql.NullTime
	stockCap sql.NullInt64
	perUser  sql.NullInt64
}

type couponRewardPoolEntryRow struct {
	entry service.CouponRewardPoolEntry
	raw   couponRewardPoolEntryRaw
}

func (row *couponRewardPoolEntryRow) scanDest() []any {
	entry := &row.entry
	raw := &row.raw
	return []any{
		&entry.ID, &entry.PoolVersionID, &entry.TemplateID, &entry.TemplateName,
		&entry.WeightBP, &entry.Enabled, &raw.startsAt, &raw.endsAt, &raw.stockCap,
		&entry.IssuedCount, &raw.perUser, &entry.SortOrder,
	}
}

func (row *couponRewardPoolEntryRow) value() service.CouponRewardPoolEntry {
	entry := row.entry
	if row.raw.startsAt.Valid {
		value := row.raw.startsAt.Time
		entry.StartsAt = &value
	}
	if row.raw.endsAt.Valid {
		value := row.raw.endsAt.Time
		entry.EndsAt = &value
	}
	if row.raw.stockCap.Valid {
		value := row.raw.stockCap.Int64
		entry.StockCap = &value
	}
	if row.raw.perUser.Valid {
		value := int(row.raw.perUser.Int64)
		entry.PerUserIssueLimit = &value
	}
	return entry
}

func (r *couponRepository) LockUserCouponForOrder(ctx context.Context, request service.CouponLockRequest) (*service.CouponLockResult, error) {
	var result *service.CouponLockResult
	err := r.withTx(ctx, func(exec sqlQueryExecutor) error {
		coupon, err := getUserCoupon(ctx, exec, request.UserCouponID, true)
		if err != nil {
			return err
		}
		if coupon == nil || coupon.UserID != request.UserID {
			return infraerrors.NotFound("COUPON_NOT_FOUND", "coupon not found")
		}
		quoteCoupon := *coupon
		if coupon.Status == service.UserCouponStatusLocked {
			if coupon.LockedOrderID == nil || *coupon.LockedOrderID != request.OrderID {
				return infraerrors.Conflict("COUPON_LOCKED", "coupon is locked by another order")
			}
			quoteCoupon.Status = service.UserCouponStatusAvailable
			quote, err := service.QuoteUserCoupon(quoteCoupon, request.OrderContext)
			if err != nil {
				return err
			}
			result = &service.CouponLockResult{Coupon: *coupon, Quote: *quote}
			return nil
		}
		quote, err := service.QuoteUserCoupon(quoteCoupon, request.OrderContext)
		if err != nil {
			return err
		}
		var row couponUserCouponRow
		err = scanSingleRow(ctx, exec, `
			UPDATE user_coupons SET
				status = 'locked', locked_order_id = $2, locked_at = $3, updated_at = NOW()
			WHERE id = $1 AND status = 'available'
				AND valid_from <= NOW() AND expires_at > NOW()
			RETURNING id, template_id, ''::text, user_id, status,
				terms_snapshot, source, source_ref, issue_batch_id,
				idempotency_key, issued_at, valid_from, expires_at,
				locked_order_id, locked_at, used_order_id, used_at,
				voided_at, void_reason, created_at, updated_at`,
			[]any{coupon.ID, request.OrderID, request.LockedAt}, row.scanBaseDest()...)
		if errors.Is(err, sql.ErrNoRows) {
			return infraerrors.Conflict("COUPON_NOT_AVAILABLE", "coupon is no longer available")
		}
		if err != nil {
			return fmt.Errorf("lock user coupon: %w", err)
		}
		locked, err := row.value()
		if err != nil {
			return err
		}
		if err := insertCouponEvent(ctx, exec, couponEventInput{
			CouponID:   &locked.ID,
			TemplateID: &locked.TemplateID,
			EventType:  "locked",
			ActorType:  "system",
			OrderID:    &request.OrderID,
			Metadata: map[string]any{
				"discount_amount": quote.DiscountAmount,
				"order_amount":    quote.OriginalAmount,
			},
		}); err != nil {
			return err
		}
		result = &service.CouponLockResult{Coupon: *locked, Quote: *quote}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *couponRepository) ReleaseUserCouponOrderLock(ctx context.Context, couponID, orderID int64, at time.Time) (*service.UserCoupon, error) {
	var released *service.UserCoupon
	err := r.withTx(ctx, func(exec sqlQueryExecutor) error {
		coupon, err := getUserCoupon(ctx, exec, couponID, true)
		if err != nil {
			return err
		}
		if coupon == nil {
			return infraerrors.NotFound("COUPON_NOT_FOUND", "coupon not found")
		}
		if coupon.Status == service.UserCouponStatusUsed && coupon.UsedOrderID != nil && *coupon.UsedOrderID == orderID {
			return infraerrors.Conflict("COUPON_ALREADY_USED", "used coupon lock cannot be released")
		}
		if coupon.Status != service.UserCouponStatusLocked || coupon.LockedOrderID == nil || *coupon.LockedOrderID != orderID {
			return infraerrors.Conflict("COUPON_LOCK_MISMATCH", "coupon is not locked by this order")
		}
		newStatus := service.UserCouponStatusAvailable
		if !coupon.ExpiresAt.After(at) {
			newStatus = service.UserCouponStatusExpired
		}
		var row couponUserCouponRow
		err = scanSingleRow(ctx, exec, `
			UPDATE user_coupons SET
				status = $2, locked_order_id = NULL, locked_at = NULL, updated_at = NOW()
			WHERE id = $1
			RETURNING id, template_id, ''::text, user_id, status,
				terms_snapshot, source, source_ref, issue_batch_id,
				idempotency_key, issued_at, valid_from, expires_at,
				locked_order_id, locked_at, used_order_id, used_at,
				voided_at, void_reason, created_at, updated_at`,
			[]any{couponID, newStatus}, row.scanBaseDest()...)
		if err != nil {
			return fmt.Errorf("release user coupon lock: %w", err)
		}
		released, err = row.value()
		if err != nil {
			return err
		}
		return insertCouponEvent(ctx, exec, couponEventInput{
			CouponID:   &released.ID,
			TemplateID: &released.TemplateID,
			EventType:  "released",
			ActorType:  "system",
			OrderID:    &orderID,
			Metadata:   map[string]any{"status": newStatus},
		})
	})
	if err != nil {
		return nil, err
	}
	return released, nil
}

func (r *couponRepository) ConsumeUserCouponOrderLock(ctx context.Context, couponID, orderID int64, at time.Time) (*service.UserCoupon, error) {
	var consumed *service.UserCoupon
	err := r.withTx(ctx, func(exec sqlQueryExecutor) error {
		coupon, err := getUserCoupon(ctx, exec, couponID, true)
		if err != nil {
			return err
		}
		if coupon == nil {
			return infraerrors.NotFound("COUPON_NOT_FOUND", "coupon not found")
		}
		if coupon.Status == service.UserCouponStatusUsed && coupon.UsedOrderID != nil && *coupon.UsedOrderID == orderID {
			consumed = coupon
			return nil
		}
		if coupon.Status != service.UserCouponStatusLocked || coupon.LockedOrderID == nil || *coupon.LockedOrderID != orderID {
			return infraerrors.Conflict("COUPON_LOCK_MISMATCH", "coupon is not locked by this order")
		}
		var row couponUserCouponRow
		err = scanSingleRow(ctx, exec, `
			UPDATE user_coupons SET
				status = 'used', used_order_id = $2, used_at = $3,
				locked_order_id = NULL, locked_at = NULL, updated_at = NOW()
			WHERE id = $1
			RETURNING id, template_id, ''::text, user_id, status,
				terms_snapshot, source, source_ref, issue_batch_id,
				idempotency_key, issued_at, valid_from, expires_at,
				locked_order_id, locked_at, used_order_id, used_at,
				voided_at, void_reason, created_at, updated_at`,
			[]any{couponID, orderID, at}, row.scanBaseDest()...)
		if err != nil {
			return fmt.Errorf("consume user coupon lock: %w", err)
		}
		consumed, err = row.value()
		if err != nil {
			return err
		}
		return insertCouponEvent(ctx, exec, couponEventInput{
			CouponID:   &consumed.ID,
			TemplateID: &consumed.TemplateID,
			EventType:  "used",
			ActorType:  "system",
			OrderID:    &orderID,
			Metadata:   map[string]any{},
		})
	})
	if err != nil {
		return nil, err
	}
	return consumed, nil
}

type couponEventInput struct {
	CouponID     *int64
	TemplateID   *int64
	IssueBatchID *int64
	EventType    string
	ActorType    string
	ActorUserID  *int64
	OrderID      *int64
	Metadata     map[string]any
}

func insertCouponEvent(ctx context.Context, exec sqlQueryExecutor, input couponEventInput) error {
	metadata, err := json.Marshal(input.Metadata)
	if err != nil {
		return fmt.Errorf("encode coupon event metadata: %w", err)
	}
	if input.ActorType == "" {
		input.ActorType = "system"
	}
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO coupon_events (
			coupon_id, template_id, issue_batch_id, event_type,
			actor_type, actor_user_id, order_id, metadata
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		input.CouponID, input.TemplateID, input.IssueBatchID, input.EventType,
		input.ActorType, input.ActorUserID, input.OrderID, metadata,
	); err != nil {
		return fmt.Errorf("insert coupon event: %w", err)
	}
	return nil
}

func couponActorID(actorID int64) *int64 {
	if actorID <= 0 {
		return nil
	}
	value := actorID
	return &value
}

func couponActorType(actorID int64) string {
	if actorID > 0 {
		return "admin"
	}
	return "system"
}

func couponPoolTemplateIDs(pool service.CouponRewardPoolVersion) []int64 {
	seen := make(map[int64]struct{}, len(pool.Entries)+1)
	ids := make([]int64, 0, len(pool.Entries)+1)
	for _, entry := range pool.Entries {
		if _, exists := seen[entry.TemplateID]; exists {
			continue
		}
		seen[entry.TemplateID] = struct{}{}
		ids = append(ids, entry.TemplateID)
	}
	if _, exists := seen[pool.FallbackTemplateID]; !exists {
		ids = append(ids, pool.FallbackTemplateID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func lockCouponRewardPoolTemplates(ctx context.Context, exec sqlQueryExecutor, pool *service.CouponRewardPoolVersion) error {
	if pool == nil {
		return fmt.Errorf("%w: published pool is missing", service.ErrCouponRewardPoolUnavailable)
	}
	for _, templateID := range couponPoolTemplateIDs(*pool) {
		if _, err := getCouponTemplate(ctx, exec, templateID, true); err != nil {
			return err
		}
	}
	return nil
}

func (r *couponRepository) DrawAndIssueCouponRewardInTx(ctx context.Context, request service.CouponRewardDrawRequest) (*service.CouponRewardIssueResult, error) {
	var result *service.CouponRewardIssueResult
	err := r.withTx(ctx, func(exec sqlQueryExecutor) error {
		existing, err := getCouponRewardDrawByIdempotency(ctx, exec, request.IdempotencyKey, true)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.Coupon.UserID != request.UserID {
				return infraerrors.Conflict("COUPON_REWARD_IDEMPOTENCY_CONFLICT", "coupon reward idempotency key belongs to another user")
			}
			result = existing
			return nil
		}

		pool, err := getPublishedCouponRewardPoolForUpdate(ctx, exec, request.Activity)
		if err != nil {
			return err
		}
		if pool == nil {
			return fmt.Errorf("%w: no published %s pool", service.ErrCouponRewardPoolUnavailable, request.Activity)
		}
		if err := service.ValidateCouponRewardPool(*pool); err != nil {
			return fmt.Errorf("%w: invalid published pool: %v", service.ErrCouponRewardPoolUnavailable, err)
		}
		// All template locks are acquired in ascending id order. Different
		// activities can share a template, so retrying random candidates must
		// not introduce cross-pool lock inversions.
		if err := lockCouponRewardPoolTemplates(ctx, exec, pool); err != nil {
			return err
		}

		entry, template, fallbackUsed, err := drawUsableCouponRewardEntry(
			ctx,
			exec,
			pool.Entries,
			pool.FallbackTemplateID,
			request.UserID,
			request.IssuedAt,
		)
		if err != nil {
			return err
		}

		source := service.CouponIssueSourceBlindbox
		if request.Activity == service.CouponRewardActivityQuiz {
			source = service.CouponIssueSourceQuiz
		} else if request.Activity == service.CouponRewardActivityCheckin {
			source = service.CouponIssueSourceCheckin
		}
		coupon, err := issueCouponWithTemplate(ctx, exec, template, service.CouponIssueInput{
			TemplateID:     template.ID,
			UserID:         request.UserID,
			Source:         source,
			SourceRef:      request.SourceRef,
			IdempotencyKey: request.IdempotencyKey,
		}, request.IssuedAt)
		if err != nil {
			return err
		}
		coupon.TemplateName = template.Name
		if err := incrementCouponRewardPoolEntryIssuedCount(ctx, exec, entry.ID); err != nil {
			return err
		}
		if _, err := exec.ExecContext(ctx, `
			INSERT INTO coupon_reward_draws (
				activity, user_id, pool_version_id, pool_entry_id, template_id,
				user_coupon_id, idempotency_key, source_ref, fallback_used
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			request.Activity, request.UserID, pool.ID, entry.ID, template.ID,
			coupon.ID, request.IdempotencyKey, request.SourceRef, fallbackUsed,
		); err != nil {
			if isUniqueConstraintViolation(err) {
				return infraerrors.Conflict("COUPON_REWARD_IDEMPOTENCY_CONFLICT", "coupon reward idempotency key has already been used")
			}
			return fmt.Errorf("insert coupon reward draw: %w", err)
		}
		result = &service.CouponRewardIssueResult{
			PoolVersionID: pool.ID,
			PoolVersion:   pool.Version,
			PoolEntryID:   entry.ID,
			TemplateID:    template.ID,
			UserCouponID:  coupon.ID,
			Coupon:        *coupon,
			ValidFrom:     coupon.ValidFrom,
			ExpiresAt:     coupon.ExpiresAt,
			FallbackUsed:  fallbackUsed,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *couponRepository) FindCouponRewardIssueByIdempotency(ctx context.Context, idempotencyKey string) (*service.CouponRewardIssueResult, error) {
	exec, err := r.exec(ctx)
	if err != nil {
		return nil, err
	}
	return getCouponRewardDrawByIdempotency(ctx, exec, idempotencyKey, false)
}

func getPublishedCouponRewardPoolForUpdate(ctx context.Context, exec sqlQueryExecutor, activity service.CouponRewardActivity) (*service.CouponRewardPoolVersion, error) {
	var id int64
	err := scanSingleRow(ctx, exec, `
		SELECT id FROM coupon_reward_pool_versions
		WHERE activity = $1 AND status = 'published'
		ORDER BY published_at DESC NULLS LAST, id DESC
		LIMIT 1 FOR UPDATE`, []any{activity}, &id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lock published coupon reward pool: %w", err)
	}
	return getCouponRewardPool(ctx, exec, id, true)
}

// drawUsableCouponRewardEntry first filters the currently issuable ordinary
// entries and then draws over their remaining weights. The fallback is not a
// weighted outcome: it is examined only after every ordinary entry is
// unavailable. This keeps an unavailable campaign entry from silently changing
// the configured mix while still allowing the fixed outer game split to settle.
func drawUsableCouponRewardEntry(
	ctx context.Context,
	exec sqlQueryExecutor,
	entries []service.CouponRewardPoolEntry,
	fallbackTemplateID int64,
	userID int64,
	at time.Time,
) (*service.CouponRewardPoolEntry, *service.CouponTemplate, bool, error) {
	ordinaryEntries := couponRewardOrdinaryEntries(entries, fallbackTemplateID)
	candidates, err := listUsableCouponRewardEntries(ctx, exec, ordinaryEntries, userID, at)
	if err != nil {
		return nil, nil, false, err
	}

	for len(candidates) > 0 {
		totalWeight := couponRewardEntryWeightTotal(candidates)
		if totalWeight <= 0 {
			break
		}
		draw, err := rand.Int(rand.Reader, big.NewInt(int64(totalWeight)))
		if err != nil {
			return nil, nil, false, fmt.Errorf("draw usable coupon reward pool entry: %w", err)
		}
		selected := couponRewardEntryForDraw(candidates, int(draw.Int64()))
		if selected == nil {
			return nil, nil, false, fmt.Errorf("%w: no eligible entry for draw", service.ErrCouponRewardPoolUnavailable)
		}

		// Entry rows are locked by the published-pool lock. Re-lock only the
		// chosen template before issuing so shared templates across activities
		// cannot exceed their own issue limit.
		template, usable, err := couponRewardEntryUsable(ctx, exec, *selected, userID, at, true)
		if err != nil {
			return nil, nil, false, err
		}
		if usable {
			return selected, template, false, nil
		}
		candidates = couponRewardEntriesWithoutID(candidates, selected.ID)
	}

	// The fallback does not repair probability mass from an unavailable normal
	// entry. It is the guaranteed result only after every ordinary candidate is
	// unavailable for this user.
	fallback := couponRewardFallbackEntry(entries, fallbackTemplateID)
	if fallback == nil {
		return nil, nil, false, fmt.Errorf("%w: fallback entry is missing", service.ErrCouponRewardPoolUnavailable)
	}
	template, usable, err := couponRewardEntryUsable(ctx, exec, *fallback, userID, at, true)
	if err != nil {
		return nil, nil, false, err
	}
	if !usable {
		return nil, nil, false, fmt.Errorf("%w: fallback coupon cannot be issued", service.ErrCouponRewardPoolUnavailable)
	}
	return fallback, template, true, nil
}

func listUsableCouponRewardEntries(
	ctx context.Context,
	exec sqlQueryExecutor,
	entries []service.CouponRewardPoolEntry,
	userID int64,
	at time.Time,
) ([]service.CouponRewardPoolEntry, error) {
	available := make([]service.CouponRewardPoolEntry, 0, len(entries))
	for _, entry := range entries {
		_, usable, err := couponRewardEntryUsable(ctx, exec, entry, userID, at, false)
		if err != nil {
			return nil, err
		}
		if usable {
			available = append(available, entry)
		}
	}
	return available, nil
}

func couponRewardEntryWeightTotal(entries []service.CouponRewardPoolEntry) int {
	total := 0
	for _, entry := range entries {
		if entry.WeightBP > 0 {
			total += entry.WeightBP
		}
	}
	return total
}

func couponRewardEntriesWithoutID(entries []service.CouponRewardPoolEntry, id int64) []service.CouponRewardPoolEntry {
	filtered := make([]service.CouponRewardPoolEntry, 0, len(entries)-1)
	for _, entry := range entries {
		if entry.ID != id {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func couponRewardOrdinaryEntries(entries []service.CouponRewardPoolEntry, fallbackTemplateID int64) []service.CouponRewardPoolEntry {
	ordinary := make([]service.CouponRewardPoolEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.TemplateID != fallbackTemplateID {
			ordinary = append(ordinary, entry)
		}
	}
	return ordinary
}

// couponRewardEntryForDraw expects draw in [0, sum(entry.weight_bp)). The
// entries can therefore represent either the configured 10,000bp pool or a
// dynamically filtered subset with a smaller total.
func couponRewardEntryForDraw(entries []service.CouponRewardPoolEntry, draw int) *service.CouponRewardPoolEntry {
	if draw < 0 {
		return nil
	}
	var cursor int
	for i := range entries {
		if entries[i].WeightBP <= 0 {
			continue
		}
		cursor += entries[i].WeightBP
		if draw < cursor {
			return &entries[i]
		}
	}
	return nil
}

func couponRewardFallbackEntry(entries []service.CouponRewardPoolEntry, templateID int64) *service.CouponRewardPoolEntry {
	for i := range entries {
		if entries[i].TemplateID == templateID && entries[i].Enabled {
			return &entries[i]
		}
	}
	return nil
}

func couponRewardEntryUsable(
	ctx context.Context,
	exec sqlQueryExecutor,
	entry service.CouponRewardPoolEntry,
	userID int64,
	at time.Time,
	lockTemplate bool,
) (*service.CouponTemplate, bool, error) {
	if !couponRewardEntryWindowHasCapacity(entry, at) {
		return nil, false, nil
	}
	if entry.PerUserIssueLimit != nil {
		var count int64
		if err := scanSingleRow(ctx, exec, `
			SELECT COUNT(*) FROM coupon_reward_draws
			WHERE pool_entry_id = $1 AND user_id = $2`, []any{entry.ID, userID}, &count); err != nil {
			return nil, false, fmt.Errorf("count coupon reward entry user issues: %w", err)
		}
		if count >= int64(*entry.PerUserIssueLimit) {
			return nil, false, nil
		}
	}
	template, err := getCouponTemplate(ctx, exec, entry.TemplateID, lockTemplate)
	if err != nil {
		return nil, false, err
	}
	if !couponRewardTemplateIssuable(template, at) {
		return nil, false, nil
	}
	return template, true, nil
}

func couponRewardEntryWindowHasCapacity(entry service.CouponRewardPoolEntry, at time.Time) bool {
	if !entry.Enabled || entry.TemplateID <= 0 {
		return false
	}
	if entry.StartsAt != nil && entry.StartsAt.After(at) {
		return false
	}
	if entry.EndsAt != nil && !entry.EndsAt.After(at) {
		return false
	}
	return entry.StockCap == nil || entry.IssuedCount < *entry.StockCap
}

func couponRewardTemplateIssuable(template *service.CouponTemplate, at time.Time) bool {
	if template == nil || template.Status != service.CouponTemplateStatusActive {
		return false
	}
	if template.TotalIssueLimit != nil && template.IssuedCount >= *template.TotalIssueLimit {
		return false
	}
	_, expiresAt, err := service.CouponExpiryForIssue(*template, at)
	return err == nil && expiresAt.After(at)
}

func incrementCouponRewardPoolEntryIssuedCount(ctx context.Context, exec sqlQueryExecutor, entryID int64) error {
	result, err := exec.ExecContext(ctx, `
		UPDATE coupon_reward_pool_entries
		SET issued_count = issued_count + 1, updated_at = NOW()
		WHERE id = $1
		  AND (stock_cap IS NULL OR issued_count < stock_cap)`, entryID)
	if err != nil {
		return fmt.Errorf("increment coupon reward pool entry issued count: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read coupon reward pool entry increment result: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: selected entry capacity changed", service.ErrCouponRewardPoolUnavailable)
	}
	return nil
}

func getCouponRewardDrawByIdempotency(ctx context.Context, exec sqlQueryExecutor, key string, forUpdate bool) (*service.CouponRewardIssueResult, error) {
	query := `SELECT d.pool_version_id, p.version, d.pool_entry_id, d.template_id,
		d.user_coupon_id, d.fallback_used,
		uc.id, uc.template_id, COALESCE(t.name, ''), uc.user_id, uc.status,
		uc.terms_snapshot, uc.source, uc.source_ref, uc.issue_batch_id,
		uc.idempotency_key, uc.issued_at, uc.valid_from, uc.expires_at,
		uc.locked_order_id, uc.locked_at, uc.used_order_id, uc.used_at,
		uc.voided_at, uc.void_reason, uc.created_at, uc.updated_at
		FROM coupon_reward_draws d
		JOIN coupon_reward_pool_versions p ON p.id = d.pool_version_id
		JOIN user_coupons uc ON uc.id = d.user_coupon_id
		JOIN coupon_templates t ON t.id = uc.template_id
		WHERE d.idempotency_key = $1`
	if forUpdate {
		query += ` FOR UPDATE OF d, uc`
	}
	var row couponRewardDrawRow
	err := scanSingleRow(ctx, exec, query, []any{key}, row.scanDest()...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get coupon reward draw by idempotency: %w", err)
	}
	return row.value()
}

type couponRewardDrawRow struct {
	result       service.CouponRewardIssueResult
	coupon       couponUserCouponRow
	userCouponID int64
}

func (row *couponRewardDrawRow) scanDest() []any {
	return append([]any{
		&row.result.PoolVersionID, &row.result.PoolVersion, &row.result.PoolEntryID, &row.result.TemplateID,
		&row.userCouponID, &row.result.FallbackUsed,
	}, row.coupon.scanBaseDest()...)
}

func (row *couponRewardDrawRow) value() (*service.CouponRewardIssueResult, error) {
	coupon, err := row.coupon.value()
	if err != nil {
		return nil, err
	}
	row.result.UserCouponID = row.userCouponID
	row.result.Coupon = *coupon
	row.result.ValidFrom = coupon.ValidFrom
	row.result.ExpiresAt = coupon.ExpiresAt
	return &row.result, nil
}
