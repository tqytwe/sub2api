# Batch image capture → usage → request ledger

Initial base: `9b367c378b04db987a36175af0b8a3e02e22da24` (#354).
Combined base: `c939d570aa1f01cadab465c38c36e0295703a9b7` (`play/main`, #355),
fast-forwarded with normal `git merge --ff-only` before the first batch commit;
no rebase or force push. The batch work was saved/restored without conflicts.
Branch: `codex/batch-image-ledger-20261010`.

## Scope and contracts

- Keep the existing batch capture transaction, idempotency keys, command
  normalization and fingerprint algorithm. A successful capture (including an
  exact duplicate) is the proof for the new usage row's fingerprint, settled flag
  and billed cost. Do not call ordinary usage billing a second time.
- Keep output-image count × frozen unit price, group/account multipliers and
  existing capture/release amounts unchanged. Tokens are not inferred from images.
- Preserve the existing retry after capture/usage-write failure. A completed job
  without a retained exact usage fingerprint must not gain a guessed fingerprint
  from its status, batch ID or price. No historical backfill or upstream replay.
- Read batch usage only through the admission/worker task reference, its exact
  user/key ownership, the capture request ID and matching committed dedup/archive
  fingerprint. Both ledger detail and usage drill use the same evidence rule.
  Cancellation/release or missing usage never creates a usage row or a zero-cost
  inference. Wallet proof remains independently visible.
- User and admin endpoints retain existing authorization and no-store responses.
  Add only image count, billing mode and the precisely linked batch ID to the
  existing usage drill contract. Do not expose prompts, keys, provider artifacts
  or other tenants' rows.
- Task, dedup and usage exact-key indexes are reused. Additive concurrent index
  migration 277 bounds periodic reconciliation for pending requests and the
  existing `/v1/images/batches` / `async/image_batch` routes. The latter routes
  require exact task ownership at proof verification; route matching is never
  settlement proof. Select at most 1000 indexed candidates before task checks;
  rotate no-task rejected requests without changing their execution/usage/settlement.
  New batch admission aliases must extend the candidate/index contract together.
  No edits to historical migrations or checksums. In particular,
  `backend/internal/repository/usage_billing_repo.go` is excluded by the current
  authorization boundary; none of #352's rejected changes is imported.

## Validation plan

Start with a real loopback HTTP/disposable PostgreSQL RED: reserve 10, capture 6
for two successful items producing three images, then attempt usage drilling.
Exercise partial success, zero output, usage write failure and retry, duplicate
terminal events, concurrent settlement, cancellation, disconnected client,
archived and conflicting proof, missing usage, retained legacy rows, user/key and
request isolation. No real provider calls or production access.

The submission fixture binds a real ledger task and persists a job/hold directly;
it does not call the public provider-submission flow. Items, cancellation and
ledger drill use the actual handlers, and capture/usage use real repositories on
disposable PostgreSQL. Authentication identities are synthetic fixture middleware;
this is not production login validation. Recovery runs the existing worker pipeline
without a configured provider.

Frontend evidence uses built assets and synthetic authenticated identities only;
it is development evidence, not the user's final production browser acceptance.
Run required broad checks serially, then independent specification and code
reviews (and fixes/review as necessary), commit, push and create a new draft PR.
Record accurate results and remaining blockers here before delivery.

## Evidence semantics refined during specification review

`BillingReference.batch_id` marks shared batch settlement. It does not claim
that each admission, retry or worker applied a charge (`applied` is false for
this shared scope); the UI says “该批次已结算”. `LinkedUsage.image_count` is nullable
for compatibility and an absent count remains unknown. Image-mode details show
image count, not invented input/output token values; ordinary token-mode details
keep their existing fields.

Reconciliation changes only settlement state on existing rows: hold evidence
means pending, committed capture/release evidence means settled. A verified batch
capture usage can also prove settlement without a wallet row. Execution and usage
states are not overwritten, and an unverified normal billing link still blocks
settlement. The read paths share the exact capture-proof predicate.

The production Batch worker creates a background context and calls capture outside
any caller-owned financial transaction. Read-only review found the repository's
private transaction context only on the separate Image Studio path. This patch
does not generalize the capture-return contract to future transactional callers.

The zero-capture/no-wallet test explicitly seeds a valid zero hold through the
real repository before capturing. It validates retained proof/read consistency,
not public free-price submission: the existing public hold helper skips zero
holds, so that separate path may lack the hold claim required by capture. It is
not changed here and no zero-cost proof is fabricated when a claim is absent.

## Review and RED/GREEN evidence

Independent specification review passed after fixing shared-batch wording,
nullable image count, zero-capture proof consistency and candidate scan bounds.
The scan test uses 20,000 unrelated rows plus 20,000 rejected same-route rows.
The original query selected only one row after filtering the population; the
fixed query selects the indexed first 1,000. A real reconciler test rotates 2,000
no-task requests across two calls while keeping identity, execution, usage and
settlement unchanged. The measured disposable plan used the new index (1.349 ms;
this is not a production benchmark).

Independent code review identified interrupted concurrent-index recovery and
passed its final re-review after the real RED/GREEN and recovery registration. The
new migration is now registered with the existing invalid-index recovery helper.
The real PostgreSQL RED cancels the actual concurrent build after its invalid
catalog entry appears: the old runner incorrectly records success with that
index still invalid. Recovery verifies a new valid index, the migration checksum,
healthy-index OID preservation and idempotent replay. Only this new index is
eligible for that recovery registration; no historical SQL was edited.

Other meaningful REDs preceded implementation: a committed capture with no ledger
usage drill, admission settlement remaining not_required, and retained zero
capture proof without a wallet row. Tests then cover the repaired contracts.
Fixture/debug failures (terminal fixture missing ended_at, PostgreSQL JSON row
counts encoded as decimals, English button text, visual-manifest headings) were
corrected separately and are not represented as product regressions or passing
coverage. The final gates and review outcomes are recorded below.

Migration 277 checksum under the runner's trimmed-content SHA-256 rule:
`239308e9fa45b74ae00f475597f5795b8308dd09b2d88d32dee91915d73ea6b0`.

## Technical validation

Initial frontend: 493 test files / 3,532 tests passed, with one existing skipped
file / four skipped tests; lint, design governance and typecheck passed. Actual
built-resource browser checks cover both roles, four widths, Chinese/English,
light/dark, keyboard, error retry and loading. CSS 200% zoom screenshots are
observational emulation only, not native-zoom usability acceptance.

The full initial `make test` passed before the four-line migration-runner recovery
registration and before integrating #355. Final backend `go test ./...` and golangci-lint were rerun on the recovery
revision and passed (0 issues). Full tagged unit, integration, race and build
results follow below; the combined frontend is checked again after #355.


## Combination and environment recovery

The #355 baseline changes welfare frontend pages/locales/tests plus an opt-in
check-in integration fixture and documents. No production backend or unit-tagged
source changes overlap this batch; the passing final backend/lint and full unit
results are reused for those identical compilation inputs. Combined frontend
checks, protocol integration, race, full build and browser assets are revalidated.

The first full protocol run hit PostgreSQL `No space left on device` while
creating a fixture database file; later fixture startup failures were retained.
The remaining service test process was stopped and the serial queue paused.
Only after no Go process was active, old current-worktree archives and that
interrupted command's temporary build directory were removed. Shared dependency
caches and unrelated worktree files were retained. The failed/terminated run is
not a pass; recovery smoke and the full protocol retry are separately recorded.

The recovery smoke passed (exit 0, 39.65 seconds), but the second protocol run
again exhausted disk. Docker inspection then identified the dominant cause:
352 anonymous, unreferenced PostgreSQL fixture volumes created during this
environment session occupied about 20 GB. With the queue paused and its test
processes stopped, those exact inventoried volumes were removed; no containers
remained, and available space returned to about 20 GB. No Go cache was modified
during this second recovery. Both failed/terminated runs are retained separately;
only the subsequent completed gates count as passing validation.

The first final Fork command also failed before protected tests could run: the
gate orchestrator omitted the fixed toolchain wrapper and resolved the system
`go`/pnpm instead (`Go: Unknown option: test`, pnpm store ENOENT). Its exit 1 and
log are retained separately. The corrected command uses the same Go/pnpm
environment as the passing test/build gates; this was an invocation correction,
not a product change or a reason to rerun already-passing full suites.

## Final combined validation

Recorded on 2026-10-11 UTC. All gates below exited 0; commands, durations, raw-log
SHA-256 summaries, failed-run history, source digest and built-resource digest
are in [validation.json](./validation.json). The final source snapshot covers
5,360 backend/frontend files, digest
`614d46d4ddfc6e9d2e40456d79285b107c8b0c04badfd54cb196f583b27f491e`.

| Gate | Seconds | Scope |
| --- | ---: | --- |
| `final-backend` | 278.10 | Full untagged backend tests and lint (0 issues) |
| `unit` | 518.04 | Full unit-tagged backend suite |
| `recovery-smoke` | 39.65 | Previously blocked service fixture |
| `protocol` | 337.75 | 443 tests/subtests; all 45 required CI names |
| `core-contracts` | 2.70 | 2 migration contracts |
| `core-repositories` | 29.43 | 201 tests/subtests; contracts + repository cover all 54 required names |
| `batch-repositories` | 8.42 | 24 tests/subtests |
| `fund` | 23.16 | Real PostgreSQL fund migration |
| `race` | 163.44 | 12 tests/subtests; all 5 batch HTTP cases |
| `combination-frontend` | 193.33 | 496 files / 3,562 tests; lint/design/typecheck |
| `build` | 118.66 | Complete backend and frontend build |
| `fork` | 252.43 | All static and protected behavior checks |
| `embed-refresh` | 10.83 | Explicit -tags=embed build after frontend assets |
| `browser-combination` | 16.78 | Real built assets, two roles and four widths |

The full frontend intentionally skips the five opt-in `CheckInView.http` fixture
tests (one file). Protocol skips only the child-process helper's standalone
entry and opt-in browser fixture; its required parent/HTTP cases all pass. The
batch browser fixture is run separately and passes. No required test is missing.

The combined browser evidence replaces the earlier after-images, while the
original before/prototype images remain unchanged. Both roles read the same exact
request/batch/usage, item image sum 3 and billed amount 6. Responsive, theme,
language, keyboard, error retry, loading and nullable-count checks pass with no
page errors. The final 360px user and English 1280px admin images were inspected.
The built index payload is present in the explicitly embedded binary.

Independent specification, code-quality and #355 combination reviews passed;
new Fork registry wording also passed sequential specification/code review.
The 85 nonshared #355 files are byte-identical to the combined base; the three
shared documents retain upstream content and add this task's records.

## Delivery boundary

The technical commit and a metadata-only follow-up are recorded in the same
handoff entry. A new draft PR targets `play/main`; its exact head and GitHub CI
are verified separately after creation. PR number is unknown before creation,
so the snapshot keeps that field null until the coordinating thread records it.
No merge, deployment, production database access or local-user acceptance was
performed. Existing public zero-price hold behavior and historical gpt22 loss
remain outside this patch's resolved scope.
