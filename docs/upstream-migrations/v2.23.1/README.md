# v2.23.1 Upstream Sync Ledger

Status: `analysis_in_progress`; `fully_migrated=false`.

## Source and base

- Upstream: `ranxi2001/sub2api` stable release `v2.23.1`.
- Annotated tag object: `3297a4f4547076ed5c5b21e0471a12490985fb80`.
- Peeled source commit: `00e00796080c632188cf9df7912050b144a1a44f`.
- Release `backend/cmd/server/VERSION`: `2.23.1`.
- Production analysis base: `tqytwe/sub2api` `play/main@9c24cd75d2369de5e65be46fbab1945c2f43f696` (PR #451).
- Integration branch: `sync/ranxi-v2.23.1-20261120`.
- Last reviewed version shown by the admin version endpoint remains `2.17.7`. The live latest-release lookup can show `2.23.1`; neither fact means this entire source range has been migrated.

## Release-note triage

These are initial code-presence checks against the current production tree, not claims that every upstream commit or edge case has been reviewed.

| v2.23.1 release area | Fork evidence found | Initial disposition |
|---|---|---|
| OpenAI-compatible video task relay and completion billing | `backend/internal/handler/openai_videos.go`, `backend/internal/service/openai_videos.go`, route and billing regression tests | Migrated with Fork capability gating, task status/content retrieval, and completion settlement. Preserve Fork account ownership and pricing policy. |
| MiniMax H3 model mapping and completed-output settlement | `backend/internal/service/minimax_h3.go`, `backend/internal/handler/minimax_h3.go`, `backend/internal/handler/grok_media.go`; admin editor `frontend/src/views/admin/MiniMaxH3VideoPricing.vue` | Fork-specific implementation and tests exist. Preserve its billing and account-boundary behavior; compare edge cases to upstream. |
| Image usage settlement/recovery | `backend/internal/service/image_usage_billing.go`, `backend/internal/handler/image_billing_response.go`, recovery and receipt tests | Migrated the per-image minimum floor and `X-Image-Usage: billing` settlement receipt while retaining Fork recovery paths. |
| Provider profit operations and wallet-cost evidence | `backend/internal/handler/admin/profit_operations.go`, `backend/internal/repository/profit_operations.go`, `backend/internal/repository/provider_profit_charge.go` | Fork has a customized provider-profit subsystem. Do not import upstream migrations or replace cost semantics without schema and accounting review. |
| New API wallet/cost synchronization, OpenAI legacy compatibility, upstream compression, account/profit fixes | New API cost settings and wallet tests, gateway compatibility, request-compression, account-ops, and profit-operation code | Runtime changes are migrated and tested; commit-level source classification and schema review remain open. |

## Migration and conflict risks

The upstream tag contains 1,100 commits after the upstream `v2.21.2` tag. A whole-tree merge is not an acceptable migration strategy for this fork. The initial migration comparison reports 172 upstream-only migration files, 187 Fork-only migration files to preserve, two same-name changed migrations, and 113 migration-number collisions against the current Fork analysis base. These are inventory counts, not migration approval.

The upstream range introduces migration filenames whose numeric prefixes already exist in the Fork, including `251`, `252`, `343`-`348`, `350`-`364`, `369`-`377`, and `380`-`388`. Names and contents differ. No SQL migration from this range has been copied or applied by this task. Each schema change must be mapped to the fork schema and assigned an unused migration number; deployed migrations must not be rewritten.

The upstream range also includes a separate QQ bot subsystem and many account, billing, gateway, and operations changes. Similar feature names do not establish schema or behavior equivalence. The complete script-generated file/migration report remains a required review artifact before closing the release.

## Work remaining

1. Generate and retain the complete `check_upstream_release.py` report for this pinned source commit; classify changed files by domain and identify commit-level Fork equivalents.
2. Review database/Ent and runtime/API changes, especially same-number migration collisions, image settlement, video state/billing, provider profit cost modes, and OpenAI transport behavior.
3. Add focused regression tests before porting any behavior that is missing or materially different.
4. Update `UpdateService`'s reviewed-source pin only after the reviewed migration scope is approved. Keep upstream release discovery read-only; never install upstream binaries into the Fork.
5. Run backend/frontend tests, builds, lint, migration checks, and `scripts/check-fork-integrity.sh`; then obtain PR CI, exact deployment evidence, and user-local browser acceptance before calling the release delivered.

## Verification evidence

Initial verification on 2026-11-20:

- GitHub release API confirmed `v2.23.1` is a stable, non-draft, non-prerelease release.
- GitHub tag API resolved the annotated tag to the commit above; the tag's `VERSION` file is `2.23.1`.
- The tag was fetched to `refs/remotes/upstream-ranxi/releases/v2.23.1` and compared with the exact `v2.21.2` tag.
- The source checker compared the pinned release tree against the current `play/main` commit; its whole-tree file counts are intentionally not interpreted as a patch to apply. The migration inventory counts above are the actionable collision signal.
- No production database, deployment, or browser actions were performed.
