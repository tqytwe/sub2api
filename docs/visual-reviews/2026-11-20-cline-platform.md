# Visual Review: Cline account setup and usage

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/account/AccountUsageCell.vue",
    "frontend/src/components/account/CNProviderBalanceCell.vue",
    "frontend/src/components/account/CNProviderQuotaCell.vue",
    "frontend/src/components/account/CreateAccountModal.vue",
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/components/account/ModelWhitelistSelector.vue",
    "frontend/src/components/account/credentialsBuilder.ts",
    "frontend/src/components/keys/UseKeyModal.vue",
    "frontend/src/composables/useModelWhitelist.ts",
    "frontend/src/constants/platforms.ts",
    "frontend/src/types/index.ts",
    "frontend/src/utils/keyGroupProviders.ts",
    "frontend/src/utils/platformColors.ts"
  ],
  "routes_or_surfaces": ["Existing administrator account creation and editing dialogs, group filters, account usage cells and the API key client configuration dialog"],
  "languages_and_themes": ["en/light", "zh/dark"],
  "states": ["Existing API key account form", "Selected Cline provider and API key fields", "Three subscription windows and USD balance from the account snapshot"],
  "viewports": ["360x900", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-20-cline-platform/prototype-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-20-cline-platform/baseline-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-11-20-cline-platform/updated-1280.png", "docs/visual-reviews/assets/2026-11-20-cline-platform/updated-360.png", "docs/visual-reviews/assets/2026-11-20-cline-platform/updated-usage.png"],
  "commands": ["Playwright Chromium isolated capture of the compiled CreateAccountModal and AccountUsageCell Vue components with a local API fixture"] ,
  "checks": {
    "keyboard": {"status": "passed", "reason": "Focused the platform button and selected it via Enter in Chromium."},
    "reduced_motion": {"status": "not-applicable", "reason": "Existing dialog and quota components own their motion. Cline adds no animation."}
  },
  "residual_risks": ["The account fixture supplies example balances and windows. Production browser acceptance and the provider account API require a real administrator and authorized Cline credentials."]
}
-->

## Scope

Cline joins the existing account platform selector as an API key provider. The official base URL is filled by default. The existing quota and balance cells display snapshots for accounts using the official API host.

## Baseline

The existing account dialog was opened on the TypeSafe API key flow to inspect platform switching, field layout and the mobile drawer.

## Prototype

The real account dialog was rendered with a Cline option in the existing platform selector. Keyboard selection and the official base URL were checked in the same dialog.

## Reuse Decision

This adds one platform to the shared selectors and palette. Credential inputs, model discovery, progress bars and balance labels use the existing components.

## State Coverage

Chromium checked platform selection with Enter at 360, 768, 1280 and 1920 pixels with no page overflow. The updated account usage fixture displayed all three windows and the USD balance through the existing cells. Loading and errors use the cells' existing refresh controls and messages.

## Viewport Coverage

The English light dialog and Chinese dark mobile dialog were rendered at the listed widths.

## Evidence

The manifest links the baseline, prototype, and updated Chromium captures.

## Residual Risk

The screenshots use production Vue components with fixture data. The real admin account, gateway authorization and Cline connection still require acceptance after deployment.
