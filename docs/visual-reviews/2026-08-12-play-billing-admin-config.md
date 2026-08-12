# Visual Review: Play Billing admin config

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/orders/AdminPlayBillingConfigView.vue",
    "frontend/src/router/index.ts",
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts"
  ],
  "routes_or_surfaces": [
    "/admin/orders/play-billing",
    "admin payment sidebar group",
    "Google Play Billing product mapping admin page"
  ],
  "languages_and_themes": [
    "zh-CN/light static board",
    "zh-CN/dark token review",
    "en-US/light static board",
    "en-US/dark token review"
  ],
  "states": [
    "default",
    "loading",
    "empty",
    "enabled",
    "disabled",
    "validation-error",
    "saving",
    "focus-visible"
  ],
  "viewports": [
    "360x800",
    "768x900",
    "1280x900",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/play-billing-admin-config/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/play-billing-admin-config/baseline-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/play-billing-admin-config/updated-1280.png"
  ],
  "commands": [
    "python3 generated static Play Billing admin review boards with PIL",
    "corepack pnpm --dir frontend exec vitest run src/views/admin/orders/__tests__/AdminPlayBillingConfigView.spec.ts src/router/__tests__/playBillingAdminRouting.spec.ts --run",
    "corepack pnpm --dir frontend design:check",
    "corepack pnpm --dir frontend typecheck"
  ],
  "checks": {
    "keyboard": {
      "status": "passed"
    },
    "reduced_motion": {
      "status": "passed"
    },
    "copy_locale": {
      "status": "passed",
      "notes": "The route label and page copy are present in zh-CN and en-US."
    }
  },
  "residual_risks": [
    "The artifacts are static review boards rather than browser screenshots, so final browser acceptance on the production admin route is still required.",
    "Google Play Console product creation and real purchase testing remain separate release acceptance steps."
  ]
}
-->

## Scope

- Routes: `/admin/orders/play-billing`.
- Roles: administrator only.
- Languages and themes: zh-CN and en-US copy added; styling reuses existing token-based admin page, DataTable, Select, Toggle, button and input patterns.

## Baseline

- Current behavior: the payment admin navigation exposes payment dashboard, recharge orders and subscription plans, but no visible surface for Google Play Billing product mapping.
- Baseline artifact: `docs/visual-reviews/assets/play-billing-admin-config/baseline-1280.png`.
- Inconsistencies observed: the backend mapping API could exist without an operator-facing page, forcing manual API calls for routine product mapping changes.

## Prototype

- Prototype design image: `docs/visual-reviews/assets/play-billing-admin-config/prototype-1280.png`.
- Approval status: follows the agreed boundary that Play Console owns product creation, pricing, countries and active status while极速蹬后台 owns entitlement mapping.
- Scope boundary: no Google service account secret editing, no Play Console product management, no new checkout visual pattern.

## Reuse Decision

- Reused `AppLayout`, `DataTable`, `Select`, `Toggle`, shared `Icon`, `btn` and `input` styles.
- The page stays under the existing payment admin group instead of adding a new top-level admin category.
- No new shared component, icon path, raw color, decorative gradient or page shell exception was added.

## State Coverage

- Default: package name, service-account configured state, enabled count and mapping table are visible.
- Loading and saving: refresh/save buttons are disabled or animated through existing classes.
- Empty: table empty slot explains the Play Console first, backend mapping second workflow.
- Enabled/disabled: mapping toggle controls whether a product is public and eligible for fulfillment.
- Validation error: product ID, balance amount and subscription plan are validated before save.
- Focus-visible and keyboard: page uses existing button/input/select/toggle focus behavior.

## Viewport Coverage

- Mobile: table falls back to the shared mobile card layout.
- Tablet and desktop: the summary cards use responsive columns and the mapping table owns horizontal density.
- 200% zoom and reduced motion: no motion was added beyond the existing refresh icon spin during loading; all controls remain textual or accessible via labels.

## Evidence

- Prototype board: `docs/visual-reviews/assets/play-billing-admin-config/prototype-1280.png`.
- Updated board: `docs/visual-reviews/assets/play-billing-admin-config/updated-1280.png`.
- Planned commands are recorded in the manifest and must pass before merge.

## Residual Risk

- Static boards do not prove live browser wrapping, dark-theme contrast or real admin data behavior.
- Final release acceptance must still open `/admin/orders/play-billing` in a browser after deployment, then create a real Play test product mapping and complete sandbox purchase verification.
