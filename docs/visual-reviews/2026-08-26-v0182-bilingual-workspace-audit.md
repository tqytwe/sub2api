# Visual Review: v0182-bilingual-workspace-audit

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/account/CreateAccountModal.vue",
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/components/admin/account/ReAuthAccountModal.vue",
    "frontend/src/composables/useAccountOAuth.ts",
    "frontend/src/components/admin/account/ScheduledTestsPanel.vue",
    "frontend/src/components/admin/channel/ModelTagInput.vue",
    "frontend/src/components/admin/usage/UsageCleanupDialog.vue",
    "frontend/src/components/auth/WechatOAuthSection.vue",
    "frontend/src/components/coupon/CouponWalletPanel.vue",
    "frontend/src/components/home/ChannelTV.vue",
    "frontend/src/components/imageStudio/ImageStudioGallery.vue",
    "frontend/src/components/modelPlaza/PlazaModelPricingTable.vue",
    "frontend/src/components/payment/OrderStatusBadge.vue",
    "frontend/src/components/payment/SubscriptionPlanCard.vue",
    "frontend/src/components/payment/SubscriptionPlanDecisionShelf.vue",
    "frontend/src/components/user/PlatformUsageBreakdown.vue",
    "frontend/src/components/user/UserErrorDetailModal.vue",
    "frontend/src/components/user/UserErrorRequestsTable.vue",
    "frontend/src/components/user/dashboard/UserDashboardStats.vue",
    "frontend/src/features/channel-monitor-v2/MonitorSettingsPanel.vue",
    "frontend/src/features/ip-risk/IPRiskActionDialog.vue",
    "frontend/src/features/ip-risk/IPRiskActionsView.vue",
    "frontend/src/features/ip-risk/IPRiskCaseDetail.vue",
    "frontend/src/features/ip-risk/IPRiskWorkbench.vue",
    "frontend/src/features/prompt-audit/components/RuntimeOverview.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/admin/accounts.ts",
    "frontend/src/i18n/locales/en/admin/resources.ts",
    "frontend/src/i18n/locales/en/batchImage.ts",
    "frontend/src/i18n/locales/en/common.ts",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/en/legacy/admin-channels.ts",
    "frontend/src/i18n/locales/en/misc.ts",
    "frontend/src/i18n/locales/en/userUsage.ts",
    "frontend/src/i18n/locales/jisudeng-home.en.ts",
    "frontend/src/i18n/locales/jisudeng-home.zh.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/admin/accounts.ts",
    "frontend/src/i18n/locales/zh/admin/resources.ts",
    "frontend/src/i18n/locales/zh/batchImage.ts",
    "frontend/src/i18n/locales/zh/common.ts",
    "frontend/src/i18n/locales/zh/dashboard.ts",
    "frontend/src/i18n/locales/zh/legacy/admin-channels.ts",
    "frontend/src/i18n/locales/zh/misc.ts",
    "frontend/src/i18n/locales/zh/userUsage.ts",
    "frontend/src/views/admin/ChannelsView.vue",
    "frontend/src/views/admin/ModelCatalogView.vue",
    "frontend/src/views/admin/AnnouncementsView.vue",
    "frontend/src/views/admin/RedeemView.vue",
    "frontend/src/views/admin/SettingsView.vue",
    "frontend/src/views/admin/SubscriptionsView.vue",
    "frontend/src/views/admin/UsageView.vue",
    "frontend/src/views/admin/orders/AdminPaymentPlansView.vue",
    "frontend/src/views/auth/WechatCallbackView.vue",
    "frontend/src/views/public/AgentTeamView.vue",
    "frontend/src/views/user/AvailableChannelsView.vue",
    "frontend/src/views/user/BatchImageGuideView.vue",
    "frontend/src/views/user/CustomPageView.vue",
    "frontend/src/views/user/KeysView.vue",
    "frontend/src/views/user/PaymentView.vue",
    "frontend/src/views/user/SpeedTestView.vue",
    "frontend/src/views/user/SubscriptionsView.vue",
    "frontend/src/views/user/UsageView.vue"
  ],
  "routes_or_surfaces": ["/admin/accounts", "/admin/channels", "/admin/model-plaza", "/admin/announcements", "/admin/redeem", "/admin/settings", "/admin/subscriptions", "/admin/usage", "/admin/proxies/risk", "/admin/proxies/actions", "/admin/prompt-audit", "/channels", "/usage", "/keys", "/keys/speed-test", "/subscriptions", "/batch-image-guide", "custom user pages", "Image Studio", "coupon wallet", "channel monitor", "home channel strip", "public agent team", "IP risk workbench and action dialog"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "focus-visible", "loading", "disabled", "empty", "error", "success"],
  "viewports": ["360x800", "768x900", "1280x800", "1920x1080"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/v0182-bilingual-workspace-audit/prototype-1440.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/v0182-bilingual-workspace-audit/baseline-1440.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/v0182-bilingual-workspace-audit/updated-1440.png"],
  "commands": [
    "cd frontend && pnpm exec playwright screenshot --viewport-size=1440,980 file:///home/codex/worktrees/sub2api-media-integration-20260826/docs/visual-reviews/assets/v0182-bilingual-workspace-audit/static-review-board.html?mode=baseline ../docs/visual-reviews/assets/v0182-bilingual-workspace-audit/baseline-1440.png",
    "cd frontend && pnpm exec playwright screenshot --viewport-size=1440,980 file:///home/codex/worktrees/sub2api-media-integration-20260826/docs/visual-reviews/assets/v0182-bilingual-workspace-audit/static-review-board.html?mode=prototype ../docs/visual-reviews/assets/v0182-bilingual-workspace-audit/prototype-1440.png",
    "cd frontend && pnpm exec playwright screenshot --viewport-size=1440,980 file:///home/codex/worktrees/sub2api-media-integration-20260826/docs/visual-reviews/assets/v0182-bilingual-workspace-audit/static-review-board.html?mode=updated ../docs/visual-reviews/assets/v0182-bilingual-workspace-audit/updated-1440.png",
    "cd frontend && pnpm exec vitest run src/i18n/__tests__/systemMessageLocalization.spec.ts src/i18n/__tests__/literalTranslationCoverage.spec.ts src/i18n/__tests__/routeLocaleRuntime.spec.ts",
    "cd frontend && pnpm design:check"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "The delivery retains existing semantic controls, dialogs, tables, and focus behavior; it only replaces system-owned display text and error fallbacks."},
    "reduced_motion": {"status": "not-applicable", "reason": "No animation, transition, or continuous motion is added."}
  },
  "residual_risks": ["The PNGs are static review boards rather than authenticated browser captures. Final browser acceptance must cover guest, user, and administrator flows in both languages, themes, and required viewports before deployment."]
}
-->

## Scope

- Surfaces: account create/edit/reauthorization errors including generic OAuth authorization URL, code exchange, and cookie fallback states, scheduled test feedback, channel model mapping controls, model-catalog media labels, Image Studio and batch-image guidance, coupon, payment, usage, speed-test, user-subscription, and prompt-audit runtime status labels, use records, available channel empty states, key/user errors, custom user-page failures, public agent-team errors, home channel-strip labels, channel-monitor and IP-risk feedback, and administrator announcement, redeem, settings, and subscription states.
- Boundary: model IDs, API paths, configured group names, configured activity names, and provider-provided values remain original source data. Only system-owned text is localized.

## Baseline

- Some fallback strings and newer server enum values could render in a language different from the route locale, or expose an untranslated key. The user usage page could also cold-load before its shared `admin.*` labels were available.
- `assets/v0182-bilingual-workspace-audit/baseline-1440.png` records the prior mixed-copy risk as a static review board.

## Prototype

- `assets/v0182-bilingual-workspace-audit/prototype-1440.png` records the approved separation: the same system status and retry state is entirely Chinese on Chinese routes and entirely English on English routes.
- Existing `BaseDialog`, tables, selects, badges, and page frames are retained. The change introduces no new page width, card, color, or interaction pattern.

## Reuse Decision

- Reused: Vue I18n route fragments, `localizedEnumOrUnknown`, existing account dialogs, `DataTable`, channel mapping controls, and user workspace page frames.
- The locale route guard remains the single language authority: unprefixed routes resolve Chinese; `/en/*` and `?lang=en` resolve English.

## State Coverage

- Default and success: known values retain localized labels.
- Loading and disabled: existing request states stay unchanged.
- Empty and error: missing models, no mapping rules, unavailable channels, coupon and batch-image guidance, channel-monitor, user-subscription, prompt-audit runtime, and IP-risk action outcomes, and unknown server enums use active-locale copy instead of raw keys or a second language.
- Focus-visible and keyboard: no control ownership or focus ordering changes.

## Viewport Coverage

- 360px and 768px: short localized error/status values remain inside existing controls.
- 1280px and 1920px: existing workspace tables and page frames remain unchanged.
- Light/dark, 200% zoom, and authenticated runtime data remain browser-acceptance gates.

## Evidence

- The baseline, prototype, and updated PNGs are Playwright-rendered static review boards with separately visible Chinese and English system copy.
- Route-locale, literal-key, message-compilation, account-model retry, raw-key fallback, and user usage cold-route tests are run in the release gate.

## Residual Risk

- Static board evidence cannot prove production chunk timing or user-configured provider values. Final browser screenshots and user acceptance remain required before deployment.
