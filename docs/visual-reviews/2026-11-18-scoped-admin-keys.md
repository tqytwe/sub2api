# Visual Review: Scoped Admin API Keys

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/settings/AdminAccessKeysSettings.vue",
    "frontend/src/components/admin/settings/__tests__/AdminAccessKeysSettings.spec.ts",
    "frontend/src/api/admin/adminAccessKeys.ts",
    "frontend/src/views/admin/SettingsView.vue",
    "frontend/src/i18n/index.ts",
    "frontend/src/i18n/locales/en/adminAccessKeys.ts",
    "frontend/src/i18n/locales/en/index.ts",
    "frontend/src/i18n/locales/zh/adminAccessKeys.ts",
    "frontend/src/i18n/locales/zh/index.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts"
  ],
  "routes_or_surfaces": ["/admin/settings existing settings page, admin API keys section"],
  "languages_and_themes": ["zh/light", "en/dark"],
  "states": ["existing key list", "create key form", "resource scope rule", "global permission selection", "loading", "error"],
  "viewports": ["1280x1000", "390x1000"],
  "artifact_mode": "browser-capture",
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-18-scoped-admin-keys/baseline-432-settings-zh-light.png"],
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-18-scoped-admin-keys/desktop-zh-light.png"],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-18-scoped-admin-keys/desktop-zh-light.png",
    "docs/visual-reviews/assets/2026-11-18-scoped-admin-keys/mobile-en-dark-edit.png"
  ],
  "commands": ["Vite dev server on loopback with Playwright Chromium; scoped key/group APIs intercepted with synthetic responses; no authenticated production calls"],
  "checks": {
    "viewport_overflow": {"status": "passed", "scenarios": 2},
    "page_errors": {"status": "passed"},
    "keyboard": {"status": "passed"},
    "reduced_motion": {"status": "passed"}
  },
  "residual_risks": [
    "Captures use synthetic keys and groups and do not prove deployed authorization or API behavior.",
    "The #432 baseline contains only the legacy full-access administrator key panel. Upstream v2.21.2 already includes scoped admin keys; the comparison uses its actual Security tab and the current integration.",
    "The tag capture includes unrelated settings below the panel; those settings are outside this review scope."
  ]
}
-->

## Scope

Evaluate the new key management section inside the existing administrator settings page. The feature adds key lifecycle controls, resource-scoped permission rules and explicit global permissions without creating a new page or user-facing entry.

## Baseline

The #432 production baseline renders the existing Security tab with the legacy full-access administrator API key panel.

## Prototype

Fixed `v2.21.2` already includes the upstream scoped-key component. It is rendered in the existing Security tab with synthetic API data; the Fork adds permission localization while keeping the page and operations.

## Reuse Decision

The implementation reuses the existing administrator settings page, shared inputs, buttons, dialogs and locale-loading flow. It adds no new route.

## State Coverage

The captures cover the existing key list and the create form with group and global permission sections. Component tests cover loading and request failures, one-time secret display, edits, rotation, revocation and retained history behavior.

## Viewport Coverage

The current component was rendered at 1280px in Chinese light mode and 390px in English dark mode with reduced motion. Both had no horizontal overflow or browser errors.

## Evidence

Playwright Chromium rendered the detached #432 baseline and fixed `v2.21.2`/current surfaces through Vite. API requests use synthetic values; no production services or credentials were accessed.

## Residual Risk

Permission labels and categories are localized by permission ID in the English and Chinese bundles; unknown future IDs retain the server-provided label. The captures do not prove deployed authorization, database migration behavior or account scope enforcement; those require integration gates and exact-SHA acceptance.

The browser rendered the actual settings component with synthetic API responses. The desktop image shows the existing key list; the mobile image shows the create form, group-scoped account permissions and independent global permissions. No production account, key or API was accessed.

The panel stayed within both viewport widths and produced no browser errors.
