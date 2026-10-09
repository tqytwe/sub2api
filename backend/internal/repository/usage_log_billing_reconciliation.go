package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// These indexes are checked against prepareUsageLogInsert by dedicated tests.
const (
	usageLogActualCostArg         = 26
	usageLogSurchargeCostArg      = 62
	usageLogBilledCostArg         = 63
	usageLogSurchargeModeArg      = 64
	usageLogSurchargeValueArg     = 65
	usageLogBillingFingerprintArg = 70
	usageLogBillingSettledArg     = 71
)

const usageLogBillingConflictClause = `ON CONFLICT (request_id, api_key_id) DO UPDATE SET
 actual_cost = EXCLUDED.actual_cost,
 billed_cost = EXCLUDED.billed_cost,
 billing_surcharge_cost = EXCLUDED.billing_surcharge_cost,
 billing_surcharge_mode = EXCLUDED.billing_surcharge_mode,
 billing_surcharge_value = EXCLUDED.billing_surcharge_value,
 billing_settled = TRUE
 WHERE usage_logs.billing_request_fingerprint IS NOT NULL
   AND usage_logs.billing_request_fingerprint <> ''
   AND usage_logs.billing_request_fingerprint = EXCLUDED.billing_request_fingerprint
   AND NOT usage_logs.billing_settled
   AND EXCLUDED.billing_settled
   AND (SELECT COUNT(*) > 0 AND BOOL_AND(proof.request_fingerprint = EXCLUDED.billing_request_fingerprint)
        FROM (
          SELECT request_fingerprint FROM usage_billing_dedup
          WHERE request_id = EXCLUDED.request_id AND api_key_id = EXCLUDED.api_key_id
          UNION ALL
          SELECT request_fingerprint FROM usage_billing_dedup_archive
          WHERE request_id = EXCLUDED.request_id AND api_key_id = EXCLUDED.api_key_id
        ) AS proof)`

func usageLogBillingQuery(query string, enabled bool, returning string) string {
	if !enabled {
		return query
	}
	query = strings.Replace(query, "ON CONFLICT (request_id, api_key_id) DO NOTHING", usageLogBillingConflictClause, 1)
	// PostgreSQL-specific tuple metadata distinguishes a fresh insertion from
	// repair. Integration tests guard this dependency; this flag never bills money.
	switch returning {
	case "single":
		query = strings.Replace(query, "RETURNING id, created_at", "RETURNING id, created_at, (xmax = 0) AS inserted", 1)
	case "batch":
		query = strings.Replace(query, "RETURNING request_id, api_key_id, id, created_at", "RETURNING request_id, api_key_id, id, created_at, (xmax = 0) AS actually_inserted", 1)
		query = strings.Replace(query, "(inserted.id IS NOT NULL) AS inserted", "COALESCE(inserted.actually_inserted, FALSE) AS inserted", 1)
	}
	return query
}

func mergeUsageLogBillingPrepared(first, incoming usageLogInsertPrepared) usageLogInsertPrepared {
	if first.billingFingerprint == "" || first.billingFingerprint != incoming.billingFingerprint ||
		first.billingSettled || !incoming.billingSettled {
		return first
	}
	first.args = append([]any(nil), first.args...)
	for _, index := range []int{usageLogActualCostArg, usageLogSurchargeCostArg, usageLogBilledCostArg, usageLogSurchargeModeArg, usageLogSurchargeValueArg, usageLogBillingSettledArg} {
		first.args[index] = incoming.args[index]
	}
	first.billingSettled = true
	return first
}

func usageLogPreparedHasBilling(prepared []usageLogInsertPrepared) bool {
	for _, row := range prepared {
		if row.billingFingerprint != "" {
			return true
		}
	}
	return false
}

// A concurrent ON CONFLICT no-op can wait for a newly committed row which was
// absent from the INSERT statement's snapshot. Resolve only those missing rows
// in one fresh statement, within the original batch deadline. No extra query is
// added to the usual insert/promotion path, and this never claims a new insert.
func resolveUsageLogBatchMissingStates(ctx context.Context, db *sql.DB, keys []string, preparedByKey map[string]usageLogInsertPrepared, inserted map[string]bool, states map[string]usageLogBatchState) error {
	conditions := make([]string, 0)
	args := make([]any, 0)
	for _, key := range keys {
		if state, ok := states[key]; ok && state.ID > 0 && !state.CreatedAt.IsZero() {
			continue
		}
		delete(states, key)
		inserted[key] = false
		prepared, ok := preparedByKey[key]
		if !ok || len(prepared.args) < 2 {
			return fmt.Errorf("usage log batch prepared state missing")
		}
		args = append(args, prepared.requestID, prepared.args[1])
		conditions = append(conditions, fmt.Sprintf("(request_id = $%d AND api_key_id = $%d)", len(args)-1, len(args)))
	}
	if len(conditions) == 0 {
		return nil
	}
	rows, err := db.QueryContext(ctx, "SELECT request_id, api_key_id, id, created_at FROM usage_logs WHERE "+strings.Join(conditions, " OR "), args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var requestID string
		var apiKeyID int64
		var state usageLogBatchState
		if err := rows.Scan(&requestID, &apiKeyID, &state.ID, &state.CreatedAt); err != nil {
			return err
		}
		if state.ID > 0 && !state.CreatedAt.IsZero() {
			key := usageLogBatchKey(requestID, apiKeyID)
			states[key] = state
			inserted[key] = false
		}
	}
	return rows.Err()
}
