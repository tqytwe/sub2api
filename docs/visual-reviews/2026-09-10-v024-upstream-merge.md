# Visual Review: v0.2.4 Upstream Merge and Gemini Capability Gate

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/account/AccountUsageCell.vue",
    "frontend/src/components/account/CnBaseUrlPresets.vue",
    "frontend/src/components/account/CreateAccountModal.vue",
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/components/account/ModelWhitelistSelector.vue",
    "frontend/src/components/account/UsageProgressBar.vue",
    "frontend/src/components/account/credentialsBuilder.ts",
    "frontend/src/components/admin/account/AccountActionMenu.vue",
    "frontend/src/components/admin/monitor/MonitorFiltersBar.vue",
    "frontend/src/components/admin/monitor/MonitorFormDialog.vue",
    "frontend/src/components/admin/monitor/MonitorTemplateManagerDialog.vue",
    "frontend/src/components/common/GroupBadge.vue",
    "frontend/src/components/common/GroupSelector.vue",
    "frontend/src/components/common/PlatformIcon.vue",
    "frontend/src/components/common/PlatformTypeBadge.vue",
    "frontend/src/components/keys/UseKeyModal.vue",
    "frontend/src/components/user/PlatformUsageBreakdown.vue",
    "frontend/src/components/user/monitor/ProviderIcon.vue",
    "frontend/src/composables/useChannelMonitorFormat.ts",
    "frontend/src/features/channel-monitor-v2/MetricCell.vue",
    "frontend/src/features/channel-monitor-v2/MonitorSettingsPanel.vue",
    "frontend/src/features/channel-monitor-v2/monitorFormat.ts",
    "frontend/src/i18n/locales/en/admin/accounts.ts",
    "frontend/src/i18n/locales/en/admin/ops.ts",
    "frontend/src/i18n/locales/en/admin/overview.ts",
    "frontend/src/i18n/locales/en/admin/settings.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/zh/admin/accounts.ts",
    "frontend/src/i18n/locales/zh/admin/ops.ts",
    "frontend/src/i18n/locales/zh/admin/overview.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/dashboard.ts",
    "frontend/src/i18n/routeScopes.ts",
    "frontend/src/utils/platformColors.ts",
    "frontend/src/views/admin/AccountsView.vue",
    "frontend/src/views/admin/AnnouncementsView.vue",
    "frontend/src/views/admin/ChannelsView.vue",
    "frontend/src/views/admin/GroupsView.vue",
    "frontend/src/views/admin/ProxiesView.vue",
    "frontend/src/views/admin/SettingsView.vue",
    "frontend/src/views/admin/ops/components/OpsSystemLogTable.vue",
    "frontend/src/views/auth/LoginView.vue",
    "frontend/src/views/user/BatchImageGuideView.vue",
    "frontend/src/views/user/ChannelStatusV2View.vue",
    "frontend/src/views/user/CustomPageView.vue",
    "frontend/src/views/user/PaymentView.vue"
  ],
  "routes_or_surfaces": ["/admin/accounts", "/admin/announcements", "/admin/channels", "/admin/groups", "/admin/proxies", "/admin/settings", "/admin/ops", "/batch-images", "/channel-status-v2", "/purchase", "login", "API key modal"],
  "languages_and_themes": ["zh-CN/light static review board", "en-US/light static review board", "dark-token static code review"],
  "states": ["default", "hover", "focus-visible", "loading", "disabled", "empty", "error", "success", "long model identifier"],
  "viewports": ["360x800", "768x1024", "1280x820"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png", "docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png", "docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"],
  "commands": ["pnpm --dir frontend vitest run src/composables/__tests__/useBatchImageAccess.spec.ts", "pnpm --dir frontend typecheck", "pnpm --dir frontend lint:check", "pnpm --dir frontend design:check"],
  "checks": {
    "keyboard": {"status": "passed", "notes": "The account action menu retains the existing focusable button sequence; group and batch-image controls use the existing native, Toggle and Select focus-visible contracts."},
    "reduced_motion": {"status": "passed", "notes": "The synchronized channel monitor uses the existing LoadingSpinner for indeterminate work and no longer adds continuous spin or pulse classes to its operational data panels."}
  },
  "residual_risks": ["This static review board is non-production evidence and does not replace a browser capture. Final acceptance must inspect authenticated Chinese and English user/admin paths at 360, 768 and 1280 pixels in light and dark themes, including long model names and protected actions."]
}
-->

## Scope

- This review covers the visible v0.2.4 upstream synchronization surfaces and
  the Gemini batch-image eligibility gate listed in the manifest.
- It preserves the existing operational layouts, forms, tables, dialogs,
  localized labels and role boundaries. It does not authorize a visual redesign
  or a production deployment.

## Baseline

- The retained v0.2.0 review boards are the closest approved compact
  administration and user-workspace compositions. The synchronized changes add
  provider metadata, monitor data and configuration details to those existing
  surfaces.
- The upstream monitor view introduced oversized card radii, decorative
  gradients and continuous animation that conflicted with the Fork's
  operational UI policy. Those styles were reduced to the established compact
  card and semantic-status treatment.

## Prototype

- The decoded 390px and 1280px static boards named in the manifest are the
  approved layout boundary reused for this synchronization.
- No page shell, marketing card system, arbitrary external link or custom
  button family is introduced. Gemini access eligibility only changes which
  existing keys can be selected; it does not add a new user-facing visual
  pattern.

## Reuse Decision

- The merge reuses `AppLayout`, `TablePageLayout`, `Dialog`, `Toggle`,
  `Select`, `Icon`, `LoadingSpinner`, existing account modals and shared table
  controls.
- MiniMax uses a provider logo and centralized platform accent token. The
  action menu remains a transient 12px overlay, while normal operational cards
  use the established compact radius.
- Monitor health bands reuse the shared `--monitor-health-*` and status CSS
  tokens rather than file-local color literals.

## State Coverage

- Default and success: model, account, channel and settings updates retain
  their existing localized confirmation and table structures.
- Loading and empty: the monitor uses the shared spinner or static placeholders
  without a continuous pulse; unavailable Gemini keys are excluded before task
  creation.
- Error and disabled: back-end batch-image enforcement remains authoritative;
  existing localized API errors, disabled controls and account-validation paths
  remain unchanged.
- Focus-visible and keyboard: existing native controls, menus and dialogs keep
  their focus order and visible focus styling.

## Viewport Coverage

- Mobile: 360px preserves compact stacking for account dialogs, key selection
  and user monitor filters.
- Tablet: 768px remains the required final browser verification point for
  navigation, tables and action menus.
- Desktop: 1280px preserves dense administration tables and monitor panes.
- Long localized strings, 200 percent zoom and dark mode remain part of final
  browser acceptance.

## Evidence

- The manifest references real decoded static review boards. They are
  development evidence, not authenticated browser screenshots.
- The Gemini composable test exercises the formerly mismatched ordinary-image
  and batch-image capability combination. Type, lint and design gates are
  listed as required non-production checks.
- The locale review also covers the MiniMax key-configuration copy and the
  shared account-usage labels on cold Channel Monitor visits. The full-locale
  audit and route-scope test prove both Chinese and English fragments load
  before their associated components render.

## Residual Risk

- Static evidence cannot prove the production API key, subscription or account
  state selected by a real mobile client. Before release, perform local-browser
  acceptance for the administrator and user routes and correlate any APP chat
  failure with a redacted request ID and stable error code.
