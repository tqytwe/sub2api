# Visual Review: daily-card-entitlements

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/orders/PlanEditDialog.vue",
    "frontend/src/views/user/SubscriptionsView.vue",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts"
  ],
  "routes_or_surfaces": [
    "/admin/payment/plans edit dialog",
    "/subscriptions"
  ],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light"],
  "states": [
    "recurring plan quota fields hidden",
    "one-time plan quota fields visible",
    "active daily card",
    "exhausted daily card",
    "queued daily cards"
  ],
  "viewports": ["360x800", "768x1024", "1280x900", "1920x1080"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/daily-card-entitlements/prototype-admin-plan-quota.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/subscription-workspace-prototype/user-admin-subscription-config-1920.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/daily-card-entitlements/prototype-admin-plan-quota.png"
  ],
  "commands": [
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend lint:check",
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend exec vitest run src/views/admin/orders/__tests__/PlanEditDialog.spec.ts"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "Existing Select, input, form submit, and BaseDialog keyboard contracts are reused."},
    "reduced_motion": {"status": "passed", "notes": "No new motion or animation is introduced."}
  },
  "residual_risks": [
    "Static review board is not a browser screenshot; authenticated production verification remains required after deployment.",
    "Long translated labels and live group names still require final checks at the listed viewports."
  ]
}
-->

## Scope

- Administrator: configure a plan as recurring quota or immutable one-time quota, with explicit USD quota and duration.
- User: display the active daily card's real usage, expiry, terminal status, and queue count instead of a calendar reset.
- The existing dialog, form controls, subscription cards, progress bars, and responsive grid remain in place.

## Baseline

- The plan editor only exposed validity and inherited group limits, so administrators could not declare one-time quota semantics.
- The subscription page inferred a day card from the parent subscription duration. Stacked purchases made that duration exceed 24 hours and could restore the legacy reset label.
- Existing subscription administration evidence is reused as the structural baseline.

## Prototype

- The prototype adds one compact quota-rule section to the existing plan dialog.
- One-time mode shows total USD quota and duration in hours; recurring mode hides those fields.
- The user page keeps its current layout and changes only the source and wording of quota lifecycle data.

## Reuse Decision

- Reused `BaseDialog`, `Select`, native `.input`, existing form validation/toasts, subscription progress bars, and current light/dark tokens.
- No new shared component, icon, page shell, or visual exception is introduced.

## State Coverage

- Default: recurring mode remains backward compatible.
- One-time: quota and duration are required and saved with the plan.
- Active: remaining time is based on the entitlement expiry, not midnight.
- Exhausted/expired: terminal wording replaces reset wording.
- Queued: the number of later cards is shown without changing the active card's 24-hour window.
- Loading, error, success, hover, focus-visible, and disabled behavior remain owned by the existing dialog and page.

## Viewport Coverage

- The quota fields use the existing one-column mobile and three-column desktop form grid.
- The subscription page retains the existing one-column mobile and two-column desktop layout.
- No viewport-scaled type, fixed page width, page-owned scrolling, or layout-moving hover behavior was added.

## Evidence

- Prototype: `prototype-admin-plan-quota.png`, a 1280x900 static review board generated before UI implementation.
- Component tests cover one-time plan payload persistence; typecheck and design checks cover the user subscription projection.
- Static artifact generation does not claim browser or production acceptance.

## Residual Risk

- Final browser evidence must be collected after deployment using real administrator and user accounts.
- Verify Chinese and English labels, light/dark themes, and active/exhausted/queued cards with production data.
