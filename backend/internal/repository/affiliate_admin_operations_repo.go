package repository

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"time"
)

func writeAffiliateOperationsAudit(ctx context.Context, client affiliateQueryExecer, userID, actorID int64, action string, detail any) error {
	data, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = client.ExecContext(ctx, `INSERT INTO affiliate_operations_audit_logs(user_id,actor_id,action,detail) VALUES(NULLIF($1,0),NULLIF($2,0),$3,$4::jsonb)`, userID, actorID, action, string(data))
	return err
}

// Every setting is validated before this transaction. The row lock and audit
// share the commit, so a failure never leaves half of a settings change applied.
func (r *affiliateRepository) UpdateAffiliateSettings(ctx context.Context, userID int64, change service.AffiliateSettingsChange) error {
	return r.withTx(ctx, func(ctx context.Context, client *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(ctx, client, userID); err != nil {
			return err
		}
		var before string
		if err := scanAffiliateRow(ctx, client, `SELECT jsonb_build_object('aff_code',aff_code,'custom',aff_code_custom,'rate',aff_rebate_rate_percent)::text FROM user_affiliates WHERE user_id=$1 FOR UPDATE`, []any{userID}, &before); err != nil {
			return err
		}
		var code any
		if change.Code != nil {
			code = *change.Code
		}
		if change.Reset {
			generated, err := generateAffiliateCode()
			if err != nil {
				return err
			}
			code = generated
		}
		_, err := client.ExecContext(ctx, `UPDATE user_affiliates SET
 aff_code=COALESCE($2::varchar,aff_code),
 aff_code_custom=CASE WHEN $3 THEN false WHEN $2::varchar IS NOT NULL THEN true ELSE aff_code_custom END,
 aff_rebate_rate_percent=CASE WHEN $3 OR $4 THEN NULL WHEN $5::numeric IS NOT NULL THEN $5::numeric ELSE aff_rebate_rate_percent END,
 updated_at=NOW() WHERE user_id=$1`, userID, code, change.Reset, change.ClearRate, change.Rate)
		if isAffiliateUniqueViolation(err) {
			return service.ErrAffiliateCodeTaken
		}
		if err != nil {
			return err
		}
		var after string
		if err := scanAffiliateRow(ctx, client, `SELECT jsonb_build_object('aff_code',aff_code,'custom',aff_code_custom,'rate',aff_rebate_rate_percent)::text FROM user_affiliates WHERE user_id=$1`, []any{userID}, &after); err != nil {
			return err
		}
		return writeAffiliateOperationsAudit(ctx, client, userID, change.ActorID, "settings_changed", map[string]any{"before": json.RawMessage(before), "after": json.RawMessage(after)})
	})
}

// The existing quota deduction, proof and audit run in one outer transaction.
// A replay compares its proof too; it never mutates an earlier payment record.
func (r *affiliateRepository) WithdrawQuotaWithEvidence(ctx context.Context, userID int64, amount float64, operationID string, evidence service.AffiliateWithdrawalEvidence) (*service.AffiliateWithdrawResult, error) {
	var result *service.AffiliateWithdrawResult
	err := r.withTx(ctx, func(ctx context.Context, client *dbent.Client) error {
		var err error
		result, err = r.WithdrawQuota(ctx, userID, amount, operationID)
		if err != nil {
			return err
		}
		paidAt, err := time.Parse(time.RFC3339, evidence.PaidAt)
		if err != nil {
			return err
		}
		paidAt = paidAt.UTC().Truncate(time.Microsecond)
		evidence.PaidAt = paidAt.Format(time.RFC3339Nano)
		if result.Replayed {
			var ref, note string
			var when sql.NullTime
			var actor sql.NullInt64
			if err = scanAffiliateRow(ctx, client, `SELECT COALESCE(external_reference,''),paid_at,COALESCE(operation_note,''),actor_id FROM user_affiliate_ledger WHERE id=$1`, []any{result.LedgerID}, &ref, &when, &note, &actor); err != nil {
				return err
			}
			if ref != evidence.ExternalReference || note != evidence.Note || !when.Valid || !when.Time.Equal(paidAt) {
				return service.ErrIdempotencyKeyConflict
			}
			result.AffiliateWithdrawalEvidence = service.AffiliateWithdrawalEvidence{ExternalReference: ref, Note: note, PaidAt: when.Time.Format(time.RFC3339Nano), ActorID: actor.Int64}
			return nil
		}
		if _, err = client.ExecContext(ctx, `UPDATE user_affiliate_ledger SET external_reference=$2,paid_at=$3,operation_note=$4,actor_id=NULLIF($5,0) WHERE id=$1`, result.LedgerID, evidence.ExternalReference, paidAt, evidence.Note, evidence.ActorID); err != nil {
			return err
		}
		result.AffiliateWithdrawalEvidence = evidence
		return writeAffiliateOperationsAudit(ctx, client, userID, evidence.ActorID, "offline_withdrawal", map[string]any{"ledger_id": result.LedgerID, "amount": amount, "external_reference": evidence.ExternalReference, "paid_at": evidence.PaidAt, "note": evidence.Note})
	})
	return result, err
}

// Public inputs select only known columns. Values are always bound parameters.
func affiliateExactRecordWhere(f service.AffiliateRecordFilter, kind, where string, args []any) (string, []any) {
	add := func(column string, value any) {
		args = append(args, value)
		clause := fmt.Sprintf("%s = $%d", column, len(args))
		if where == "" {
			where = "WHERE " + clause
		} else {
			where += " AND " + clause
		}
	}
	if kind == "invites" {
		if f.InviterID > 0 {
			add("ua.inviter_id", f.InviterID)
		}
		if f.InviteeID > 0 {
			add("ua.user_id", f.InviteeID)
		}
		if f.UserID > 0 {
			add("ua.inviter_id", f.UserID)
		}
	} else {
		if f.InviterID > 0 {
			add("ual.user_id", f.InviterID)
		}
		if f.InviteeID > 0 {
			add("ual.source_user_id", f.InviteeID)
		}
		if f.SourceOrderID > 0 {
			add("ual.source_order_id", f.SourceOrderID)
		}
		if f.CampaignID > 0 {
			add("rr.campaign_id", f.CampaignID)
		}
		if f.UserID > 0 {
			add("ual.user_id", f.UserID)
		}
		if f.Action != "" {
			add("ual.action", f.Action)
		}
		appendClause := func(clause string) {
			if where == "" {
				where = "WHERE " + clause
			} else {
				where += " AND " + clause
			}
		}
		if f.Currency != "" {
			add("COALESCE(rr.currency,'USD')", f.Currency)
		}
		switch f.SourceType {
		case "campaign":
			appendClause("ual.referral_reward_id IS NOT NULL")
		case "regular":
			appendClause("ual.referral_reward_id IS NULL AND ual.source_order_id IS NOT NULL")
		case "other":
			appendClause("ual.referral_reward_id IS NULL AND ual.source_order_id IS NULL")
		}
		switch f.Status {
		case "frozen":
			appendClause("ual.action='accrue' AND COALESCE(rr.status,'')<>'revoked' AND ual.frozen_until > NOW()")
		case "available":
			appendClause("ual.action='accrue' AND COALESCE(rr.status,'')<>'revoked' AND (ual.frozen_until IS NULL OR ual.frozen_until <= NOW())")
		case "revoked":
			appendClause("(ual.action='revoke' OR rr.status='revoked')")
		}
	}
	return where, args
}

func affiliateRecordScope(kind string, f service.AffiliateRecordFilter) (string, string, []any) {
	var from, where string
	var args []any
	switch kind {
	case "invites":
		from = `FROM user_affiliates ua JOIN users invitee ON invitee.id=ua.user_id JOIN users inviter ON inviter.id=ua.inviter_id JOIN user_affiliates inviter_aff ON inviter_aff.user_id=ua.inviter_id`
		where, args = buildAffiliateRecordWhere(f, "ua.created_at", []string{"inviter.email", "inviter.username", "invitee.email", "invitee.username", "ua.inviter_id::text", "ua.user_id::text", "inviter_aff.aff_code"})
	case "rebates":
		from = `FROM user_affiliate_ledger ual JOIN users inviter ON inviter.id=ual.user_id LEFT JOIN users invitee ON invitee.id=ual.source_user_id LEFT JOIN payment_orders po ON po.id=ual.source_order_id LEFT JOIN referral_campaign_rewards rr ON rr.id=ual.referral_reward_id`
		where, args = buildAffiliateRecordWhere(f, "ual.created_at", []string{"inviter.email", "inviter.username", "invitee.email", "invitee.username", "po.id::text", "po.out_trade_no", "po.payment_type", "po.status"})
		if where == "" {
			where = "WHERE ual.action IN ('accrue','revoke')"
		} else {
			where += " AND ual.action IN ('accrue','revoke')"
		}
	default:
		from = `FROM user_affiliate_ledger ual JOIN users u ON u.id=ual.user_id LEFT JOIN referral_campaign_rewards rr ON rr.id=ual.referral_reward_id`
		where, args = buildAffiliateRecordWhere(f, "ual.created_at", []string{"u.email", "u.username", "u.id::text"})
		if where == "" {
			where = "WHERE ual.action IN ('transfer', 'withdraw')"
		} else {
			where += " AND ual.action IN ('transfer', 'withdraw')"
		}
	}
	where, args = affiliateExactRecordWhere(f, kind, where, args)
	return from, where, args
}

func (r *affiliateRepository) AffiliateRecordReport(ctx context.Context, kind string, filter service.AffiliateRecordFilter, export bool, actorID int64) (*service.AffiliateRecordReport, error) {
	client := clientFromContext(ctx, r.client)
	from, where, args := affiliateRecordScope(kind, filter)
	out := &service.AffiliateRecordReport{Summary: service.AffiliateRecordSummary{Currency: "USD", Amounts: []service.AffiliateRecordAmounts{}}, Rows: []map[string]any{}}
	if err := scanAffiliateRow(ctx, client, "SELECT COUNT(*) "+from+" "+where, args, &out.Summary.RecordCount); err != nil {
		return nil, err
	}
	if export && out.Summary.RecordCount > 10000 {
		return nil, infraerrors.New(422, "AFFILIATE_EXPORT_TOO_LARGE", "export is limited to 10000 records; narrow the filters")
	}
	if kind == "invites" {
		// Relationship counts are not earnings. Money is available on the rebate
		// report with its own time, source and currency filters.
		out.Summary.Currency = "n/a"
	} else {
		source := "'wallet'"
		currency := "'USD'"
		if kind == "rebates" {
			source = "CASE WHEN rr.id IS NOT NULL THEN 'campaign' WHEN ual.source_order_id IS NOT NULL THEN 'regular' ELSE 'other' END"
			currency = "COALESCE(rr.currency,'USD')"
		}
		q := `SELECT ` + source + `,` + currency + `,COALESCE(SUM(ual.amount),0)::float8,COALESCE(SUM(ual.amount) FILTER(WHERE ual.action='accrue' AND COALESCE(rr.status,'')<>'revoked' AND (ual.frozen_until IS NULL OR ual.frozen_until<=NOW())),0)::float8,COALESCE(SUM(ual.amount) FILTER(WHERE ual.action='accrue' AND COALESCE(rr.status,'')<>'revoked' AND ual.frozen_until>NOW()),0)::float8 ` + from + " " + where + " GROUP BY 1,2 ORDER BY 1,2"
		rows, err := client.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, err
		}
		currencies := map[string]bool{}
		total := 0.0
		for rows.Next() {
			var a service.AffiliateRecordAmounts
			if err = rows.Scan(&a.SourceType, &a.Currency, &a.TotalAmount, &a.AvailableAmount, &a.FrozenAmount); err != nil {
				_ = rows.Close()
				return nil, err
			}
			out.Summary.Amounts = append(out.Summary.Amounts, a)
			currencies[a.Currency] = true
			total += a.TotalAmount
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
		if len(currencies) > 1 {
			out.Summary.Currency = "mixed"
		} else {
			out.Summary.TotalAmount = &total
			for currency := range currencies {
				out.Summary.Currency = currency
			}
		}
	}
	if !export {
		return out, nil
	}
	filter.Page = 1
	filter.PageSize = 10001
	var records any
	var err error
	switch kind {
	case "invites":
		records, _, err = r.ListAffiliateInviteRecords(ctx, filter)
	case "rebates":
		records, _, err = r.ListAffiliateRebateRecords(ctx, filter)
	case "transfers":
		records, _, err = r.ListAffiliateTransferRecords(ctx, filter)
	}
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(records)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	if err = decoder.Decode(&out.Rows); err != nil {
		return nil, err
	}
	if len(out.Rows) > 10000 {
		return nil, infraerrors.New(422, "AFFILIATE_EXPORT_TOO_LARGE", "export is limited to 10000 records; narrow the filters")
	}
	if err = writeAffiliateOperationsAudit(ctx, client, filter.UserID, actorID, "records_exported", map[string]any{"kind": kind, "filter": filter, "row_count": len(out.Rows), "timezone": filter.Timezone}); err != nil {
		return nil, err
	}
	return out, nil
}

var _ service.AffiliateAtomicSettingsRepository = (*affiliateRepository)(nil)
var _ service.AffiliateEvidenceWithdrawalRepository = (*affiliateRepository)(nil)
var _ service.AffiliateRecordReportingRepository = (*affiliateRepository)(nil)
