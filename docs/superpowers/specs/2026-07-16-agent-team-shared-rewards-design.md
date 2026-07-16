# Agent Team Shared Rewards Design

## Goal

Replace the disabled captain-only affiliate bonus with a truthful monthly team
cashback system that distributes a balance reward according to each member's
billable contribution and supports complete team membership lifecycle actions.

## Approved Reward Rules

Team spend is the sum of successful usage-log `actual_cost` generated while a
user is an active member of that team. Periods are calendar months in
`Asia/Shanghai`.

The highest reached tier determines the rebate rate:

| Monthly team spend | Rebate rate |
|---:|---:|
| `$20` | 2% |
| `$100` | 3% |
| `$500` | 4% |
| `$2000` | 5% |

The monthly pool is `min(team spend * reached rate, $250)`. If no tier is
reached, no settlement or allocation is created.

The pool is distributed by each member's contribution divided by total team
spend. Members with zero contribution receive zero. Amounts use fixed
eight-decimal arithmetic. Any rounding remainder is assigned to the largest
contributor, with the lowest user ID as the deterministic tie breaker.

Rewards are credited directly to user balance. They do not pass through the
affiliate quota system.

## Membership Attribution

Each membership has `joined_at` and nullable `left_at`. Contribution includes
usage whose timestamp is within both the settlement month and the membership's
active interval. Usage before joining is not counted. Usage before leaving
remains attributed to the old team; usage after leaving is not counted there.

A user may have only one active membership but can have multiple historical
memberships. Historical rows are never moved between teams.

## Team Lifecycle

The supported actions are:

- create a team and become its captain;
- join by invite code when the user has no active membership;
- leave voluntarily;
- transfer captaincy to another active member;
- remove a member as captain.

A captain cannot leave or remove themselves while other active members remain.
They must transfer captaincy first. A one-member team's captain may leave, which
closes the membership and archives the empty team. Archived teams reject new
joins while retaining membership, contribution, and settlement history.

All lifecycle operations are authenticated, transactional, and produce an
audit event. Authorization errors use explicit domain codes.

## Persistence Model

`play_teams` gains nullable `archived_at`. `play_team_members` becomes a
membership-history table by adding `left_at` and
replacing the global `user_id` unique constraint with a partial unique index for
rows where `left_at IS NULL`.

`play_team_settlements` stores one immutable result per team and month:

- team and period identity;
- contribution window and total spend;
- reached threshold and rebate rate;
- pool amount and cap;
- status (`pending`, `processing`, `completed`, `partial`, `failed`);
- timestamps and last error.

`play_team_reward_allocations` stores one row per contributing member:

- settlement and user identity;
- contribution and contribution ratio;
- reward amount;
- payout status, idempotency key, paid timestamp, and last error.

Foreign keys retain settlement history if a membership later ends. Monetary
columns use `DECIMAL(20,8)`.

## Settlement Flow

On the first day of each month, a leader-locked scheduled job settles the
previous month. The same operation is exposed to administrators for a safe
manual retry.

For each team, the job snapshots memberships and qualifying usage, determines
the tier, and inserts the settlement and allocations. Each allocation is paid
through the existing transactional balance-grant path with idempotency key
`team_reward:{team_id}:{YYYY-MM}:{user_id}`.

The settlement becomes `completed` only when every positive allocation is paid.
A crash or downstream failure leaves unpaid allocations retryable. Paid
allocations are never paid twice. Settlement creation and payout can be rerun
without changing an already completed snapshot.

The obsolete `play_team_affiliate_enabled`, token threshold, and captain bonus
settings remain readable during migration but are ignored by the new system and
removed from user-facing semantics. The known `source_user_id=0` affiliate path
is not used.

## Configuration

New settings store:

- the global shared-reward switch;
- tier threshold/rate JSON;
- the `$250` monthly pool cap.

Admin validation requires strictly increasing positive thresholds, increasing
rates in `(0, 1]`, unique thresholds, and a positive cap. Invalid updates do not
replace the current configuration.

## API And UI

The team summary shows current-month actual spend, the reached tier, the next
tier, estimated pool, and member spend percentages. Token contribution remains
available as an informational metric but is not used for rewards.

New endpoints cover leave, captain transfer, member removal, and settlement
history. The team page explains the calendar month, actual-cost basis, tier
table, proportional split, monthly cap, and next-month automatic settlement.
It never claims a reward was paid unless the allocation is confirmed paid.

The admin Play area exposes the reward switch, tier editor, cap, settlement
status, and retry action. The old captain-only copy is removed.

## Error Handling

Concurrent joins rely on the partial unique active-membership index. Lifecycle
changes use row locks around team and membership state. Settlement payout errors
are recorded per allocation and do not roll back payouts already confirmed in
separate idempotent transactions. Manual retries only process unpaid rows.

## Tests

Tests cover tier boundaries, cap enforcement, fixed-precision allocation,
rounding remainder, zero contributors, joining and leaving mid-month, multiple
historical memberships, captain transfer, removal authorization, concurrent
joins, settlement idempotency, partial failure retry, and exact balance-ledger
reconciliation. Frontend tests cover current progress, rules, lifecycle actions,
and truthful paid status. Migration tests cover constraint replacement and
preservation of existing members.

## Acceptance Criteria

- Current members keep their active memberships after migration.
- Mid-month membership changes attribute spend only to valid intervals.
- A settlement's allocation sum exactly equals its pool.
- Balance changes exactly equal paid allocation rows and reward-ledger entries.
- Repeated jobs and retries do not duplicate rewards.
- No team reward writes affiliate quota or references user ID zero.
- User and admin pages display the same settlement values as the database.

## Out Of Scope

Cross-team leagues, team-owned wallets, custom captain shares, manual member
claiming, and retroactive settlement of months before deployment are excluded.
