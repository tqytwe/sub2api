# Visual Review: admin-fund-order-navigation

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts"
  ],
  "routes_or_surfaces": [
    "/admin/funds expanded sidebar group",
    "/admin/orders expanded sidebar group",
    "/admin/funds/refunds tab title and empty state"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "zh-CN/dark",
    "en-US/light",
    "en-US/dark"
  ],
  "states": [
    "fund management expanded",
    "order management expanded",
    "active balance return item",
    "active recharge order item",
    "empty balance return queue"
  ],
  "viewports": [
    "360x800",
    "1280x820"
  ],
  "artifact_mode": "static-review-board",
  "baseline_artifacts": [
    "docs/visual-reviews/assets/admin-fund-order-navigation/before-admin-fund-order-navigation.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/admin-fund-order-navigation/after-admin-fund-order-navigation.png"
  ],
  "commands": [
    "python3 static PNG artifact generation for admin fund and order sidebar review",
    "pnpm install --frozen-lockfile",
    "pnpm exec vitest run src/router/__tests__/fundManagementRouting.spec.ts src/router/__tests__/withdrawalRouting.spec.ts src/views/admin/__tests__/AdminFundsView.stepup.spec.ts",
    "pnpm exec vitest run src/i18n/__tests__/localesMessageCompile.spec.ts src/i18n/__tests__/adminManagementLocaleKeys.spec.ts",
    "pnpm exec vue-tsc --noEmit"
  ],
  "checks": {
    "keyboard": {
      "status": "not-applicable",
      "reason": "This change only moves existing sidebar links and renames labels; focusable controls remain the same router links."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "No animation or motion behavior was added or changed."
    }
  },
  "residual_risks": [
    "Static review boards do not replace the final administrator browser check after deployment.",
    "The production sidebar still needs manual zh/en and light/dark confirmation after the deploy switches assets."
  ]
}
-->

## Scope

- Routes: `/admin/funds`, `/admin/funds/refunds`, and `/admin/orders`.
- Roles: administrator.
- Languages and themes: Chinese and English labels were reviewed for both light and dark sidebar usage.

## Baseline

- Current behavior: `充值订单 / Recharge Orders` appeared under Fund Management while `/admin/orders` also appeared inside Order Management, creating two visible entries for the same route.
- Baseline screenshot or recording: `before-admin-fund-order-navigation.png`.
- Inconsistencies observed: `退款申请 / Refund Requests` made wallet balance returns look like payment-order refunds, which was misleading next to the payment order refund actions.

## Reuse Decision

- Shared layouts and components reused: existing `AppSidebar.vue`, route definitions, and locale files.
- New shared pattern, if any: none.
- Design-system exception, if any: none.

## State Coverage

- Default: Fund Management now lists balance return requests, reward withdrawals, gift balance, and historical gift review.
- Hover and active: no hover, active, or focus class was changed; the same sidebar link component continues to own those states.
- Focus-visible and keyboard: the changed items remain router links inside the existing sidebar navigation.
- Loading, disabled, empty, error and success: no loading or disabled state changed; the balance return empty state copy changed in both languages.

## Viewport Coverage

- Mobile: the change is label and grouping only; sidebar collapse behavior is unchanged.
- Tablet: grouping remains under the existing sidebar layout.
- Desktop: static review board confirms Recharge Orders is no longer duplicated in Fund Management and appears under Order Management.
- Wide or short screen: no page width, height, or scrolling behavior changed.
- 200% zoom and reduced motion: no motion or layout sizing rule changed in this patch.

## Evidence

- Updated screenshot or recording: `after-admin-fund-order-navigation.png`.
- Automated visual or overlap checks: design-governance manifest validates changed visible files and artifact media signatures.
- Commands run: static PNG artifact generation, targeted Vitest, i18n compile checks, and TypeScript check listed in the manifest.

## Residual Risk

- Known limitations: this is a static review board rather than a live browser screenshot; final local browser acceptance should confirm the deployed sidebar.
- Follow-up owner: release owner should verify `/admin/funds` and `/admin/orders` after deployment in Chinese and English.
