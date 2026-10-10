# P1 isolated subscription/package counter acceptance

Status: **RED / not accepted**. This is test and evidence work, not a production
fix, deployment, or production/browser sign-off. The shared handoff ledger and
P0's production billing/Ops files were not changed.

## Baseline and isolation

- Remote `tqytwe/sub2api` `play/main` and local baseline:
  `726289bdb757e572eac10ab5f12c689c4dc087b5` (rechecked 2026-10-10 UTC).
- Branch: `codex/p1-subscription-counter-acceptance-20261010`.
- Isolated worktree: `/workspace/sub2api-p1`.
- Read repository `AGENTS.md`, `docs/DELIVERY_WORKFLOW.md`,
  `docs/PROJECT_HYGIENE.md`, and PR checklist. No `.agents`/`.codex` skill files
  were present in this environment or repository.
- Go 1.27.2; PostgreSQL 18.1 in a disposable Docker container, loopback only.
- Reuses `openIsolatedBillingDB` from the existing billing acceptance harness:
  requires an empty dedicated test database, then runs `repository.ApplyMigrations`.
  Runtime evidence: **402 migrations**, last filename
  `275_gateway_request_ledger.sql`. No ledger-only schema replacement.
- No real account credentials, provider calls, production access, operating-rule
  changes, migration changes, or paid generations. The outbound adapter rejects
  any origin other than its exact `httptest` server and rejects redirects.

## Runnable contract

From the repository root (Go and Docker required):

```bash
./scripts/test-subscription-counters.sh
./scripts/test-subscription-counters.sh '^TestSubscriptionCounterHTTP/edges/wrong_group_rolls_back$'
./scripts/test-subscription-counters.sh '^TestSubscriptionCounterHTTP/edges/exhausted_replay_must_remain_idempotent$'
./scripts/test-subscription-counters.sh '^TestSubscriptionCounterHTTP/(subscription|package)/(concurrent_duplicate|http_client_disconnect_observed_usage)$' -race
```

Each invocation owns a fresh container/database; an occupied port fails rather
than reusing or resetting another database. The wrapper removes only its own
container and anonymous volume. Do not run multiple wrappers simultaneously.
The new PR-only acceptance workflow runs the full strict matrix, preserving its
exit status. Known failures are not skips, expected failures, or softened asserts.

The HTTP fixture uses synthetic authentication and a thin routing adapter to
call the **real** `OpenAIGatewayService.Forward`, production ledger transport,
`RecordUsage`, `UsageBillingRepository.Apply`, and asynchronous usage-log writer.
User/admin subscription list and linked-usage HTTP reads use the production
handlers, services, repositories, DTOs, and database. It does not exercise the
complete production auth/scheduler/usage-worker route assembly, a browser, every
provider protocol, or a real payment purchase. Those remain distinct acceptance
scopes; the HTTP fixture must not be presented as full production acceptance.

For each successful logical request the tests independently check:

- Nonzero seeded before/after values for daily/weekly/monthly USD and package
  request/amount/token counters. Ordinary subscriptions have no entitlement.
- Ordinary USD deltas equal `SubscriptionCost` (documented 8-decimal monetary
  quantization); package amount equals `BilledCost` at its 10-decimal schema scale.
- Four disjoint token buckets are **82 input + 20 output + 7 cache creation +
  11 cache read = 120**, parsed from actual synthetic upstream HTTP responses.
- Exactly one applied settlement, dedup row, usage row, and fingerprint-bound
  verified billing link per independent private request. Public request headers
  and upstream response IDs deliberately repeat.
- Subscription/entitlement link IDs come from the admitted snapshot; a renewal
  becoming the current displayed package does not rewrite the charged ID.
- Wallet balance stays 100; both user and admin API reads match persisted counters
  and the linked billed amount, including settled zero charges.
- Main delta scenarios retain their window anchors. Separate tests cover calendar
  midnight daily reset, rolling 7-day/30-day anchors, stale-reset CAS after a real
  settlement, and explicit reset. The month contract is 30 days, not calendar month.

The matrix includes sequential replay, eight concurrent duplicate callbacks,
two independent requests, canceled billing context, a real HTTP client cancel
after upstream usage is observed, partial output plus upstream failure, zero
multiplier with surcharge configured, frozen request-time peak pricing plus
surcharge, request/amount/token exhaustion, expiry, invalid ownership, a targeted
PostgreSQL constraint failure after package increment, retry repair, and reset.

## Reproduced blockers (unchanged baseline production code)

### Wrong group entitlement is charged

Test: `TestSubscriptionCounterHTTP/edges/wrong_group_rolls_back`.

1. Seed a user, subscription/group, API key and active package; capture the admitted
   subscription/package snapshot.
2. Change **only the fixture entitlement's group** to another group of the same
   user, retaining its ID and active status.
3. Send real local HTTP through gateway forwarding, observed usage and settlement.
4. Expected: reject the mismatched relationship; all six deltas zero; no dedup or
   verified billing proof. Actual: no settlement error, dedup=1, verified=1;
   daily/weekly/monthly each +0.00026263, package requests +1, amount +0.0002626250,
   tokens +120. This is an injected internal snapshot mismatch; this test does not
   claim an unauthenticated external exploit or an observed production incident.

Relevant baseline code: `backend/internal/repository/usage_billing_repo.go`
`validateUsageBillingPackageEntitlementOwnership` (line 730), called before claim
at lines 116–123. It validates ID, user and active status, but not the package /
subscription / billing-group relationship; the increment query also omits group.

Minimal fix proposal for P0/main-thread coordination: validate the immutable
relationship against the actual subscription and billing group inside the same
transaction, including the free-package path, without relying on the current
UI-selected package. Preserve both wrong-user and wrong-group rejection tests.

### Exact replay fails after the first settlement exhausts the package

Test: `TestSubscriptionCounterHTTP/edges/exhausted_replay_must_remain_idempotent`.

1. Set the fixture request limit to current usage +1.
2. A single admitted request settles successfully and makes the package exhausted.
3. Replay the same request context, result, payload, subscription and entitlement.
4. Actual: `usage billing user does not own api key or subscription`. Counters
   remain charged exactly once; **this is a failed idempotent acknowledgment, not
   a reproduced double charge**.

Relevant baseline code: `usage_billing_repo.go:116–123` validates mutable active
status before `claimUsageBillingKey`; the exhausted entitlement fails at line 732.

Minimal fix proposal: after validating immutable ownership, recognize an existing
committed request/fingerprint and return verified dedup evidence before mutable
eligibility checks for a new charge. Do not simply remove eligibility checks or
change request identities. Review the frozen entitlement binding when doing so;
current `buildUsageBillingFingerprint` does not include `PackageEntitlementID`.

## Review and delivery

Author specification and quality checks covered the independent price oracle, four token buckets,
real writer/repository use, exact row cardinalities, rollback injection point,
strict errors, callback synchronization, private-ID separation, nullable billed
amounts, local-only network guard, and preservation of production files.
An independent reviewer must review the final diff and both RED reproductions
before this task can be accepted. A draft evidence PR is not merge approval.

Run results, exact tested file hashes, validation outcomes, and outstanding gates
are recorded in [evidence.json](./evidence.json), [counter-matrix.txt](./counter-matrix.txt),
and [counter-race.txt](./counter-race.txt). The PR description records the delivery
commit, PR URL, and CI outcome, avoiding a self-referential commit hash in this file. Shared
handoff/index changes are deferred to the main thread to avoid parallel ownership
conflicts. No merge or deployment is authorized by this record.
