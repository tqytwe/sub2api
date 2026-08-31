# v0.1.182 Growth Qualification Contract

Status: implementation candidate. The production `play_checkin_enabled` and
`play_quiz_enabled` switches are currently off. This document specifies the
rule for newly recorded actions; it does not claim that rewards are being
issued in production.

## Product Rule

Signing in, checking in, and answering the daily quiz remain available to every
authenticated user. Reward mode is server-owned and never derived from a
browser flag, localStorage value, IP address, or User-Agent.

| Tier | Eligibility | Outcome for new check-in or quiz action |
| --- | --- | --- |
| `active` | Verified email, account age at least 3 days, and one qualifying real-use signal | Existing coupon, redeem-code, or balance reward flow remains available. |
| `explorer` | Any requirement is missing | The action is recorded and returns non-transferable growth energy. It cannot create a coupon, redeem code, balance credit, or withdrawal entitlement. |

An account becomes `active` when all base requirements hold and at least one of
the following server-side signals holds at evaluation time:

The verified-email signal is read from durable `auth_identities.verified_at`.
For historical GitHub, Google, and OIDC identities created before that column
was populated, an exact `metadata.email_verified` value of `true`, `1`, or
`yes` is accepted without unsafe JSON-to-boolean casts. A successful verified
OAuth login also backfills `verified_at`; an unverified login never clears an
existing timestamp.

1. A usage record in the prior 7 days has `actual_cost > 0` or `billed_cost > 0`.
2. A completed `payment_orders` balance recharge in the prior 30 days has net
   qualifying CNY value at least `10`, with `order_type='balance'`. Coupon-aware
   orders use their immutable `qualifying_recharge_amount`; legacy orders
   without a settlement value fall back to `amount`. A refund reduces that
   value in the same proportion as `refund_amount / amount`, clamped to zero
   through one per order, so no over-refund can make the aggregate negative.
   This intentionally excludes payment fees, coupon-funded face value and
   campaign/VIP credit from the real-recharge signal. Orders without a non-null
   `completed_at`, outside the evaluation window, in a non-CNY currency, or
   dated in the future are excluded. Legacy rows with a missing
   `payment_currency` are treated as CNY for compatibility because the
   historical payment contract was CNY-only; non-CNY provider snapshots are
   preserved by the payment settlement migration and excluded.
3. A non-deleted `user_subscriptions` row is active for the current time.

Existing historical balances, coupons, redeem codes, and rewards are not
recomputed, frozen, clawed back, or converted. A user automatically returns to
the redeemable mode after satisfying the rule; no manual whitelist is needed.
The current check-in makeup operation remains redeemable-only. Explorer users
see a normal unavailable response rather than a punitive account action.

Blindbox is a paid, redeemable-reward draw. An `active` user retains the
existing idempotent open flow. An `explorer` user cannot start a new open: the
service does not debit balance, draw a coupon/redeem/balance reward, or record
Growth Energy. A completed historical blindbox request still replays before
the current eligibility gate. Generic redemption of already-issued coupons or
redeem codes remains ungated so that this rule never retroactively removes
user property.

## Server Contract

`growth_eligibility` is included in check-in status/result, quiz today/submit,
and authenticated blindbox status responses. A rejected new blindbox open uses
`PLAY_GROWTH_REWARD_INELIGIBLE` with the same server-owned tier, reward mode and
primary reason metadata:

```text
{
  tier: "explorer" | "active",
  reward_mode: "energy" | "redeemable",
  primary_reason: "email_unverified" | "account_too_new" |
                  "no_recent_activity" | "eligible",
  email_verified, account_age_days, has_recent_usage,
  net_balance_recharge_30d, has_active_subscription
}
```

The browser only renders this payload. It does not decide eligibility. A quiz
that was completed in energy mode returns `previous_growth_energy` from the
stored action snapshot after reload, instead of inferring history from the
user's current eligibility.

## Immutable Evidence And Idempotency

Migration `262_play_growth_qualification.sql` adds:

- `play_growth_eligibility_snapshots`, one row per `(user_id, source,
  action_id)`, storing the evaluated signals, tier, reason and `v1` rule
  version. This keeps repeated paid blindbox draws independently auditable.
- `play_growth_energy_ledger`, with a unique `action_id`, for non-transferable
  energy actions.
- Foreign-key columns from `play_checkins` and `play_quiz_attempts` to their
  eligibility snapshots.

Migration `266_play_growth_reward_snapshot_links.sql` additionally adds nullable
foreign-key links from blind-box actions and balance reward ledger entries to
the immutable snapshot, plus the persisted qualification rule version. It does
not backfill or reinterpret historical rewards.

The activity row is inserted before its snapshot so the existing daily unique
constraint remains the concurrency winner. Snapshot creation, activity link,
coupon/redeem issuance, and balance ledger updates share one transaction. For
redeemable balance rewards, the existing `play_reward_ledger.detail` adds
`growth_eligibility_snapshot_id`, `growth_rule_version`, and `growth_tier`.
Coupon and redeem-code check-in/quiz actions retain immutable proof through
the activity row's snapshot foreign key. Blindbox has no single daily activity
row, so its snapshot ID is linked directly on `play_blindbox_opens`; the same
ID, rule version and tier remain in the immutable `play_reward_ledger.detail`
for every open.

Risk controls remain graduated: existing endpoint limits, daily uniqueness,
reward-budget settings and abnormal-redemption monitoring can delay fulfillment
or require manual review. They must not automatically ban an account based only
on an IP address or User-Agent.

## Observation Before Enablement

Before either reward switch is enabled, collect a two-week read-only cohort.
The fixed dashboard fields are participation denominator, 7/30-day real-call
ratio, first-recharge conversion, coupon redemption, actual reward cost, D7
retention, abnormal-redemption rate, and appeal false-positive rate. Because
the report includes each participant's following 30-day real-use signal, the
cohort end must be at least 30 days old before it can approve a rollout. The
default administrative report therefore selects a 14-day window ending 30 days
ago; an explicitly requested newer window remains readable for diagnosis but
is rejected by the approval service. No budget threshold means both switches
remain off. Operations may approve a 10-20% rollout only after the mature
cohort shows improved real use/retention with controlled cost.

The cohort query counts the first qualifying completed CNY balance recharge
after a participant's first action, rather than counting a repeat recharge as a
new conversion. Actual reward cost includes positive check-in, check-in makeup,
quiz, and blindbox ledger amounts. The current schema does not yet have a
durable abnormal-redemption or appeal-review source. Those two ratios are
returned as unavailable (`null`), never fabricated as zero. While either is
unavailable, a rollout approval is rejected and the reward switches stay
closed.

## Operations Approval And Budget Gate

Migration `268_play_growth_governance.sql` is the only enablement gate for new
daily-participation rewards generated by this qualification flow: normal
check-in, check-in makeup, quiz, and blindbox. It does not govern Arena,
Agent Team, Team Affiliate, referral campaigns, or their existing independent
settlement/budget/audit controls. It adds two append-only tables:

- `play_growth_governance_approvals` records an operator's approved or revoked
  decision, the reviewed cohort, rule version, rollout percentage, reason, and
  approval budget.
- `play_growth_reward_budget_ledger` records one immutable reservation for each
  approved reward action. It is not a balance ledger and it never alters prior
  user property.

The latest row ordered by its monotonic `id` is authoritative. A revoke is a
new row, not an update to the earlier approval. All approval, revocation and
reservation mutations acquire the same PostgreSQL transaction advisory lock.
Every reward endpoint first reads the state for a clear user-facing result,
then, after that lock is acquired in the reward transaction, rereads the latest
decision and spent budget before reserving. This avoids a stale
`READ COMMITTED` statement snapshot after lock contention. Therefore a revoke
committed before the reservation linearization point prevents the reward; a
reservation that already owns that transaction lock completes atomically with
its activity, qualification snapshot, coupon/redeem issue, and balance ledger
update and remains auditable.

Budget amounts use a deliberately conservative unit, which must be chosen and
understood by operations before approval:

- Balance rewards reserve the positive credited amount.
- Coupon and redeem-code rewards reserve one budget unit each because final
  value is selected under the issuer lock.
- Blindbox reserves its positive prize value, never the user's paid open cost.
- A `checkin_makeup` balance ledger event reserves under the `checkin` budget
  family so the immutable budget table has exactly the three configured action
  families: check-in, quiz, and blindbox.

The reservation ledger retains its `user_id` with `ON DELETE RESTRICT`; it
does not cascade-delete an immutable budget decision during a hard-delete.
Normal user lifecycle continues to use the existing soft-delete domain. Any
future privacy retention design must preserve an auditable, non-reusable action
ownership reference before it changes this foreign-key policy.

This unit is not automatically CNY or USD for every reward type. Cohort actual
reward cost remains a financial observation, while the budget ledger is a
conservative reservation ceiling; the service does not falsely compare them as
one currency. The operator's approval reason must record the budget-unit
interpretation and the reviewed cohort cost. The service rejects non-positive
or non-finite amounts, a cohort shorter than 14 days, a non-current rule
version, and any rollout outside 10-20%.

Admin operations use the following protected API contract:

- `GET /api/v1/admin/play/growth/cohort?start=&end=` returns read-only cohort
  evidence.
- `GET /api/v1/admin/play/growth/governance` returns the latest decision and
  remaining budget.
- `POST /api/v1/admin/play/growth/governance/approve` and
  `POST /api/v1/admin/play/growth/governance/revoke` require the existing
  step-up authentication middleware. The browser is never allowed to decide
  eligibility, rollout inclusion, or budget availability.

### Admin Workbench

`/admin/play-ops?tab=growth-governance` is the operations workbench for this
contract. It renders the returned cohort, latest append-only governance state,
budget amount/spend/remaining amount, rollout, rule version, and audited
reason. It does not derive a green state from local storage, a cached response,
or a ratio defaulted to zero.

For the default review, the browser intentionally sends no `start` or `end`
override. The server selects a two-week cohort whose end is at least 30 days in
the past, so the 7/30-day real-call and retention observations have had time to
mature. An arbitrary fresh two-week range is not approval-ready. When any
required metric is `null`, appears in `unavailable_metrics`, or the server
marks `metrics_available=false`, the workbench displays it as unavailable and
keeps approval disabled. This is a user-interface guard only; the approval API
revalidates every condition server-side.

Approval and revocation are separate dialogs with a 10-500 character reason.
The approval dialog sends the exact cohort currently returned by the server,
the operator-entered conservative budget ceiling, and a 10-20 percent rollout.
Both mutations use the existing session TOTP step-up flow and refresh the
server state afterward. The workbench does not invent a currency conversion
between the financial cohort's actual reward cost and the reservation budget
unit.

## Required Release Evidence

- Migration contract test and qualification service tests.
- Check-in and quiz active/explorer path tests, including snapshot links,
  energy idempotency, and reward-ledger detail for balance rewards.
- User-local production acceptance with real but non-sensitive test data after
  an authorized deployment. The cohort and switch changes are separate
  operations approval steps.
