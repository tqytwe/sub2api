# WS interrupted-turn observed usage

- Branch: `codex/ws-observed-usage-20261010`
- Initial baseline: `2fbe13d90a2381a3bc8e7495a0c8b00abe2cfe3a`
- Reviewed cache patch integrated with a normal fast-forward: `1f37c7a8312f06218036fcbb0335842a6b23e50f`; no shared modified files or conflicts.
- After `e0344f556` completed all six CI checks, main `3843ff3e931349595b8793b52504b02a177f12c9` (read-only quota queries, PR #347) was integrated with a normal merge held before commit for final gates. The only shared file was `docs/FORK_CUSTOMIZATIONS.md`; both billing boundaries merged without conflict, and no production WS files overlap.
- Isolated worktree: `/workspace/ws-observed-usage`
- Protected customization: `FORK-BILLING-010`; existing pricing, subscription, image and mandatory-settlement paths remain in use.

## Scope and evidence

The ctx_pool relay previously returned nil after upstream read failures and non-disconnect downstream write failures despite having parsed usage. Its preemption branch skipped AfterTurn. The passthrough relay retained nonterminal usage in turnUsage but returned only completed-turn totals; the adapter returned early on cancellation and supplied nil on other errors.

The patch preserves a current-turn snapshot. Passthrough exposes `RelayResult.UnfinishedTurn` separately from the completed aggregate, joins the upstream reader before taking it, and passes it to the existing `AfterTurn(turn, result, err)` before cancellation, lease-loss or preemption exits. ctx_pool uses the same result builder for terminal and interrupted paths. Observed usage prevents retry/failover of that generation.

Completed response IDs are guarded against duplicate terminal frames. The handler claims billable turn callbacks once, copies their result for the mandatory worker, and supplies a private `gateway:ws:<random namespace>:<turn>` fallback when upstream response ID is absent. Existing upstream response IDs keep their identity. Unknown text usage is not estimated or charged; a metadata-only `usage_unknown` diagnostic identifies user/key/account/turn. Full request lifecycle persistence is a separate task.

No production credentials, paid requests, production account changes, production log settings, deleted-key exploit reproduction, history backfill or historical extra charges were used. All upstreams are synthetic and all sockets/databases are isolated test services. This defect does not establish the cause of the reported 827.38765-point historical difference.

## TDD and review

- RED: `TestWSObservedUsageSurvivesInterruptedTurn` reproduced nil partial results and missing cancellation/preemption callbacks in both modes.
- RED: `TestRelayTerminalSettlementIdentity` reproduced duplicate terminal callbacks and omitted no-ID terminal callbacks.
- GREEN: service coverage includes completed turn + partial turn, read failure, real client-write deadline, cancellation, preemption, lease loss, no usage and disconnect after a terminal.
- Handler coverage includes both modes with/without response IDs for completed, in-progress then disconnect, incomplete, failed and error; verifies user/key/account and exact per-turn token values. A metered first rate-limit error must settle without replay.
- Relay coverage verifies separate completed/unfinished usage, read/write/cancel, terminal dedup and no-ID turns. Handler claim coverage exercises 32 concurrent duplicate claims and distinct stable private turn IDs.
- PostgreSQL coverage connects the actual relay result to RecordUsage, injects a settlement failure, verifies an unsettled zero-charge row retains observed tokens, then concurrently retries the same result and reconciles two usage rows, two dedup rows, two wallet debits and the exact final balance.
- Specification review: bounded to observed current-turn usage, identity and dedup; no aggregate re-billing or new pricing rules.
- Code quality review: upstream reader is joined before snapshot; callback claims are synchronized; queued results and billing context are independent per turn; zero-usage retries do not claim a billable turn.
- Full-suite compatibility review caught and fixed two boundaries: clear the passthrough metering guard after each completed turn, and retain nil-result cleanup for admission rejected before an upstream write. Ignore duplicate frames before policy/lifecycle hooks. Both existing regression tests pass. The prior cyber test now verifies metered errors cannot fail over while unmetered errors retain their existing failover behavior.
- Parent independent review found two additional blocking regressions in the first submitted head; both have isolated RED/GREEN evidence. Bare error followed by the same response's completed/done/incomplete terminal settled the fallback too early and hid the real terminal. The relay now lets that authoritative terminal replace the pending error; tests include errors with/without ID or usage, previous and next turns, repeated terminal frames, exact aggregate usage and downstream delivery.
- The same-account HTTP bridge retry reused handler hooks while restarting the Proxy-local turn counter at 1. A completed earlier turn's claim could therefore suppress later usage. Each Proxy call now binds every hook to an immutable logical-turn offset; retry keeps the current logical turn and missing-ID billing key. A real handler/OAuth bridge test covers first-turn success, second-turn 429, same-account retry success, exact user/key/account attribution and two usage rows, with and without response IDs. A binding test verifies later turns and late duplicate callbacks retain distinct identities.
- Follow-up independent review required the authoritative terminal itself to omit ID. The complete created/error/final ID-presence matrix produced 19 RED cases, including the distinct-ID failed-terminal boundary. All 32 same-turn combinations and four distinct-ID cases now pass: absent IDs do not establish a different turn, a missing terminal ID retains the pending turn's observed ID, and explicitly different IDs remain separate. An unknown-ID error before a new turn now has an explicit response.created boundary in its regression fixture. Full relay tests passed (6.019s), followed by service/handler cross-layer tests (23.262s / 2.365s).
- Final self-review reproduced a separate boundary with no response IDs: after a bare error, the client explicitly sends another response.create before the next terminal. The failing test settled only the new 3/2 tokens and lost the old 5/1. The relay now recognizes that accepted client turn and preserves its pending start when settling the prior error; both turns settle independently (8/3 total). The complete relay package passed after this fix (6.327s).

## Delivery gates

The submitted `e0344f556` passed all six CI checks. The final missing-ID and client-turn boundary fixes are now combined with main `3843ff3`; fresh local gates below precede the normal merge commit and final-head CI. The PR remains draft pending final-head CI and parent review.

| Gate | Result |
| --- | --- |
| `make test` | Backend default suite, golangci-lint (0 issues), frontend lint/typecheck/design governance; frontend 491 files / 3494 tests passed, 1 file / 2 tests skipped |
| `go -C backend test -tags=unit ./...` | Full unit-tagged suite passed |
| `make build` | Backend and frontend production builds passed |
| `./scripts/check-fork-integrity.sh` | All protected Fork checks passed |
| Related service/relay/handler/admin `go test -race` | All four packages passed (6.864s / 7.145s / 3.704s / 1.145s), including authoritative-terminal, accepted-next-client-turn, retry-identity and read-only quota regressions |
| `TestWSObservedUsagePostgresExactlyOnce` with integration tag | Isolated PostgreSQL passed (15.338s); failure retention, concurrent dedup, wallet and usage reconciled |
| `node scripts/check-doc-links.mjs` | Document index and local targets passed |

The first full-test compilation hit the 17GB environment memory limit while other builds were running; the service compiler was killed. Final default/unit suites and race compile at `GOMAXPROCS=2 GOFLAGS=-p=1`; build and Fork checks run sequentially at `GOMAXPROCS=2 GOFLAGS=-p=2`. The final lint run also verifies the corrected test connection cleanup. No further OOM occurred, and neither issue remains a failed gate.

The separate request-ledger task can consume `UnfinishedTurn` and `AfterTurn` without submitting usage again. All handler hooks receive the logical turn number across same-account Proxy retries. Freeze the per-turn context before forwarding; `openAIWSTurnSettlement.context(parent, turn)` provides the stable missing-ID billing key within one selected-account scope, including its same-account retries. Turn number alone is not globally unique across account reselection, and known upstream response IDs still take precedence in RecordUsage. The existing callback claim may return early for duplicates, so any ledger terminal update must have its own idempotency boundary.

Merge/deployment and the required local-browser product acceptance are owned by the parent thread; this branch must remain a draft PR until its CI and the combined-branch review pass.
