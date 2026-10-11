# Batch image capture → usage → request ledger

Initial base: `9b367c378b04db987a36175af0b8a3e02e22da24` (#354).
Combined base: `c939d570aa1f01cadab465c38c36e0295703a9b7` (`play/main`, #355),
fast-forwarded with normal `git merge --ff-only` before the first batch commit;
no rebase or force push. The batch work was saved/restored without conflicts.
Branch: `codex/batch-image-ledger-20261010`.

## Scope and contracts

- Keep the existing batch capture transaction, idempotency keys, command
  normalization and fingerprint algorithm. Only capture success followed by a
  read-only check of job ownership and all retained hot/archive fingerprints
  certifies the new usage row's fingerprint, settled flag and billed cost. A
  duplicate's nil error alone is insufficient. Do not call ordinary usage billing
  a second time.
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

The optional proof reader uses a single statement snapshot after capture and a
one-second deadline detached from client cancellation. An unavailable reader,
read error or contradictory proof does not change capture success, real
`actual_cost`/`image_count` persistence or job completion. It leaves the new
fingerprint empty and `billing_settled=false`; the default unverified billed cost
is not a free-usage claim. A completed retry does not later invent that missing
fingerprint, so this remains an observable audit gap, not automatic repair.
Existing wallet capture proof remains independently visible; this degraded usage
row does not establish an exact usage/billing link. Any future recovery requires
a separately designed exact-evidence flow; a successful usage INSERT alone is
insufficient, and no repair or historical backfill is implemented here.

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

## Initial combined validation (15eb0f57)

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

## Hosted CI follow-up

The draft PR #356 head `15eb0f57f48dfc2025240cdfbb4437e3bdfcb221`
had seven successful checks and one failed ledger-contract job:
[run 38097649870 / job 114346796783](https://github.com/tqytwe/sub2api/actions/runs/38097649870/job/114346796783).
GitHub tested synthetic merge `5d5fe553db50bf6f3bf8f169937fcb768ab34fe6`
with parents that head and `c939d570aa1f01cadab465c38c36e0295703a9b7`.
The sole failing test was
`TestRequestLedgerRealtimeLateTerminalCannotConsumeNextTurn`: an immediate
`upstream_response_id` scan saw SQL NULL before the asynchronous evidence writer
finished. Terminal state is synchronous; identifier evidence is queued. The test
now waits at most five seconds for both exact turn IDs, preserving the pending
turn and execution assertions. Missing or misattributed evidence still fails.
There is no COALESCE, skip or change to runtime WS logic. Ten isolated repeats
passed. The failed hosted job skipped its required-name verification step, so it
does not count as a complete 45/45 hosted contract pass.

Independent review also exposed a separate batch recovery defect: a matching hot
capture proof plus a contradictory archive proof could create a new usage row
marked settled. The real PostgreSQL RED first commits capture, rejects the usage
INSERT, adds the conflicting archived fingerprint, then retries. It observed
`settled=true billed=6 actual=6 images=3`; the balance showed only one capture.
The new optional reader now checks all retained proof and job ownership before
annotating new usage. The GREEN preserves the same real image usage and single
capture while leaving settlement metadata unverified. A separate real PostgreSQL
read-error injection verifies that failed evidence reads still allow usage and
job completion. The original capture/dedup transaction file remains unchanged.

The CI required-name list now explicitly includes the late-terminal test and
both new batch regression cases. Follow-up verification reruns affected backend,
full unit, protocol, race, embedded build and Fork gates serially. The frontend
source and built assets are unchanged from the initial combined validation;
its full frontend and actual-resource browser results are reused with that
source-equivalence boundary. Follow-up results and log hashes are recorded
separately from the original evidence.

### Follow-up gate results

All eight follow-up gates exited 0. Commands, durations, exact required-test
names, source/resource digests and failure-log hashes are recorded in
[ci-followup-validation.json](./ci-followup-validation.json). The protocol run
executed all 48 required names, including the formerly intermittent Realtime
test. The race run passed all seven batch HTTP cases plus that Realtime case.
Independent specification and subsequent code review passed this follow-up.

| Gate | Seconds | Scope |
| --- | ---: | --- |
| `target-unit` | 84.45 | Batch settlement unit regressions |
| `proof-http` | 35.13 | All 7 batch HTTP/PG cases |
| `backend` | 600.50 | Full untagged backend tests and lint |
| `unit` | 403.88 | Full unit-tagged backend suite |
| `protocol` | 377.03 | 445 tests/subtests; 48 required names |
| `race` | 191.65 | 15 tests/subtests; 7 batch HTTP + late Realtime |
| `embed-build` | 44.48 | New backend binary with unchanged verified frontend assets |
| `fork` | 139.80 | Full static and protected behavior checks |

Frontend 496 files / 3,562 passing tests and the built-resource browser matrix
are reused only for unchanged frontend source and byte-identical assets. The new
backend embedded build contains that same verified index. The original core
migration/repository checks remain recorded separately; no old gate is relabeled
as a new run. Documentation links, handoff structure and eight handoff unit tests,
frontend design governance and diff formatting also pass.

## Delivery boundary
Draft [PR #356](https://github.com/tqytwe/sub2api/pull/356) targets `play/main`.
The initial technical commit was `57b11c4bece17aae9e14d338f54b24d0789fdc2c`;
`15eb0f57f48dfc2025240cdfbb4437e3bdfcb221` recorded only its handoff metadata.
The current correction's source digest is in the follow-up validation artifact;
its technical SHA is recorded by a subsequent metadata-only commit in the same
handoff entry. The exact final remote head and its new hosted CI are verified
separately after pushing the correction. The old failed hosted run is retained.
No merge, deployment, production database access or local-user acceptance was
performed. Existing public zero-price hold behavior, persistent evidence-read
gaps and historical gpt22 loss remain outside this patch's resolved scope.

## Mainline combination #357

The coordinator merged #357 into `play/main` as
`1bfc7e2edb788ca11a23ff936f99db904c4b4199`. This branch then uses a normal
non-rebase merge with first parent `0a2c51e130cf108ab51016643f0eba7bfa516646`
and second parent that exact mainline SHA. The merge was staged with
`--no-ff --no-commit`, had no conflicts, and was validated before committing.
No force push, production merge or deployment is performed by this task.

Before adding the new delivery metadata, all 51 batch-only files and all 104
#357-only files matched their respective parents byte-for-byte. The only shared
files were `docs/FORK_CUSTOMIZATIONS.md`, `docs/README.md` and the handoff JSON;
both feature records remain. The handoff now records #357's known merge without
claiming that its production deployment or local-user acceptance was rechecked.
The reviewed batch technical fix `fbc103a5a0b7fcc9b103536e47eb347cac0895d3`,
restricted billing repository and migration 277 are unchanged. Migration 277's
trimmed checksum remains
`239308e9fa45b74ae00f475597f5795b8308dd09b2d88d32dee91915d73ea6b0`.

#357's production backend delta is the blindbox DTO projection; its frontend
changes are the quiz/blindbox states and recovery contracts. The combination
adds no authored runtime changes. Independent read-only specification review,
then code review, found no merge blocker or required extra cross-feature contract.
The coordinator also reported two successful root reviews of the previous batch
head and a successful 277 publication precheck; those are not relabeled as local
executions or new-head CI.

The combination gates below all passed. Full ledger protocol 48/48 evidence from
the unchanged batch runtime/migration inputs remains in the prior artifact; the
new targeted race and actual built-resource checks are separately recorded.
The new exact-head hosted CI must complete before coordinating a production
merge. Results, log hashes, source/resource digests and parent preservation are
in [merge-357-validation.json](./merge-357-validation.json).

| Gate | Seconds |
| --- | ---: |
| `full-test` | 450.53 |
| `unit` | 291.18 |
| `welfare-contract` | 51.58 |
| `checkin-contract` | 13.13 |
| `batch-realtime-race` | 52.49 |
| `full-build` | 90.16 |
| `embed-build` | 10.27 |
| `fork` | 139.04 |
| `browser` | 17.73 |

Full frontend summary: Test Files  499 passed | 3 skipped (502); Tests  3601 passed | 8 skipped (3609); Duration  135.86s (transform 25.79s, setup 26.23s, collect 62.37s, tests 69.45s, environment 156.27s, prepare 26.07s).

Welfare and check-in PostgreSQL/HTTP/Vue contracts run separately from the default
frontend suite's opt-in skips. Batch race covers all seven HTTP cases and the
late-terminal Realtime case, 15 tests/subtests total. Full build and explicit
embed build use the merged frontend. Both batch roles and four widths run again
against these new assets with real disposable HTTP/PG; three images and a single
capture of six agree, and no page errors occur. New browser metadata is in
[browser-evidence-357.json](./browser-evidence-357.json); previous screenshots and
evidence are preserved. New screenshot files remain in the task workspace with
hashes in the validation artifact. The welfare visual matrix is reused only for
its byte-identical reviewed UI source, not claimed as a new welfare browser run.

The audit-gap boundary remains: proof read errors preserve real usage but do not
certify settlement, and completed retries do not restore a missing fingerprint.
No historical repair, upstream generation or production acceptance is claimed.
