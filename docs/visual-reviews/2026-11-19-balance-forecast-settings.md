# Balance Forecast Settings

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/settings.ts",
    "frontend/src/views/admin/SettingsView.vue",
    "frontend/src/views/admin/__tests__/SettingsView.spec.ts",
    "frontend/src/i18n/locales/en/admin/settings.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts"
  ],
  "routes_or_surfaces": ["/admin/settings notification settings"],
  "languages_and_themes": ["zh-CN/light", "en-US/light", "zh-CN/dark", "en-US/dark"],
  "states": ["disabled", "enabled", "invalid input", "saved", "error"],
  "viewports": ["360x800", "768x800", "1280x800"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-19-balance-forecast-settings/prototype.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/wallet-ledger-table-width/baseline-wallet-ledger-left-table.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/wallet-ledger-table-width/updated-wallet-ledger-full-width-table.png"],
  "commands": ["pnpm exec vitest run src/views/admin/__tests__/SettingsView.spec.ts", "pnpm design:check", "pnpm typecheck"],
  "checks": {"keyboard": {"status": "passed", "notes": "Existing form controls and Toggle remain keyboard accessible."}, "reduced_motion": {"status": "passed", "notes": "No motion was added."}},
  "residual_risks": ["Final production browser acceptance must verify the settings card in light and dark themes."]
}
-->

## Baseline

The existing notification section already contains the balance-low notification
controls. The forecast controls reuse that section and its existing `Toggle`,
`input`, card, spacing, and light/dark tokens.

## Prototype

`prototype_artifacts: docs/visual-reviews/2026-07-25-admin-balance-surcharge-details.md`

The existing admin balance settings review is the approved visual reference for
the reused notification card. No new visual pattern or navigation entry was added.

## Scope

- Route: `/admin/settings`, admin notification settings.
- Roles: administrator only.
- Scope: forecast configuration and persistence; no change to balance calculation or billing.

## Reuse Decision

- Reuses the existing balance notification card, `Toggle`, native number inputs, spacing, and semantic theme tokens.
- No new route, card family, icon, or navigation entry was added.

## State Coverage

- disabled: only the forecast toggle is visible
- enabled: threshold-hours and maximum-notifications inputs are visible
- invalid input: browser min/max constraints plus server-side validation remain active
- light/dark: existing card and input tokens are reused

## Viewport Coverage

- 360px: inputs stack through the existing responsive grid.
- 768px and 1280px: the two numeric controls share one row with the established card padding.
- Keyboard and reduced motion: native inputs and existing Toggle remain keyboard accessible; no motion was added.

## Evidence

- Prototype: `docs/visual-reviews/assets/2026-11-19-balance-forecast-settings/prototype.png`.
- Automated evidence: SettingsView component test, design governance, and typecheck.
- Production browser evidence remains a post-deployment acceptance step.

## Residual Risk

This change was implemented in the server worktree without a production browser
capture. The user must verify `/admin/settings` in both themes after deployment.
