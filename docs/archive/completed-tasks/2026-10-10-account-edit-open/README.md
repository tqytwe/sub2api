# Account editor opening recovery

Status: local acceptance, independent reviews and full local gates passed. Draft PR CI is recorded on the PR; merge, deployment and production acceptance belong to the main thread.

## Scope and source

- Worktree: `/workspace/account-edit-fix`; branch `codex/account-edit-open-fix-20261010`.
- Initial reproduction baseline: `1f37c7a8312f06218036fcbb0335842a6b23e50f`.
- Final implementation base: `3843ff3e931349595b8793b52504b02a177f12c9`, fetched from actual `origin/play/main`; includes #347 read-only quota and account-refresh changes.
- Earlier #343 is already merged. This is a separate minimal repair; no new migration, backend update, editor field rewrite, import/export change, or release change.
- Relevant invariants: FORK-UI-012, FORK-BILLING-010, FORK-DEPLOY-006. TPS, tooltip width, memory fixtures, pricing/surcharges, upgrade source and release protections remain unchanged.
- No merge or deployment is authorized for this delegated task; the main thread coordinates production order.

## Defect and repair

The built application can become stuck after an EditAccountModal JavaScript fetch fails. The account detail request succeeds, but Vue catches the async component loader error internally, so global `window.error` and `unhandledrejection` recovery does not run. `showEdit` stays true; clicking another row or the same row again does not restart the failed component.

The editor async loader now closes its failed instance, shows the existing translated error Toast, and invokes the existing one-reload-per-route/session recovery. A persistent fault shows an error without looping reloads. The detail-fetch and editor submission logic are unchanged. A quality-review follow-up adds an unmounted-view guard so a delayed loader failure cannot reload a different page after navigation. Its unit test and production-build browser test first reproduced the unwanted reload (RED).

This is a locally reproduced defect, not a claim that it is the unique cause of the reported production incident. Authenticated production logs were unavailable: public requests from this environment returned HTTP 403 or a browser certificate error. No production credentials were requested, inspected or used.

## Acceptance matrix

| Scenario | Actual evidence | Result |
| --- | --- | --- |
| Baseline module failure, network restored, repeat clicks | Real detail GET 200, one blocked built JS asset, repeat GET 200 but zero dialogs | RED: reproduced |
| Same fault with fix | One automatic reload; API key and OAuth editors open after network recovery | PASS |
| Leave account page while module pending | Delayed rejection must not reload or show a stale error on the new page | PASS (unit and rebuilt-browser RED→GREEN) |
| Persistent module failure | Exactly one automatic reload; visible Chinese error; manual reload after restoration opens; cancel/reopen works | PASS |
| OpenAI API key × complete/null/legacy | UI edit/cancel/reopen; one PUT for double submit; SQL + GET + refresh control comparison | 3/3 PASS |
| OpenAI OAuth × complete/null/legacy | Same real save and reload contract | 3/3 PASS |
| Detail GET failure | Injected 503 hit once, visible error, no dialog/PUT, unchanged SQL hash; next real GET opens | PASS |
| Optional TLS profile failure | Injected 503 hit once; editor still opens and can cancel/reopen | PASS |
| Normal user | Actual HTTP GET and PUT both 403; unchanged account SQL hash | PASS |
| Chinese light/dark desktop | Actual built-app 1600×1000 and 1280×900 screenshots inspected | PASS |
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

Independent specification review (`/root/spec_review`): PASS after confirming the six data cases and completed tail checks; incremental unmounted-view guard also PASS.

Independent quality review (`/root/quality_review`): PASS after repair. The reviewer identified P2: a pending loader failure could reload a new route after the account view unmounted. The added instance-local disposed flag and early `fail()` return close that issue; unit RED→GREEN covers it. Real-browser RED reproduces the issue; rebuilt-browser GREEN confirms the repaired route remains stable. Full gate results are recorded below. The reviewer also checked bounded reloads, per-instance state, actual Vue async wrapper behavior, fault-injection hit counts and credential-safe evidence.

Production and user-local acceptance remain pending regardless of local gate results.

### Final local gates

[Structured gate results and tested source hashes](local-gates.json): `make build`, `make test`, `make test-backend-unit`, exact core PostgreSQL/Redis workflow commands, `scripts/test-checkin-contract.sh`, Fork integrity, `pnpm design:verify` and documentation links all exit 0. Frontend default suite: 491 files / 3496 tests passed, with the two live-HTTP tests skipped there and both passed in the separate check-in contract. Core integration: 54 required checks, 203 test/subtest passes, zero skipped. Targeted async recovery suite: 16 tests passed. A final full build after the disposal guard also passed.

The final built frontend passes [late failure after navigation](final-disposed-route.txt), [transient module failure recovery](final-repro-green.txt), and [persistent failure bounds/recovery](final-persistent-chunk.txt); [final HTTP events](final-green-browser-events.json). The six save/readback cases were exercised before the disposal guard; their editor fields, API requests and backend behavior are unchanged by that guard. The initial and final transient-fault runs both open API-key and OAuth editors on the latest base.

No production merge, deployment, production credential use, or user-local acceptance is claimed. The commit SHA and final CI runs are provided in the draft PR delivery record, avoiding a self-referential commit hash in this file.

Account exports still omit group policies and must not be treated as policy backups.
