# Visual Review: Codex configuration and account concurrency

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/accounts.ts",
    "frontend/src/components/keys/CodexQuickConfigModal.vue",
    "frontend/src/views/admin/DashboardView.vue",
    "frontend/src/views/user/KeysView.vue",
    "frontend/src/i18n/locales/en/admin/overview.ts",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/zh/admin/overview.ts",
    "frontend/src/i18n/locales/zh/dashboard.ts",
    "frontend/src/utils/codexQuickConfig.ts"
  ],
  "routes_or_surfaces": [
    "/keys",
    "/admin/dashboard"
  ],
  "languages_and_themes": [
    "zh/light",
    "en/dark",
    "en/light",
    "zh/dark"
  ],
  "states": [
    "Supported and Claude Code only key groups",
    "Open and close configuration dialog",
    "Dashboard snapshot slot count"
  ],
  "viewports": [
    "360x800",
    "768x900",
    "1280x900",
    "1920x1080"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-20-codex-config-dashboard/prototype-user-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-20-codex-config-dashboard/baseline-user-1280.png",
    "docs/visual-reviews/assets/2026-11-20-codex-config-dashboard/baseline-admin-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-20-codex-config-dashboard/updated-user-1280.png",
    "docs/visual-reviews/assets/2026-11-20-codex-config-dashboard/updated-user-360.png",
    "docs/visual-reviews/assets/2026-11-20-codex-config-dashboard/updated-admin-1280.png",
    "docs/visual-reviews/assets/2026-11-20-codex-config-dashboard/updated-admin-360.png"
  ],
  "commands": [
    "Playwright Chromium loads the compiled Vue application with fixture APIs.",
    "Vitest exercises the generated script, catalog loading and modal actions."
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "Enter opens the eligible key dialog and Escape closes it."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "Both pages and the configuration dialog rendered and responded with reduced motion enabled."
    }
  },
  "residual_risks": [
    "Browser captures use fixture API responses and a synthetic API key. Native Windows execution and production product acceptance remain to be verified."
  ]
}
-->

## Scope

Add Codex quick configuration to API key rows and display account concurrency on the admin dashboard.

## Baseline

The key action row offered manual instructions and CC Switch. The dashboard performance card showed RPM and TPM.

## Prototype

The first compiled build places the new action next to the existing key actions and opens the existing dialog shell.

## Reuse Decision

Reuse the key table, BaseDialog, existing button classes, Icon, and the performance card.

## State Coverage

The browser fixture includes an OpenAI key and a Claude Code only key. It checks action visibility, keyboard open and dismissal, and the numeric concurrency response. Script and modal tests cover catalog loading, failure states, clipboard and download.

## Viewport Coverage

Chromium exercised both pages at 360, 768, 1280 and 1920 pixels, in light and dark mode with both languages.

## Evidence

The manifest references the rendered baseline, first compiled draft and final fixture captures.

## Residual Risk

Production acceptance and a native Windows execution check are pending.
