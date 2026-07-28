# Visual Review: daily-card-lifecycle

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/SubscriptionsView.vue",
    "frontend/src/views/admin/orders/PlanEditDialog.vue",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts"
  ],
  "routes_or_surfaces": [
    "/admin/subscriptions",
    "/admin/payment/plans edit dialog"
  ],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light"],
  "states": [
    "active daily card",
    "exhausted daily card",
    "expired daily card",
    "one-time plan validation error"
  ],
  "viewports": ["360x800", "768x1024", "1280x900", "1644x650"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/daily-card-lifecycle/prototype-admin-subscriptions-effective-status.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/daily-card-lifecycle/baseline-admin-subscriptions.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/daily-card-lifecycle/prototype-admin-subscriptions-effective-status.png"
  ],
  "commands": [
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend lint:check",
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend exec vitest run src/utils/__tests__/subscriptionQuota.spec.ts src/views/admin/orders/__tests__/PlanEditDialog.spec.ts"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "Existing table, Select, button, and BaseDialog keyboard behavior is reused."},
    "reduced_motion": {"status": "passed", "notes": "The progress transition is limited to width and color; no continuous motion was added."}
  },
  "residual_risks": [
    "The prototype is a static review board, not an authenticated production browser capture.",
    "Final acceptance must verify the exhausted account in the administrator table and confirm its API request is rejected."
  ]
}
-->

## Scope

- Administrator subscription rows use the immutable entitlement quota, usage, expiry, and terminal status for daily cards.
- The quota-reset action is hidden for daily cards because their quota is never reset.
- One-time plan duration is rejected when it exceeds the parent subscription validity.

## Baseline

- The reported production row showed `$17.28 / $16.00` while its status remained active.
- The table read legacy parent daily usage and status, so midnight resets and exhausted cards could appear reusable.
- Baseline artifact: `assets/daily-card-lifecycle/baseline-admin-subscriptions.png`.

## Prototype

- Prototype artifact: `assets/daily-card-lifecycle/prototype-admin-subscriptions-effective-status.png`.
- The existing dense table layout is retained; only the data source, terminal wording, progress state, and applicable actions change.
- No new page shell, navigation, dialog, card, icon, or color pattern is introduced.

## Reuse Decision

- Reused the current admin table, semantic status badges, progress bar, action buttons, `Select`, and `BaseDialog` patterns.
- The existing plan dialog fields and validation presentation remain responsible for errors and disabled states.
- No design-system exception is required.

## State Coverage

- Active: shows entitlement usage and the exact entitlement expiry time.
- Exhausted: shows the quota capped at its immutable limit, an exhausted status, and no reset action.
- Expired: shows the entitlement expiry status rather than a midnight reset.
- Validation error: prevents a one-time duration longer than parent validity before submission and repeats validation on the backend.
- Loading, empty, request error, hover, focus-visible, disabled, and success behavior remain unchanged in the reused components.

## Viewport Coverage

- The change does not alter table tracks, page width, gutters, scrolling, or responsive breakpoints.
- Existing mobile horizontal table access and desktop density remain unchanged at 360, 768, 1280, and wide desktop widths.
- No viewport-scaled text or layout-moving interaction was introduced; reduced motion keeps the same static terminal state.

## Evidence

- Baseline and prototype are valid PNG artifacts and were inspected at 1644x650.
- Targeted component and utility tests cover quota selection, immutable plan duration, and terminal display helpers.
- `design:check`, lint, typecheck, full tests, and build are release gates and are recorded in the final delivery evidence.

## Residual Risk

- Authenticated browser evidence must be collected on production after Zeabur deploys the merged commit.
- The reported account must show `$16.00 / $16.00`, `已耗尽`, and receive a daily-card exhaustion rejection on a new request.
