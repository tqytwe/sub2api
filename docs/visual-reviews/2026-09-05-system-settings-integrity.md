# Visual Review: system-settings-integrity

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/i18n/locales/en/admin/settings.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts",
    "frontend/src/views/admin/SettingsView.vue"
  ],
  "routes_or_surfaces": ["/admin/settings"],
  "languages_and_themes": ["zh-CN/light", "en-US/light"],
  "states": ["default", "configured secret", "manual Codex version", "auto-sync Codex version", "threshold boundary"],
  "viewports": ["360x800", "1280x800"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/system-settings-integrity/prototype-user-provided-codex-settings.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/system-settings-integrity/baseline-static-review-board-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/system-settings-integrity/updated-static-review-board-1280.png", "docs/visual-reviews/assets/system-settings-integrity/updated-static-review-board-360.png"],
  "commands": ["Chromium headless file screenshot of the non-sensitive static review board", "pnpm --dir frontend run typecheck", "pnpm --dir frontend exec vitest run src/views/admin/__tests__/SettingsView.spec.ts", "pnpm --dir frontend exec vitest run src/i18n/__tests__/routeLocaleRuntime.spec.ts src/i18n/__tests__/lazyLocaleScope.spec.ts src/i18n/__tests__/adminManagementLocaleKeys.spec.ts"],
  "checks": {
    "keyboard": { "status": "passed", "notes": "Restored controls reuse existing native inputs and Toggle components; SettingsView tests locate controls with stable test IDs." },
    "reduced_motion": { "status": "passed", "notes": "This restoration adds no motion and reuses the existing Toggle implementation." }
  },
  "residual_risks": ["This static review board and user-provided reference are not an integrated browser capture. A production-consistent isolated environment and final administrator browser acceptance in Chinese and English remain required before production merge."]
}
-->

## Scope

- Routes: `/admin/settings`, limited to existing registration, scheduling, gateway forwarding and CAPTCHA cards.
- Roles: administrator only; router and sidebar access were inspected and retain the existing admin boundary.
- Languages and themes: Chinese default and explicit English locale are exercised by route locale tests. The static board uses Chinese default copy and does not stand in for full theme validation.

## Baseline

- Current behavior: `origin/play/main` retained the settings response and form state but omitted six existing settings surfaces from `SettingsView.vue`; two recovered fields were also absent from the update payload.
- Baseline screenshot or recording: `assets/system-settings-integrity/baseline-static-review-board-1280.png` records the code-reviewed missing-control state without loading a service or credentials.
- Inconsistencies observed: an operator could not discover Codex version synchronization, platform pause thresholds or CAPTCHA configuration state, and the hidden domain/threshold values could not reliably round-trip through save.

## Prototype

- Prototype design image: `assets/system-settings-integrity/prototype-user-provided-codex-settings.png` is the user-provided reference screenshot of the desired existing Codex settings pattern.
- Approval status: the approved implementation plan constrained the work to restoring upstream v0.2.0 controls while preserving the fork page, payment, Play, image and bilingual behavior.
- Scope boundary: no migration, OAuth credential, group, channel, pricing or production setting value changes; no Astra model implementation in this PR.

## Reuse Decision

- Shared layouts and components reused: existing `AppLayout`, administration cards, native input styles and `Toggle` controls.
- New shared pattern, if any: none. Stable test IDs are test hooks only and do not change layout or interaction semantics.
- Design-system exception, if any: none.

## State Coverage

- Default: registration domain quota, five global threshold inputs, Codex version input and auto-sync toggle use existing compact administration form patterns.
- Hover and active: the restored controls inherit the existing input and Toggle hover/active behavior.
- Focus-visible and keyboard: no custom focus removal or keyboard handlers were added; native inputs and Toggle retain existing behavior.
- Loading, disabled, empty, error and success: empty secret values preserve the backend secret by submitting `undefined`; configured flags show only status. Save/error/success behavior remains the existing page implementation.

## Viewport Coverage

- Mobile: `updated-static-review-board-360.png` verifies the review composition collapses cards and threshold fields without horizontal overflow.
- Tablet: the production template uses the existing responsive grid classes; actual integrated tablet capture remains pending.
- Desktop: `updated-static-review-board-1280.png` verifies the restored content fits the established two-column review composition.
- Wide or short screen: no page frame, fixed width or scrolling ownership was added.
- 200% zoom and reduced motion: no new continuous motion; final browser zoom validation remains an acceptance requirement.

## Evidence

- Updated screenshot or recording: the two updated static review boards show the restored control inventory and security-status presentation using non-sensitive values.
- Automated visual or overlap checks: the SettingsView test asserts all six surfaces, password inputs, status text, threshold normalization and payload exclusion of the synced Codex version; cold locale tests verify `admin-settings` scope for the route.
- Commands run: the manifest lists all non-production local checks. No production server, database, settings API or credential was accessed.

## Residual Risk

- Known limitations: static review artifacts cannot validate real authenticated rendering, CSS inheritance, dark mode, 200 percent zoom or a production-consistent PostgreSQL/Redis/proxy/secret mount environment.
- Follow-up owner: release owner must complete the isolated-environment test and administrator local browser acceptance in Chinese and English before merging or deploying.
