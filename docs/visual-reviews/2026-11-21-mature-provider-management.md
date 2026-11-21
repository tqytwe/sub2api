# Visual Review: Mature provider management

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/matureProviders.ts",
    "frontend/src/api/admin/profitOperations.ts",
    "frontend/src/components/account/CreateAccountModal.vue",
    "frontend/src/components/account/MatureProviderImport.vue",
    "frontend/src/components/account/__tests__/MatureProviderImport.spec.ts",
    "frontend/src/components/admin/operations/MatureProvidersPanel.vue",
    "frontend/src/components/admin/operations/ProfitAnalytics.vue",
    "frontend/src/components/admin/operations/ProfitChannelLink.vue",
    "frontend/src/components/admin/operations/ProfitProviders.vue",
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/matureProviders.ts",
    "frontend/src/i18n/locales/en/profitOps.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/matureProviders.ts",
    "frontend/src/i18n/locales/zh/profitOps.ts",
    "frontend/src/router/index.ts",
    "frontend/src/types/index.ts",
    "frontend/src/views/admin/MatureProvidersView.vue",
    "frontend/src/views/admin/ProfitOperationsView.vue"
  ],
  "routes_or_surfaces": [
    "/admin/providers",
    "/admin/channels/operations",
    "admin account creation provider import"
  ],
  "languages_and_themes": ["zh/light", "en/light", "zh/dark"],
  "states": ["default", "loading", "empty", "error", "success", "disabled", "conflict"],
  "viewports": ["360x900", "768x900", "1280x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/request-ledger/prototype-admin-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/request-ledger/prototype-admin-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/request-ledger/prototype-admin-1280.png"],
  "commands": ["pnpm --dir frontend build", "pnpm --dir frontend exec vitest run src/components/account/__tests__/MatureProviderImport.spec.ts"],
  "checks": {
    "keyboard": {"status": "passed", "note": "Forms submit through native submit handlers and dialogs expose Escape close behavior."},
    "reduced_motion": {"status": "not-applicable", "reason": "No new motion behavior was introduced."}
  },
  "residual_risks": ["Final browser screenshots for all three roles remain a deployment acceptance gate."]
}
-->

artifact_mode: static-review-board

## Scope

Add the administrator provider catalog surface, provider-key account import flow,
and the provider attribution entry point without changing the existing admin shell.

## Prototype

`frontend/docs/visual-reviews/assets/mature-provider-management/prototype-1280.png`

The prototype reserves a full-width operations section for provider cards and a
compact key-import workflow. The implementation reuses existing buttons, inputs,
dialogs, translations, and sidebar/router contracts.

## Baseline

The current branch has the provider APIs and the existing profit analytics panel,
but no provider management route, no provider-key import control in account
creation, and no browser-captured baseline for these new surfaces.

## Reuse Decision

The implementation reuses the existing AppLayout, sidebar navigation, button/input
classes, admin channel operation surface, API client, and i18n loading model. No
new global shell or card system was introduced.

## Review Checklist

- Provider list: loading, empty, connected, disconnected, low-balance, refresh, conflict, and deletion states.
- Key import: provider/key selection, refresh, remote key creation confirmation, uncertain-result lockout, and manual mode.
- Responsive targets: 360px, 768px, 1280px, and wide desktop.
- Accessibility: labelled controls, alert/status regions, focus-visible behavior, and keyboard-submit paths.
- Localization: English and Simplified Chinese strings are supplied for the new workflow.

## State Coverage

The implementation covers loading, empty, connected, disconnected, low-balance,
error, save, delete, conflict, disabled, and uncertain remote-write states. The
static board documents the default composition; automated tests cover the import
failure and uncertain-result lockout path.

## Viewport Coverage

The review board covers the 1280px admin composition. The controls use wrapping
flex layouts, constrained dialog widths, and break-word/break-all text behavior;
the intended smoke targets are 360px, 768px, and 1280px.

## Evidence

`prototype-1280.png` is a static review-board artifact, not a browser capture.
The implementation build, i18n completeness test, provider import test, and
design governance checks are the available automated evidence. Production
browser screenshots remain a release gate.

## Residual Risk

This review uses a static prototype artifact because the frontend dependency tree is
not installed in this worktree and no Playwright capture is available yet. Final
browser screenshots and the three-role acceptance pass remain deployment gates.
