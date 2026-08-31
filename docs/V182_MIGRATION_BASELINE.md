# v0.1.182 Production Schema Baseline

Status: verified read-only on 2026-08-29 before the v0.1.182 governance
migrations are released.

## Scope And Method

- Source baseline: `origin/play/main@ab69d89a851a0b17a270707e1a7e6293b3dbe0eb`.
- Production database was inspected through the Zeabur PostgreSQL service with
  read-only `SELECT` statements. No setting, feature switch, schema object, or
  application data was modified.
- PostgreSQL reports version `18.4`.
- The migration runner tracks complete filenames and checksums in
  `schema_migrations`; numeric prefixes are not unique migration identities.

## Migration Lineage

| Item | Count | Interpretation |
| --- | ---: | --- |
| Production `schema_migrations` rows | 373 | Current deployed lineage before this change. |
| Repository SQL migrations | 373 | Includes the seven new, not-yet-deployed migrations below. |
| Production-only historical migrations | 7 | Retired daily-card lineage. They are already applied in production and must not be recreated, re-run, or deleted. |
| Repository-only migrations | 7 | New additive v0.1.182 governance migrations, pending deployment. |

Production-only, retired history:

```text
188_growth_world_v1.sql
228_daily_card_entitlements.sql
230_daily_card_lifecycle_reconciliation.sql
231_allow_zero_daily_card_holds.sql
232_daily_card_redeem_entitlement_sources.sql
233_usage_log_daily_card_entitlements.sql
247_daily_card_request_replays.sql
```

Pending additive migrations:

```text
262_play_growth_qualification.sql
263_public_status_snapshots.sql
264_public_status_ttft_window_index_notx.sql
265_play_growth_eligibility_orders_index_notx.sql
266_play_growth_reward_snapshot_links.sql
267_public_status_ops_aggregation_watermark.sql
268_play_growth_governance.sql
269_play_membership_manual_contributions.sql
```

`033_ops_monitoring_vnext.sql` remains historical and is not a candidate for
replay. No migration in this delivery clears, drops, or backfills historical
production data to manufacture a matching lineage.

## Pre-Deployment Data Shape

The authoritative production measurements are recorded in
`V182_DATABASE_PREFLIGHT_2026-08-29.md`; this section intentionally repeats
only those exact read-only values rather than older planner estimates. They
are capacity context, not product metrics or a replacement for post-deployment
acceptance.

| Item | Read-only production value |
| --- | ---: |
| `usage_logs` rows | 659,931 |
| `usage_logs.MAX(created_at)` | `2026-08-29 08:49:45.172489+00` |
| `ops_metrics_hourly` rows | 6,129 |
| Overall `ops_metrics_hourly` rows | 728 |
| `ops_metrics_hourly.MAX(bucket_start)` | `2026-08-29 07:00:00+00` |

`payment_orders` contains `list_amount`, `qualifying_recharge_amount`,
`refund_amount`, `payment_currency`, and `completed_at`. `usage_logs`
contains `first_token_ms`, `actual_cost`, and `billed_cost`. The new tables
`play_growth_eligibility_snapshots`, `play_growth_energy_ledger`,
`public_status_snapshots`, and `ops_aggregation_watermarks` did not exist yet,
as expected before migrations 262--268.

## Runtime Baseline

The currently read production settings are deliberately recorded as a rollout
constraint, not changed by this delivery:

| Setting | Current value | Required rollout handling |
| --- | --- | --- |
| `play_checkin_enabled` | `false` | Keep disabled during the two-week observation period. |
| `play_quiz_enabled` | `false` | Keep disabled during the two-week observation period. |
| `channel_monitor_mode` | `v1` | Do not switch to V2 as part of locale compatibility work. |
| `frontend_url` | legacy non-TLS, non-`www` canonical value | Update only through the audited administrator settings flow after deployment; then verify reset, notification, and NextChat links. |

## Deployment And Recovery Requirements

1. Restore the production database to an isolated rehearsal database and run
   migrations 262--268 there before any production application deployment.
2. Preserve migration order. `264` and `265` are `_notx` concurrent-index
   migrations and must be run through the repository migration runner rather
   than inside a transaction wrapper.
3. After the application is deployed, verify `schema_migrations` checksums,
   the two new growth tables, `public_status_snapshots`,
   `ops_aggregation_watermarks`, foreign keys, check constraints, and the two
   concurrently-created indexes. Starting from the current 373 production
   rows, a clean application of all seven additive migrations would result in
   380 rows while retaining the seven retired daily-card records.
4. Do not enable check-in or quiz, and do not set a reward budget, until the
   read-only cohort has the documented two weeks of evidence and an operations
   approval exists.
