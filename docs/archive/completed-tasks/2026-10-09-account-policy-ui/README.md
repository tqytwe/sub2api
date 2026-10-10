# Account policy UI delivery evidence

Scope: complete the account group-model editor and retained-history totals on top of PR #340, plus the confirmed Group Copy Accounts policy-loss defect and atomic account-edit boundary. No new migration, upstream version change, export-format extension, production data mutation, real OAuth flow, or paid upstream request.

## Source and reviews

- Worktree: `/workspace/account-policy-ui`; branch: `codex/account-policy-ui-20261009`.
- PR #340 actual merged base: `4080e2ac7e93dc9f435e0c0a4652834635cba7be`; tree `2f2151ef4d83cc1e62a11039e4807fb3f9d50cd3`. The branch subsequently merged actual `origin/play/main` `9e458b12db91d1c36402c288d61ef7a352beee1c` (tree `29ec07502e5b3e49d4325ea5178a0ad7622e0278`, PR #342) with a normal merge, without rebase or force push.
- Latest combination: normal merge `be51c3f67deca915077bb03648471c0e7c5493ac` (tree `fc4648a98213bf59836682a0887bc855ba871ccb`) includes actual `origin/play/main` `b23e44625669a36a8f7ac10667e4b04b1904b7db` (tree `f388d0fc3de2ec09ecc763efd432d57eea8c38f0`, PR #341 and #342). Independent specification and quality compatibility reviews passed. Account code, migrations and release protection are byte-identical to the reviewed `25c0bcb7c` revision; the new base changes Plan projection and differential editing plus their tests/evidence.
- Pinned upstream: ranxi2001/sub2api `v2.10.3`, `fd1b5ee4eeb20961fbb783fa6f136a1704271e90`.
- Independent specification review: passed after restricting editing to standard mode (simple-mode DTOs omit policies) and rejecting oversized source-policy unions.
- Independent quality review: passed after making group attributes, memberships, policies and scheduler notification one transaction, with account-then-sorted-group lock order. The 2026-10-10 specification and quality re-reviews also passed for the optional-zero fields and atomic account writer, including shadow proxy/cache coverage and isolated fixture cleanup.
- Protected fork boundaries: FORK-BILLING-010, FORK-PRICING-005 and FORK-DEPLOY-006 remain unchanged. TPS, tooltip widths, memory fixtures, pricing/surcharge logic and release safeguards are retained.

## Compact acceptance matrix

The local joint run for account revision `25c0bcb7cfe5c0a767c61a67e65aaaf37a916a61` (tree `39699563b87292695e8502a1ddad66ee6857e3f4`) passed every browser/HTTP/SQL row below on 2026-10-10 UTC, including `/v1/models` and oversized-union HTTP rollback. The [sanitized run output](joint-results.txt) records the executed checks. These browser results were retained after the account-code-identical merge `be51c3f67`; they are not represented as a new browser run on that merge. Separate unit and repository evidence covers the rows that name those suites.

| Surface / action | Required durable result | Evidence |
| --- | --- | --- |
| Account editor: edit one group | Full policy snapshot preserves other groups; SQL, detail GET and scheduler cache agree | Real built UI → HTTP → PostgreSQL → GET → Redis → refresh/reopen |
| Blank, null, empty object, wildcard | Blank removes that group's extra restriction; null preserves; `{}` clears all; wildcard intersects account support | Real HTTP/DB plus local `/v1/models`, no forwarding |
| Other fields, unchanged groups, cancel | Omit unchanged policy field; cancel issues no PUT | Browser request capture and database queries |
| Duplicate submit, backend failure, retry | One pending write; activation, changed credentials and policy all roll back; SQL/GET/Redis unchanged; draft retained; retry commits all | Synchronous UI double-submit and real outbox constraint failure |
| Shadow proxy and billing intent | Parent/child proxies roll back together, no cache publication before commit; omitted multiplier preserves current value, explicit zero applies | Real PostgreSQL integration tests; cache callback reads committed DB state |
| Membership/bulk/clone | Retain surviving policies, remove departed bindings, re-added group unrestricted; account/group clones preserve policy | Actual endpoints, SQL and detail GET |
| Group Copy Accounts | Surviving target policies/priority retained; new members inherit union; unrestricted source yields unrestricted; departed members removed | Group UI → HTTP → DB → GET; six PostgreSQL integration tests |
| Copy failure and concurrency | Oversized union or notification failure leaves no partial group attributes/membership or orphan; reciprocal empty copies do not deadlock | Real service/repository PostgreSQL tests |
| Permissions and validation | Ordinary-user admin requests denied; invalid IDs/types/length rejected without mutation | Real middleware/API and normal-user browser route guard |
| Retained-history totals | 1,207,000 tokens and account cost 12; today's account cost 2 and user charge 9 remain distinct | Local usage records → stats endpoint → rendered text |
| Zero stats and legacy/error states | No history returns 0/0; retained tokens with zero account cost remains zero despite user charge 9; failed lifetime query leaves fields absent | Real single/batch HTTP and UI screenshots; service/component tests for failure, missing fields, loading/error |
| Chinese desktop, keyboard, pagination/filter | 1280×900 / 1600×1000 light/dark, keyboard focus, pages 20/3 and empty filter | Actual browser and real paginated endpoints |

## Semantics and limits

Policies are edited in standard administrator mode. Simple mode deliberately does not offer policy controls or submit a policy snapshot because its existing detail projection omits policy fields and filters composite bindings; this change does not alter that historical membership filtering contract.

An omitted `group_allowed_models` or JSON `null` preserves policies for surviving bindings. An explicit map replaces the snapshot: missing or empty entries have no additional restriction. The UI only sends that map after a semantic policy change. Limits intersect supported account models; `*` is a suffix wildcard, and alias plus mapped-target checks still apply in the existing routing core. Empty is not deny-all.

Copy Accounts retains a surviving target binding's policy and priority. Newly copied accounts receive the union of selected source bindings; an unrestricted source makes that union unrestricted. A union over the backend limits is rejected atomically. Source and target groups are locked in sorted order after account locks.

The cumulative labels mean statistics from retained usage history and account cost. They neither represent user charges nor reconstruct deleted historical records. Account exports still omit group policies and are **not a policy backup**.

## Review boundary fixes

The previous cumulative scalar fields used `omitempty`, which hid valid zeros. Optional pointers now distinguish a successful zero result (including a successful empty query) from a failed lifetime query, which remains absent for compatibility. No cost calculation or surcharge rule changed.

The existing account update sequence could commit activation and credentials before a later group-policy failure. [The baseline real-HTTP reproduction](boundary-red-results.txt) shows absent zero fields and an account remaining active after a rejected combined edit. The new production-required repository capability puts attributes, explicit billing settings, inherited shadow proxies, group membership, policies and outbox into one transaction. It reads the result in that transaction and only refreshes scheduler snapshots after commit. Caller-owned transactions use their transactional outbox. The legacy path is only reachable by narrowly constructed package test doubles; the production constructor requires the atomic capability.

Regression tests were confirmed red for single/batch/fallback zero serialization and both policy-only and rebind activation failures before implementation. The fixed repository suite also checks shadow rollback, no pre-commit cache writes, post-commit database-visible snapshots, and billing intent. The browser runner injects a real outbox constraint, attempts inactive→active plus credentials and policy changes, checks SQL/GET/Redis rollback, preserves the form and retries successfully.

## Local joint-test setup

The archived [runner](live-acceptance.cjs) uses no API mock or interception. It expects disposable containers `policy-ui-postgres` (PostgreSQL 18.1, port 55432, database `sub2api_policy_test`) and `policy-ui-redis` (Redis 8.4, port 56379), a migrated actual backend bound to `127.0.0.1:8080`, and the production frontend build served on `127.0.0.1:4174` with `/api/` forwarded to that backend. Only synthetic identities and task-owned fixture records are used. An initial disabled synthetic user prevents bootstrap from generating an administrator password; all test session credentials are created in memory. The runner reads the local fixture JWT secret without logging it, signs short-lived sessions, and never creates a persistent external authorization.

Chromium is `/usr/bin/chromium`; set `PLAYWRIGHT_MODULE` to the installed Playwright module path if it is not on Node's module search path. The runner intentionally mutates its fixed disposable database and must not be adapted to point at production. The test account's upstream base is localhost port 9, billing probes are disabled, and model-discovery checks never forward requests. The local API key is generated in memory and only used for model discovery. After final joint acceptance on 2026-10-10, the local backend/proxy, both named disposable containers and their volumes, and fixture application data were removed. No joint-test service or fixture credential remains active.

Baseline/prototype screenshots are design-only mocked-data browser captures made before implementation. Updated screenshots use the actual local backend/database; see the [visual review](../../../visual-reviews/2026-10-09-account-policy-ui.md). They are local technical evidence, not production acceptance.

## Gate investigation history

An initial `make test` failed the unchanged `TestServerTimingConnectorRecordsDriverCallsWithoutRowLifetime` during concurrent compilation; the complete rerun passed. The new repository interface initially exposed one missing tagged server-test stub, which was fixed and subsequently passed. Under concurrent full-suite load, the unchanged tagged refresh/WebSocket timing tests produced three failures across runs: `TestOpenAIWSConnPool_AcquireQueueWaitMetrics`, `TestTokenRefreshService_LateSuccessPastAttemptDeadlineIsRejected`, and `TestRefreshIfNeeded_LateSuccessAfterDeadlineDoesNotPersist`. The service late-deadline case passed 20 isolated repetitions. The existing refresh paths use `ctx.Err()` after the provider returns; callback scheduling delay remains a possible deadline-boundary concern outside this UI/policy batch. These production refresh implementations have not been changed or their assertions weakened. The final complete tagged suite is run after other suites finish, with `GOFLAGS="-p=1 -parallel=1"` and `GOMAXPROCS=2`; its result is recorded below.

## Gates and remaining boundary

Local gate results for account revision `25c0bcb7c` are recorded in [gate-results.txt](gate-results.txt), with PostgreSQL/Redis coverage in [core-results.txt](core-results.txt). The latest combination's independently rerun gates are recorded in [combined-gate-results.txt](combined-gate-results.txt). The PR description records the exact final head and its remote checks. Old-head CI is not used as evidence for a new head. Required commands include `make test`, `make test-backend-unit`, `make build`, `./scripts/check-fork-integrity.sh`, design governance, the exact bounded PostgreSQL/Redis workflow scripts, and the newly merged check-in HTTP/frontend contract.

This task publishes a draft PR only. Merge order, production deployment SHA/health and final user-local browser acceptance (visitor, ordinary user and administrator) remain the main thread's responsibility. No deployment or production acceptance is claimed by these local results. Mobile and Canvas are explicitly outside this batch.
