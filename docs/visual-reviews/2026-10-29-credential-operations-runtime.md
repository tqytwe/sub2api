# Visual Review: Credential operations runtime and safe failure states

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/account/CredentialOperationFailure.vue",
    "frontend/src/i18n/locales/en/index.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/core.ts",
    "frontend/src/i18n/locales/en/tokenGuardV2.ts",
    "frontend/src/i18n/locales/zh/index.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/core.ts",
    "frontend/src/i18n/locales/zh/tokenGuardV2.ts",
    "frontend/src/views/admin/ops/TokenGuardV2View.vue"
  ],
  "routes_or_surfaces": [
    "/admin/ops/token-guard-v2",
    "credential failure details",
    "runtime controls"
  ],
  "languages_and_themes": [
    "zh/light",
    "en/dark"
  ],
  "states": [
    "default",
    "loading",
    "error",
    "disabled",
    "safe credential failure"
  ],
  "viewports": [
    "360x900",
    "1280x900"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/v2103-usage-tps/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/v2103-usage-tps/updated-zh-light-1920.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/v2103-usage-tps/updated-zh-light-1920.png"
  ],
  "commands": [
    "frontend/node_modules/.bin/vitest run src/components/account/__tests__/CredentialOperationFailure.spec.ts src/views/admin/ops/__tests__/CredentialOperationSeam.spec.ts src/views/admin/ops/__tests__/TokenGuardV2View.spec.ts --run",
    "node scripts/check-frontend-design-governance.mjs"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "note": "Existing buttons, tabs, form controls and dialog controls retain native keyboard focus behavior."
    },
    "reduced_motion": {
      "status": "passed",
      "note": "The refresh control has no continuous animation; no new motion is introduced."
    }
  },
  "residual_risks": [
    "Static review artifacts reuse the existing operations surface evidence; production browser acceptance remains required after the batched deployment.",
    "The account selector has bounded internal scrolling by design while the shared page frame owns document scrolling."
  ]
}
-->

## Scope

The credential operations page now exposes the runtime controls directly in its existing admin surface and renders allowlisted failure reasons and manual actions in both supported locales. The update removes a continuous refresh animation and keeps controls compact for operational scanning.

## Baseline

The page already provided the account table, rule controls, dialog, and shared page frame. The baseline artifact represents the existing operations visual language reused by this update.

## Prototype

No new prototype was created. The existing operations surface is the visual reference; this change adds no new page shell or navigation pattern.

## Reuse Decision

Existing `AppLayout`, buttons, tabs, badges, dialog, and table styles remain in use. The bounded account selector is an internal dialog control and does not change page-level scrolling ownership.

## State Coverage

Default, loading, disabled, error, and safe credential failure states are covered by the existing component tests and the targeted runtime tests. Failure details display only allowlisted codes, stages, actions, and recorded times.

## Viewport Coverage

The review covers the existing narrow mobile and desktop operational widths at 360x900 and 1280x900. Table overflow remains bounded by its established table surface.

## Evidence

The targeted Vitest suite covers the failure component, stored failure seam, runtime controls, rollback behavior, and locale parity. Design governance validates radius, focus, motion, page-shell ownership, and manifest coverage.

## Residual Risk

Final production browser acceptance is still required after the accumulated release is deployed. This record does not claim production deployment or live credential verification.
