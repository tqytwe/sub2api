# Visual Review: Play Billing multi-row mapping editor

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/orders/AdminPlayBillingConfigView.vue",
    "frontend/src/views/admin/orders/__tests__/AdminPlayBillingConfigView.spec.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts"
  ],
  "routes_or_surfaces": [
    "/admin/orders/play-billing",
    "Google Play Billing product mapping editor"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "zh-CN/dark",
    "en-US/light",
    "en-US/dark"
  ],
  "states": [
    "default",
    "editable-card",
    "enabled",
    "disabled",
    "validation-error",
    "loading",
    "saving",
    "empty",
    "focus-visible"
  ],
  "viewports": [
    "360x800",
    "768x900",
    "1280x900",
    "1920x1039"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/play-billing-multiline/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/play-billing-multiline/baseline-1920.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/play-billing-multiline/updated-1280.png",
    "docs/visual-reviews/assets/play-billing-multiline/updated-360.png"
  ],
  "commands": [
    "python3 generated static multi-row Play Billing review boards with Pillow",
    "file docs/visual-reviews/assets/play-billing-multiline/*.png",
    "corepack pnpm --dir frontend exec vitest run src/views/admin/orders/__tests__/AdminPlayBillingConfigView.spec.ts --run",
    "corepack pnpm --dir frontend design:check",
    "corepack pnpm --dir frontend typecheck",
    "corepack pnpm --dir frontend lint:check"
  ],
  "checks": {
    "keyboard": {
      "status": "passed"
    },
    "reduced_motion": {
      "status": "passed"
    }
  },
  "residual_risks": [
    "The evidence is a static review board rather than a browser capture; final local browser acceptance is still required on the deployed administrator route.",
    "Real production mappings and the authenticated admin session should be checked after deployment."
  ]
}
-->

## Scope

- Route: `/admin/orders/play-billing`.
- Role: administrator only.
- Languages and themes: existing Chinese and English resources, with shared light/dark tokens.

## Baseline

- Current behavior: the page used one dense nine-column editable table row. Even after sticky columns were disabled, the row still forced operators to read and edit a very wide strip.
- Baseline artifact: `docs/visual-reviews/assets/play-billing-multiline/baseline-1920.png`.
- User-reported issue: price, entitlement, consumable and action controls appeared compressed or visually adjacent in one horizontal row.

## Prototype

- Prototype artifact: `docs/visual-reviews/assets/play-billing-multiline/prototype-1280.png`.
- Approval status: implementation follows the user's requested multi-row layout.
- Scope boundary: only the Play Billing admin editor changes; backend contracts, checkout-info, purchase verification and other DataTable pages remain unchanged.

## Reuse Decision

- Reused `AppLayout`, `Select`, `Toggle`, shared `Icon`, `btn`, `input`, semantic surface tokens and existing API/state logic.
- Replaced the editable `DataTable` surface with one bordered product card per mapping.
- Each card is split into responsive rows: identity/type, display/entitlement, and price/currency.
- No new icon, shared table behavior, page shell width, raw color, gradient or large-radius pattern was added.

## State Coverage

- Default and editable card: every field has a visible label and full-width control within its card.
- Hover and active: existing shared button, input, select and toggle behavior remains in force.
- Focus-visible and keyboard: controls remain native/focusable; copy/delete buttons retain `aria-label`, title and focus-visible rings.
- Loading, disabled, empty, error and success: loading placeholders, disabled controls during save, empty state, field-specific validation messages and existing success/error toasts are preserved.

## Viewport Coverage

- Mobile 360px: fields stack to one column and do not require horizontal scrolling.
- Tablet 768px: fields use two columns while preserving readable labels and controls.
- Desktop 1280px: fields use four columns across each row, with entitlement spanning two columns.
- Wide 1920px: cards remain bounded by the shared workspace content without a spreadsheet-like strip.
- 200% zoom and reduced motion: no new animation was added; card controls remain in normal document flow.

## Evidence

- Baseline: `docs/visual-reviews/assets/play-billing-multiline/baseline-1920.png`.
- Prototype: `docs/visual-reviews/assets/play-billing-multiline/prototype-1280.png`.
- Updated desktop: `docs/visual-reviews/assets/play-billing-multiline/updated-1280.png`.
- Updated mobile: `docs/visual-reviews/assets/play-billing-multiline/updated-360.png`.
- Automated regression: the component test asserts that mappings render as cards, the old DataTable is absent, the product ID stays `min-w-0`, and all card labels/actions are present.
- Commands run: listed in the manifest.

## Residual Risk

- Static boards are not authenticated browser screenshots. The user must open the deployed admin page and verify a real mapping at desktop and narrow width.
- This change does not alter Google Play Console product setup or backend purchase fulfillment.
