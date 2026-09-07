# Visual Review: v0.2.1 Upstream Merge Safety

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/account/CreateAccountModal.vue",
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/components/account/ModelWhitelistSelector.vue",
    "frontend/src/components/account/UpstreamRequestIdHeaderField.vue",
    "frontend/src/components/admin/channel/PricingEntryCard.vue",
    "frontend/src/components/admin/channel/types.ts",
    "frontend/src/components/admin/group/CodexManifestAccountsField.vue",
    "frontend/src/components/admin/group/ReasoningEffortPolicyFields.vue",
    "frontend/src/components/admin/usage/UsageTable.vue",
    "frontend/src/components/common/HelpTooltip.vue",
    "frontend/src/components/keys/UseKeyModal.vue",
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/components/modelPlaza/PlazaModelPricingTable.vue",
    "frontend/src/components/payment/PaymentStatusPanel.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/admin/accounts.ts",
    "frontend/src/i18n/locales/en/admin/channels.ts",
    "frontend/src/i18n/locales/en/admin/overview.ts",
    "frontend/src/i18n/locales/en/admin/resources.ts",
    "frontend/src/i18n/locales/en/admin/settings.ts",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/admin/accounts.ts",
    "frontend/src/i18n/locales/zh/admin/channels.ts",
    "frontend/src/i18n/locales/zh/admin/overview.ts",
    "frontend/src/i18n/locales/zh/admin/resources.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts",
    "frontend/src/i18n/locales/zh/dashboard.ts",
    "frontend/src/views/admin/AccountsView.vue",
    "frontend/src/views/admin/ChannelsView.vue",
    "frontend/src/views/admin/GroupsView.vue",
    "frontend/src/views/admin/SettingsView.vue",
    "frontend/src/views/admin/UsageView.vue",
    "frontend/src/views/admin/groupsReasoningEffort.ts"
  ],
  "routes_or_surfaces": ["/admin/accounts", "/admin/channels", "/admin/groups", "/admin/settings", "/admin/usage", "payment status panel", "API key configuration"],
  "languages_and_themes": ["zh-CN/light static review board", "en-US/light static review board"],
  "states": ["default", "focus-visible", "loading", "disabled", "error", "success"],
  "viewports": ["390x900", "1280x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png",
    "docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png"
  ],
  "baseline_artifacts": ["docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"],
  "updated_artifacts": [
    "docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png",
    "docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png"
  ],
  "commands": [
    "pnpm --dir frontend vitest run src/views/admin/__tests__/SettingsView.spec.ts src/router/__tests__/adminSettingsRoute.spec.ts src/i18n/__tests__/coldRouteLocaleScopes.spec.ts",
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend typecheck"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "Custom account and group switches retain a visible focus-visible ring; native inputs and existing Dialog controls remain unchanged."},
    "reduced_motion": {"status": "passed", "notes": "The merged payment and composite-route loading indicators no longer use continuous rotation."}
  },
  "residual_risks": ["The static review board is non-production design evidence, not an authenticated product screenshot. Real 360, 768 and 1280 route checks in Chinese and English, light and dark themes, and final local-browser acceptance remain required before release."]
}
-->

## Scope

- Routes and surfaces: upstream model, Fast, reasoning-effort, usage, settings,
  payment-status and account configuration changes listed in the manifest.
- Roles: administrator, ordinary user and member routes remain outside this static
  board's authenticated scope.
- Boundary: preserve the existing operational layouts and only restore upstream
  controls, metadata and accessibility behavior. No new page shell, route,
  free-form link or visual language is introduced.

## Baseline

- The retained v0.2.0 safety review board is the closest reviewed composition
  for the account, group, settings, pricing and payment surfaces. It is reused
  because this merge preserves those layouts rather than redesigning them.
- The relevant regression was not visual hierarchy: custom switches had lost a
  keyboard-visible focus treatment, and payment/loading states used styles that
  violated the current token and motion contract.

## Prototype

- Prototype design images: the two decoded static review boards named in the
  manifest, at 390px and 1280px.
- Approval boundary: the approved v0.2.1 synchronization requires semantic
  preservation of 极速蹬 behavior while absorbing Astra, Fast and reasoning
  support. This record does not authorize production deployment.

## Reuse Decision

- Reused existing administration cards, native controls, existing Dialog and
  Toast behavior, `Icon.vue`, payment-provider logos and responsive layout.
- No new component family, hand-drawn icon, page max-width, gradient, card
  nesting or arbitrary business color was added.
- Payment channel identity remains in its existing logo asset; adjacent state
  styling uses semantic token classes.

## State Coverage

- Default and success: model metadata, settings controls and payment result
  composition preserve their existing hierarchy.
- Focus-visible: account and group switches use a 2px primary focus ring.
- Loading and disabled: route refresh and payment handoff preserve disabled
  interaction and no longer rotate indefinitely.
- Error and empty: no new error presentation was introduced; existing localized
  Toast and inline form paths remain the contract.

## Viewport Coverage

- Mobile: the 390px static board verifies the compact stacking contract.
- Desktop: the 1280px static board verifies the operational-density contract.
- Tablet, dark theme, long localized content and 200 percent zoom require the
  real authenticated browser checks recorded as residual risk.

## Evidence

- The prototype and updated artifacts are real decodable PNG static review
  boards, not browser captures.
- The targeted settings, route and locale tests cover Chinese/English loading,
  admin protection, stable control IDs and payload boundaries.

## Residual Risk

- This evidence does not replace a browser screenshot of the merged product,
  nor the user's final local production acceptance. Before release, validate
  360/768/1280px, light/dark, Chinese/English, keyboard focus, disabled/loading
  and long-content states using authenticated administrator and user accounts.
