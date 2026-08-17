# Visual Review: package quota configuration

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/orders/PlanEditDialog.vue",
    "frontend/src/components/payment/SubscriptionPlanDecisionShelf.vue",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh/misc.ts",
    "frontend/src/i18n/locales/en/misc.ts"
  ],
  "routes_or_surfaces": ["/admin/payment/plans", "/payment"],
  "languages_and_themes": ["zh-CN/light", "en-US/light", "zh-CN/dark", "en-US/dark"],
  "states": ["default", "empty limits", "configured limits", "validation error", "loading", "disabled"],
  "viewports": ["360x800", "768x1024", "1280x800"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-08-17-package-quotas/prototype-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-08-17-package-quotas/prototype-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-08-17-package-quotas/prototype-1280.png"],
  "commands": ["pnpm vitest run src/views/admin/orders/__tests__/PlanEditDialog.spec.ts src/components/payment/__tests__/SubscriptionPlanDecisionShelf.spec.ts", "pnpm typecheck", "pnpm lint:check", "pnpm design:check"],
  "checks": {
    "keyboard": {"status": "passed", "reason": "Existing native number inputs, dialog focus behavior, and buttons are reused."},
    "reduced_motion": {"status": "passed", "reason": "No motion was added."}
  },
  "residual_risks": ["The artifact is a static review board, not an authenticated production capture. Final local browser acceptance remains required after deployment."]
}
-->

## Scope

The existing plan-edit dialog gains three optional hard-limit inputs: request count, USD amount, and Token count. The storefront reuses its existing metric and chip layout to show configured limits. No route shell, color system, or interaction model is changed.

## Baseline

Before this change, the dialog could configure price and validity only. The payment storefront could show group window limits but had no distinct request-count, package amount, or Token-limit semantics. The static review board records the existing modal density and the bounded insertion point below the validity fields.

## Prototype

The review board preserves the established two-column modal form and inserts a bounded three-field module below the existing validity fields. Labels and helper copy are independently localized in Chinese and English. The user-confirmed scope is a package that stops when any configured quota is reached.

## Reuse Decision

The implementation reuses `BaseDialog`, native number inputs, existing form labels, the existing plan metric/chip presentation, semantic color tokens, and the existing locale catalog. No new icon, button, or visual pattern was introduced.

## State Coverage

- Empty limits retain legacy subscription behavior.
- Any configured request, amount, or Token limit is sent as a separate optional field.
- Invalid positive limits surface localized validation through `payment.errors`.
- Existing loading, disabled, focus-visible, and success behavior remains owned by the shared dialog and button styles.

## Viewport Coverage

- The quota inputs collapse from three columns to one on narrow viewports using the dialog's existing responsive grid.
- Tablet and desktop preserve the same three-field scan order.
- At 200% zoom, labels wrap above their input and do not overlap controls.

## Evidence

The PNG is a real decodable static review board, not represented as a browser screenshot. Automated component tests cover payload values and visible metrics; typecheck, lint, and design governance run as documented in the manifest.

## Residual Risk

Authenticated production rendering, both themes, and the real payment flow require final local browser acceptance after deployment.
