# Visual Review: payment-shelf-display

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/payment/SubscriptionPlanCard.vue",
    "frontend/src/components/payment/SubscriptionPlanDecisionShelf.vue",
    "frontend/src/views/admin/orders/AdminPaymentPlansView.vue",
    "frontend/src/views/admin/orders/PlanStorefrontConfigPanel.vue",
    "frontend/src/views/user/PaymentView.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts"
  ],
  "routes_or_surfaces": [
    "/purchase configurable subscription shelves",
    "/admin/payment/plans storefront config panel",
    "/purchase subscription confirmation quota summary",
    "/purchase subscription product detail quota cards"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "zh-CN/dark",
    "en-US/light",
    "en-US/dark"
  ],
  "states": [
    "custom shelf selected with 29.9 plan as default spotlight",
    "custom label chips visible on shelf and plan list",
    "admin adds and deletes shelves",
    "admin assigns 30 plans into selected shelf",
    "admin adds and deletes custom plan labels",
    "monthly quota plan with zero daily and weekly limits",
    "subscription confirmation after selecting a plan",
    "product detail modal quota section"
  ],
  "viewports": [
    "360x800",
    "768x1024",
    "1280x760"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/payment-shelf-display/before-payment-shelf-display.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/payment-shelf-display/after-payment-shelf-display.png",
    "docs/visual-reviews/assets/payment-shelf-display/after-payment-storefront-user-desktop.png",
    "docs/visual-reviews/assets/payment-shelf-display/after-payment-storefront-user-mobile.png",
    "docs/visual-reviews/assets/payment-shelf-display/after-payment-storefront-admin-config.png"
  ],
  "commands": [
    "python3 static PNG artifact generation for payment shelf review",
    "prototype HTML screenshots under /home/dell/.codex/visualizations/2026/07/22/019f88d2-dc19-72d3-ad7f-89fd05d46593/",
    "pnpm install --frozen-lockfile",
    "pnpm vitest run src/components/payment/__tests__/SubscriptionPlanCard.spec.ts src/components/payment/__tests__/SubscriptionPlanDecisionShelf.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/views/admin/orders/__tests__/PlanStorefrontConfigPanel.spec.ts",
    "go test -tags=unit ./internal/service -run 'TestPaymentStorefrontConfig|TestUpdatePaymentStorefrontConfig'",
    "go test -tags=unit ./internal/handler -run 'TestCheckout.*Storefront|TestCheckoutPlanJSONIncludesProductDisplayFields'",
    "pnpm typecheck",
    "pnpm design:check"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "Shelf tabs, plan list buttons, save actions, checkboxes, and selects keep visible labels or aria labels and do not rely on hover-only controls."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "The new shelf and admin config interactions use color and border transitions only; no continuous motion or layout-shifting animation was added."
    }
  },
  "residual_risks": [
    "Static review boards do not replace final rendered browser acceptance on the deployed purchase page.",
    "Prototype screenshots show the intended layout; production data still needs admin to place the 29.9/30 plan into the desired shelf and set it as the default spotlight.",
    "Final deployed browser acceptance still needs authenticated user and admin checks against live data."
  ]
}
-->

## Scope

- Routes: `/purchase` subscription tab, `/admin/payment/plans` storefront config panel, plan detail modal, and selected-plan confirmation.
- Roles: authenticated user and administrator.
- Languages and themes: Chinese and English labels were reviewed from the shared i18n keys, existing admin controls, and payment card structure for light and dark mode.

## Baseline

- Current behavior: subscription shelves were derived from hardcoded platform/category filters and old per-plan display fields, so users could enter the purchase page and not immediately see the requested 29.9/30 package.
- Baseline screenshot or recording: `before-payment-shelf-display.png`.
- Inconsistencies observed: stored zero quota values such as `daily_limit_usd=0` were rendered as `$0`, which made a monthly package look broken or unusable even when it had a valid monthly limit.

## Reuse Decision

- Shared layouts and components reused: existing `PaymentView.vue` payment flow, `SubscriptionPlanCard.vue`, platform color helpers, admin table/filter controls, `Icon.vue`, and existing payment detail modal structure.
- New shared pattern, if any: `SubscriptionPlanDecisionShelf.vue` is the focused purchase decision surface used after selecting a configured shelf.
- Design-system exception, if any: none.

## State Coverage

- Default: the first enabled configured shelf is selected; its configured `default_plan_id` becomes the left-side spotlight.
- Hover and active: shelf tabs, right-side plan options, admin rows, and save/add/delete actions use border/color feedback without layout movement.
- Focus-visible and keyboard: buttons, checkboxes, selects, and text inputs remain native focus targets; icon-only move/delete actions have aria labels.
- Loading: the admin config panel shows an inline spinner while loading and disables save while saving.
- Disabled, empty, error and success: empty shelf/tag states render inline text; API errors go through existing app toasts; save success keeps the updated config in place.

## Viewport Coverage

- Mobile: the purchase decision shelf stacks into a single-column flow and the top shelf tabs scroll horizontally.
- Tablet: shelf controls and plan list stay readable with fixed touch targets.
- Desktop: the purchase page uses a left spotlight plus a right scan list, which handles long plan lists better than a flat 30-card grid.
- Admin desktop: shelf/tag lists sit beside assignment checklists so operators can configure categories and labels without leaving the plans page.
- 200% zoom and reduced motion: no viewport-scaled typography, negative letter spacing, continuous motion, or hover-only operation was added.

## Evidence

- Updated screenshot or recording: `after-payment-shelf-display.png`, `after-payment-storefront-user-desktop.png`, `after-payment-storefront-user-mobile.png`, and `after-payment-storefront-admin-config.png`.
- Automated visual or overlap checks: design-governance manifest validates changed visible files and media artifact signatures.
- Commands run: prototype screenshot generation, targeted Go tests, targeted Vitest, TypeScript check, and design governance commands listed in the manifest.

## Residual Risk

- Known limitations: this is a static review board and prototype screenshot set rather than a live production-browser acceptance run.
- Follow-up owner: release owner should verify `/purchase` and `/admin/payment/plans` after deployment with live data, then confirm the 29.9/30 plan is assigned to the intended shelf and default spotlight.
