# Visual Review: account-ops-notifications

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/router/index.ts",
    "frontend/src/i18n/index.ts",
    "frontend/src/i18n/routeScopes.ts",
    "frontend/src/i18n/locales/en/accountOps.ts",
    "frontend/src/i18n/locales/zh/accountOps.ts",
    "frontend/src/stores/accountOps.ts",
    "frontend/src/api/admin/accountOps.ts",
    "frontend/src/components/admin/operations/AccountOpsAddChannelDialog.vue",
    "frontend/src/components/admin/operations/AccountOpsBatchRuleDialog.vue",
    "frontend/src/components/admin/operations/AccountOpsChannelFields.vue",
    "frontend/src/components/admin/operations/AccountOpsDeliveryDetails.vue",
    "frontend/src/components/admin/operations/AccountOpsEmailDialog.vue",
    "frontend/src/components/admin/operations/AccountOpsGlobalSettings.vue",
    "frontend/src/components/admin/operations/AccountOpsGroupDialog.vue",
    "frontend/src/components/admin/operations/AccountOpsRuleDialog.vue",
    "frontend/src/components/admin/operations/AccountOpsRuleList.vue",
    "frontend/src/components/admin/operations/AccountOpsWebhookDialog.vue",
    "frontend/src/components/admin/operations/SmartOpsNav.vue",
    "frontend/src/views/admin/AccountOpsView.vue",
    "frontend/src/views/admin/__tests__/AccountOpsView.spec.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en/core.ts",
    "frontend/src/i18n/locales/zh/core.ts"
  ],
  "routes_or_surfaces": [
    "/admin/account-ops notification channels, account rules and delivery records",
    "/admin/smart-ops sidebar entry"
  ],
  "languages_and_themes": [
    "zh-CN/light and dark candidate build",
    "en-US/light candidate build"
  ],
  "states": [
    "rules and channels",
    "batch rule dialog",
    "delivery history",
    "loading",
    "disabled",
    "empty",
    "validation error",
    "delivery failure",
    "saved",
    "test delivery"
  ],
  "viewports": [
    "1120x1015",
    "1440x1100",
    "360x800",
    "768x1024",
    "1280x820"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/account-ops-notifications/prototype.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/account-ops-notifications/baseline.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/account-ops-notifications/actual-360-dark.png",
    "docs/visual-reviews/assets/account-ops-notifications/actual-1280-en-light.png"
  ],
  "commands": [
    "pnpm exec vitest run src/views/admin/__tests__/AccountOpsView.spec.ts src/components/admin/operations/__tests__/AccountOps*.spec.ts",
    "pnpm typecheck",
    "pnpm design:check",
    "Playwright browser verification of the production bundle with fixture API responses"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "Component tests cover tab navigation and dialog keyboard interactions."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "No continuous motion remains on the new route."
    }
  },
  "residual_risks": [
    "The baseline and prototype are upstream captures; the updated captures render this candidate bundle with fixture API responses.",
    "Production user/admin role and delivery acceptance remain pending."
  ]
}
-->

## Scope

The change adds `/admin/account-ops` for administrators. It manages account balance and quota rules, groups, notification channels, and delivery records. The sidebar links to it under Smart Operations. It reuses `AppLayout`, `BaseDialog`, `Toggle`, `Icon`, existing account APIs, and the current locale-loading mechanism.

## Baseline

The baseline showed only request-failure reminders, with no persisted balance or OAuth quota rules, batch editing, generic HTTPS webhook, confirmed recovery, or channel delivery history. `baseline.png` is the upstream feature author's browser capture before that feature, not a capture of this fork's rendered page.

## Prototype

`prototype.png` is the upstream account rules and channel page from commit `126240eac`. It uses synthetic account names and balances. The source image was inspected before adapting the page to this fork's current card radius, route frame, and motion rules.

## Reuse Decision

The route uses the existing workspace frame and shared dialog, toggle, icon, and admin navigation patterns. It introduces no new palette or page shell. Operational repeated sections use the governed 8px card radius. Refresh has no continuous animation.

## State Coverage

- Default: rules/channels tab and delivery-history tab.
- Hover and active: shared link, action, and tab behavior.
- Focus-visible and keyboard: native buttons, form fields, tab controls, and dialogs; component tests cover tab and dialog interactions.
- Loading: initial requests, refresh, saves, and webhook test action.
- Disabled: global notifications off, incomplete forms, and missing encryption setup.
- Empty: no accounts, no rules, and no delivery records.
- Error: field validation, API errors, and per-channel delivery failure/retry.
- Success: saved configuration, test delivery, alert, and recovery records.

## Viewport Coverage

The reference screenshots cover the 1120px and 1440px source layouts. The candidate's built route was checked at 360px (Chinese, dark), 768px (Chinese, light), and 1280px (English, light), with no page-width overflow. The test opened the delivery history via the keyboard and checked the empty state. 200% zoom was not checked.

## Evidence

The baseline and prototype are browser screenshots from the upstream feature history. The updated images show the actual candidate bundle after adjusting the dark-mode button contrast and shortening the English header. The browser used fixture data. No real email or webhook was sent.

## Residual Risk

Production acceptance remains outstanding. The candidate browser check used fixture data; delivery destinations are configured through admin settings.
