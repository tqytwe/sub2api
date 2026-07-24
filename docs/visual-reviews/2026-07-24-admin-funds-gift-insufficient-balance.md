# Visual Review: admin funds gift insufficient balance

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts"
  ],
  "routes_or_surfaces": [
    "/admin/funds",
    "admin gift balance error alert"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "en-US/light",
    "zh-CN/dark",
    "en-US/dark"
  ],
  "states": [
    "gift request rejected with BALANCE_LEDGER_INSUFFICIENT_BALANCE",
    "existing generic fund error fallback"
  ],
  "viewports": [
    "360x800",
    "768x800",
    "1280x858"
  ],
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
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend exec vitest run src/views/admin/__tests__/AdminFundsView.stepup.spec.ts src/api/__tests__/admin.funds.spec.ts"
  ],
  "checks": {
    "keyboard": {
      "status": "not-applicable",
      "reason": "The change adds localized error copy only and does not alter focus order, controls, or keyboard handlers."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "No motion, transition, animation, or timed visual behavior is changed."
    }
  },
  "residual_risks": [
    "The reused static review board is not a fresh browser screenshot; final acceptance requires the deployed backend fix and an authenticated production browser session."
  ]
}
-->

## Scope

This review covers the administrator funds page when a gift balance request receives `BALANCE_LEDGER_INSUFFICIENT_BALANCE`. The implementation adds Chinese and English localized copy for that backend reason while preserving the existing alert/toast rendering path.

## Baseline

The production screenshot supplied by the user shows `/admin/funds` rendering the generic Chinese failure alert after `POST /api/v1/admin/funds/gifts` returns `400`. The alert container, spacing, color, and placement already exist and are not changed by this fix.

Baseline artifact: `docs/visual-reviews/assets/admin-balance-flow-fund-types/before-admin-balance-flow-fund-types.png`.

## Prototype

No new layout prototype is needed. The intended visual result is the same existing funds-management error surface with clearer localized text instead of an unmapped backend reason or generic failure.

Prototype artifact: `docs/visual-reviews/assets/admin-balance-flow-fund-types/prototype-admin-balance-flow-fund-types.png`.

## Reuse Decision

The change reuses the existing `AdminFundsView` error mapping, i18n objects, alert styling, form layout, buttons, inputs, and dark/light theme tokens. No component, route, spacing, color, or typography rule is introduced.

## State Coverage

- Error: `BALANCE_LEDGER_INSUFFICIENT_BALANCE` now maps to specific Chinese and English copy.
- Fallback: other fund error reasons keep the existing fallback behavior.
- Loading, success, disabled, hover, active, and focus-visible states are unchanged.

## Viewport Coverage

The visible container and wrapping behavior are unchanged across mobile, tablet, and desktop because only the locale string values changed. The Chinese copy is short enough for the existing full-width alert. The English copy is a single sentence and can wrap inside the same alert if needed.

## Evidence

Commands planned or run for this change:

```bash
pnpm --dir frontend design:check
pnpm --dir frontend typecheck
pnpm --dir frontend exec vitest run src/views/admin/__tests__/AdminFundsView.stepup.spec.ts src/api/__tests__/admin.funds.spec.ts
```

Updated artifact: `docs/visual-reviews/assets/admin-balance-flow-fund-types/after-admin-balance-flow-fund-types.png`.

Backend behavior is covered separately by the balance ledger tests that allow positive credits toward an existing negative balance.

## Residual Risk

The reused static review board is not a fresh browser screenshot. Final confirmation still requires the merged build to deploy and an administrator to retry the gift action in a local authenticated browser session.
