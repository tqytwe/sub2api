# Growth Regression Rollback Design

## Goal

Restore the documented Growth / Play behavior that was live at `f091ab60`, then deploy and verify the current production experience as a normal user.

The rollback addresses two confirmed regressions introduced by `ebce8edf` and `dc102f78`:

- The Play Hub can become completely empty when one newly added growth query fails.
- The public home statistics endpoint converts refresh failures into a successful all-zero response.

## Source Of Truth

Acceptance behavior comes from the active documents:

- `docs/GROWTH_PLAY.md`
- `docs/IMAGE_STUDIO.md`
- `docs/growth-analytics.md`

Files under `docs/archive/` are historical context only. They are not implementation or acceptance requirements.

## Scope

1. Revert `dc102f78` and `ebce8edf` in reverse chronological order.
2. Preserve all earlier commits, including Image Studio, model pricing, billing, fork integrity, and CI changes.
3. Add a new forward-only compensating migration for persistent configuration changed by migration 188.
4. Add focused regression coverage for the restored home statistics contract and Play Hub behavior.
5. Push the repaired `play/main`, wait for deployment health, and verify production with the supplied normal-user account.
6. Record a requirement-to-production acceptance matrix and fix any in-scope mismatch found during verification.

## Database Strategy

Migration `188_growth_world_v1.sql` may already be recorded in production and may have created tables or changed settings. Rollback must not edit the applied migration and must not drop its tables.

A new migration will compensate only for persistent user-visible configuration that migration 188 removed. In particular, it will restore the default `blindbox_pool_upgrade` perk for V2 and V3 when the persisted VIP tier configuration still matches the affected default tier structure. Custom tier structures will not be overwritten.

The new growth tables can remain unused. Keeping them is safer than destructive rollback and preserves any data already written.

## Restored Contracts

### Home Statistics

Restore `GET /api/v1/public/home-stats` to the documented response:

```json
{
  "total_requests": 1234567,
  "availability_pct": 99.97,
  "avg_ttft_ms": 842,
  "has_live_data": true
}
```

The frontend restores its existing synthetic fallback and live-data overlay. Backend failures must not be represented as truthful zero usage.

### Play Hub

Restore the existing `/play` Hub cards, balance, VIP, campaign, daily quests, Image Studio, check-in, Arena, blindbox, quiz, and Agent Team behavior defined in `docs/GROWTH_PLAY.md`.

The rollback removes the newly coupled aggregate, ticket, team workflow, public activity, and admin growth configuration paths from the runtime. Existing Hub content must remain visible under the normal feature flags.

## Verification

### Before Push

- Confirm the reverse diff removes only the two target commits plus adds the compensating migration and focused tests.
- Run backend unit tests for Play, public statistics, and migrations.
- Run frontend tests for home statistics and Play Hub-related behavior.
- Run frontend type checking and production build.
- Run fork integrity checks.

### Deployment

- Integrate the repair into `play/main` without touching unrelated `origin/main` history.
- Push using the repository deployment workflow.
- Wait until the production health endpoint succeeds and the deployed revision is observable.
- Check public home statistics before authenticated verification.

### Normal-User Production Acceptance

Using the user-supplied account, verify:

- Home statistics show non-empty fallback or live values, never an unavailable all-zero snapshot.
- Play Hub shows balance and enabled cards.
- Check-in, Arena monthly/daily views, blindbox, quiz, and Agent Team pages load.
- Image Studio loads templates, model selection, estimate controls, and existing works without blank content.
- Models/pricing displays the user's available groups and prices.
- Navigation contains the documented Growth / Play entries and no unintended admin-only entries.

Actions that spend balance, submit rewards, generate images, alter teams, or otherwise mutate user data will not be executed unless needed and explicitly authorized. Read-only page and API verification is sufficient for this pass.

## Failure Handling

If deployment fails, inspect the build/runtime logs and fix the deployment before production login verification. If a production mismatch belongs to an earlier preserved commit, document the exact requirement and failing behavior, add a focused test, and repair it without broadening the rollback.

## Non-Goals

- Reimplementing the unified growth-world feature from the two reverted commits.
- Dropping migration 188 tables or deleting data.
- Reverting Image Studio, pricing, billing, or upstream synchronization work.
- Performing destructive or balance-changing actions with the supplied production user.
