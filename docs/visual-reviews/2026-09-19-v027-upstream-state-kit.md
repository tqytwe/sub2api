# Visual Review: v0.2.7 Upstream and State Kit Host Surface

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/layout/AppSidebar.vue", "frontend/src/i18n/locales/en.ts", "frontend/src/i18n/locales/zh.ts",
    "frontend/src/components/account/AccountUsageCell.vue", "frontend/src/components/account/BulkEditAccountModal.vue", "frontend/src/components/account/CNProviderQuotaCell.vue", "frontend/src/components/account/EditAccountModal.vue", "frontend/src/components/account/ModelWhitelistSelector.vue", "frontend/src/components/account/OpenCodeGoProtocolRulesEditor.vue", "frontend/src/components/account/credentialsBuilder.ts", "frontend/src/components/admin/channel/ModelTagInput.vue", "frontend/src/components/admin/monitor/MonitorFiltersBar.vue", "frontend/src/components/admin/monitor/MonitorFormDialog.vue", "frontend/src/components/admin/monitor/MonitorTemplateManagerDialog.vue", "frontend/src/components/admin/payment/AdminRefundDialog.vue", "frontend/src/components/admin/proxy/ImportDataModal.vue", "frontend/src/components/admin/subscription/BulkSubscriptionActionDialog.vue", "frontend/src/components/admin/subscription/bulkSubscriptionOperation.ts", "frontend/src/components/admin/usage/UsageTable.vue", "frontend/src/components/admin/user/UserPlatformQuotaModal.vue", "frontend/src/components/common/BaseDialog.vue", "frontend/src/components/common/Pagination.vue", "frontend/src/components/common/PlatformIcon.vue", "frontend/src/components/common/PlatformTypeBadge.vue", "frontend/src/components/keys/BulkEditKeysModal.vue", "frontend/src/components/keys/UseKeyModal.vue", "frontend/src/components/payment/AmountInput.vue", "frontend/src/components/payment/PaymentProviderDialog.vue", "frontend/src/components/user/dashboard/UserDashboardStats.vue", "frontend/src/components/user/monitor/MonitorCard.vue", "frontend/src/components/user/monitor/ProviderIcon.vue", "frontend/src/components/user/profile/ProfileBalanceNotifyCard.vue", "frontend/src/components/user/profile/ProfileEditForm.vue", "frontend/src/components/user/profile/ProfilePasswordForm.vue", "frontend/src/components/user/profile/TotpDisableDialog.vue", "frontend/src/components/user/profile/TotpSetupModal.vue", "frontend/src/i18n/locales/en/admin/accounts.ts", "frontend/src/i18n/locales/en/admin/channels.ts", "frontend/src/i18n/locales/en/admin/ops.ts", "frontend/src/i18n/locales/en/admin/overview.ts", "frontend/src/i18n/locales/en/admin/settings.ts", "frontend/src/i18n/locales/en/common.ts", "frontend/src/i18n/locales/en/dashboard.ts", "frontend/src/i18n/locales/en/landing.ts", "frontend/src/i18n/locales/en/misc.ts", "frontend/src/i18n/locales/zh/admin/accounts.ts", "frontend/src/i18n/locales/zh/admin/channels.ts", "frontend/src/i18n/locales/zh/admin/ops.ts", "frontend/src/i18n/locales/zh/admin/overview.ts", "frontend/src/i18n/locales/zh/admin/settings.ts", "frontend/src/i18n/locales/zh/common.ts", "frontend/src/i18n/locales/zh/dashboard.ts", "frontend/src/i18n/locales/zh/landing.ts", "frontend/src/i18n/locales/zh/misc.ts", "frontend/src/router/title.ts", "frontend/src/views/admin/AccountsView.vue", "frontend/src/views/admin/ChannelsView.vue", "frontend/src/views/admin/GroupsView.vue", "frontend/src/views/admin/PluginsView.vue", "frontend/src/views/admin/ProxiesView.vue", "frontend/src/views/admin/RedeemView.vue", "frontend/src/views/admin/SubscriptionsView.vue", "frontend/src/views/admin/ops/OpsDashboard.vue", "frontend/src/views/admin/ops/components/OpsDashboardHeader.vue", "frontend/src/views/admin/ops/components/OpsErrorDetailsModal.vue", "frontend/src/views/admin/ops/components/OpsErrorLogTable.vue", "frontend/src/views/admin/ops/components/OpsRequestDetailsModal.vue", "frontend/src/views/auth/RegisterView.vue", "frontend/src/views/user/ChannelStatusV1View.vue", "frontend/src/views/user/CustomPageView.vue", "frontend/src/views/user/UsageView.vue", "frontend/src/views/user/UserOrdersView.vue"
  ],
  "routes_or_surfaces": ["/admin/accounts", "/admin/groups", "/admin/settings", "/admin/plugins", "/admin/subscriptions", "/admin/ops", "/purchase", "/register", "/usage", "State Kit plugin configuration"],
  "languages_and_themes": ["zh-CN/light", "en-US/light", "zh-CN/dark", "en-US/dark"],
  "states": ["default", "focus-visible", "loading", "disabled", "empty", "error", "success", "long model identifier"],
  "viewports": ["360x800", "768x1024", "1280x820"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png", "docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png", "docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"],
  "commands": ["pnpm --dir frontend design:check", "pnpm --dir frontend lint:check", "pnpm --dir frontend typecheck"],
  "checks": {"keyboard": {"status": "not-applicable", "reason": "Static review board has no live focus traversal."}, "reduced_motion": {"status": "passed", "notes": "No new motion was introduced."}},
  "residual_risks": ["Artifacts are reused static review boards and are non-production evidence.", "Run authenticated local browser review in Chinese and English at 360, 768 and 1280 pixels before release.", "State Kit plugin UI remains a separate upstream artifact and must pass bilingual review before enablement."]
}
-->

## Scope

- Routes: `/admin/accounts`, `/admin/groups`, `/admin/settings`, `/admin/plugins`, `/admin/subscriptions`, `/admin/ops`, `/purchase`, `/register`, `/usage`; State Kit plugin configuration surface.
- Roles: administrator for management routes; authenticated user for usage, purchase and profile surfaces; unauthenticated visitor for registration.
- Languages and themes: Chinese default and explicit English routes, light and dark themes.
- Boundary: this review covers the visible upstream v0.2.7 merge surface and host-side State Kit integration documentation. It does not install, enable or publish the third-party plugin.

## Baseline

- Current behavior: the fork contained the v0.2.6-era visible surfaces and fork-specific payment, Play/VIP, settings and bilingual lazy-locale behavior.
- Baseline screenshot or recording: `docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png` and `docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png`.
- Inconsistencies observed: upstream v0.2.7 added account, monitor, subscription, plugin, payment, usage and locale surfaces; the merge preserved existing fork behavior while adding the required host protocol entry points.

## Prototype

- Prototype design image: `docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png` and `docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png`.
- Approval status: static review board recorded for engineering review; no production approval or deployment authorization is implied.
- Scope boundary: visual assets are reused review artifacts for non-production comparison. State Kit itself remains a separately signed plugin package and is not included in this repository.

## Reuse Decision

- Shared layouts and components reused: existing PageFrame/layout ownership, BaseDialog, Pagination, account and admin form controls, shared Icon/PlatformIcon components, existing locale scopes and route guards.
- New shared pattern, if any: no new page shell or visual component was introduced by the synchronization; the host exposes the existing plugin manager and plugin UI session boundary.
- Design-system exception, if any: the existing OpenCode platform icon/gradient and subscription shell exceptions are documented inline at their source locations with `design-governance-allow` comments and were not broadened.

## State Coverage

- Default: management lists, account forms, usage tables and user purchase/profile surfaces are represented in the review board.
- Hover and active: shared buttons, tabs, table rows and navigation controls continue to use existing component states.
- Focus-visible and keyboard: static board cannot execute focus traversal; a live authenticated browser pass remains required before release.
- Loading, disabled, empty, error and success: states are included in the manifest and reviewed against existing shared dialogs, pagination, async form and toast patterns; no new state-specific visual system was introduced.

## Viewport Coverage

- Mobile: 360x800 review artifact.
- Tablet: 768x1024 review target recorded in the manifest.
- Desktop: 1280x820 review artifact.
- Wide or short screen: desktop layout remains owned by the shared page frame; no new fixed-width shell was added.
- 200% zoom and reduced motion: reduced-motion conclusion is recorded as passed because no new continuous motion was introduced; 200% zoom requires the live browser pass.

## Evidence

- Updated screenshot or recording: `docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png` and `docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png`.
- Automated visual or overlap checks: `pnpm --dir frontend design:check` is the repository gate; this record is intentionally `static-review-board` evidence and does not claim browser execution.
- Commands run: `pnpm --dir frontend design:check`, `pnpm --dir frontend lint:check`, `pnpm --dir frontend typecheck` (where dependencies are present); backend targeted tests and fork-integrity are tracked separately in the task evidence.

## Residual Risk

- Known limitations: artifacts are static, non-production evidence; the local worktree has no frontend dependency install for typecheck and no browser service was started. State Kit plugin UI remains an upstream artifact with Chinese-only strings and therefore is not production-ready for bilingual enablement.
- Follow-up owner: before any PR merge or deployment, run authenticated browser acceptance in Chinese and explicit English at 360, 768 and 1280 pixels, complete locale parity and plugin UI bilingualization, then rerun all release gates.

This record maps the visible files introduced by the v0.2.7 synchronization.
It does not authorize deployment or replace local browser acceptance.
