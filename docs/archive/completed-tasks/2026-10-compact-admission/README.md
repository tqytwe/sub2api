# OpenAI routing, admission and response completion consistency

Status: local validation complete on the recorded baseline; this task delivers a review branch, draft PR and exact-SHA CI. PR metadata records the delivery commit and CI results. The root thread owns any later merge or deployment under its separate authorization.

## Scope and baseline

- Investigation baseline: `4080e2ac7e93dc9f435e0c0a4652834635cba7be` (`play/main`).
- Branch: `codex/compact-admission-model-20261009`, isolated worktree.
- Authorized intermediate base: `9e458b12db91d1c36402c288d61ef7a352beee1c` (#342).
- Intermediate authorized base: `b23e44625669a36a8f7ac10667e4b04b1904b7db` (#341 + #342), adopted by ordinary fast-forward merge after the running tests finished. Check-in DTO, plan editing, their tests/evidence and migration-test path changes do not overlap this patch.
- Validated authorized base: `2fbe13d90a2381a3bc8e7495a0c8b00abe2cfe3a` (#343), adopted by ordinary fast-forward only after all b23 local gates finished successfully. There is no file overlap. Independent specification and quality reviews found the transactional account/group-policy update compatible with authoritative admission, route fingerprints and model restrictions. All local combination gates passed.
- Preserve all public/routed/account model permissions, account eligibility and credential checks, channel restrictions, billing identities and frozen pricing timestamps.
- No production reads or writes, paid upstream requests, configuration changes, SQL changes, key-deletion reproduction, merge of this patch into `play/main` or deployment.
- Protected Fork behavior remains unchanged: `FORK-PRICING-005`, `FORK-BILLING-010`, `FORK-IMAGE-011`, `FORK-DEPLOY-006`. No new Fork policy or UI behavior is introduced; the registry and concurrent UI/governance work remain untouched.

## Findings

The supplied production evidence records two compact admission rejections after a model cooldown, but omits their enum reasons. It does not establish which admission guard rejected either request.

Code and failing tests establish separate inconsistencies:

1. Scheduling transient cooldown checks resolved the ordinary account mapping, while legacy compact forwarding can use a compact mapping or configured compact fallback. Request-level channel mapping was also lost in these checks.
2. Persisted model cooldown and sticky/previous-response checks could use the ordinary mapped model, disagreeing with actual compact forwarding.
3. Initial compact admission checked the routed body model before the actual outbound model had been resolved; a normal-model cooldown could therefore reject an otherwise eligible compact request.
4. Admission errors retained the client `admission_unavailable` response and ops `api_error` / `internal` classification, without a structured enum diagnosis.

The fix shares outbound model resolution for scheduling and compact initial admission. It preserves native-v2's explicit non-legacy mode, passthrough and raw-Chat semantics, and the final authoritative admission check. A late rejection never adds an automatic replay. Ordinary Responses retain their original initial boundary because image-only requests can undergo a later model conversion.

Diagnostics include only account ID, a truncated SHA-256 request-ID hash and an allowlisted reason. The forwarding boundary records predicate, credential and route denials once; standalone admission callers retain their own diagnosis. Request bodies, model names, account names, credentials and raw request IDs are not added to this event.

## Responses tools and error completion

The additional supplied production evidence links a local `gpt-6.1-sol` tools protocol rejection and a subsequent appended SSE `response.failed` to the same request. No additional production access was used. The Chat-only fallback guard predates this patch and remains in place.

Failing isolated tests establish two gaps:

- A Responses request could select a Chat-only OpenAI account even when the actual account-mapped model is `gpt-6.1-sol` and effective tools require Responses. Selection now uses the same mapping and effective-tool semantics as forwarding, including top-level tools, additional tools, completed tool-search discoveries with a declared client `tool_search`, and `tool_choice: none`. All three schedulers, sticky/previous-response reuse, latest-DB checks and final admission apply the predicate. The presence calculation first reuses the existing legacy ingress normalization: `messages/functions` becomes effective tools only when native `input` is absent or null. The original request body is not replaced by this projection. Other mapped models and non-OpenAI providers retain existing behavior. A capability-only empty pool gets a clear local 503; a late local denial cannot replay or penalize upstream health. If the scheduler snapshot still advertises Responses while the DB has just changed to Chat-only, selection can report the generic local 503 instead; this race still safely rejects the incompatible account.
- An already written JSON error was mistaken for a started SSE stream. Completed JSON errors now suppress fallback writes; a real SSE stream receives one terminal event. Fallback writers record completion and preserve the original status/message, including the final protocol guard. Compact and image heartbeat synchronization remains intact.

There are no account configuration edits, automatic probes, tool stripping, protection bypasses or production requests.

Scheduling stores only tool presence, reading top-level and additional-tools declarations without decoding native input/tool extensions. Discovery promotion cannot add the first declaration because it requires an existing `tool_search`. Malformed native tool declarations are conservatively treated as present: eligible Responses routes retain their existing validation, including Responses Lite `error.param`; an all-Chat-only sol pool can reject earlier with the capability 503. Legacy ingress conversion errors still stop before selection.

## Verification record

- TDD RED: ordinary-vs-compact transient and persisted cooldowns, channel mapping, global fallback, passthrough, reselection, initial forward admission and absent diagnostics failed on the original implementation for the expected reasons.
- Further compact quality-review RED→GREEN: advanced selection without handler context and ordinary/compact sticky/previous-response cooldown cases, including stale snapshots, now pass.
- Final combined targeted tests on `b23e446…` plus this patch: service, handler and repository passed with `-tags=unit`, including all `Keepalive` cases. Legacy `messages/functions` was separately RED before reusing ingress normalization; the final GREEN includes that correction and native-input/no-tools/malformed-legacy cases.
- Isolated PostgreSQL: both integration tests passed against disposable PostgreSQL 18.1 and Redis 8.4 with the repository migrations. They commit model cooldown, membership and protocol changes and verify authoritative admission observes them. An initial fixture-only compile failure was corrected by using the generated composite-key delete predicate; both reviewers confirmed the exact account/group cleanup scope.
- Final 2fbe PostgreSQL combination: all 11 main tests passed (23.342s), covering `TestAccountAdminUpdate.*`, `TestGroupCopyAccounts.*` and both added OpenAI admission tests against disposable PostgreSQL/Redis and full migrations.
- Full gates on the b23 combination passed: `make test-backend-unit`, `make test`, `make build`, Fork integrity, check-in contract and document links. Tests include default backend tests, golangci-lint (0 issues), frontend lint/design/typecheck and 489 passing frontend files / 3,476 passing tests (repository-existing 1 file / 2 test skips). Final 2fbe combination validation follows separately.
- The 2fbe combination: `make test-backend-unit`, `make test`, `make build`, Fork integrity, check-in contract and document links all passed, including default backend tests, golangci-lint (0 issues), frontend lint/design/typecheck and 491 passing frontend files / 3,491 passing tests (the same existing 1 file / 2 test skips).
- The first full unit run exposed two native image hotpath failures (unrelated large JSON numbers were decoded) and two OAuth Responses Lite error-field failures from the new projection. The projection was narrowed to raw declaration presence; existing validators and the original regression tests are retained. The expanded service/handler/repository targeted suite passed (28.872s / 0.089s / 0.286s), including those original failures, all Keepalive cases and the new raw-presence boundaries. Independent specification and quality follow-up reviews approved this correction. The subsequent full-unit gate passed.
- The next full-unit run passed those regressions but hit the unchanged `TestOpsSystemLogSinkSuppressesRetriesDuringBackoff`: its nominal 300ms observation ran past the 800ms backoff boundary. The existing test passed 20 isolated repetitions (6.355s), without changing code, assertions or timeouts; the subsequent full-unit gate passed.
- Initial frontend full run: 3462 passed, 3 timed out during concurrent Go compilation. The two affected files passed separately (83 tests), and the later serial full run passed as recorded above without timeout/assertion changes.
- Independent specification review: compact, protocol/completion, legacy-ingress handling, integration-fixture correction, final baseline combination and delivery documentation approved.
- Independent code-quality review: all implementation follow-ups, integration fixture, final baseline combination and delivery documentation approved after specification review.
- Draft PR, delivery SHA and exact-SHA CI: recorded in the PR delivery metadata.
- Production deployment, health and local-browser acceptance: outside the authorized scope; not performed.

Rollback is a normal revert of this patch after review. No schema, pricing or configuration rollback is required.
