# V182 P1 Home And Public Status Contract

## Scope And Ownership

This contract covers the first-party public status surface at `/status` and
`/en/status`, the read-only API at `GET /api/v1/public/status-summary`, and
the landing-page status entry. It does not replace the compatibility endpoint
`GET /api/v1/public/home-stats`; that endpoint retains its existing aggregate
semantics until a separately approved compatibility retirement.

`/status` and `/en/status` are public but intentionally `noindex,nofollow`.
They are an operational transparency surface, not a search landing page.

The landing page no longer publishes LMSPEED branding, third-party LMSPEED
assets, external LMSPEED links, an LMSPEED anchor, or an LMSPEED noscript
fallback. It links only to the first-party status route. The authenticated
`/keys` speed-test workflow is outside this scope and remains unchanged.

## Public API Contract

The endpoint returns the last completed immutable snapshot, never a live scan
of `usage_logs`:

```json
{
  "total_requests": 0,
  "availability": { "value_pct": null, "sample_count": 0, "window_start": null, "window_end": null },
  "ttft": { "p50_ms": null, "p95_ms": null, "sample_count": 0, "window_start": null, "window_end": null },
  "data_through": null,
  "computed_at": "2026-08-29T14:05:00Z",
  "freshness": "unavailable"
}
```

Field definitions:

- `total_requests` is the cumulative recorded `usage_logs` count with
  `created_at < window_end` when the snapshot is written. It is an as-of
  snapshot count, not a realtime counter; backfilled historical snapshots do
  not include calls recorded after their window boundary.
- `availability.value_pct` is the SLA success percentage across the 30 days
  preceding the snapshot `window_end`; `sample_count` is
  `success_count + error_count_sla` over the same `ops_metrics_hourly` window.
- `availability.window_start` / `availability.window_end` identify that exact
  completed UTC 30-day interval. `ttft.window_start` / `ttft.window_end` do
  the same for the exact completed UTC 24-hour interval. The public status
  page renders these server-owned boundaries alongside each metric's sample
  count; it does not reconstruct them from the browser clock. A missing
  boundary remains explicitly unavailable.
- `ttft.p50_ms` and `ttft.p95_ms` are PostgreSQL `percentile_cont(0.50)` and
  `percentile_cont(0.95)` over raw non-null `usage_logs.first_token_ms` in the
  24 hours preceding `window_end`; `sample_count` is the matching raw-log row
  count. Hourly percentiles are never averaged or weighted into a claimed
  global percentile.
- `data_through` is the durable `ops_aggregation_watermarks.completed_through`
  boundary for the hourly aggregation job, capped by the snapshot `window_end`.
  It advances even when a completed hour has no traffic and therefore no
  `ops_metrics_hourly` overall row. It is the user-visible business data
  watermark; the last non-empty metrics bucket is not used as a proxy.
- `computed_at` is audit metadata only. It must not be used as a realtime or
  availability claim.

`freshness` is derived from `data_through`, not `computed_at`:

- `fresh`: watermark age is at most 90 minutes.
- `delayed`: watermark age is more than 90 minutes and at most six hours.
- `unavailable`: snapshot, availability samples, TTFT samples, or watermark
  are missing; the watermark is materially in the future; or it is older than
  six hours.

The browser re-evaluates these thresholds locally so a cached response cannot
remain green past a threshold. Loading, failed fetch, or absent samples render
an explicit unavailable state rather than invented latency or success values.
If the status repository is temporarily unavailable, the HTTP cache may serve
the last snapshot, but it re-evaluates `freshness` against the request clock
before returning it; the cached object cannot remain green after its watermark
passes a threshold.

## Snapshot Writer And Concurrency

`PublicStatusSnapshotWorker` starts with the server process, runs once
immediately, and then aligns to the next five-minute boundary. Each run targets
only the preceding completed UTC hour:

- at `14:03Z`, it targets `13:00Z`;
- at `14:06Z`, it targets `14:00Z`.

Each pass also performs bounded recovery for missing windows in the recent
six-hour horizon. It considers windows newest-first and processes at most four
missing hours per run, so a restart cannot trigger an unbounded historical scan.
Before each insert the worker checks that
`ops_aggregation_watermarks.completed_through >= window_end`; the snapshot SQL
repeats that predicate inside the `INSERT ... SELECT` to close the check/write
race. The aggregation job advances that watermark only after every hourly
chunk succeeds, including an empty hour, and advances it monotonically.

### Initial Aggregation Completeness

Migration 267 intentionally creates no synthetic hourly watermark. When the
hourly row is absent, the aggregation job first materializes the entire 30-day
availability horizon ending at the latest stable UTC hour. It runs that fixed
window in 24-hour source chunks under the existing five-minute job timeout and
writes `completed_through` only after every chunk succeeds. This is a bounded
bootstrap, not an unbounded historical scan.

Until that one bootstrap completes, the snapshot worker finds no readiness
watermark and public status remains explicitly unavailable. A timeout, a
late-chunk failure, or a watermark-write failure leaves no completion row, so
the next run retries the same bounded 30-day window rather than publishing an
incomplete availability percentage. After a successful bootstrap, normal runs
replay the most recent two hours for late-arriving rows; a stale watermark
recovers in six-hour bounded slices with that same overlap.

Across multiple application replicas it first takes a PostgreSQL advisory
leader lock. The insert uses `ON CONFLICT (window_end) DO NOTHING`, so retries
and concurrent starts cannot rewrite an already recorded hour. Migration 263
also installs a database trigger that rejects `UPDATE` and `DELETE` on snapshot
rows, so immutability is not only an application convention. Public HTTP only
selects `ORDER BY window_end DESC LIMIT 1` from this table.

The worker is bounded by a two-minute context timeout. Failure is logged and
the prior immutable snapshot remains available. It does not turn a stale
snapshot into green based on the worker's clock. A missing overall row in
`ops_metrics_hourly` is not treated as proof that an empty hour failed: the
durable aggregation watermark is the completion signal.

## Migration And Performance Contract

Migrations must be applied in this order:

1. `262_play_growth_qualification.sql` creates growth qualification evidence
   and the non-cash energy ledger.
2. `263_public_status_snapshots.sql` creates the snapshot table, latest-row
   index and update/delete rejection trigger.
3. `264_public_status_ttft_window_index_notx.sql` creates the partial
   `usage_logs(created_at DESC) INCLUDE(first_token_ms)` index concurrently.
4. `265_play_growth_eligibility_orders_index_notx.sql` creates the bounded
   completed-balance-order lookup index concurrently.
5. `266_play_growth_reward_snapshot_links.sql` adds nullable audit links for
   new redeemable rewards.
6. `267_public_status_ops_aggregation_watermark.sql` creates the monotonic
   aggregation completion watermark used by the worker readiness checks.
7. `268_play_growth_governance.sql` adds the append-only operations approval
   and reward-budget ledger used to keep redeemable growth rewards closed until
   a reviewed cohort and bounded rollout are approved.

The concurrent index migrations must remain non-transactional. They are local
indexes for bounded TTFT and recharge lookups; they are not a claim that the
cumulative request-count scan is free. Before production rollout, run the
worker queries on a restored production-scale database and retain
`EXPLAIN (ANALYZE, BUFFERS)` evidence for the 24-hour percentile range,
recharge lookup, watermark check, and snapshot write duration.

The rehearsal must also execute the first hourly aggregation bootstrap and
record its duration and maximum query memory. If the fixed 30-day bootstrap
does not complete inside the five-minute job budget, do not weaken the
watermark or manufacture a fresh status: tune the query/index plan or provide
an explicitly reviewed bootstrap implementation first.

Production schema preflight is read-only and archived with the release record:

```sql
SELECT version();
SELECT COUNT(*) AS migration_rows FROM schema_migrations;
SELECT version FROM schema_migrations ORDER BY version;
SELECT table_name, column_name, data_type
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name IN ('usage_logs', 'ops_metrics_hourly', 'public_status_snapshots')
ORDER BY table_name, ordinal_position;
SELECT indexname, indexdef
FROM pg_indexes
WHERE schemaname = 'public'
  AND tablename IN ('usage_logs', 'public_status_snapshots')
ORDER BY tablename, indexname;
SELECT COUNT(*) AS usage_log_rows, MAX(created_at) AS usage_log_watermark
FROM usage_logs;
SELECT MAX(bucket_start) AS ops_hourly_watermark FROM ops_metrics_hourly;
SELECT job_name, completed_through, updated_at
FROM ops_aggregation_watermarks
WHERE job_name = 'ops_preaggregation_hourly';
```

The known production migration history contains seven retired daily-card
migrations that are absent from the repository. They are historical lineage,
not a request to recreate or clean production state. Do not rebuild them, do
not alter `schema_migrations` to manufacture parity, and never re-run the
historical destructive `033_ops_monitoring_vnext.sql`.

## Home Visual And Motion Contract

The first screen presents product capability, a primary create-key action, a
secondary documentation action, and only cumulative recorded calls plus a data
watermark/status entry. It does not market unexplained success rate or TTFT.

`HeroSphere` uses a static poster on mobile, reduced motion, Save-Data, low
memory, and low-core devices. Eligible desktop sessions render the enhancement
only after the initial content is interactive, for one 1.6-second run capped
at 20fps and 240 particles. The CTA never waits for geography resources, and
static mode makes no Earth LOD fetch.

The canvas selects its ink/paper palette from the current `html.dark` class.
Because Canvas2D cannot consume CSS semantic tokens directly, the component has
one reviewed art-only paint conversion helper; it redraws the poster whenever
the theme class changes. This preserves visibility in dark mode without adding
a second interactive surface or changing semantic status colors.

## Release Gates

Before release, pass the status service/repository/handler/router tests,
migration contract test, focused home/status Vitest, `pnpm design:check`,
`pnpm lint:check`, `pnpm typecheck`, frontend build, backend build, and
`./scripts/check-fork-integrity.sh`. After merge, verify the actual Zeabur SHA,
health endpoint, public API JSON, CSP/XFO, rendered asset hash, and migration
result. Final guest, ordinary-user, and administrator acceptance is a
user-owned local-browser gate; this document is not production proof.
