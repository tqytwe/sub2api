# Visual Review: admin surcharge report controls

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/dashboard.ts",
    "frontend/src/i18n/locales/en/admin/overview.ts",
    "frontend/src/i18n/locales/zh/admin/overview.ts",
    "frontend/src/views/admin/DashboardView.vue"
  ],
  "routes_or_surfaces": ["/admin/dashboard surcharge income module"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light"],
  "states": ["default", "search", "sorted", "loading", "disabled", "empty", "error"],
  "viewports": ["390x844", "1280x800"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/admin-surcharge-controls/prototype-surcharge-controls.png",
    "docs/visual-reviews/assets/admin-surcharge-controls/prototype-surcharge-controls-mobile.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/admin-dashboard-billing-surcharge/baseline-admin-dashboard-usage-cards.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/admin-surcharge-controls/prototype-surcharge-controls.png",
    "docs/visual-reviews/assets/admin-surcharge-controls/prototype-surcharge-controls-mobile.png"
  ],
  "commands": ["Chromium headless static review board render"],
  "checks": {
    "keyboard": { "status": "passed", "notes": "Search, sort, refresh, page size and pagination use native input, select and button controls." },
    "reduced_motion": { "status": "passed", "notes": "No motion was added to the surcharge controls." }
  },
  "residual_risks": [
    "Static review-board renders the intended control layout, not the authenticated application. The final admin browser check remains required after deployment."
  ]
}
-->

## Scope

- Route: `/admin/dashboard`.
- Role: administrator.
- Languages and themes: Chinese and English labels; existing light/dark semantic styles.

## Baseline

- Current behavior: the module shows summary totals and a fixed-order list; its rolling 24-hour timestamps are rejected by the report endpoint, then displayed as zero.
- Baseline: existing dashboard usage-card screenshot, before query and sort controls.
- Inconsistency: report request failure looked like valid zero income, and operators could not find or reorder rows.

## Prototype

- Prototype image: the static control board at desktop and mobile widths.
- Approval status: the user requested search and sorting in the existing admin dashboard module.
- Scope boundary: keep the current panel and summary; add search, sortable columns and page size, and show request errors distinctly.

## Reuse Decision

- Reused the existing dashboard card, inputs, buttons, table, icon, loading spinner and pagination patterns.
- No new shared component or design-system exception.

## State Coverage

- Default: existing surcharge summary and detail columns remain in place.
- Hover, active, focus-visible and keyboard: native buttons/inputs/select; sort buttons expose `aria-sort`.
- Loading and disabled: report controls disable during the request; table exposes `aria-busy`.
- Empty: no matching surcharge rows retain the existing empty state.
- Error: visible report error and unknown totals (`—`), never a false zero.
- Success: results replace the previous page and retain the selected search/sort state.

## Viewport Coverage

- Mobile: controls wrap and details remain in the existing horizontally scrollable table.
- Tablet and desktop: search actions share the panel header; summary remains four columns at large widths.
- Wide or short screen: no page-shell dimensions or scrolling ownership changed.
- 200% zoom and reduced motion: no viewport-sized typography or motion was added; final browser check remains outstanding.

## Evidence

- The static review board was rendered with the installed Chromium binary at 390px and 1280px widths.
- Automated interaction evidence is in `DashboardView.spec.ts`; backend handler and repository tests cover timestamp parsing, query filters and sorting.

## Residual Risk

- This board is not an authenticated application capture and does not verify production data. The administrator must verify the live dashboard after deployment.
