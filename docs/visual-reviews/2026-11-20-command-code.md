# Command Code account support

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/account/AccountUsageCell.vue",
    "frontend/src/components/account/CNProviderBalanceCell.vue",
    "frontend/src/components/account/CNProviderQuotaCell.vue",
    "frontend/src/components/account/CreateAccountModal.vue",
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/components/account/OpenCodeGoProtocolRulesEditor.vue",
    "frontend/src/components/account/credentialsBuilder.ts",
    "frontend/src/components/common/PlatformIcon.vue",
    "frontend/src/components/keys/UseKeyModal.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/admin/accounts.ts",
    "frontend/src/i18n/locales/en/admin/overview.ts",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/admin/accounts.ts",
    "frontend/src/i18n/locales/zh/admin/overview.ts",
    "frontend/src/i18n/locales/zh/dashboard.ts"
  ],
  "routes_or_surfaces": [
    "/admin/accounts existing account forms and usage table",
    "/admin/groups existing platform selector",
    "/admin/settings existing scheduling thresholds",
    "/keys existing client snippets"
  ],
  "languages_and_themes": [
    "zh/light",
    "en/dark",
    "en/light",
    "zh/dark"
  ],
  "states": [
    "existing provider selected",
    "Command Code selected",
    "catalog fallback",
    "default official endpoints"
  ],
  "viewports": [
    "360x800",
    "768x900",
    "1280x900",
    "1920x1080"
  ],
  "artifact_mode": "browser-capture",
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-20-command-code/baseline-create-1280.png"
  ],
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-20-command-code/prototype-create-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-20-command-code/updated-create-1280.png",
    "docs/visual-reviews/assets/2026-11-20-command-code/updated-create-360.png"
  ],
  "commands": [
    "Playwright Chromium production build route with intercepted fixture APIs and admin account selection",
    "pnpm typecheck",
    "pnpm design:check"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "scenarios": 4
    },
    "viewport_overflow": {
      "status": "passed",
      "scenarios": 4
    },
    "reduced_motion": {
      "status": "passed"
    }
  },
  "residual_risks": [
    "Production browser acceptance is pending. Browser capture used the built administrator route with fixture responses. No live provider credentials were used."
  ]
}
-->

## Scope

Command Code appears in the existing account and group workflows. The account form uses the shared protocol and endpoint controls. The existing account usage column shows credit balance and plan windows for accounts on the official host.

## Baseline

Captured the built admin accounts route and opened the OpenCode form, the nearest existing provider workflow with per-model protocol routing.

## Prototype

Annotated the same built account form before implementation to mark the new platform option, existing routing controls and usage surface.

## Reuse Decision

Use the existing platform selector, dialog, protocol rule editor, usage cells and scheduling threshold settings.

## State Coverage

Enter opened the existing create dialog and selected Command Code. The form displayed the official endpoint and catalog fallback controls. Tab kept focus inside the dialog, and Escape closed it and restored focus to the add account button. Backend fixtures exercise credit eligibility and organization balance lookup.

## Viewport Coverage

The browser opened the form at 360, 768, 1280 and 1920 pixels in light and dark themes with Chinese and English settings.

## Evidence

The browser loaded production bundles with disposable fixture account and settings responses.

## Residual Risk

Authenticated production browser acceptance and verification against a live provider account remain pending.
