# KyrenPay EasyPay PayNow Config Visual Review

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
    "KyrenPay PayNow custom method setup"
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
    "API base URL hint visible",
    "currency selector default CNY",
    "currency selector set to USD or HKD",
    "sensitive PKey left blank to preserve stored secret"
  ],
  "viewports": [
    "390x844",
    "768x1024",
    "1280x820"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/kyrenpay-easypay-paynow-config/prototype-kyrenpay-easypay-paynow-config.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/kyrenpay-easypay-paynow-config/baseline-kyrenpay-easypay-paynow-config.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/kyrenpay-easypay-paynow-config/updated-kyrenpay-easypay-paynow-config.png"
  ],
  "commands": [
    "python3 generated static KyrenPay EasyPay PayNow review boards with PIL",
    "pnpm --dir frontend exec vitest run src/components/payment/__tests__/providerConfig.spec.ts src/components/payment/__tests__/PaymentProviderDialog.spec.ts --run",
    "git diff --check"
  ],
  "checks": {
    "keyboard": {
      "status": "passed"
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "The change adds static form fields and helper text only; no animation or transition behavior changes."
    }
  },
  "residual_risks": [
    "The artifacts are static review boards rather than browser screenshots, so final browser acceptance on the production admin dialog is still required.",
    "The review does not include live KyrenPay credentials or a real merchant checkout because secrets must stay out of artifacts."
  ]
}
-->

## Scope

This review covers the admin payment provider dialog when configuring an EasyPay-compatible KyrenPay instance. The visible change is limited to adding an EasyPay API base URL hint and an EasyPay payment currency selector so PayNow can be configured without changing the modal layout or creating a new payment settings surface.

## Baseline

Before this change, the EasyPay credential block showed PID, PKey, API Base URL, and optional channel IDs. The API Base URL field had no KyrenPay-specific guidance, so an admin could paste the merchant PID into the URL field. EasyPay also had no visible currency selector, which left PayNow unable to declare USD or HKD through the existing provider configuration UI.

Baseline artifact: `docs/visual-reviews/assets/kyrenpay-easypay-paynow-config/baseline-kyrenpay-easypay-paynow-config.png`.

## Prototype

The prototype keeps the current modal structure and reuses the existing input, sensitive-field, Select, custom-method, and helper-text patterns. The only visible additions are a hint under API Base URL and a standard payment currency Select with the existing currency option list.

Prototype artifact: `docs/visual-reviews/assets/kyrenpay-easypay-paynow-config/prototype-kyrenpay-easypay-paynow-config.png`.

## Reuse Decision

The implementation reuses `PaymentProviderDialog.vue` field rendering and `PROVIDER_CONFIG_FIELDS`. No new component, color token, page shell, icon system, or layout primitive was added. Existing default handling applies `CNY` until the admin selects a provider-supported currency such as `USD` or `HKD`.

## State Coverage

The review covers editing an existing EasyPay provider, leaving PKey blank to preserve the stored secret, adding a PayNow custom method, showing the KyrenPay API URL hint, and choosing a non-CNY currency for PayNow. Validation continues to use the existing required-field behavior and custom-method validation.

## Viewport Coverage

The changed surface is an existing wide modal with stacked form rows. The static boards cover mobile, tablet, and desktop intent. Since the change adds one standard Select row plus helper copy, wrapping remains owned by the existing dialog grid and field stack.

## Evidence

Updated artifact: `docs/visual-reviews/assets/kyrenpay-easypay-paynow-config/updated-kyrenpay-easypay-paynow-config.png`.

Commands run:

```bash
python3 generated static KyrenPay EasyPay PayNow review boards with PIL
pnpm --dir frontend exec vitest run src/components/payment/__tests__/providerConfig.spec.ts src/components/payment/__tests__/PaymentProviderDialog.spec.ts --run
git diff --check
```

## Residual Risk

The artifacts are static review boards, not browser screenshots with production data. After deployment, final browser acceptance should still open the admin payment provider dialog and the user checkout page on the user's local machine.
