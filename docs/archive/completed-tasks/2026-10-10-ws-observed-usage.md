# WS interrupted-turn observed usage

- Branch: `codex/ws-observed-usage-20261010`
- Initial baseline: `2fbe13d90a2381a3bc8e7495a0c8b00abe2cfe3a`
- Reviewed cache patch integrated with a normal fast-forward: `1f37c7a8312f06218036fcbb0335842a6b23e50f`; no shared modified files or conflicts.
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

## Delivery gates

All final local gates passed before commit/push (exit code 0):

| Gate | Result |
| --- | --- |
| `make test` | Backend default suite, golangci-lint (0 issues), frontend lint/typecheck/design governance; frontend 491 files / 3491 tests passed, 1 file / 2 tests skipped |
| `go -C backend test -tags=unit ./...` | Full unit-tagged suite passed |
| `make build` | Backend and frontend production builds passed |
| `./scripts/check-fork-integrity.sh` | All protected Fork checks passed |
| Related service/relay/handler `go test -race` | All three packages passed (6.460s / 7.175s / 1.352s) |
| `TestWSObservedUsagePostgresExactlyOnce` with integration tag | Isolated PostgreSQL passed (15.338s); failure retention, concurrent dedup, wallet and usage reconciled |
| `node scripts/check-doc-links.mjs` | Document index and local targets passed |

The first full-test compilation hit the 17GB environment memory limit while other builds were running; the service compiler was killed. Subsequent gates ran sequentially with `GOMAXPROCS=2 GOFLAGS=-p=2`. The final lint run also verifies the corrected test connection cleanup. Neither issue remains a failed gate.

The separate request-ledger task can consume `UnfinishedTurn` and `AfterTurn` without submitting usage again. Freeze its per-turn context before forwarding; `openAIWSTurnSettlement.context(parent, turn)` provides the stable missing-ID billing key within one handler attempt. Turn number alone is not globally unique, and known upstream response IDs still take precedence in RecordUsage. The existing callback claim may return early for duplicates, so any ledger terminal update must have its own idempotency boundary.

Merge/deployment and the required local-browser product acceptance are owned by the parent thread; this branch must remain a draft PR until its CI and the combined-branch review pass.
