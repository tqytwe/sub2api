# Visual Review: admin dashboard billing surcharge

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/dashboard.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/admin/overview.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/admin/overview.ts",
    "frontend/src/views/admin/DashboardView.vue"
  ],
  "routes_or_surfaces": ["/admin/dashboard surcharge income module"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light"],
  "states": ["default", "loading", "empty", "paged details", "disabled pagination", "refresh"],
  "viewports": ["360x800", "768x900", "1280x800"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/admin-dashboard-billing-surcharge/prototype-admin-dashboard-surcharge-module.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/admin-dashboard-billing-surcharge/baseline-admin-dashboard-usage-cards.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/admin-dashboard-billing-surcharge/updated-admin-dashboard-surcharge-module.png"
  ],
  "commands": [
    "npm run design:check",
    "npm run typecheck",
    "npm run build"
  ],
  "checks": {
    "keyboard": { "status": "passed", "notes": "Refresh and pagination remain native buttons with disabled states." },
    "reduced_motion": { "status": "passed", "notes": "No animation or continuous motion was added." }
  },
  "residual_risks": [
    "Static review board only; final administrator browser screenshot and production acceptance remain required after deployment."
  ]
}
-->

## Scope

- Route: `/admin/dashboard`.
- Role: administrator.
- Languages and themes: Chinese light/dark and English light copy are covered by locale keys and semantic dashboard classes.

## Baseline

- Current behavior: the administrator dashboard shows usage totals, charts, rankings and quick actions, but has no direct surcharge income summary or detail list.
- Baseline screenshot or recording: `baseline-admin-dashboard-usage-cards.png` is a static board that records the existing card/table density boundary.
- Inconsistencies observed: surcharge money exists in usage rows but requires indirect inspection outside the main dashboard.

## Prototype

- Prototype design image: `prototype-admin-dashboard-surcharge-module.png`.
- Approval status: the user requested an administrator-visible surcharge income module while keeping user-facing surcharge details hidden.
- Scope boundary: add one operational card before quick actions; do not change the dashboard page shell, chart layout, navigation or user dashboard.

## Reuse Decision

- Reused the existing dashboard card, table, icon, button, loading spinner and pagination button styles.
- No new shared component or visual system was introduced.
- No design-system exception is required.

## State Coverage

- Default: summary metrics show today, selected range, total surcharge and row count.
- Loading: table area uses the existing `LoadingSpinner`.
- Empty: detail table shows localized empty copy.
- Error: request errors clear the surcharge report without blocking the rest of the dashboard.
- Disabled: pagination and refresh buttons use native disabled state while loading or at bounds.
- Hover and focus-visible: native button/table row behaviors follow existing dashboard classes.

## Viewport Coverage

- Mobile: summary cards collapse to a two-column grid and details remain horizontally scrollable.
- Tablet: card grid and table preserve the existing dashboard spacing.
- Desktop: summary cards use four columns and the table fits the workspace card.
- Wide or short screen: no page-level width, centering, height or scrolling ownership was added.
- 200% zoom and reduced motion: no viewport-scaled typography or motion was added.

## Evidence

- Updated screenshot or recording: `updated-admin-dashboard-surcharge-module.png` static review board.
- Automated visual or overlap checks: design governance validates changed files and static artifacts.
- Commands run: `npm run design:check`, `npm run typecheck`, `npm run build`.

## Residual Risk

- Static review boards are not live browser captures and do not prove authenticated production data.
- Follow-up owner: release owner should verify the surcharge card and details in the user's local administrator browser after Zeabur deployment.
