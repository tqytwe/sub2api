# Visual Review: v0.2.0 Upstream Safety

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/account/AccountStatusIndicator.vue",
    "frontend/src/components/account/AccountUsageCell.vue",
    "frontend/src/components/account/BulkEditAccountModal.vue",
    "frontend/src/components/account/CNProviderQuotaCell.vue",
    "frontend/src/components/account/CreateAccountModal.vue",
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/components/account/ModelWhitelistSelector.vue",
    "frontend/src/components/account/UsageProgressBar.vue",
    "frontend/src/components/account/credentialsBuilder.ts",
    "frontend/src/components/admin/account/ReAuthAccountModal.vue",
    "frontend/src/components/admin/channel/IntervalRow.vue",
    "frontend/src/components/admin/channel/PricingEntryCard.vue",
    "frontend/src/components/admin/channel/types.ts",
    "frontend/src/components/admin/group/ReasoningEffortPolicyFields.vue",
    "frontend/src/components/admin/usage/UsageFilters.vue",
    "frontend/src/components/admin/usage/UsageStatsCards.vue",
    "frontend/src/components/admin/usage/UsageTable.vue",
    "frontend/src/components/admin/user/UserAllowedGroupsModal.vue",
    "frontend/src/components/auth/EmailOAuthButtons.vue",
    "frontend/src/components/auth/LinuxDoOAuthSection.vue",
    "frontend/src/components/channels/SupportedModelChip.vue",
    "frontend/src/components/common/MonitorQuotaView.vue",
    "frontend/src/components/keys/UseKeyModal.vue",
    "frontend/src/components/modelPlaza/PlazaModelPricingTable.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/admin/accounts.ts",
    "frontend/src/i18n/locales/en/admin/channels.ts",
    "frontend/src/i18n/locales/en/admin/overview.ts",
    "frontend/src/i18n/locales/en/admin/resources.ts",
    "frontend/src/i18n/locales/en/admin/settings.ts",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/en/misc.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/admin/accounts.ts",
    "frontend/src/i18n/locales/zh/admin/channels.ts",
    "frontend/src/i18n/locales/zh/admin/overview.ts",
    "frontend/src/i18n/locales/zh/admin/resources.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts",
    "frontend/src/i18n/locales/zh/dashboard.ts",
    "frontend/src/i18n/locales/zh/misc.ts",
    "frontend/src/views/admin/AccountsView.vue",
    "frontend/src/views/admin/ChannelsView.vue",
    "frontend/src/views/admin/GroupsView.vue",
    "frontend/src/views/admin/RedeemView.vue",
    "frontend/src/views/admin/SettingsView.vue",
    "frontend/src/views/admin/UsageView.vue",
    "frontend/src/views/admin/groupsOpenAIFast.ts",
    "frontend/src/views/admin/groupsReasoningEffort.ts",
    "frontend/src/views/auth/RegisterView.vue",
    "frontend/src/views/user/PaymentView.vue",
    "frontend/src/views/user/UsageView.vue"
  ],
  "routes_or_surfaces": [
    "API key usage dialog",
    "Model Plaza pricing table",
    "admin account, channel, group, setting and usage surfaces",
    "registration, payment and user usage surfaces"
  ],
  "languages_and_themes": [
    "zh-CN/light static review board",
    "targeted en-US labels on static review board"
  ],
  "states": [
    "default",
    "focus-visible contract",
    "loading and reduced-motion contract",
    "tiered and flat pricing"
  ],
  "viewports": ["390x900", "1280x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png",
    "docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png",
    "docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png"
  ],
  "commands": [
    "firefox --headless --window-size 1280,900 --screenshot docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png file://.../prototype.html",
    "playwright screenshot -b chromium --viewport-size 390,900 --full-page file://.../prototype.html docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png"
  ],
  "checks": {
    "keyboard": {
      "status": "not-applicable",
      "reason": "Static review boards cannot execute the authenticated product keyboard flow."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "Static review boards document the contract but cannot prove runtime media-query behavior."
    }
  },
  "residual_risks": [
    "Static review boards are not product browser screenshots; authenticated guest, user and admin final acceptance is still required.",
    "Real 360, 768, 1280 and wide-screen routes still require light and dark theme, Chinese and English, keyboard, zoom and reduced-motion browser checks."
  ]
}
-->

## Scope

- Routes and surfaces: API key usage dialog, Model Plaza pricing, account and
  channel administration, group policies, settings, usage, registration,
  payment and user usage.
- Roles: the merge affects public, guest, user and admin surfaces. The static
  board covers only the API key dialog and pricing/focus contracts without an
  authenticated role.
- Languages and themes: the board uses Chinese with targeted English labels on
  a light surface. It is not full-route bilingual or dark-theme evidence.

## Baseline

- Current behavior: the fork's existing page shells, semantic colors and
  operational density remain the visual baseline. The safety fixes change
  configuration output, pricing information and focus/loading states rather
  than introducing a new page composition.
- Baseline artifact: `prototype-1280.png` is a static comparison board showing
  the retained shell and component proportions. It is not a v0.1.182 product
  browser capture and must not be used as production acceptance evidence.
- Inconsistencies observed during review: Gemini groups could not reach their
  Codex configuration branch; tiered Model Plaza rows could repeat a flat-price
  row; newly merged controls needed explicit focus-visible and reduced-motion
  treatment.

## Prototype

- Prototype design images: `prototype-1280.png` and `prototype-390.png`.
- Approval status: the user approved proceeding with the recommended safety
  repair order. This board is implementation review evidence, not approval to
  commit, push, merge or deploy.
- Scope boundary: preserve the existing layouts and tokens; repair only the
  affected information and interaction states.

## Reuse Decision

- Existing dialog, tabs, code block, pricing rows, buttons, form controls,
  semantic colors and responsive stacking are retained.
- No new page shell, card system, icon registry, button family, gradient or
  arbitrary color was introduced.
- The existing compact inline provider quota glyph remains because there is no
  matching shared Icon path. Its loading rotation is transient and explicitly
  disabled for reduced-motion users.

## State Coverage

- Default: the static board shows stable API key and pricing content.
- Hover and active: no new hover-only action or layout-shifting active state was
  introduced; these states still require authenticated browser confirmation.
- Focus-visible and keyboard: group Fast switches and reasoning-policy actions
  now retain visible focus rings. Keyboard order and activation are not proven
  by a static artifact.
- Loading, disabled, empty, error and success: quota and catalog loading
  rotations exist only during requests and use `motion-reduce:animate-none`.
  The broader merged surfaces require real-route state checks.

## Viewport Coverage

- Mobile: the 390x900 board stacks all sections without horizontal overflow;
  configuration lines and pricing labels remain readable.
- Tablet: no rendered 768px product evidence is available yet.
- Desktop: the 1280x900 board keeps the pricing and focus panels aligned and
  leaves stable space around the API key configuration.
- Wide or short screen: no rendered 1600px or short-height product evidence is
  available yet.
- 200% zoom and reduced motion: source contracts were reviewed, but runtime
  behavior needs authenticated browser acceptance.

## Evidence

- Updated static boards: `prototype-1280.png` and `prototype-390.png`.
- Automated visual or overlap checks: both PNG files were decoded and visually
  inspected; the 390px board has no clipped or overlapping content. The design
  governance gate validates artifact structure and source-level exceptions.
- Commands run: Firefox rendered the 1280px board. Playwright Chromium rendered
  the 390px board after Firefox's headless compositor failed to produce the
  mobile file.

## Residual Risk

- These artifacts cover the high-risk safety repairs, not every state of all 51
  upstream-visible changed files. Code and structured manifest coverage do not
  replace real application screenshots.
- Before release, perform real browser checks at 360, 768, 1280 and wide
  viewports for Chinese and English, light and dark themes, keyboard navigation,
  200% zoom, reduced motion, loading/error/empty states and long content.
- Final acceptance still belongs to the user's local production browser as
  guest, normal user and administrator after a separately authorized release.
