# Account editor opening recovery

Status: the final candidate removes automatic editor-error reloads to preserve other unsaved dialogs. Current combined-build browser and gate results are recorded in [manual-local-gates.json](manual-local-gates.json); older automatic-reload results below are historical only. Exact-head CI is tracked in draft PR #349. Merge, deployment and production acceptance belong to the main thread.

## Scope and source

- Worktree: `/workspace/account-edit-fix`; branch `codex/account-edit-open-fix-20261010`.
- Initial reproduction baseline: `1f37c7a8312f06218036fcbb0335842a6b23e50f`.
- Original implementation base: `3843ff3e931349595b8793b52504b02a177f12c9`. Final normal merge includes actual production `7291ae9a2be4db7d97b8b641d053f7822276dc23` (#346 WebSocket observed usage) and preserves the prior PR head `d0ac716e4952e8918f535b2487f9e49db597f839` ancestry.
- Earlier #343 is already merged. This is a separate minimal repair; no new migration, backend update, editor field rewrite, import/export change, or release change.
- Relevant invariants: FORK-UI-012, FORK-BILLING-010, FORK-DEPLOY-006. TPS, tooltip width, memory fixtures, pricing/surcharges, upgrade source and release protections remain unchanged.
- No merge into `play/main` or deployment is authorized for this delegated task; the main thread coordinates production order.

## Defect and repair

The built application can become stuck after an EditAccountModal JavaScript fetch fails. The account detail request succeeds, but Vue catches the async component loader error internally, so global `window.error` and `unhandledrejection` recovery does not run. `showEdit` stays true; clicking another row or the same row again does not restart the failed component.

The editor async loader closes its failed instance and shows the existing translated error Toast. It never reloads automatically: users first handle unsaved work, then explicitly refresh after asset recovery. Repeated failures remain on the current page. The API endpoint, editor fields, request payload and save logic are unchanged. Editor detail-response ownership is now guarded as described below. A quality-review follow-up adds an unmounted-view guard so a delayed loader failure cannot reload a different page after navigation. Its unit test and production-build browser test first reproduced the unwanted reload (RED).

This is a locally reproduced defect, not a claim that it is the unique cause of the reported production incident. Authenticated production logs were unavailable: public requests from this environment returned HTTP 403 or a browser certificate error. No production credentials were requested, inspected or used.

## Same-page unsaved draft regression and final recovery policy

The previous `d0ac716` editor-local automatic recovery had a second confirmed defect: hold the Edit JavaScript request, open Create, type a draft, then reject the old module request. [Actual RED](same-page-draft-red.txt) records two document navigations, disappearance of Create and its draft, zero account mutations, and unchanged SQL count. [Unit RED](same-page-unit-red.txt) independently fails because automatic recovery was invoked. The abandoned automatic-reload behavior is retained only in historical evidence below.

The final minimal change removes the editor-local recovery-helper call. The failed editor is unmounted and its pending detail intent invalidated, but other dialogs remain intact. Chinese and English Toast text instructs the user to finish unsaved work before refreshing. No new dialog or broad routing change is introduced. The disposed-view guard, latest-edit generation and existing global route recovery remain unchanged.

[Actual GREEN](manual-draft.txt) checks the built combined application: the Create draft survives, only the initial document navigation occurs, no account write occurs and SQL count is unchanged. After explicitly cancelling Create and restoring the asset, a deliberate page reload opens Edit; cancel/reopen still works. [23 targeted tests](same-page-unit-green.txt) include this draft case, the current-generation races and route chunk helper tests. See [persistent-fault replay](manual-persistent.txt) for repeated failed clicks with zero automatic reloads. [Full six-case matrix](manual-matrix.txt), [current real HTTP events](manual-matrix-browser-events.json), [request race](manual-request-race.txt), [unmounted late failure](manual-disposed.txt), and [keyboard](manual-keyboard.txt) also pass on the final combined build. Browser-native failed dynamic imports may remain cached until the instructed manual refresh; a unit retry is not claimed to bypass that browser cache.

## Detail request race found in independent main-thread review

The initial `6575f149f86a0da4c0cb952cc4504d5591b388d0` candidate still inherited an account-detail race from the base. Click A, then B while A's real HTTP detail response is delayed: B opens, then A's response changes the editor to A and discards B's draft. A subsequent real PUT targets A; PostgreSQL and detail GET confirm A was written while B was the last selected row. Cancelling B also permits late A to reopen. This is an observed save-target risk, not a hypothetical concern or a claim that a B draft is silently sent unchanged to A.

[Unit RED](request-race-unit-red.txt) and [actual browser/HTTP/SQL RED](request-race-browser-red.txt) demonstrate the defect. The minimal follow-up keeps a generation counter for edit intent and a single pending edit request. Consecutive clicks on the same account share a GET; selecting A→B→A starts a fresh final A request. Only the current generation on a mounted view may apply results or errors. Modal close (including save completion), chunk failure and view unmount invalidate the intent. Promise identity controls pending cleanup. The shared detail loader for test/statistics actions and all business fields remain unchanged.

[22 passing targeted tests](request-race-unit-green.txt) include independent A1/A2 responses, same-account deduplication and stale errors after cancel/unmount. [New-build browser GREEN](race-browser-race.txt) proves B stays selected, its draft survives, the actual PUT/SQL/GET target is B, A stays unchanged, and cancel prevents late reopening. `request-race-browser.cjs` delays a `route.fetch()` response from the actual local server; it does not fabricate account details or save responses. `EXPECT_RACE_FIXED=1` runs its GREEN assertions; the default records the RED behavior against the old build.

## Acceptance matrix

| Scenario | Actual evidence | Result |
| --- | --- | --- |
| Baseline module failure, network restored, repeat clicks | Real detail GET 200, one blocked built JS asset, repeat GET 200 but zero dialogs | RED: reproduced |
| Same fault with final fix | No automatic reload; prompt, explicit reload after network recovery, API key/OAuth cancel/reopen | PASS |
| Leave account page while module pending | Delayed rejection must not reload or show a stale error on the new page | PASS (unit and rebuilt-browser RED→GREEN) |
| Persistent module failure | Zero automatic reloads across repeat clicks; visible Chinese error; explicit reload after restoration opens; cancel/reopen works | PASS |
| OpenAI API key × complete/null/legacy | UI edit/cancel/reopen; one PUT for double submit; SQL + GET + refresh control comparison | 3/3 PASS |
| OpenAI OAuth × complete/null/legacy | Same real save and reload contract | 3/3 PASS |
| Detail GET failure | Injected 503 hit once, visible error, no dialog/PUT, unchanged SQL hash; next real GET opens | PASS |
| Optional TLS profile failure | Injected 503 hit once; editor still opens and can cancel/reopen | PASS |
| Normal user | Actual HTTP GET and PUT both 403; unchanged account SQL hash | PASS |
| Chinese light/dark desktop | Actual built-app 1600×1000 and 1280×900 screenshots inspected | PASS |
| A→B, late A; then save/cancel | New production build, actual PUT + SQL + GET targets B; B draft survives; cancel stays closed | PASS |
| A1→B→A2 and duplicate A clicks | Independent Promise results preserve A2; consecutive A shares one GET; stale errors ignored | Unit PASS |
| Production incident attribution/deployment/user-local acceptance | Outside this isolated task | Pending main thread |

The six data cases check every submitted credential/extra key against PostgreSQL without printing values, scalar fields against SQL and detail GET, group policy against SQL and GET, preservation of an unknown extra field, and all non-password editor controls after refresh. API-key password inputs intentionally return blank on reread; persisted values are checked privately in memory. No real OAuth exchange or paid upstream request occurs: accounts are inactive, unschedulable, have synthetic random credentials and a loopback-only unavailable base URL. The usage column is hidden during these editor tests.

The null fixture represents a preexisting legacy JSON-null row. The current OpenAI extra-field trigger rejects a direct new JSON-null insert, so fixture setup temporarily uses `session_replication_role=replica` in the disposable database for that seed only, restoring `origin` immediately. The actual UI writes run with all normal triggers enabled. Full database migrations run before all fixtures. The OAuth fixture omits an API-key-only probe flag because the server deliberately filters that inapplicable field; no product behavior was changed to accommodate it.

## Evidence and replay

- [Unit RED](unit-red.txt), [unit GREEN](unit-green.txt): async component loader regression with actual Vue async-component machinery; existing route recovery tests included.
- [Late-failure unit RED](unit-disposed-red.txt), [unit GREEN](unit-disposed-green.txt), [late-failure browser RED](browser-disposed-red.txt).
- [Keyboard check](keyboard.txt): Enter opens, Tab moves within editor, keyboard cancel closes without writing.
- [Browser RED](repro-red.txt), [browser GREEN](repro-green.txt), [persistent-fault result](persistent-chunk-results.json).
- [Six data cases](matrix-data-results.txt), [real save/read HTTP events](matrix-browser-events.json), [runtime errors](matrix-page-errors.json).
- [Failure/permission/theme cases](matrix-tail.txt), [HTTP events](matrix-tail-browser-events.json), [runtime errors](matrix-tail-page-errors.json).
- [Visual review](../../../visual-reviews/2026-10-10-account-edit-open.md).

The first full matrix run completed all six data cases but its tail timed out because its failure-injection glob omitted the GET timezone query parameter. The archived data log intentionally retains that timeout. Injection now matches URL pathname and asserts its hit count. `matrix-tail.cjs` reran only the previously unverified tail against the six saved local rows; its separate passing record closes the gap. No failed test was reclassified as a pass.

The archived `.cjs` and `.py` files are exact task-local harnesses, not production utilities. They require a dedicated disposable setup at their explicit `/workspace/edit-fix-evidence` paths: PostgreSQL container `edit-fix-postgres`, database `sub2api_edit_test` on loopback 55434; Redis container `edit-fix-redis` on loopback 56381; the built Go server on 8082 and production frontend static proxy on 4176. Never point them at an existing database or production service. `common.cjs` generates short-lived local identities in memory from that disposable database; it never prints tokens. Build the target revision into `server-latest`, create the task-local app-data directory, run `serve.py` and `proxy.py`, then run the browser scripts with `PLAYWRIGHT_MODULE` pointing to the installed Playwright module. `repro.cjs` is the expected-failing baseline test; `repro-green.cjs` creates fixtures used by `persistent-chunk.cjs`; `matrix.cjs` is the corrected full replay and `matrix-tail.cjs` is the recorded continuation only.

For disposable container creation, use the exact pinned images and loopback bindings below. The trust setting applies only to this empty local fixture database. Use `docker rm -f -v` on these two named containers after stopping the task-owned server/proxy; never prune unrelated Docker resources.

```bash
docker run -d --name edit-fix-postgres -e POSTGRES_HOST_AUTH_METHOD=trust -e POSTGRES_DB=sub2api_edit_test -p 127.0.0.1:55434:5432 postgres:18.1-alpine3.23
docker run -d --name edit-fix-redis -p 127.0.0.1:56381:6379 redis:8.4-alpine
```

## Review and gates

The final manual-recovery change passed independent specification review after its actual browser GREEN. Independent quality review also PASS: no product must-fix; generation invalidation, disposal guard, actual zero-auto-reload behavior, complete current matrix and credential-safe evidence were checked. Final local gates are recorded below. The combined tree passes `make build` and the full frontend suite: 491 files / 3503 tests; the same two live-HTTP tests are skipped there and were already passed by the separate unchanged check-in contract gate. The first frontend command stopped at design-check because the just-generated real screenshot had not yet been copied into the repository; [original failure](manual-frontend-evidence-pending.txt) is retained. After copying the valid PNG, the remaining gates were rerun. The unchanged backend was already covered by the prior complete gates; production #346 compatibility also passed actual browser HTTP/SQL tests before this narrow UI follow-up. The main thread explicitly requested retaining completed gates without repeating unrelated backend suites. The separate deadline fix is not part of this PR.


Independent specification review (`/root/spec_review`): PASS after confirming the six data cases and completed tail checks; incremental unmounted-view guard also PASS.

Independent quality review (`/root/quality_review`): PASS after repair. The reviewer identified P2: a pending loader failure could reload a new route after the account view unmounted. The added instance-local disposed flag and early `fail()` return close that issue; unit RED→GREEN covers it. Real-browser RED reproduces the issue; rebuilt-browser GREEN confirms the repaired route remains stable. Full gate results are recorded below. The reviewer also checked bounded reloads, per-instance state, actual Vue async wrapper behavior, fault-injection hit counts and credential-safe evidence.

The request-race follow-up received a second independent specification and quality PASS after checking its actual browser PUT/SQL/GET target, preserved draft, cancelled late response, full six-case matrix and all seven new browser/build exit codes. No new code blocker remains. Its full gates and exact next SHA CI are tracked separately from the initial candidate.

Production and user-local acceptance remain pending regardless of local gate results.

### Historical initial candidate local gates

[Structured gate results and tested source hashes](local-gates.json): `make build`, `make test`, `make test-backend-unit`, exact core PostgreSQL/Redis workflow commands, `scripts/test-checkin-contract.sh`, Fork integrity, `pnpm design:verify` and documentation links all exit 0. Frontend default suite: 491 files / 3496 tests passed, with the two live-HTTP tests skipped there and both passed in the separate check-in contract. Core integration: 54 required checks, 203 test/subtest passes, zero skipped. Initial targeted async recovery suite: 16 tests passed; after the request-race follow-up, 22 tests pass. A final full build after the disposal guard also passed.

The earlier automatically recovering frontend passed [late failure after navigation](final-disposed-route.txt), [transient module failure recovery](final-repro-green.txt), and [persistent failure bounds/recovery](final-persistent-chunk.txt); [final HTTP events](final-green-browser-events.json). The original six save/readback cases and subsequent disposal-guard tests are retained as historical evidence. The request-race follow-up passes the [complete matrix](race-browser-matrix.txt), [transient chunk recovery](race-browser-chunk.txt), [persistent error bounds](race-browser-persistent.txt), [late module failure after navigation](race-browser-disposed.txt) and [keyboard check](race-browser-keyboard.txt) on its new production build. [Real HTTP events](race-matrix-browser-events.json) and [empty runtime-error record](race-matrix-page-errors.json) are separate from the earlier historical runs. That revision has its own gate/source record before push.

### Follow-up gate failure investigation

The first follow-up `make test` failed in the unchanged backend `TestTokenRefreshService_LateSuccessPastAttemptDeadlineIsRejected` (timeout expected, nil returned). The [failure excerpt](preexisting-timeout-failure.txt) and [independent assessment](preexisting-timeout-risk.md) preserve this separately from the account-editor repair. There is a preexisting deadline/`Err()` scheduling window; this test-double failure is not proof of production or actual PostgreSQL impact. A subsequent tagged unit run and 100 isolated executions of the exact test passed. A passing repeat does not fix or dismiss that risk. The [complete serial rerun](race-test-serial-summary.txt) passed with 491 frontend files / 3502 tests; the two skipped live HTTP tests passed separately. [Follow-up gates and tested source hashes](race-local-gates.json) record every exit code, including the first failed run. [Isolated result](timeout-isolated-summary.txt) records all 100 passes. All other follow-up gates passed, including full build, tagged unit, core54/203, checkin HTTP, Fork, design and docs. The main thread subsequently authorized a separate backend correction and deterministic regression; the editor PR does not include that fix.

No production merge, deployment, production credential use, or user-local acceptance is claimed. Normal merging of production ancestry into this review branch is authorized and does not publish to production. The commit SHA and final CI runs are provided in the draft PR delivery record, avoiding a self-referential commit hash in this file.

Account exports still omit group policies and must not be treated as policy backups.
