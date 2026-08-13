# Visual Review: Play Billing mapping table layout fix

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/orders/AdminPlayBillingConfigView.vue"
  ],
  "routes_or_surfaces": [
    "/admin/orders/play-billing",
    "Google Play Billing editable product mapping table"
  ],
  "languages_and_themes": [
    "zh-CN/light static board",
    "zh-CN/dark token review",
    "en-US/light static board",
    "en-US/dark token review"
  ],
  "states": [
    "default",
    "editable-row",
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
    "1280x820",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/play-billing-layout-fix/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/play-billing-layout-fix/baseline-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/play-billing-layout-fix/updated-1280.png"
  ],
  "commands": [
    "python3 generated static Play Billing layout review boards with PIL",
    "file docs/visual-reviews/assets/play-billing-layout-fix/*.png",
    "corepack pnpm --dir frontend exec vitest run src/views/admin/orders/__tests__/AdminPlayBillingConfigView.spec.ts --run",
    "corepack pnpm --dir frontend design:check",
    "corepack pnpm --dir frontend typecheck"
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
    "The artifacts are static review boards rather than browser screenshots, so final browser screenshot acceptance on the production admin route is still required after deployment.",
    "Real Google Play product data and service-account status still need production browser verification."
  ]
}
-->

## Scope

- Routes: `/admin/orders/play-billing`.
- Roles: administrator only.
- Languages and themes: no copy changes; existing zh-CN/en-US strings and shared light/dark tokens are reused.

## Baseline

- Current behavior: the editable Play Billing mapping table has nine dense columns. The shared `DataTable` defaults to sticky first and sticky action columns, so the right-side action column can visually cover the consumable and price controls.
- Baseline artifact: `docs/visual-reviews/assets/play-billing-layout-fix/baseline-1280.png`.
- Inconsistencies observed: this page is a spreadsheet-like form, not a read-only list. Sticky action columns are useful on simple tables, but in this form they can overlap inputs and make operators think the row is broken.

## Prototype

- Prototype design image: `docs/visual-reviews/assets/play-billing-layout-fix/prototype-1280.png`.
- Approval status: emergency layout fix based on the reported production screenshot; scope is limited to preventing overlap on this Play Billing mapping table.
- Scope boundary: no shared `DataTable` default changes, no route/sidebar changes, no new controls, no Play Console or purchase-flow behavior changes.

## Reuse Decision

- Reused `AppLayout`, `DataTable`, `Select`, `Toggle`, shared `Icon`, `btn` and `input` styles.
- Disabled sticky first/action columns only for this editable mapping table, then assigned stable per-column minimum widths so the table scrolls horizontally when needed.
- No new shared component, icon, raw color, decorative gradient, radius exception or page shell exception was added.

## State Coverage

- Default and editable row: columns keep enough width for product ID, display text, entitlement, price, consumable and action controls.
- Hover and active: unchanged; shared button, input, select and toggle interactions still own these states.
- Focus-visible and keyboard: unchanged shared controls remain focusable. The action column is now in normal flow, so keyboard focus cannot be visually hidden under an overlay.
- Loading, disabled, empty, error and success: existing table loading/empty slots, disabled controls, validation errors and save toast behavior are preserved.

## Viewport Coverage

- Mobile: the shared mobile card layout is preserved; edited input wrappers now use `min-w-0` so fields do not force the card wider than the viewport.
- Tablet: cards remain readable under the shared `DataTable` mobile breakpoint.
- Desktop: the desktop table receives explicit column widths and horizontal scrolling instead of overlapping fixed columns.
- Wide or short screen: workspace route still uses the available admin content width and the table remains scrollable when the content exceeds the viewport.
- 200% zoom and reduced motion: no motion was added; only the existing user-triggered refresh spinner remains.

## Evidence

- Baseline board: `docs/visual-reviews/assets/play-billing-layout-fix/baseline-1280.png`.
- Prototype board: `docs/visual-reviews/assets/play-billing-layout-fix/prototype-1280.png`.
- Updated board: `docs/visual-reviews/assets/play-billing-layout-fix/updated-1280.png`.
- Automated layout regression: `AdminPlayBillingConfigView.spec.ts` now asserts sticky first/action columns are disabled and the dense columns carry stable minimum widths.
- Commands run: see manifest.

## Residual Risk

- These are static review boards, not a live browser capture. After this patch is deployed, the production page should still be opened in a browser at `/admin/orders/play-billing` and checked against real mappings.
- This patch fixes the admin mapping table only; it does not configure Play Console products, Google service-account keys or real purchase verification.
