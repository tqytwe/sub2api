# Truthful Home Statistics Design

## Goal

Make public, admin, and user home statistics display authoritative production
values and provide a repeatable post-deployment reconciliation between database
queries, API responses, and rendered UI.

## Current Problem

The public home page starts at a synthetic `12,847,360` request baseline and
increments it locally. It therefore displays millions of requests while the
production API and database contain thousands. Public availability is a
heuristic based on account health, and the field labelled average TTFT currently
uses average full-request duration.

Admin and user dashboards use real data but need explicit reconciliation after
reward changes because blindbox and team rewards alter balances and the same
usage logs feed dashboard totals.

## Public Metric Definitions

The public page displays three real metrics:

- **Cumulative API requests:** exact `COUNT(*)` of successful requests recorded
  in `usage_logs` at the snapshot reference time.
- **30-day availability:** `success_count / (success_count + error_count_sla)`
  from overall `ops_metrics_hourly` rows for the trailing 30 days.
- **24-hour average TTFT:** sample-count-weighted `ttft_avg_ms` from overall
  `ops_metrics_hourly` rows for the trailing 24 hours.

Overall ops rows have both `platform` and `group_id` null. Availability excludes
business-limited errors according to the existing SLA error classification.
TTFT uses only rows with recorded first-token samples and reports its sample
window honestly.

If a denominator or TTFT sample count is zero, the corresponding value is null
and the UI renders `--`.

## Backend Architecture

A dedicated public-home-stats query returns the three sanitized metrics and a
`computed_at` timestamp. It counts `usage_logs` directly for cumulative
successful requests and reads operational aggregates for availability and TTFT.
It does not infer availability from account counts and does not map average
duration to TTFT.

The response remains cacheable for a short bounded interval. Cache metadata
includes the source aggregate timestamps so a stale response is detectable. An
upstream query failure returns the last successfully cached real snapshot when
available; it never creates synthetic values.

## Frontend Behavior

The home composable removes the synthetic baseline, synthetic request rate,
synthetic uptime wave, and synthetic latency wave. It stores only the last
successful real snapshot and its timestamp.

Number-transition animation may interpolate from the previous real value to the
new real value, but the final rendered and accessible value must exactly equal
the API response. On first load without a snapshot, fields show `--` until the
request completes. A failed refresh retains the last real snapshot and marks it
stale internally without incrementing it.

Labels state `30-day availability` and `24-hour average TTFT` in localized copy
so the aggregation windows are not ambiguous.

## Admin And User Reconciliation

The post-deployment acceptance procedure captures values at a single reference
time and uses the application timezone.

Admin checks include total requests, today's requests, total and today's Token,
total and today's actual cost, model distribution, and aggregation watermark.
User checks for the test account include total and today's requests, Token,
actual cost, API key count, model distribution, and recent usage.

Database queries use the same token formula as each endpoint. Differences such
as inclusion of cache-read Token are documented per field rather than hidden.
Dynamic counters are compared using a captured upper and lower timestamp so
traffic during verification cannot create false failures.

Reward verification separately reconciles blindbox and team balance ledger
entries; reward credits must not alter API usage counts or Token totals.

## Automated Acceptance

Frontend tests mock a public stats response and assert the visible and
accessible values exactly match it. They also verify that values do not change
with elapsed browser time and that failed refreshes retain only a previously
real snapshot.

Backend tests verify total-request mapping, SLA availability calculation,
sample-weighted TTFT, null behavior, and cached-real fallback. Production smoke
checks compare public API values to direct aggregate queries and compare admin
and user API values to scoped SQL.

## Acceptance Criteria

- No source file contains or uses the `12,847,360` synthetic baseline.
- Public cumulative requests equal the authoritative database/API total at the
  captured reference time.
- Availability and TTFT use the documented windows and actual ops aggregates.
- Admin and user pages match their API responses and reconciliation SQL.
- Reward payouts change balances and ledgers only, not usage statistics.
- Aggregation watermarks continue advancing after deployment.

## Out Of Scope

Historical reconstruction before retained ops windows, public exposure of
platform or account details, and marketing estimates are excluded.
