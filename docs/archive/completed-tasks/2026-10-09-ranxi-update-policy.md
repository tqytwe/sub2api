# ranxi discovery / tqytwe publication safety review

- Own change scope: software update source and publication safety only. No independent TPS, account routing, billing, migration, mobile app or Canvas changes. The branch stacks the authorized dependency #338 exactly as described below.
- Isolated worktree: `/workspace/sub2api-ranxi`; branch: `codex/ranxi-release-source`.
- Fork baseline: `7508cb0cd8a9c38a33805795922ec13662d7753e` (`play/main`, includes #337).
- Live source verification on 2026-10-09: `ranxi2001/sub2api` official release `v2.10.3`, draft=false, prerelease=false; annotated tag `95479ee5222e25765b1ab55baa513714db3ca184`, peeled commit `fd1b5ee4eeb20961fbb783fa6f136a1704271e90`.
- Dependency: PR #338 at `0462fe370cf862453c6261218a55ff61e0d3d966` is stacked unchanged with explicit main-thread authorization. Its source lock, release verifier/fixtures, merge ledger, migration inventory, sync playbook, Fork registry and PR template are reused. Duplicate experimental pin files were removed before implementation. This update-policy change does not claim full upstream integration.

## 1. Update service and cache review

Specification review: all mutation entry points (update, selected rollback, legacy backup rollback) must reject before network/filesystem I/O when no verified fork artifacts are approved. Upstream discovery must accept only stable ranxi metadata and never expose installable upstream assets. Compare the ranxi release to the reviewed upstream pin, not the independently numbered fork build.

Quality review: removed the now-unreachable in-place replacement/extraction path, including unchecked local `.backup` restoration. Preserved the release client API and historical Go module identity. Redis key and payload both include the source/policy identity; incompatible, expired, future-dated or foreign-link cache entries fail closed. Warning fallback remains available only for compatible cache data.

Evidence: baseline tests reproduced acceptance of old-source cache and unofficial release metadata. The revised tests cover source/release builds, upstream newer/equal versions, API JSON policy fields, wrong source/draft/prerelease, compatible cache reuse, incompatible cache fallback and real HTTP handlers returning 409. Redis test proves the old `update:latest` key is not consumed.

## 2. VersionBadge review

Specification review: ranxi release discovery, verified fork installation guidance and source rollback guidance are separate. Both source and CI builds show the guidance; no upstream image/installer command or install/restart action is available. No newer release does not imply that an unverified fork binary is installable.

Quality review: existing badge, overlay, typography and Icon are reused. Foreign cached links cannot escape the stable ranxi release namespace. Refresh preserves loading/disabled semantics, errors are visible, and Escape returns focus. Runtime core, misc and compatibility locale trees contain the same new copy. Tests assert no update/rollback/restart API calls and non-admin behavior.

Evidence: [visual record](../../visual-reviews/2026-10-09-ranxi-update-policy.md) includes real isolated component baseline/prototype/final captures; Chinese/English, light/dark, 360/768/1280/1920 pixels. These are development fixtures, not production acceptance.

## 3. Publication and documentation review

Specification review: no `v*` push trigger; only explicit dispatch using the fork's reviewed `play/main` workflow. True publication requires an existing tag on current `origin/play/main` and a matching VERSION already reviewed in source. No workflow step writes/pushes VERSION or creates a tag. Owner-derived registries and existing archive/commit/hash validation remain.

Quality review: revalidate the branch and remote fork tag commit (including annotated tags) before registry publication; reject missing or moved remote tags, do not persist checkout credentials, and do not supply a publishing token during dry run. Direct image-script invocation defaults to dry run and rejects invalid mode values. Per-target and aggregate provenance binds repository, source SHA, workflow SHA/run ID, archive hashes, and the exact source-lock digest/snapshot. Publication also requires the runtime review pin to match that lock. The existing PR CI now runs both source and release policy tests offline. All publishing/notification side effects retain dry-run guards. Historical tag workflow definitions cannot be retroactively protected by this file: importing/pushing upstream tags remains forbidden and repository settings are outside this task.

Evidence: release-tool tests first failed for automatic tag triggering, unreviewed source, VERSION mutation and implicit image publication, then passed with the guards. The actual local dry-run plan preserved VERSION bytes, mtime and complete Git status. Publication command tests use a fake Docker executable; no registry push or GitHub release was performed.

Documentation review: [fork source guide](../../../deploy/FORK_SOURCE_BUILD.md) is linked from root/deployment entry points. Original Wei-Shaw scripts and weishaw image examples are explicitly identified as original-distribution references; no nonexistent fork image is substituted. Docker OCI source is tqytwe; module/import/generated names, authors/LICENSE, model-price-repo, supplier version sync, legal confirmations and original CLA remain unchanged.

## Validation and handoff

- Targeted backend service/handler/cache tests: passed.
- Frontend targeted locale/runtime/store/component suite: 84 tests passed; final badge rerun passed.
- Release helper suite: 21 tests passed; source verifier suite: 3 tests passed; shell syntax, Docker resource and scoped GitHub-token installer tests passed.
- Design governance, frontend lint/typecheck and local documentation links: passed.
- Aggregate `make test`: passed before stacking and again on the combined #338 branch (backend default tests, Go lint with 0 issues, frontend lint/typecheck and all 489 frontend files / 3461 tests).
- Additional backend unit suite, `make build`, and Fork integrity: passed on the combined #338 branch. Remote CI results are recorded in the PR after branch publication.
- Source verifier: live stable-release verification passed; two offline runs returned identical output, with clean worktree and unchanged local tag inventory. The explicit source fetch used `--no-tags` and a remote-only release ref; no tags were pushed.
- Combined unit investigation: one full unit run failed the unchanged `TestInflightEstimate_AccountMappingNoDBAndBoundedMemory` process-heap assertion (8,547,584 bytes vs 8,388,608). Ten isolated repetitions and the subsequent complete unit rerun passed. Billing code/tests and thresholds were not edited; the initial failure log was retained. The #338 task subsequently identified an incompletely initialized test fixture creating per-call caches/cleanup goroutines, and owns the fixture-only fix; its final production commit will be merged rather than adjusting thresholds or repeatedly retrying this failure here.
- Environment recovery: initial aggregate checks hit `/tmp` capacity (8.8 GB), not a source regression. Task-owned Go caches/temp files moved to `/workspace`; interrupted gates were restarted sequentially with explicit tooling/cache paths.
- Independent review: the main-thread reviewer accepted the runtime fail-closed, cache identity and UI allowlist boundaries, and found a P2 in the generated full-release footer plus stale workflow-dispatch documentation. The original footer preservation assertion missed the unsafe original installer recommendation. The follow-up removes that recommendation and nonexistent `main` links, pins source/guide links to the release commit, distinguishes provenance from signature/approval, and documents `--ref play/main` separately from the application `tag` input. A generated-publisher regression test was observed failing before the fix; all 21 helper tests now pass. The follow-up also supplies the complete release SHA to snapshot builds, and tests both template-rendering stages. Its complete `make test`, `make build`, and Fork integrity gates passed. Incremental review of the follow-up commit remains pending.
- Initial PR CI: all five checks passed for `470c672de9c0dd2b792767675c3ba03a4186ec92`, including Protected behavior (19m47s), release source policy, documentation, and both security scans. This result predates the footer follow-up; subsequent commit results are recorded separately in the PR.
- Deployment/production settings/credentials: untouched. Merge, deployment commit/health checks and user local-browser acceptance remain with the main thread.
