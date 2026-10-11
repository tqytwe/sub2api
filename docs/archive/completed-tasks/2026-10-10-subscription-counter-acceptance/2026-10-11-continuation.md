# Counter acceptance after merging the current baseline

Recorded 2026-10-11 UTC, before the final draft push. **All local runtime gates
and the sequential independent specification/quality reviews passed.** Remote CI
and main-thread review belong to the final PR head and are recorded in
[PR #352](https://github.com/tqytwe/sub2api/pull/352), not inferred from this file.
No production access, paid generation, production merge or deployment occurred.

## Source and publication boundary

- Original fix: `5864a5a23a0d55294fab1e1b0a2ed497106499ff`.
- Normally merged baseline: `23d06337cbc315cb92d3fc18e94902fe21e7342d`, verified
  against the remote again after the full build. There were no merge conflicts.
- Runtime/review index tree before these evidence additions:
  `a189161edb2c2df4c6c07a379c93356e419145f6`.
- Tested backend tree: `e4523652b1ee94eb948329d4ed26b38a6729fba9`.
- Tested frontend tree: `6b5a31fa5b5fb3fd120a5d34a72cd9082d12991d`.
- Relative to the merged baseline, production changes remain only the original
  two areas of `usage_billing_repo.go` (37 additions / 9 deletions). That complete
  file, all three counter tests, the wrapper and workflow are byte-identical to
  the earlier fix's tested-source manifest. No P0 ledger, Ops, gateway writer,
  operating rule, schema or frontend changes were added by this task.
- The user's explicit authorization cleared the original upload rejection.
  Retrying the same GitHub tree action once returned
  `3328384067bf682373dffc3d8124c77c6356cdc1`, identical to the original fix tree;
  a subsequent GitHub GET read back that exact tree. This object upload did not
  itself update the draft branch.
- The published RED checkpoint `a90886aa69814612030a8b3b6096768f6241faa7` has
  exactly the same tree as local RED ancestor `c267052ff1cae08b36812d937ef8db8f89bee6b4`.
  Preserve both histories when updating the existing branch; do not use the RED
  checkpoint as a fixed release or force-push away its ancestry.

## Fresh runtime results

| Gate | Result |
| --- | --- |
| Full-migration HTTP counter matrix, `-race` | 42/42 leaf scenarios passed, no race diagnostic; PostgreSQL 18.1, 404 migrations, last filename `277_batch_image_ledger_reconcile_index_notx.sql` |
| `make test` | Passed: full backend tests and lint (0 issues), frontend lint/design/type checks and 499 files / 3601 tests passed |
| `make test-backend-unit` | Full backend suite with the `unit` tag passed |
| Existing ledger/protocol/batch-image integration workflow command | All six test packages passed; all 48 workflow-required cases actually passed |
| `make build` | Backend and frontend production builds passed |
| `./scripts/check-fork-integrity.sh` | Passed |
| Integration/billing-acceptance diff lint against the new base | 0 issues |
| `./scripts/test-checkin-contract.sh` | Passed, including the live Vue/HTTP/PostgreSQL case |
| `./scripts/test-welfare-contract.sh` | Passed, including quiz/blindbox Vue/HTTP/PostgreSQL reward and recovery variants |
| Documentation links and diff whitespace | Passed; checked again after final evidence edits |

The default frontend suite intentionally skipped three PG-contract files (eight
tests); the two explicit contract scripts above subsequently executed those
entry points against disposable PostgreSQL HTTP fixtures. The ledger run's
browser-fixture and child-process entry-point skips are recorded in the JSON;
all 48 required acceptance cases, including killed-process recovery, passed.
No skip is reported as an executed test.

Reproduce the primary regression matrix with:

```bash
./scripts/test-subscription-counters.sh '^TestSubscriptionCounterHTTP$' -race
```

The wrapper uses only its disposable loopback database and exact synthetic
upstream origin. Local HTTP/API tests are not production or browser acceptance.

## Evidence and review

- [Fresh complete counter matrix](./2026-10-11-counter-matrix.txt).
- [Commands, exit codes, timestamps and SHA-256 evidence](./2026-10-11-evidence.json).
- [Validation excerpts](./2026-10-11-validation.txt).
- [Sequential independent specification and quality reviews](./2026-10-11-review.md).

The original 402-migration RED/GREEN records remain historical and unmodified.
The reviewers examined the source and existing execution evidence independently;
they did not rerun the tests themselves. Neither review is a production sign-off.

Both reviews retain the preexisting same-user/group entitlement-ID binding gap
for separate P0 coordination, plus administrator-concurrency and missing-field
coverage suggestions. The two authorized repairs do not claim to solve arbitrary
entitlement substitution or the complete production auth/scheduler/worker path.
The main thread owns final review, production merge/deployment and user-machine
browser acceptance.
