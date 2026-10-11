# Admin request routing detail

## Contract and authorized scope

Initial base: `fb2c9747b58ed5d980a36ea6a2316ead2aa03b7e` (`play/main`, merged #360).
Isolated worktree: `/workspace/sub2api-routing-ui`; branch: `codex/admin-routing-detail-20261011`.
Dependency read: #359 `ae832b16b63139b2395008a1f2073d7cb15161f7`, not merged when work began.
[Backend coordination](https://github.com/tqytwe/sub2api/pull/359#issuecomment-6108343601).
No backend, billing, auth protocol, migration, history backfill or release change is authorized here.

`GET /api/v1/admin/requests/:id/routing` accepts the existing private request UUID. It is admin-only,
uses existing authentication and no-store, and returns the normal response envelope. Invalid UUID is
400, missing request 404, read failure 503; existing auth denies guests/users. There is no user routing
endpoint. Existing summary/attempts/billing/wallet/diagnostics APIs remain independent and paginated.

Exact source: `backend/internal/requestledger/routing.go`, `routing_query.go`,
`backend/internal/handler/routing_handler.go` at the dependency SHA above.
Payload: `request_id`, `coverage_version`, `status`, `selection_count`, `event_count`, `veto_count`,
`omitted_count`, `incomplete`, `source_population_complete`, `events`.
Each event: `selection_no`, `stage`, `reason`, optional `account_id` (omitted at zero), `value`, `at`.
The endpoint returns at most 128 events / a 32 KiB stored event batch, not a cursor page. UI uses local
20-event pages within that bounded response and explicit refresh; it does not invent API cursors.
During pending/incomplete states `event_count` can differ from returned events: show both, do not reject
that valid loss/delay evidence or fabricate empty proof. Empty version is valid for historical data.

## Display semantics

Six distinct states: complete, truncated, pending, incomplete, not_recorded, unsupported_coverage.
Complete describes the captured scope only. Truncation shows omitted_count; pending invites refresh;
incomplete preserves recorded evidence and its gap; historical not_recorded is not reconstructable;
unsupported_coverage names the current OpenAI-compatible HTTP text-only boundary. WS, media and actual
Anthropic/Gemini dispatch are excluded. Existing 27 historical 503s and gpt22 losses are not restored.

Show candidate scope and raw coverage version, selection count, expected/stored event counts, veto count,
omissions and incomplete flag. Pre-filtered candidate count zero never means a group has no accounts.
Selection numbers never identify upstream attempts. Selection_selected/wait never proves forwarding.
Highlight the first **recorded** veto in returned order, with account/stage/selection and reason; never
rerun policy or sort by guessed priority. A version-specific explicit whitelist distinguishes real veto
reasons from selection, pool and observation events. Unknown reason/version is not inferred as veto.
No body, credentials, arbitrary error text, upstream URL/header or account credential payload is read.
All identifiers render as text, not HTML or untrusted links.

## Lifecycle and failure isolation

Mount the region only for an authenticated administrator in the admin view. Reuse #360's detailScope,
waitForSessionReady and official auth session identity, without a second identity protocol. Close,
request switch, logout, role/account/session switch abort and clear; late success/error cannot publish.
Same-session token renewal preserves scope. Routing 403/404/503/invalid payload becomes a local retryable
error; billing, usage and their independent cursors remain usable. Manual refresh replaces the routing
snapshot and resets only its local page. No polling, generation, mutation, billing or migration calls.

## TDD / acceptance plan

First real HTTP RED with actual RequestLedgerView, Pinia/auth actions, Axios/interceptors and a synthetic
Node HTTP contract server. Verify exact path, six states, actual first veto (not observation), zero pool,
truncation, scope, selected semantics, local paging, 503 recovery, malformed/mismatched data, user/guest
denial and UI absence, cancellation/late response, same-session refresh and account/session isolation.
Real built resources must repeat the role/state/viewport checks. Before #359 lands these are explicitly
frontend contract simulations, not merged HTTP/PG routing integration. Once the dependency enters main,
ordinary merge and run real handler/PG combination checks. No merge/deployment by this task.

Sequential independent specification and code quality reviews, applicable full tests/build/Fork gates,
resource hashes, source SHA, exact dependency status and residual production acceptance must be recorded.

## Initial fb baseline verification (before auth PR362 combination)

Full make test passed: backend tests + lint 0 issues, frontend 504 files / 3720 tests passed,
3 files / 8 pre-existing opt-in tests skipped. Full build, embed build/test, Fork integrity,
design/handoff8/docs-link checks passed. Sequential independent reviews passed. Actual
compiled routing contract browser and separate core HTTP/PG captures passed within the
recorded scope. All356 final assets match the captures. Logs/hashes are in validation.json;
pre-362 preserves the original result before subsequent auth combination work.

The parent subsequently authorized an ordinary merge of PR362 mainline
6ae024e6b732b64b2ce6f7ba56c5be4e20b88bdb. New auth ownership behavior will be preserved,
with combination tests/resource captures before push. PR359 remains unmerged. Its latest checked
head `add3fa4dbe8c59e0276e0043eb09270a4b630f89` has the same three contract files byte-for-byte.
The PR359 branch has only been fetched/read, never merged into this UI branch.

## Final PR362 combination verification

Ordinary mainline merge preserves all four auth files exactly as6ae, all six routing production
files exactly as initial UI commit aa804d32, and backend/migrations/dependencies unchanged.
Full make test passed: backend default suite/lint, frontend506 files/3869 tests passed,
3 files/8 existing opt-in tests skipped. Full build, embed build/test and Fork integrity passed.
Sequential reviews passed (spec4/4, quality44/44); new HTTP file24/24 passed.
New actual compiled routing contract browser includes reverse login responses and owned storage
publication failure/recovery; independent actual HTTP/PG core ledger capture passes both roles,
6 USD/3 images, zero page errors. These remain distinct from pending routing PG integration.
All356 captured resource hashes match final output; see validation.json/assets.sha256.
Latest old/new user360 header measurements both466px; earlier489px and first combination503px
are retained. Dialogs remain360px, admin widths within viewport; no global responsive claim.

Draft PR: [363](https://github.com/tqytwe/sub2api/pull/363). Tested combination commit: `7462d8fc8730b064e7c56447a9570a53ab54a9a8`.
The final PR head may include documentation-only handoff metadata; source preservation proof
and build hashes remain attached to this tested combination. No merge or deployment performed.
