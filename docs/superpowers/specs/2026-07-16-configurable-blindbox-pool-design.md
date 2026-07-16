# Configurable Blindbox Pool Design

## Goal

Restore the increased blindbox rewards as the only live prize pool, make the
backend and frontend consume the same versioned configuration, and preserve an
auditable record of the pool used for every open.

## Current Problem

The production database contains `play_blindbox_pool_json` for the increased
pool, but the current application ignores it. The backend draws from a legacy
hard-coded pool and the frontend independently renders the same legacy values.
The rollback commit `35f665a36` removed the configurable implementation while
the forward-applied database artifacts remained.

The live legacy pool costs `$0.50`, has an expected reward of `$0.30`, and has a
maximum reward of `$2.00`. The approved pool costs `$0.50`, has an expected
reward of `$0.45`, and has a maximum reward of `$20.00`.

## Approved Pool

The authoritative default setting is:

```json
{
  "version": "season-1-v1",
  "cost": 0.5,
  "rtp_cap": 0.9,
  "tiers": [
    { "amount": 0.05, "weight": 4000 },
    { "amount": 0.20, "weight": 3000 },
    { "amount": 0.50, "weight": 1800 },
    { "amount": 1.00, "weight": 800 },
    { "amount": 3.00, "weight": 300 },
    { "amount": 10.00, "weight": 90 },
    { "amount": 20.00, "weight": 10 }
  ]
}
```

Weights use a fixed denominator of `10000`. The expected reward is `$0.45`, so
the pool reaches but does not exceed the configured 90% RTP cap.

## Configuration Authority

`settings.play_blindbox_pool_json` is the sole prize pool source. The service
parses and validates it on every settings-cache refresh. Missing or invalid
configuration falls back to the approved default and records an operational
warning without exposing raw configuration to users. Failure to read settings
at all remains fail-closed and does not permit an open.

Validation requires:

- a non-empty version;
- a positive cost;
- an RTP cap in `(0, 1]`;
- between 1 and 32 tiers;
- non-negative finite reward amounts;
- positive integer weights totaling exactly `10000`;
- expected reward less than or equal to `cost * rtp_cap`.

The existing `play_blindbox_cost` setting remains readable for migration
compatibility but no longer controls opens after the configurable pool is
enabled. The pool's `cost` field is authoritative.

## Backend Behavior

`GetBlindboxStatus` returns the effective pool together with daily limits and
open eligibility. `OpenBlindbox` validates the effective pool, draws with
cryptographic randomness over the configured weights, charges the configured
cost, and credits the selected reward through the existing idempotent balance
grant path.

Every open stores:

- `pool_version`;
- `open_source`, initially `paid` for balance-funded opens;
- cost, reward, and net amount;
- the existing idempotency key and server date.

Existing rows remain `legacy-v1`. No historical reward is recalculated.

## API Contract

The authenticated `GET /api/v1/play/blindbox/status` response gains the pool
object. A public read-only `GET /api/v1/play/blindbox/pool` endpoint returns only
the feature state and sanitized pool fields needed for an accurate preview.

`POST /api/v1/play/blindbox/open` adds `pool_version` and `open_source` to the
result. Existing response fields remain compatible.

## Admin And User Interface

The admin Play settings expose an editable tier table with amount and weight,
plus version, cost, and RTP cap. The editor shows total weight, expected reward,
and effective RTP, and refuses invalid saves using the same rules as the
backend.

The user page renders prize amounts and probabilities from the API response. It
does not contain a fallback hard-coded prize array. If the API cannot provide an
effective pool because runtime settings are unavailable, opening is disabled
and the page shows an unavailable state.

The text claiming that VIP tiers may upgrade the pool is removed. The existing
`blindbox_pool_upgrade` VIP perk is not treated as functional until a separate
VIP pool is designed and implemented.

## Migration Strategy

A new forward migration must be used because production may already have
artifacts from migration 188. It must:

- insert the approved pool only when `play_blindbox_pool_json` is missing;
- preserve an existing valid operator-configured pool;
- add `pool_version` and `open_source` columns with `IF NOT EXISTS`;
- backfill legacy rows without altering cost or reward values;
- avoid dropping historical tables or rewriting previously applied migration
  files.

## Error Handling

Invalid admin configuration returns a validation error and leaves the previous
pool unchanged. A random source failure returns an error and does not charge the
user. Balance grant, open audit, and reward ledger writes remain transactional
and idempotent.

## Tests

Backend tests cover default parsing, valid custom pools, invalid JSON, weight
totals, RTP limits, deterministic draw boundaries, idempotent opens, and audit
fields. Handler tests cover public and authenticated pool responses. Frontend
tests verify API-driven rendering and disabled behavior when no pool is
available. Migration tests verify forward-only and idempotent SQL.

## Acceptance Criteria

- Production API and page show the approved seven tiers.
- A controlled boundary test can select every configured tier.
- New open rows contain the effective pool version and source.
- No runtime path uses the legacy five-tier hard-coded array.
- Existing balances and historical opens remain unchanged.

## Out Of Scope

Free tickets, region gating, a separate VIP pool, and campaign-specific prize
pools are not part of this restoration.
