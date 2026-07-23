# KyrenPay Merchant Display Name Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/payment/providerConfig.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/admin/settings.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts"
  ],
  "routes_or_surfaces": [
    "admin payment provider edit dialog",
    "EasyPay provider credential fields",
    "KyrenPay checkout merchant display-name setup"
  ],
  "languages_and_themes": [
    "zh-CN light static board",
    "zh-CN dark inherited token review",
    "en-US light copy review",
    "en-US dark inherited token review"
  ],
  "states": [
    "existing EasyPay instance edit mode",
    "custom PayNow method selected",
    "KyrenPay API base URL configured",
    "merchant display name filled",
    "merchant display name cleared",
    "currency selector set to USD"
  ],
  "viewports": [
    "390x844",
    "768x1024",
    "1280x820"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/kyrenpay-merchant-display-name/prototype-kyrenpay-merchant-display-name.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/kyrenpay-merchant-display-name/baseline-kyrenpay-merchant-display-name.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/kyrenpay-merchant-display-name/updated-kyrenpay-merchant-display-name.png"
  ],
  "commands": [
    "python3 generated static KyrenPay merchant display-name review boards with PIL",
    "pnpm --dir frontend exec vitest run src/components/payment/__tests__/providerConfig.spec.ts src/components/payment/__tests__/PaymentProviderDialog.spec.ts --run",
    "git diff --check"
  ],
  "checks": {
    "keyboard": {
      "status": "passed"
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "The change adds a static optional input and helper text only; no animation or transition behavior changes."
    }
  },
  "residual_risks": [
    "The artifacts are static review boards rather than browser screenshots, so final browser acceptance on the production admin dialog is still required.",
    "The review does not include live KyrenPay checkout credentials or a real payment attempt because secrets must stay out of artifacts."
  ]
}
-->

## Scope

This review covers the EasyPay provider edit dialog in admin payment settings. The visible change is one optional merchant display-name field that lets admins set the value KyrenPay shows on the hosted checkout page instead of letting the upstream fall back to account metadata such as the login email.

## Baseline

Before this change, the EasyPay credential block included PID, PKey, API Base URL, currency, and optional channel IDs. There was no dedicated control for KyrenPay's checkout merchant display name, so the upstream hosted page could show the KyrenPay account email when the merchant profile name was blank.

Baseline artifact: `docs/visual-reviews/assets/kyrenpay-merchant-display-name/baseline-kyrenpay-merchant-display-name.png`.

## Prototype

The prototype keeps the current modal structure and reuses the existing `PROVIDER_CONFIG_FIELDS` renderer. The new field is optional, clearable, and placed after API Base URL because it is KyrenPay/EasyPay checkout metadata rather than a channel ID or credential secret.

Prototype artifact: `docs/visual-reviews/assets/kyrenpay-merchant-display-name/prototype-kyrenpay-merchant-display-name.png`.

## Reuse Decision

The implementation does not add a new component, color token, layout primitive, or modal section. It reuses the existing text-input, optional-label, helper-text, config filtering, and callback URL patterns already present in `PaymentProviderDialog.vue`.

## State Coverage

The review covers editing an existing EasyPay provider, preserving PKey by leaving it blank, configuring KyrenPay API Base URL, filling the merchant display name, clearing that optional value, and keeping PayNow currency set to USD.

## Viewport Coverage

The changed surface is an existing wide modal with stacked form rows. The static boards cover mobile, tablet, and desktop intent. Since the change adds one standard input row plus helper copy, wrapping and scrolling remain owned by the existing dialog field stack.

## Evidence

Updated artifact: `docs/visual-reviews/assets/kyrenpay-merchant-display-name/updated-kyrenpay-merchant-display-name.png`.

Commands run:

```bash
python3 generated static KyrenPay merchant display-name review boards with PIL
pnpm --dir frontend exec vitest run src/components/payment/__tests__/providerConfig.spec.ts src/components/payment/__tests__/PaymentProviderDialog.spec.ts --run
git diff --check
```

## Residual Risk

The artifacts are static review boards, not browser screenshots with production data. After deployment, final browser acceptance should still open the admin payment provider dialog, enter the merchant display name, and create a small KyrenPay checkout order on the user's local machine.
