# OAuth refresh deadline callback race

Status: deterministic regression and both independent reviews passed; [complete local gate results](local-gates.json) recorded separately from exact-commit CI in the draft PR. Separate backend repair authorized by the main thread after the editor task's full test exposed the preexisting defect. No merge or deployment by this agent.

## Source and scope

- Worktree `/workspace/refresh-deadline-fix`, branch `codex/refresh-deadline-fix-20261010`.
- Initial reproduction/review base `3843ff3e931349595b8793b52504b02a177f12c9`. Final integration base `7291ae9a2be4db7d97b8b641d053f7822276dc23`, fetched from actual `play/main` after #346 merged. Its WebSocket changes do not touch the deadline repair files. The incomplete earlier full-test run was stopped when this drift was discovered; no pass is claimed for it. All completed targeted evidence is retained.
- Independent from editor PR349, including its normal main-merge/manual-recovery follow-up. Neither patch needs the other's code; their production ordering and revalidation belong to the main thread.
- No schema, endpoint, UI, pricing, scheduling policy, upgrade source or deployment guard change. Existing TPS, tooltip and memory fixtures remain unchanged.

## Observed defect and minimal repair

The editor branch's first complete `make test` failed in the unchanged `TestTokenRefreshService_LateSuccessPastAttemptDeadlineIsRejected`: a synthetic refresher slept 30ms against a 10ms attempt budget, but refresh returned nil. The first failure and investigation are archived in PR349. Repeating the test successfully did not repair the bug.

`TokenRefreshService.refreshWithRetryWithRateGate` and `OAuthRefreshAPI.RefreshIfNeeded` relied on `ctx.Err()` after the provider returned. Go records deadline cancellation through a scheduled callback, so a passed deadline can coexist with a nil context error. The common helper now preserves a real context error first, then rejects `now >= deadline`. Its optional instance-local clock is nil in production and uses `time.Now`, including Go's monotonic comparison; there is no mutable global clock.

The fallback and real unified API check this boundary before credential persistence. Service classification and parent-context exits use the same rule so an expired parent does not trigger retries or cooldown writes. An internal attempt timeout retains the existing transient failure handling. Credentials already successfully persisted before only detached cleanup crosses the attempt deadline still count as success and synchronize cache state, without retry or provider-breaker evidence.

## Acceptance matrix

| Case | Evidence | Result |
| --- | --- | --- |
| Fallback and service→real API, exact deadline / +1ns, cancellation callback pending | Synchronous clock, `ctx.Err()==nil`, zero credential writes and unchanged repository document | RED→GREEN |
| Fallback and service→real API, deadline−1ns | One successful write, no cooldown | PASS |
| Parent deadline expired before provider result is accepted | No credential/error/cooldown write, no retry | RED→GREEN |
| Direct request API, Grok CAS and Gemini ordinary update path | Late credentials rejected before either persistence branch | PASS |
| Commit succeeds, then lock-release cleanup crosses attempt deadline | One write, success, no retry/cooldown/breaker | PASS |
| Original 30ms/10ms timeout test | Unchanged assertion and timing | PASS |
| Existing parent cancellation and durable cleanup/cache synchronization | Existing tests retained unchanged | PASS |
| Real PostgreSQL/Redis repository regressions | 54 required checks / 203 test-subtest passes, zero skips | PASS |
| Production impact of the original failure | Not established by a repository-double failure | Not claimed |

## TDD and independent review

[Deterministic RED](red.txt) records six failed boundary/parent cases against the prior behavior (only clock seams added). [First GREEN](green.txt) records the minimal correction plus the original timeout/cancellation tests. [Final targeted run](targeted-final.txt) includes 9 top-level tests and 19 test/subtest passes, adding on-time success, direct request paths and existing post-persist cleanup coverage.

The local tests advance only an instance clock during synchronous callbacks. The real context has a long timeout and is asserted uncancelled at the boundary. This controls the exact state without sleeps or relying on scheduler load. Existing sleep-based regression tests are unchanged. These tests exercise the real service/API with repository/cache doubles; they are not claimed as an actual PostgreSQL late-write reproduction or production incident evidence. No real OAuth authorization, provider call, production identity, credential or data is used.

Independent specification review `/root/spec_review`: final targeted acceptance PASS. Independent quality review `/root/quality_review`: PASS, no required code changes; confirmed the final targeted cases and evidence limits.

## Gates and replay

Use the repository Go toolchain and run `go test -tags=unit ./internal/service -run 'TestRefreshDeadline|TestTokenRefreshService_LateSuccessPastAttemptDeadlineIsRejected|TestTokenRefreshService_ParentCancellationStopsRetryWithoutAccountMutation|TestTokenRefreshService_PersistedSuccessCrossingAttemptDeadlineStaysSuccessful|TestPathA_ParentCancellationAfterPersistStillSynchronizesCacheState|TestRefreshIfNeeded_LateSuccessAfterDeadlineDoesNotPersist' -count=100 -v` from `backend` for the repeated targeted check. Complete `make test`, `make build`, tagged backend unit, isolated core PostgreSQL/Redis integration, check-in HTTP contract, Fork integrity and documentation gates run separately. The [100-repeat result](repeated-summary.txt) passed all 900 top-level / 1900 test-subtest executions, including the original timing assertions. Complete gate results and source hashes are recorded in [local-gates.json](local-gates.json): all seven commands exit 0. `make test` passes 491 frontend files / 3494 tests with two live-HTTP skips; the separate HTTP contract passes both. [Selected command output](completed-gates-summary.txt) records the actual suite and integration counts.

Production deployment and user-local acceptance remain with the main thread. The editor PR's six actual UI→HTTP→SQL→GET→reload cases are separate evidence and are not relabeled as tests of this deadline fix.
