# Account policy UI delivery evidence

Scope: complete the account group-model editor and retained-history totals on top of PR #340, plus the confirmed Group Copy Accounts policy-loss defect. No new migration, upstream version change, export-format extension, production data mutation, real OAuth flow, or paid upstream request.

## Source and reviews

- Worktree: `/workspace/account-policy-ui`; branch: `codex/account-policy-ui-20261009`.
- Actual production merge base: `4080e2ac7e93dc9f435e0c0a4652834635cba7be`; tree `2f2151ef4d83cc1e62a11039e4807fb3f9d50cd3`. This is PR #340's actual merged production commit, not its former candidate branch.
- Pinned upstream: ranxi2001/sub2api `v2.10.3`, `fd1b5ee4eeb20961fbb783fa6f136a1704271e90`.
- Independent specification review: passed after restricting editing to standard mode (simple-mode DTOs omit policies) and rejecting oversized source-policy unions.
- Independent quality review: passed after making group attributes, memberships, policies and scheduler notification one transaction, with account-then-sorted-group lock order.
- Protected fork boundaries: FORK-BILLING-010, FORK-PRICING-005 and FORK-DEPLOY-006 remain unchanged. TPS, tooltip widths, memory fixtures, pricing/surcharge logic and release safeguards are retained.

## Compact acceptance matrix

The final-build local joint run passed every browser/HTTP/SQL row below on 2026-10-09 UTC, including `/v1/models` and oversized-union HTTP rollback. The [sanitized run output](joint-results.txt) records the executed checks. Separate unit and repository evidence covers the rows that name those suites.

| Surface / action | Required durable result | Evidence |
| --- | --- | --- |
| Account editor: edit one group | Full policy snapshot preserves other groups; SQL, detail GET and scheduler cache agree | Real built UI → HTTP → PostgreSQL → GET → Redis → refresh/reopen |
| Blank, null, empty object, wildcard | Blank removes that group's extra restriction; null preserves; `{}` clears all; wildcard intersects account support | Real HTTP/DB plus local `/v1/models`, no forwarding |
| Other fields, unchanged groups, cancel | Omit unchanged policy field; cancel issues no PUT | Browser request capture and database queries |
| Duplicate submit, backend failure, retry | One pending write; failed transaction rolls back; draft retained; retry succeeds | Synchronous UI double-submit and real outbox constraint failure |
| Membership/bulk/clone | Retain surviving policies, remove departed bindings, re-added group unrestricted; account/group clones preserve policy | Actual endpoints, SQL and detail GET |
| Group Copy Accounts | Surviving target policies/priority retained; new members inherit union; unrestricted source yields unrestricted; departed members removed | Group UI → HTTP → DB → GET; six PostgreSQL integration tests |
| Copy failure and concurrency | Oversized union or notification failure leaves no partial group attributes/membership or orphan; reciprocal empty copies do not deadlock | Real service/repository PostgreSQL tests |
| Permissions and validation | Ordinary-user admin requests denied; invalid IDs/types/length rejected without mutation | Real middleware/API and normal-user browser route guard |
| Retained-history totals | 1,207,000 tokens and account cost 12; today's account cost 2 and user charge 9 remain distinct | Local usage records → stats endpoint → rendered text |
| Legacy stats, zero, loading/error | Missing fields hidden; explicit zero shown; existing states retained | Component tests |
| Chinese desktop, keyboard, pagination/filter | 1280×900 / 1600×1000 light/dark, keyboard focus, pages 20/3 and empty filter | Actual browser and real paginated endpoints |

## Semantics and limits

Policies are edited in standard administrator mode. Simple mode deliberately does not offer policy controls or submit a policy snapshot because its existing detail projection omits policy fields and filters composite bindings; this change does not alter that historical membership filtering contract.

An omitted `group_allowed_models` or JSON `null` preserves policies for surviving bindings. An explicit map replaces the snapshot: missing or empty entries have no additional restriction. The UI only sends that map after a semantic policy change. Limits intersect supported account models; `*` is a suffix wildcard, and alias plus mapped-target checks still apply in the existing routing core. Empty is not deny-all.

Copy Accounts retains a surviving target binding's policy and priority. Newly copied accounts receive the union of selected source bindings; an unrestricted source makes that union unrestricted. A union over the backend limits is rejected atomically. Source and target groups are locked in sorted order after account locks.

The cumulative labels mean statistics from retained usage history and account cost. They neither represent user charges nor reconstruct deleted historical records. Account exports still omit group policies and are **not a policy backup**.

## Local joint-test setup

The archived [runner](live-acceptance.cjs) uses no API mock or interception. It expects disposable containers `policy-ui-postgres` (PostgreSQL 18.1, port 55432, database `sub2api_policy_test`) and `policy-ui-redis` (Redis 8.4, port 56379), a migrated actual backend bound to `127.0.0.1:8080`, and the production frontend build served on `127.0.0.1:4174` with `/api/` forwarded to that backend. Only synthetic identities and task-owned fixture records are used. An initial disabled synthetic user prevents bootstrap from generating an administrator password; all test session credentials are created in memory. The runner reads the local fixture JWT secret without logging it, signs short-lived sessions, and never creates a persistent external authorization.

Chromium is `/usr/bin/chromium`; set `PLAYWRIGHT_MODULE` to the installed Playwright module path if it is not on Node's module search path. The runner intentionally mutates its fixed disposable database and must not be adapted to point at production. The test account's upstream base is localhost port 9, billing probes are disabled, and model-discovery checks never forward requests. The local API key is generated in memory and only used for model discovery. After successful joint acceptance, the local server/proxies, both named disposable containers and fixture application data were removed; no test service remains running.

Baseline/prototype screenshots are design-only mocked-data browser captures made before implementation. Updated screenshots use the actual local backend/database; see the [visual review](../../../visual-reviews/2026-10-09-account-policy-ui.md). They are local technical evidence, not production acceptance.

## Gate investigation history

An initial `make test` failed the unchanged `TestServerTimingConnectorRecordsDriverCallsWithoutRowLifetime` during concurrent compilation; the complete rerun passed. The new repository interface initially exposed one missing tagged server-test stub, which was fixed and subsequently passed. Under concurrent full-suite load, the unchanged tagged refresh/WebSocket timing tests produced three failures across runs: `TestOpenAIWSConnPool_AcquireQueueWaitMetrics`, `TestTokenRefreshService_LateSuccessPastAttemptDeadlineIsRejected`, and `TestRefreshIfNeeded_LateSuccessAfterDeadlineDoesNotPersist`. The service late-deadline case passed 20 isolated repetitions. The existing refresh paths use `ctx.Err()` after the provider returns; callback scheduling delay remains a possible deadline-boundary concern outside this UI/policy batch. These production refresh implementations have not been changed or their assertions weakened. The final complete tagged suite is run after other suites finish, with `GOFLAGS="-p=1 -parallel=1"` and `GOMAXPROCS=2`; its result is recorded below.

## Gates and remaining boundary

Final gate results and remote PR/head are recorded in the PR description after completion. The exact CI PostgreSQL/Redis scripts passed locally: 49/49 required checks, 198 test/subtest passes, no skips. `make build`, design governance and Fork integrity passed. `make test` passed (Go suite, golangci-lint: 0 issues; frontend lint/typecheck and 491 files / 3,480 tests). The separate complete tagged unit gate is undergoing final serial confirmation. Required local commands are `make test`, `make test-backend-unit`, `make build`, `./scripts/check-fork-integrity.sh`, and the bounded PostgreSQL/Redis suite in `.github/workflows/core-migration-integration.yml` (including all six new copy-policy tests).

This task publishes a draft PR only. Merge order, production deployment SHA/health and final user-local browser acceptance (visitor, ordinary user and administrator) remain the main thread's responsibility. No deployment or production acceptance is claimed by these local results. Mobile and Canvas are explicitly outside this batch.
