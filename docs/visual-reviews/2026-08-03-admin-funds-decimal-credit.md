# Visual Review: admin funds decimal credit

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/funds.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/views/admin/AdminFundsView.vue"
  ],
  "routes_or_surfaces": ["/admin/funds/grants"],
  "languages_and_themes": ["zh-CN/light", "en-US/light", "zh-CN/dark", "en-US/dark"],
  "states": ["default", "valid decimal 0.5", "invalid zero or precision greater than 8", "loading", "success", "server error"],
  "viewports": ["360x800", "768x800", "1280x858"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/admin-balance-flow-fund-types/prototype-admin-balance-flow-fund-types.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/admin-balance-flow-fund-types/before-admin-balance-flow-fund-types.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/admin-balance-flow-fund-types/after-admin-balance-flow-fund-types.png"
  ],
  "commands": [
    "pnpm design:check",
    "pnpm typecheck",
    "pnpm exec vitest run src/api/__tests__/admin.funds.spec.ts src/views/admin/__tests__/AdminFundsView.stepup.spec.ts"
  ],
  "checks": {
    "keyboard": {"status": "passed", "reason": "The existing form controls and focus order remain unchanged."},
    "reduced_motion": {"status": "not-applicable", "reason": "No motion behavior changes."}
  },
  "residual_risks": ["Static review evidence does not replace final browser acceptance by an authenticated administrator after deployment."]
}
-->

## Scope

The administrator gift-balance and offline-recharge amount fields now accept a positive ledger amount with up to eight decimal places. This removes the unrelated withdrawal whole-unit restriction while preserving the existing layout, controls, step-up flow, and error surface.

## Baseline

The existing funds-management form displays integer-only amount guidance and presents its validation message in the existing full-width alert. The baseline review board is `docs/visual-reviews/assets/admin-balance-flow-fund-types/before-admin-balance-flow-fund-types.png`.

## Prototype

The prototype is `docs/visual-reviews/assets/admin-balance-flow-fund-types/prototype-admin-balance-flow-fund-types.png`. It confirms the existing two-column form layout remains appropriate; only the accepted numeric format and input keyboard mode change.

## Reuse Decision

The review reuses the existing funds-management static review board because this is an input-mode and locale-copy change, not a layout change. `AdminFundsView` continues to use the existing AppLayout, card, input, button, alert, and TOTP step-up components. No new visual pattern, color, spacing, or motion is introduced.

## State Coverage

- Default: both credit fields describe positive amounts with up to eight decimals.
- Valid: `0.5` and `30.00` submit through the existing step-up protected API flow.
- Error: zero, negative values, non-numeric input, and precision above eight decimals retain the entered value and show the localized validation message.
- Loading, success, server-error, focus-visible, hover, and disabled states reuse existing components without visual changes.

## Viewport Coverage

The new Chinese and English strings may wrap within the existing full-width form and alert containers; no fixed-width control or page geometry changed. Mobile, tablet, desktop, dark theme, and final authenticated administrator acceptance remain release checks.

## Evidence

The existing static review board remains valid because the form geometry, component reuse, and alert placement are unchanged. Targeted Vitest coverage verifies valid and invalid decimal input behavior; `pnpm design:check`, `pnpm typecheck`, and `pnpm lint:check` verify visible-file mapping and source safety.

## Residual Risk

Static review evidence does not replace authenticated administrator acceptance after deployment. The final check must submit a `0.5` gift balance on the production administrator page and confirm the corresponding ledger entry.
