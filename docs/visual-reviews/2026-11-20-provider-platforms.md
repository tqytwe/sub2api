# Visual Review: Provider platform integration

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/settings.ts",
    "frontend/src/api/admin/users.ts",
    "frontend/src/components/account/AccountUsageCell.vue",
    "frontend/src/components/account/CNProviderBalanceCell.vue",
    "frontend/src/components/account/CNProviderQuotaCell.vue",
    "frontend/src/components/account/CreateAccountModal.vue",
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/components/account/ModelWhitelistSelector.vue",
    "frontend/src/components/account/OpenCodeGoProtocolRulesEditor.vue",
    "frontend/src/components/account/QwenCacheSettings.vue",
    "frontend/src/components/account/__tests__/CreateAccountModal.spec.ts",
    "frontend/src/components/account/__tests__/EditAccountModal.spec.ts",
    "frontend/src/components/account/credentialsBuilder.ts",
    "frontend/src/components/account/qwenCacheSettings.ts",
    "frontend/src/components/common/PlatformIcon.vue",
    "frontend/src/components/keys/UseKeyModal.vue",
    "frontend/src/composables/useModelWhitelist.ts",
    "frontend/src/constants/__tests__/platforms.spec.ts",
    "frontend/src/constants/platforms.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/admin/accounts.ts",
    "frontend/src/i18n/locales/en/admin/overview.ts",
    "frontend/src/i18n/locales/en/common.ts",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/admin/accounts.ts",
    "frontend/src/i18n/locales/zh/admin/overview.ts",
    "frontend/src/i18n/locales/zh/common.ts",
    "frontend/src/i18n/locales/zh/dashboard.ts",
    "frontend/src/types/index.ts",
    "frontend/src/utils/keyGroupProviders.ts",
    "frontend/src/utils/platformColors.ts",
    "frontend/src/views/admin/ChannelsView.vue"
  ],
  "routes_or_surfaces": [
    "Administrator accounts create/edit and quota/balance",
    "Administrator groups and channels",
    "User API key client config"
  ],
  "languages_and_themes": [
    "zh/light",
    "en/dark",
    "en/light",
    "zh/dark"
  ],
  "states": [
    "Provider selection",
    "API key and provider endpoint",
    "Qwen cache options",
    "Quota and balance for supported providers"
  ],
  "viewports": [
    "360x800",
    "768x900",
    "1280x900",
    "1920x1080"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-20-provider-platforms/qwen-prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-20-cline-platform/baseline-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-20-provider-platforms/Cline-360.png",
    "docs/visual-reviews/assets/2026-11-20-provider-platforms/Command-Code-360.png",
    "docs/visual-reviews/assets/2026-11-20-provider-platforms/Mistral-360.png",
    "docs/visual-reviews/assets/2026-11-20-provider-platforms/Qwen-1280.png",
    "docs/visual-reviews/assets/2026-11-20-provider-platforms/Qwen-360.png"
  ],
  "commands": [
    "Playwright Chromium on the combined Vue production build with local fixture responses"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "Enter selected each provider and exposed its corresponding endpoint in the compiled account dialog."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "The account dialog was exercised with reduced motion enabled at each viewport."
    }
  },
  "residual_risks": [
    "The combined capture uses fixture data. Production tests with authorized provider credentials and user/admin browser acceptance remain separate."
  ]
}
-->

## Scope

Add Qwen, Mistral, Cline and Command Code to the existing gateway account and group setup.

## Baseline

The production base build showed the same account dialog and an existing OpenCode or TypeSafe credential flow.

## Prototype

The first compiled Qwen form reused the account dialog, endpoint input and existing boolean controls for optional caching.

## Reuse Decision

The four providers use the current account form, group and channel selectors, usage cells and existing API key configuration dialog.

## State Coverage

Selection and base URL were checked for all four providers with Enter. Individual platform reviews cover the balance and quota fixtures. The group, model protocol, and save operations are covered by automated tests.

## Viewport Coverage

The combined build was checked at 360, 768, 1280 and 1920 pixels with no page overflow and no browser errors.

## Evidence

The manifest links the baseline, the earlier Qwen build and the final combined account dialog captures.

## Residual Risk

This records the rendered compiled frontend against fixture APIs. Real accounts and user permissions are verified separately on production.
