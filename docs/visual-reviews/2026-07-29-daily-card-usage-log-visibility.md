# Visual Review: daily-card usage log visibility

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/UsageView.vue",
    "frontend/src/components/admin/usage/UsageTable.vue",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts"
  ],
  "routes_or_surfaces": [
    "/admin/usage"
  ],
  "languages_and_themes": [
    "zh-CN light",
    "zh-CN dark",
    "en-US light",
    "en-US dark"
  ],
  "states": [
    "usage table with daily-card entitlement",
    "usage table without daily-card entitlement",
    "loading table",
    "empty usage table",
    "mobile horizontal table scroll"
  ],
  "viewports": [
    "390x844",
    "1280x860",
    "1600x900"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/daily-card-admin-reset-routing/prototype-daily-card-admin-reset-routing.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/admin-dashboard-billing-surcharge/baseline-admin-dashboard-usage-cards.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/daily-card-admin-reset-routing/updated-daily-card-admin-reset-routing.png"
  ],
  "commands": [
    "pnpm --dir frontend run typecheck",
    "pnpm --dir frontend run lint",
    "pnpm --dir frontend vitest run src/components/admin/usage/__tests__/UsageTable.spec.ts src/views/admin/__tests__/UsageView.spec.ts"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "notes": "The change only adds a read-only table column and reuses the existing table keyboard path."
    },
    "reduced_motion": {
      "status": "passed",
      "notes": "No animation, transition, or motion behavior was introduced."
    }
  },
  "residual_risks": [
    "Static evidence is reused from the existing admin review boards because this environment does not have an authenticated browser session.",
    "Production acceptance should spot-check an admin usage row after migration 233 has populated subscription_entitlement_id."
  ]
}
-->

## Scope

- Routes: `/admin/usage`.
- Roles: administrators reviewing billing and request consumption records.
- Languages and themes: Chinese and English labels in the existing light and dark admin usage table.

## Baseline

- Current behavior: daily-card usage records do not expose the entitlement identity in the admin usage table.
- Baseline screenshot or recording: `docs/visual-reviews/assets/admin-dashboard-billing-surcharge/baseline-admin-dashboard-usage-cards.png`.
- Inconsistencies observed: administrators can see normal cost columns, but cannot distinguish which daily-card entitlement a request settled against.

## Prototype

- Prototype design image: `docs/visual-reviews/assets/daily-card-admin-reset-routing/prototype-daily-card-admin-reset-routing.png`.
- Approval status: follows the existing dense admin table style.
- Scope boundary: add one read-only daily-card column only; no storefront, page shell, filter, pagination, or modal redesign.

## Reuse Decision

- Shared layouts and components reused: existing admin table, responsive scroll container, tag text style, loading state, empty state, and locale helpers.
- New shared pattern, if any: none.
- Design-system exception, if any: none.

## State Coverage

- Default: rows with `subscription_entitlement_id` show a compact daily-card identifier.
- Hover and active: unchanged because the new value is read-only text.
- Focus-visible and keyboard: table navigation and controls are unchanged.
- Loading, disabled, empty, error and success: existing usage table states remain unchanged.

## Viewport Coverage

- Mobile: the column participates in the existing horizontal table scroll.
- Tablet: no new wrapping or stacked layout is introduced.
- Desktop: the column stays in the existing table track and uses compact text.
- Wide or short screen: unchanged page frame behavior.
- 200% zoom and reduced motion: no motion or viewport-scaled typography was added.

## Evidence

- Updated screenshot or recording: `docs/visual-reviews/assets/daily-card-admin-reset-routing/updated-daily-card-admin-reset-routing.png`.
- Automated visual or overlap checks: design governance validates changed-file coverage and artifact integrity; Vitest validates the new column rendering.
- Commands run: `pnpm --dir frontend run typecheck`, `pnpm --dir frontend run lint`, and focused usage table tests.

## Residual Risk

- Known limitations: static review-board evidence is reused; an authenticated production browser check should confirm a real row after deployment.
- Follow-up owner: release verifier.
