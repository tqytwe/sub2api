# Balance Ledger Rollout

## Goal

All future mutations of `users.balance` and `users.frozen_balance` should go
through `BalanceLedgerService.ApplyDelta()`, so the balance update and the
`balance_transactions` row are committed atomically and cache invalidation runs
only after commit.

## Current P1 Slice

Implemented in this rollout:

- `BalanceLedgerService.ApplyDelta()` reuses an existing ent transaction from
  context when present; otherwise it opens and commits its own SQL transaction.
- `/admin/users/:id/balance-history` prefers `balance_transactions` when the
  user has matching ledger rows, and falls back to the P0 legacy union only when
  the new ledger has no matching rows.
- Admin balance adjustments create the legacy `redeem_codes` audit row and the
  unified balance transaction in the same transaction.
- Normal balance redeem codes use `ApplyDelta()` with source
  `balance` / `redeem_code:<id>`.
- Payment balance fulfillment still reuses redeem-code redemption, but overrides
  the ledger source to `payment_recharge` / `payment_order:<id>`.
- Promo-code grants use `ApplyDelta()` with source `promo_bonus`.
- Affiliate quota transfers use `ApplyDelta()` with source `affiliate_balance`;
  the legacy `user_affiliate_ledger` transfer row remains as the business audit
  source and is linked by `source_id`.
- Auth-source first-bind balance grants use `ApplyDelta()` with source
  `auth_first_bind_grant`.
- The legacy synchronous `UsageService.Create()` charge path uses
  `ApplyDelta()` with source `usage_charge` and `allow_overdraft` policy.
- Gateway/API usage billing now injects the ledger-aware repository; balance
  charges use `usage_charge` with request/api-key metadata and the
  `allow_overdraft` policy.
- Batch Image and Image Studio hold/capture/release flows use
  `ApplyDelta()` with `image_balance_hold`, `image_balance_capture`, and
  `image_balance_release`, updating available and frozen balances atomically.
- Provider refund balance deduction and gateway-failure rollback use
  `ApplyDelta()` with sources `refund` and `reversal`; audit logs include the
  ledger keys for lookup.
- Play balance grants use `ApplyDelta()` from the shared grant path, covering
  check-in, makeup check-in, quiz, blindbox net change, arena settlement, daily
  arena settlement, and team shared rewards. `team_affiliate_bonus` remains a
  non-balance idempotency/affiliate-quota record.
- `balance_ledger_direct_write_guard_test.go` fails if a new direct
  balance/frozen-balance writer appears outside the audited allowlist.

## Remaining Direct Writers

These paths still need dedicated migration PRs before the rollout is complete:

- User creation/update default balance snapshots in
  `backend/internal/repository/user_repo.go`.
- Legacy nil-ledger fallback paths in
  `backend/internal/repository/usage_billing_repo.go`,
  `backend/internal/service/gateway_usage_billing.go`, and migrated services.
  Production wiring should keep the ledger-aware repository/service injected.
- Legacy repository fallback methods:
  `UpdateBalance`, `DeductBalance`, and `ApplyRedeemBalanceAdjustment`.

## Source Type Rules

- `payment_recharge`: paid balance orders.
- `balance`: normal user redeem-code balance grants.
- `admin_balance`: admin balance adjustment, with notes preserved in metadata.
- `promo_bonus`: promo-code balance grants.
- `affiliate_balance`: affiliate quota transfer to available balance.
- `auth_first_bind_grant`: auth-source first-bind default balance grant.
- `usage_charge`: API request balance charge.
- `image_balance_hold`: Image Studio/Batch Image available balance moved into
  frozen balance before work starts.
- `image_balance_capture`: Image Studio/Batch Image final settlement from a
  prior hold.
- `image_balance_release`: Image Studio/Batch Image hold release.
- `refund`: provider refund balance deduction.
- `reversal`: rollback/reversal of a prior balance operation.
- Play sources are kept as their existing source values:
  `checkin`, `checkin_makeup`, `quiz`, `blindbox`, `arena_settlement`,
  `arena_daily_settlement`, and `team_shared_reward`.

## Backfill Rules

- Backfill scripts must be idempotent, batchable, and dry-run capable.
- Do not invent historical `balance_before` / `balance_after` values when the
  source table does not contain reliable snapshots.
- Use stable idempotency keys. For already-defined P0 sources, keep the existing
  keys where possible:
  `payment_order:<id>`, `redeem_code:<id>`, `promo_code_usage:<id>`,
  `play_reward:<id>`, `usage_log:<id>`, and `payment_refund:<id>`.
- Low-confidence signup/default-balance records must use `estimated` or
  `needs_review`.
- Historical Image Studio/Batch Image frozen-balance lifecycle rows are not
  backfilled unless a reliable hold/capture/release source and snapshots are
  available; do not reconstruct them from current balances.

## Verification

For every rollout PR:

```bash
go test ./internal/service -run 'Test(AdminBalanceFlow|BalanceLedger|Redeem|Promo|Play)'
go test ./migrations -run 'TestBalanceTransactions'
make test
GOFLAGS=-buildvcs=false make build
./scripts/check-fork-integrity.sh
git diff --check
```
