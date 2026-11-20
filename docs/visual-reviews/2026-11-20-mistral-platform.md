# Visual Review: Mistral account setup and usage

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/settings.ts",
    "frontend/src/components/account/AccountUsageCell.vue",
    "frontend/src/components/account/CNProviderBalanceCell.vue",
    "frontend/src/components/account/CNProviderQuotaCell.vue",
    "frontend/src/components/account/CreateAccountModal.vue",
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/components/account/ModelWhitelistSelector.vue",
    "frontend/src/components/account/credentialsBuilder.ts",
    "frontend/src/constants/platforms.ts",
    "frontend/src/i18n/locales/en/admin/accounts.ts",
    "frontend/src/i18n/locales/zh/admin/accounts.ts",
    "frontend/src/types/index.ts",
    "frontend/src/utils/keyGroupProviders.ts"
  ],
  "routes_or_surfaces": [
    "Existing administrator account creation and editing dialogs, group and account filters, account usage cells, and scheduling settings"
  ],
  "languages_and_themes": [
    "en/light",
    "zh/dark"
  ],
  "states": [
    "Existing API key account form",
    "Selected Mistral provider with official base URL",
    "Monthly budget utilization and USD balance from the account snapshot"
  ],
  "viewports": [
    "360x900",
    "768x900",
    "1280x900",
    "1920x900"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-20-mistral-platform/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-20-mistral-platform/baseline-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-20-mistral-platform/updated-1280.png",
    "docs/visual-reviews/assets/2026-11-20-mistral-platform/updated-360.png",
    "docs/visual-reviews/assets/2026-11-20-mistral-platform/updated-usage.png"
  ],
  "commands": [
    "Playwright Chromium isolated capture of the compiled CreateAccountModal and AccountUsageCell Vue components with a local API fixture"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "Focused the platform button and selected it via Enter in Chromium."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "Existing dialog and quota components own their motion. The Mistral option adds no animation."
    }
  },
  "residual_risks": [
    "Chromium screenshots use synthetic balances and windows. Production acceptance needs an administrator with an authorized Mistral account."
  ]
}
-->

## Scope

Mistral joins the existing account platform selector as an API key provider. The official base URL is filled by default. The existing quota and balance cells display snapshots for accounts using the official API host.

## Baseline

The existing account dialog was opened on the TypeSafe API key flow to inspect platform switching, field layout and the mobile drawer.

## Prototype

The real account dialog was rendered with a Mistral option in the existing platform selector. Keyboard selection and the official base URL were checked in the same dialog.

## Reuse Decision

This adds one platform to the existing selectors. Credential inputs, model discovery, progress bars and balance labels use the existing components.

## State Coverage

Chromium checked platform selection with Enter at 360, 768, 1280 and 1920 pixels with no page overflow. The updated account usage fixture displayed the monthly window and USD balance through the existing cells. Loading and errors use the cells' existing refresh controls and messages.

## Viewport Coverage

The English light dialog and Chinese dark mobile dialog were rendered at the listed widths.

## Evidence

The manifest links the baseline, prototype, and updated Chromium captures.

## Residual Risk

The screenshots use production Vue components with fixture data. The real admin account, gateway authorization and Mistral connection still require acceptance after deployment.
