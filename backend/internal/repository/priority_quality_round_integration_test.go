//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func priorityQualityFixture(t *testing.T) (*usageLogRepository, int64, time.Time) {
	t.Helper()
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "priority-quality-" + uuid.NewString(), Platform: service.PlatformOpenAI})
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(context.Background(), `DELETE FROM accounts WHERE id=$1`, account.ID)
		require.NoError(t, err)
	})
	return &usageLogRepository{sql: integrationDB}, account.ID, time.Now().UTC().Truncate(time.Microsecond)
}

func priorityQualityPlan(t *testing.T, accountID int64, action string, revision time.Time) int64 {
	t.Helper()
	var id int64
	config := fmt.Sprintf(`{"quality":{"action":%q},"parallel_count":2}`, action)
	err := integrationDB.QueryRowContext(context.Background(), `INSERT INTO scheduled_test_plans
		(account_id,model_id,pelican_config,quality_rule_updated_at) VALUES($1,'public-model',$2,$3) RETURNING id`, accountID, config, revision).Scan(&id)
	require.NoError(t, err)
	return id
}

func priorityQualityResult(t *testing.T, planID int64, round, verdict, action, model string, parallel any, at time.Time) {
	t.Helper()
	status, message := "success", ""
	if verdict == "incorrect" {
		status, message = "failed", "answer_mismatch"
	} else if verdict == "unknown" {
		status, message = "failed", "probe_failed"
	}
	config, err := json.Marshal(map[string]any{"model_id": model, "parallel_count": parallel})
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(context.Background(), `INSERT INTO scheduled_test_results
		(plan_id,status,error_message,started_at,finished_at,created_at,pelican_config,quality_round_id,quality_action,quality_judgment)
		VALUES($1,$2,$3,$4,$4,$4,$5,$6,$7,jsonb_build_object('verdict',$8::text))`, planID, status, message, at, string(config), round, action, verdict)
	require.NoError(t, err)
}

func TestPriorityQualityRoundsPostgreSQLAttributionAndCompleteness(t *testing.T) {
	repo, accountID, now := priorityQualityFixture(t)
	observe := priorityQualityPlan(t, accountID, "observe_only", now.Add(-time.Hour))
	quarantine := priorityQualityPlan(t, accountID, "disable_scheduling", now.Add(-time.Hour))
	at := now.Add(-time.Minute)
	for i := 0; i < 2; i++ {
		// Opaque round IDs may coincide across plans; they are two distinct rounds.
		priorityQualityResult(t, observe, "same-opaque-id", "correct", "observed", "public-model", 2, at)
		priorityQualityResult(t, quarantine, "same-opaque-id", "incorrect", "scheduling_disabled", "public-model", 2, at)
		priorityQualityResult(t, observe, "unknown", "unknown", "inconclusive", "public-model", 2, at)
		priorityQualityResult(t, observe, "before-rule-update", "correct", "observed", "public-model", 2, now.Add(-2*time.Hour))
		priorityQualityResult(t, observe, "future", "correct", "observed", "public-model", 2, now.Add(time.Hour))
		priorityQualityResult(t, observe, "wrong-model", "correct", "observed", "different-model", 2, at)
		priorityQualityResult(t, observe, "malformed-parallel", "correct", "observed", "public-model", "2", at)
		priorityQualityResult(t, observe, "", "correct", "observed", "public-model", 2, at)
		priorityQualityResult(t, observe, "old-window", "correct", "observed", "public-model", 2, now.Add(-25*time.Hour))
	}
	// A later wrong-model member cannot be filtered away to fake a complete round.
	priorityQualityResult(t, observe, "extra-wrong-model", "correct", "observed", "public-model", 2, at)
	priorityQualityResult(t, observe, "extra-wrong-model", "correct", "observed", "public-model", 2, at)
	priorityQualityResult(t, observe, "extra-wrong-model", "correct", "observed", "other-model", 2, at)
	// Completion after an edit cannot legitimize a probe that started before it.
	for i := 0; i < 2; i++ {
		priorityQualityResult(t, observe, "crossed-revision", "correct", "observed", "public-model", 2, at)
	}
	_, err := integrationDB.ExecContext(context.Background(), `UPDATE scheduled_test_results SET started_at=$2 WHERE plan_id=$1 AND quality_round_id='crossed-revision'`, observe, now.Add(-2*time.Hour))
	require.NoError(t, err)
	priorityQualityResult(t, observe, "partial", "correct", "observed", "public-model", 2, at)
	priorityQualityResult(t, observe, "mixed-unknown", "correct", "observed", "public-model", 2, at)
	priorityQualityResult(t, observe, "mixed-unknown", "unknown", "inconclusive", "public-model", 2, at)
	priorityQualityResult(t, observe, "stale", "correct", "observed", "public-model", 2, at)
	priorityQualityResult(t, observe, "stale", "correct", "stale_run", "public-model", 2, at)
	priorityQualityResult(t, observe, "inconsistent-parallel", "correct", "observed", "public-model", 1, at)
	priorityQualityResult(t, observe, "inconsistent-parallel", "correct", "observed", "public-model", 2, at)
	priorityQualityResult(t, observe, "null-model-member", "correct", "observed", "public-model", 2, at)
	priorityQualityResult(t, observe, "null-model-member", "correct", "observed", "public-model", 2, at)
	_, err = integrationDB.ExecContext(context.Background(), `UPDATE scheduled_test_results SET pelican_config=jsonb_set(pelican_config,'{model_ids}','[null]') WHERE id=(SELECT MAX(id) FROM scheduled_test_results WHERE plan_id=$1 AND quality_round_id='null-model-member')`, observe)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		priorityQualityResult(t, observe, "excess-results", "correct", "observed", "public-model", 2, at)
	}
	for _, measurement := range []string{"legacy_forward", "attributed_v1"} {
		t.Run(measurement, func(t *testing.T) {
			query := service.PrioritySchedulingQuery{AccountIDs: []int64{accountID}, Model: "public-model", InboundEndpoint: "/v1/responses", Since: now.Add(-30 * time.Second), LatencyMeasurement: measurement}
			query.QualitySince = now.Add(-24 * time.Hour)
			signals, err := repo.ReadPrioritySchedulingSignals(context.Background(), query)
			require.NoError(t, err)
			require.Equal(t, 2, signals[accountID].QualitySamples)
			require.Equal(t, 1, signals[accountID].QualityPassed)
			require.Zero(t, signals[accountID].Samples, "quality does not fabricate latency history")
			for _, model := range []string{"absent-model", "different-model"} {
				query.Model = model
				signals, err = repo.ReadPrioritySchedulingSignals(context.Background(), query)
				require.NoError(t, err)
				require.Zero(t, signals[accountID].QualitySamples, "the runner executes plan.model_id; a contradictory snapshot cannot reattribute it")
			}
		})
	}
}

func TestPriorityQualityRoundsPostgreSQLRuleRevisionAndAccountScope(t *testing.T) {
	repo, accountID, now := priorityQualityFixture(t)
	planID := priorityQualityPlan(t, accountID, "observe_only", now.Add(-time.Hour))
	at := now.Add(-time.Minute)
	for i := 0; i < 2; i++ {
		priorityQualityResult(t, planID, "valid", "correct", "observed", "public-model", 2, at)
	}
	query := service.PrioritySchedulingQuery{AccountIDs: []int64{accountID}, Model: "public-model", Since: now.Add(-24 * time.Hour)}
	read := func(want int) {
		t.Helper()
		signals, err := repo.ReadPrioritySchedulingSignals(context.Background(), query)
		require.NoError(t, err)
		require.Equal(t, want, signals[accountID].QualitySamples)
	}
	read(1)
	// Runtime lease/finish writes must not invalidate an unchanged rule.
	_, err := integrationDB.ExecContext(context.Background(), `UPDATE scheduled_test_plans SET running_until=clock_timestamp()+interval '1 minute',updated_at=clock_timestamp() WHERE id=$1`, planID)
	require.NoError(t, err)
	read(1)
	_, err = integrationDB.ExecContext(context.Background(), `UPDATE scheduled_test_plans SET running_until=NULL,last_run_at=clock_timestamp(),updated_at=clock_timestamp() WHERE id=$1`, planID)
	require.NoError(t, err)
	read(1)
	// A real rule edit invalidates existing rounds, even though the model stayed the same.
	_, err = integrationDB.ExecContext(context.Background(), `UPDATE scheduled_test_plans SET pelican_config=jsonb_set(pelican_config,'{quality,expected_answer}','"changed"') WHERE id=$1`, planID)
	require.NoError(t, err)
	read(0)
	addFreshRound := func(round string) {
		t.Helper()
		for i := 0; i < 2; i++ {
			priorityQualityResult(t, planID, round, "correct", "observed", "public-model", 2, time.Now().Add(-time.Microsecond))
		}
		read(1)
	}
	addFreshRound("after-edit")
	_, err = integrationDB.ExecContext(context.Background(), `UPDATE scheduled_test_plans SET enabled=false WHERE id=$1`, planID)
	require.NoError(t, err)
	read(0)
	_, err = integrationDB.ExecContext(context.Background(), `UPDATE scheduled_test_plans SET enabled=true WHERE id=$1`, planID)
	require.NoError(t, err)
	read(0)
	addFreshRound("after-enable")
	_, err = integrationDB.ExecContext(context.Background(), `UPDATE accounts SET platform='anthropic' WHERE id=$1`, accountID)
	require.NoError(t, err)
	read(0)
	_, err = integrationDB.ExecContext(context.Background(), `UPDATE accounts SET platform='openai' WHERE id=$1`, accountID)
	require.NoError(t, err)
	read(1)
	_, err = integrationDB.ExecContext(context.Background(), `UPDATE accounts SET deleted_at=clock_timestamp() WHERE id=$1`, accountID)
	require.NoError(t, err)
	read(0)
}

func TestPriorityQualityRoundsPostgreSQLKnownMismatchAndSingleProbeDefaults(t *testing.T) {
	repo, accountID, now := priorityQualityFixture(t)
	planID := priorityQualityPlan(t, accountID, "observe_only", now.Add(-time.Hour))
	at := now.Add(-time.Minute)
	// As upstream, a complete round containing a proven answer mismatch fails
	// even when a different completed probe is inconclusive.
	priorityQualityResult(t, planID, "known-mismatch", "incorrect", "observed", "public-model", 2, at)
	priorityQualityResult(t, planID, "known-mismatch", "unknown", "inconclusive", "public-model", 2, at)
	query := service.PrioritySchedulingQuery{AccountIDs: []int64{accountID}, Model: "public-model", Since: now.Add(-24 * time.Hour)}
	signals, err := repo.ReadPrioritySchedulingSignals(context.Background(), query)
	require.NoError(t, err)
	require.Equal(t, 1, signals[accountID].QualitySamples)
	require.Zero(t, signals[accountID].QualityPassed)
	// Go omits zero parallel_count; both an omitted field and explicit zero
	// represent the existing single-probe default, not an incomplete round.
	_, err = integrationDB.ExecContext(context.Background(), `UPDATE scheduled_test_plans SET pelican_config=jsonb_set(pelican_config,'{parallel_count}','0') WHERE id=$1`, planID)
	require.NoError(t, err)
	for _, round := range []string{"default-zero", "default-omitted"} {
		priorityQualityResult(t, planID, round, "correct", "observed", "public-model", 0, time.Now().Add(-time.Microsecond))
	}
	_, err = integrationDB.ExecContext(context.Background(), `UPDATE scheduled_test_results SET pelican_config=pelican_config-'parallel_count' WHERE plan_id=$1 AND quality_round_id='default-omitted'`, planID)
	require.NoError(t, err)
	signals, err = repo.ReadPrioritySchedulingSignals(context.Background(), query)
	require.NoError(t, err)
	require.Equal(t, 2, signals[accountID].QualitySamples)
	require.Equal(t, 2, signals[accountID].QualityPassed)
	// A multi-model plan lacks trustworthy per-probe model attribution in the
	// current runner, including old snapshots which omit model_ids entirely.
	_, err = integrationDB.ExecContext(context.Background(), `UPDATE scheduled_test_plans SET pelican_config=jsonb_set(pelican_config,'{model_ids}','["public-model","other-model"]') WHERE id=$1`, planID)
	require.NoError(t, err)
	priorityQualityResult(t, planID, "multi-model-unknown", "correct", "observed", "public-model", 0, time.Now().Add(-time.Microsecond))
	signals, err = repo.ReadPrioritySchedulingSignals(context.Background(), query)
	require.NoError(t, err)
	require.Zero(t, signals[accountID].QualitySamples)
}

func TestPriorityQualityRoundsPostgreSQLMultiModelAttribution(t *testing.T) {
	type probe struct {
		model, verdict, action string
		parallel               int
		omitTargets, omitModel bool
		omitParallel           bool
		targets                []string
	}
	pair := func(a, b string) []probe {
		return []probe{{model: "public-model", verdict: a, parallel: 2},
			{model: "other-model", verdict: b, parallel: 2},
			{model: "public-model", verdict: a, parallel: 2},
			{model: "other-model", verdict: b, parallel: 2}}
	}
	for _, tc := range []struct {
		name            string
		probes          []probe
		zeroDefault     bool
		legacyPrimary   bool
		samples, passed [2]int
	}{
		{name: "all_correct", probes: pair("correct", "correct"), samples: [2]int{1, 1}, passed: [2]int{1, 1}},
		{name: "declared_targets_override_legacy_primary", legacyPrimary: true, probes: pair("correct", "correct"), samples: [2]int{1, 1}, passed: [2]int{1, 1}},
		{name: "zero_and_omitted_defaults", zeroDefault: true, probes: []probe{{model: "public-model", verdict: "correct"},
			{model: "other-model", verdict: "correct", omitParallel: true}}, samples: [2]int{1, 1}, passed: [2]int{1, 1}},
		{name: "other_model_unknown", probes: pair("correct", "unknown"), samples: [2]int{1, 0}, passed: [2]int{1, 0}},
		{name: "other_model_incorrect", probes: pair("correct", "incorrect"), samples: [2]int{1, 1}, passed: [2]int{1, 0}},
		{name: "primary_unknown", probes: pair("unknown", "correct"), samples: [2]int{0, 1}, passed: [2]int{0, 1}},
		{name: "missing_member", probes: pair("correct", "correct")[:3]},
		{name: "duplicate_replaces_missing", probes: []probe{{model: "public-model", verdict: "correct", parallel: 2},
			{model: "public-model", verdict: "correct", parallel: 2},
			{model: "public-model", verdict: "correct", parallel: 2},
			{model: "other-model", verdict: "correct", parallel: 2}}},
		{name: "undeclared_member", probes: append(pair("correct", "correct"), probe{model: "undeclared-model", verdict: "correct", parallel: 2})},
		{name: "legacy_no_targets", probes: []probe{{model: "public-model", verdict: "correct", parallel: 2, omitTargets: true},
			{model: "other-model", verdict: "correct", parallel: 2, omitTargets: true},
			{model: "public-model", verdict: "correct", parallel: 2, omitTargets: true},
			{model: "other-model", verdict: "correct", parallel: 2, omitTargets: true}}},
		{name: "missing_explicit_model", probes: []probe{{model: "public-model", verdict: "correct", parallel: 2, omitModel: true},
			{model: "other-model", verdict: "correct", parallel: 2},
			{model: "public-model", verdict: "correct", parallel: 2},
			{model: "other-model", verdict: "correct", parallel: 2}}},
		{name: "inconsistent_targets", probes: []probe{{model: "public-model", verdict: "correct", parallel: 2, targets: []string{"public-model", "different-model"}},
			{model: "other-model", verdict: "correct", parallel: 2},
			{model: "public-model", verdict: "correct", parallel: 2},
			{model: "other-model", verdict: "correct", parallel: 2}}},
		{name: "stale_member", probes: []probe{{model: "public-model", verdict: "correct", parallel: 2, action: "stale_run"},
			{model: "other-model", verdict: "correct", parallel: 2},
			{model: "public-model", verdict: "correct", parallel: 2},
			{model: "other-model", verdict: "correct", parallel: 2}}},
		{name: "inconsistent_parallel", probes: []probe{{model: "public-model", verdict: "correct", parallel: 1},
			{model: "other-model", verdict: "correct", parallel: 2},
			{model: "public-model", verdict: "correct", parallel: 2},
			{model: "other-model", verdict: "correct", parallel: 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, accountID, now := priorityQualityFixture(t)
			planID := priorityQualityPlan(t, accountID, "observe_only", now.Add(-time.Hour))
			targets := []string{"public-model", "other-model"}
			_, err := integrationDB.ExecContext(context.Background(), `UPDATE scheduled_test_plans SET pelican_config=jsonb_set(pelican_config,'{model_ids}',$2::jsonb) WHERE id=$1`, planID, `["public-model","other-model"]`)
			require.NoError(t, err)
			if tc.legacyPrimary {
				_, err = integrationDB.ExecContext(context.Background(), `UPDATE scheduled_test_plans SET model_id='legacy-primary' WHERE id=$1`, planID)
				require.NoError(t, err)
			}
			if tc.zeroDefault {
				_, err = integrationDB.ExecContext(context.Background(), `UPDATE scheduled_test_plans SET pelican_config=jsonb_set(pelican_config,'{parallel_count}','0') WHERE id=$1`, planID)
				require.NoError(t, err)
			}
			at := time.Now().Add(-time.Microsecond)
			for _, p := range tc.probes {
				action := p.action
				if action == "" {
					action = "observed"
				}
				priorityQualityResult(t, planID, tc.name, p.verdict, action, p.model, p.parallel, at)
				cfg := map[string]any{}
				if !p.omitParallel {
					cfg["parallel_count"] = p.parallel
				}
				if !p.omitModel {
					cfg["model_id"] = p.model
				}
				if !p.omitTargets {
					cfg["model_ids"] = targets
					if p.targets != nil {
						cfg["model_ids"] = p.targets
					}
				}
				body, err := json.Marshal(cfg)
				require.NoError(t, err)
				_, err = integrationDB.ExecContext(context.Background(), `UPDATE scheduled_test_results SET pelican_config=$2 WHERE id=(SELECT MAX(id) FROM scheduled_test_results WHERE plan_id=$1)`, planID, string(body))
				require.NoError(t, err)
			}
			for _, measurement := range []string{"legacy_forward", "attributed_v1"} {
				for i, model := range targets {
					got, err := repo.ReadPrioritySchedulingSignals(context.Background(), service.PrioritySchedulingQuery{AccountIDs: []int64{accountID}, Model: model, Since: now.Add(-time.Hour), LatencyMeasurement: measurement, InboundEndpoint: "/v1/responses"})
					require.NoError(t, err)
					require.Equal(t, tc.samples[i], got[accountID].QualitySamples, "%s %s", measurement, model)
					require.Equal(t, tc.passed[i], got[accountID].QualityPassed, "%s %s", measurement, model)
					require.Zero(t, got[accountID].Samples)
				}
			}
		})
	}
}

func TestPriorityQualityRuleRevisionMigrationPreservesOldWriters(t *testing.T) {
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	schema := pq.QuoteIdentifier("priority_quality_" + uuid.NewString()[:8])
	_, err = tx.ExecContext(ctx, `CREATE SCHEMA `+schema+`; SET LOCAL search_path TO `+schema+`,public;
		CREATE TABLE scheduled_test_plans(id BIGINT PRIMARY KEY,account_id BIGINT,model_id TEXT,enabled BOOLEAN,
		pelican_config JSONB,updated_at TIMESTAMPTZ,running_until TIMESTAMPTZ);
		INSERT INTO scheduled_test_plans VALUES(1,42,'fixture',true,'{"quality":{}}',NOW()-interval '1 day',NULL)`)
	require.NoError(t, err)
	sqlBytes, err := migrations.FS.ReadFile("337_quality_rule_signal_revision.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(sqlBytes))
	require.NoError(t, err)
	var initial, repeated time.Time
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT quality_rule_updated_at FROM scheduled_test_plans WHERE id=1`).Scan(&initial))
	_, err = tx.ExecContext(ctx, string(sqlBytes))
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT quality_rule_updated_at FROM scheduled_test_plans WHERE id=1`).Scan(&repeated))
	require.Equal(t, initial, repeated, "reapplying the migration must not erase a stable rule revision")
	// Old insert/update writers omit the additive column and retain their contract.
	_, err = tx.ExecContext(ctx, `INSERT INTO scheduled_test_plans(id,account_id,model_id,enabled,pelican_config,updated_at) VALUES(2,43,'fixture',true,'{"quality":{}}',NOW());
		UPDATE scheduled_test_plans SET updated_at=clock_timestamp(),running_until=clock_timestamp() WHERE id=1`)
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT quality_rule_updated_at FROM scheduled_test_plans WHERE id=1`).Scan(&repeated))
	require.Equal(t, initial, repeated)
	_, err = tx.ExecContext(ctx, `UPDATE scheduled_test_plans SET enabled=false WHERE id=1`)
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT quality_rule_updated_at FROM scheduled_test_plans WHERE id=1`).Scan(&repeated))
	require.True(t, repeated.After(initial))
	var count int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM scheduled_test_plans WHERE quality_rule_updated_at IS NOT NULL`).Scan(&count))
	require.Equal(t, 2, count)
}

func TestPriorityQualityRuleRevisionMigrationLockTimeoutAndRollback(t *testing.T) {
	ctx := context.Background()
	schemaName := "priority_quality_" + uuid.NewString()[:8]
	schema := pq.QuoteIdentifier(schemaName)
	_, err := integrationDB.ExecContext(ctx, `CREATE SCHEMA `+schema+`;
		CREATE TABLE `+schema+`.scheduled_test_plans(id BIGINT PRIMARY KEY,account_id BIGINT,model_id TEXT,enabled BOOLEAN,
		pelican_config JSONB,updated_at TIMESTAMPTZ,running_until TIMESTAMPTZ);
		INSERT INTO `+schema+`.scheduled_test_plans VALUES(1,42,'fixture',true,'{"quality":{}}',NOW(),NULL)`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, cleanupErr := integrationDB.ExecContext(ctx, `DROP SCHEMA `+schema+` CASCADE`)
		require.NoError(t, cleanupErr)
	})
	migration, err := migrations.FS.ReadFile("337_quality_rule_signal_revision.sql")
	require.NoError(t, err)
	blocker, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	_, err = blocker.ExecContext(ctx, `LOCK TABLE `+schema+`.scheduled_test_plans IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	attempt, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = attempt.Rollback() }()
	_, err = attempt.ExecContext(ctx, `SET LOCAL search_path TO `+schema+`,public`)
	require.NoError(t, err)
	_, err = attempt.ExecContext(ctx, string(migration))
	var pgErr *pq.Error
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "55P03", string(pgErr.Code), "bounded lock wait must fail instead of blocking production startup indefinitely")
	require.NoError(t, attempt.Rollback())
	require.NoError(t, blocker.Rollback())
	assertLegacyTable := func() {
		t.Helper()
		var columns, rows int
		require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=$1 AND table_name='scheduled_test_plans' AND column_name='quality_rule_updated_at'`, schemaName).Scan(&columns))
		require.Zero(t, columns, "failed or rolled-back additive migration must not leave a partial column")
		require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+schema+`.scheduled_test_plans`).Scan(&rows))
		require.Equal(t, 1, rows)
	}
	assertLegacyTable()
	// Retrying after the lock is released applies cleanly, and transaction
	// rollback restores the old schema and rows without a down migration.
	retry, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = retry.Rollback() }()
	_, err = retry.ExecContext(ctx, `SET LOCAL search_path TO `+schema+`,public`)
	require.NoError(t, err)
	_, err = retry.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	var initialized int
	require.NoError(t, retry.QueryRowContext(ctx, `SELECT COUNT(*) FROM scheduled_test_plans WHERE quality_rule_updated_at IS NOT NULL`).Scan(&initialized))
	require.Equal(t, 1, initialized)
	require.NoError(t, retry.Rollback())
	assertLegacyTable()
}
