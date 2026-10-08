# Visual Review: v0.2.13 Upstream Merge

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/account/AccountPriorityCell.vue", "frontend/src/components/account/AccountStatusIndicator.vue", "frontend/src/components/account/AccountUsageCell.vue", "frontend/src/components/account/BulkEditAccountModal.vue", "frontend/src/components/account/ClaudeResetCreditsCell.vue", "frontend/src/components/account/CreateAccountModal.vue", "frontend/src/components/account/EditAccountModal.vue", "frontend/src/components/account/ModelWhitelistSelector.vue", "frontend/src/components/account/OpenAIQuotaResetCell.vue", "frontend/src/components/account/OpenAIReferralCell.vue", "frontend/src/components/account/OpenCodeGoUsageCell.vue", "frontend/src/components/account/TempUnschedStatusModal.vue", "frontend/src/components/account/UsageProgressBar.vue", "frontend/src/components/account/credentialsBuilder.ts", "frontend/src/components/admin/BackupArchiveSettings.vue", "frontend/src/components/admin/channel/IntervalRow.vue", "frontend/src/components/admin/channel/ModelTagInput.vue", "frontend/src/components/admin/channel/PricingEntryCard.vue", "frontend/src/components/admin/channel/types.ts", "frontend/src/components/admin/group/GroupRPMOverridesModal.vue", "frontend/src/components/admin/group/GroupRateMultipliersModal.vue", "frontend/src/components/admin/monitor/MonitorTemplateApplyPickerDialog.vue", "frontend/src/components/admin/user/GroupReplaceModal.vue", "frontend/src/components/admin/user/UserAllowedGroupsModal.vue", "frontend/src/components/admin/user/UserApiKeysModal.vue", "frontend/src/components/admin/user/UserBalanceHistoryModal.vue", "frontend/src/components/admin/user/UserBalanceModal.vue", "frontend/src/components/admin/user/UserPlatformQuotaModal.vue", "frontend/src/components/channels/SupportedModelChip.vue", "frontend/src/components/common/BaseDialog.vue", "frontend/src/components/common/DateRangePicker.vue", "frontend/src/components/common/ImageUpload.vue", "frontend/src/components/common/PlatformTypeBadge.vue", "frontend/src/components/common/SearchInput.vue", "frontend/src/components/common/Select.vue", "frontend/src/components/common/SubscriptionProgressMini.vue", "frontend/src/components/keys/UseKeyModal.vue", "frontend/src/components/modelPlaza/PlazaGroupSection.vue", "frontend/src/components/modelPlaza/PlazaModelPricingTable.vue", "frontend/src/components/payment/PaymentProviderDialog.vue", "frontend/src/components/user/MonitorDetailDialog.vue", "frontend/src/components/user/UserAttributeForm.vue", "frontend/src/components/user/UserAttributesConfigModal.vue", "frontend/src/components/user/UserErrorDetailModal.vue", "frontend/src/components/user/UserPlatformQuotaCell.vue", "frontend/src/components/user/dashboard/UserDashboardStats.vue", "frontend/src/components/user/profile/ProfileInfoCard.vue", "frontend/src/components/user/profile/TotpDisableDialog.vue", "frontend/src/components/user/profile/TotpSetupModal.vue", "frontend/src/i18n/locales/en/admin/accounts.ts", "frontend/src/i18n/locales/en/admin/channels.ts", "frontend/src/i18n/locales/en/admin/ops.ts", "frontend/src/i18n/locales/en/admin/overview.ts", "frontend/src/i18n/locales/en/admin/settings.ts", "frontend/src/i18n/locales/en/dashboard.ts", "frontend/src/i18n/locales/en/misc.ts", "frontend/src/i18n/locales/zh/admin/accounts.ts", "frontend/src/i18n/locales/zh/admin/channels.ts", "frontend/src/i18n/locales/zh/admin/ops.ts", "frontend/src/i18n/locales/zh/admin/overview.ts", "frontend/src/i18n/locales/zh/admin/settings.ts", "frontend/src/i18n/locales/zh/common.ts", "frontend/src/i18n/locales/zh/dashboard.ts", "frontend/src/i18n/locales/zh/misc.ts", "frontend/src/views/admin/AccountsView.vue", "frontend/src/views/admin/BackupView.vue", "frontend/src/views/admin/ChannelsView.vue", "frontend/src/views/admin/DashboardView.vue", "frontend/src/views/admin/GroupsView.vue", "frontend/src/views/admin/RiskControlView.vue", "frontend/src/views/admin/SettingsView.vue", "frontend/src/views/admin/UsersView.vue", "frontend/src/views/admin/affiliates/AdminAffiliateRecordsTable.vue", "frontend/src/views/admin/affiliates/AffiliateOfflineWithdrawDialog.vue", "frontend/src/views/admin/affiliates/affiliateWithdrawOperation.ts", "frontend/src/views/admin/groupModelAllowlist.ts", "frontend/src/views/admin/ops/components/LogRetentionSelect.vue", "frontend/src/views/admin/ops/components/OpsSystemLogTable.vue", "frontend/src/views/user/ChannelStatusV1View.vue", "frontend/src/views/user/KeysView.vue", "frontend/src/views/user/UsageView.vue"
  ],
  "routes_or_surfaces": ["/admin/accounts", "/admin/users", "/admin/groups", "/admin/channels", "/admin/settings", "/admin/backup", "/admin/risk-control", "/admin/dashboard", "/admin/ops", "/admin/affiliates", "/keys", "/usage", "/dashboard", "/profile", "/channel-status", "/models", "/purchase"],
  "languages_and_themes": ["zh-CN/light", "en-US/light", "zh-CN/dark", "en-US/dark"],
  "states": ["default", "focus-visible", "loading", "disabled", "empty", "error", "success", "IME composition", "dialog reopen scroll reset"],
  "viewports": ["360x800", "768x1024", "1280x820"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png", "docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png", "docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"],
  "commands": ["pnpm --dir frontend design:check", "pnpm --dir frontend lint:check", "pnpm --dir frontend typecheck", "vitest run"],
  "checks": {"keyboard": {"status": "not-applicable", "reason": "Static review board has no live focus traversal."}, "reduced_motion": {"status": "passed", "notes": "New upstream spinners and the priority saving dot carry motion-reduce:animate-none with reviewed allowances."}},
  "residual_risks": ["Artifacts are reused static review boards and are non-production evidence.", "Final acceptance requires the user's local browser at https://www.jisudeng.com/ as guest, normal user and administrator, in Chinese and English, light and dark, at 360, 768 and 1280 pixels."]
}
-->

## Scope

- Routes: admin accounts, users, groups, channels, settings, backup, risk control, dashboard, ops and affiliates; user keys, usage, dashboard, profile, channel status, model plaza and purchase.
- Roles: administrator for management routes; authenticated user for keys, usage, dashboard, profile and purchase; guest for public model and channel status surfaces.
- Languages and themes: Chinese default and explicit English, light and dark themes.
- Boundary: visible surfaces introduced or changed by merging upstream v0.2.7 to v0.2.13 into the fork. Fork payment, VIP, campaign, coupon, onboarding and bulk user action surfaces keep their production behavior.

## Baseline

- Current behavior: production `play/main` at `450148f00` (upstream v0.2.7 base plus fork customizations).
- Baseline screenshot or recording: `docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png`.
- Inconsistencies observed: upstream added account priority quick edit, Claude reset credits, OpenCode Go usage, backup archive selection, offline affiliate withdrawal, user API key and error detail dialogs, group RPM and rate overrides, and log retention controls. Upstream also shipped an amount-tier recharge bonus UI that overlaps the fork's VIP and campaign recharge quote.

## Prototype

- Prototype design image: `docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png` and `docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png`.
- Approval status: static review board for engineering review only; no production approval is implied.
- Scope boundary: reused review artifacts; the live check happens during local browser acceptance.

## Reuse Decision

- Shared layouts and components reused: existing page frame, BaseDialog, Pagination, SearchInput, DateRangePicker, Icon, admin form controls and locale scopes.
- Fork overlaps kept: user purchase amounts, order tables, payment status and admin order detail stay on the fork VIP and campaign quote; the upstream recharge bonus tier editor is removed and its badges and bonus rows are not merged. User KeysView keeps the fork onboarding flow without bulk selection, admin UsersView keeps the server-side bulk action dialog, DateRangePicker presets still apply immediately, and CC Switch import keeps the fork gateway root and `/v1` Codex endpoint.
- Upstream fixes adopted: dialog body scroll resets on reopen, SearchInput IME composition, calendar-based subscription expiry labels.
- Design-system exception, if any: upstream spinners, the priority stepper glyphs, the priority inline input focus treatment, the backup archive overlay radius and the affiliate user search scroll area carry inline `design-governance-allow` comments. The TypeSafe platform tab now uses `transition` instead of `transition-all`.

## State Coverage

- Default: management lists, account cells, dialogs and user surfaces are represented in the review board.
- Hover and active: shared buttons, tabs, table rows and stepper controls keep existing component states.
- Focus-visible and keyboard: the static board cannot run focus traversal, so the live browser pass covers it.
- Loading, disabled, empty, error and success: new spinners stop for reduced-motion users; empty and error states reuse the existing dialog and toast patterns.

## Viewport Coverage

- Mobile: 360x800 review artifact.
- Tablet: 768x1024 review target recorded in the manifest.
- Desktop: 1280x820 review artifact.
- Wide or short screen: layout stays owned by the shared page frame; no new route shell was added.
- 200% zoom and reduced motion: reduced motion is handled by `motion-reduce:animate-none` on new spinners; 200% zoom needs the live browser pass.

## Evidence

- Updated screenshot or recording: `docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png` and `docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png`.
- Automated visual or overlap checks: `pnpm --dir frontend design:check` is the repository gate. This record is `static-review-board` evidence and does not claim a browser run.
- Commands run: `design:check`, `lint:check`, `typecheck` and `vitest run` locally; backend tests, build and fork integrity run on the server and are tracked in the delivery evidence.

## Residual Risk

- Known limitations: artifacts are static and non-production; no browser service was started for this record.
- Follow-up owner: before release, the user completes local browser acceptance at `https://www.jisudeng.com/` as guest, normal user and administrator, covering Chinese and English, light and dark, and the 360, 768 and 1280 pixel widths.

This record maps the visible files introduced by the v0.2.13 synchronization.
It does not authorize deployment or replace local browser acceptance.
