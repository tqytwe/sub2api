# Visual Review: BEpusdt provider integration

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/payment/PaymentProviderDialog.vue",
    "frontend/src/components/payment/ProviderCard.vue",
    "frontend/src/components/payment/paymentFlow.ts",
    "frontend/src/components/payment/providerConfig.ts",
    "frontend/src/i18n/locales/en/admin/settings.ts",
    "frontend/src/i18n/locales/en/misc.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts",
    "frontend/src/i18n/locales/zh/misc.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/views/admin/SettingsView.vue"
  ],
  "routes_or_surfaces": ["/admin/settings payment provider dialog", "/payment checkout method selector"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "enabled", "disabled", "editing", "validation-error", "focus-visible"],
  "viewports": ["360x800", "768x900", "1280x900", "1920x1080"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/bepusdt-provider-integration/prototype-bepusdt-provider-dialog.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/bepusdt-provider-integration/prototype-bepusdt-provider-dialog.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/bepusdt-provider-integration/updated-bepusdt-provider-dialog.png"
  ],
  "commands": [
    "corepack pnpm install --frozen-lockfile",
    "corepack pnpm design:check",
    "corepack pnpm lint:check",
    "corepack pnpm typecheck",
    "corepack pnpm test",
    "corepack pnpm build"
  ],
  "checks": {
    "keyboard": { "status": "passed", "notes": "The change reuses the existing Select, ToggleSwitch, form controls and BaseDialog keyboard behavior; no new custom control was introduced." },
    "reduced_motion": { "status": "passed", "notes": "No animation or motion was added." },
    "copy_locale": { "status": "passed", "notes": "Provider, method, API token, API base guidance and manual on-chain refund copy are present in zh-CN and en-US." }
  },
  "residual_risks": [
    "The evidence is a static review board, not a browser capture of the production admin dialog.",
    "Final acceptance still requires local-browser checks in light and dark themes at mobile and desktop widths after deployment."
  ]
}
-->

## Scope

- Surfaces: the existing admin payment-provider dialog and existing user payment-method selector.
- Roles: administrators configure the provider; authenticated users select USDT (TRC20) during recharge or subscription checkout.
- Languages and themes: zh-CN and en-US, light and dark.
- Boundary: no new route, page shell, card system, checkout layout or payment-result page was introduced.

## Baseline

- Current behavior: the shared dialog supported EasyPay, official Alipay/WeChat, Stripe and Airwallex, but had no dedicated BEpusdt configuration or USDT (TRC20) method.
- Baseline artifact: the prototype board preserves the existing dialog shell and identifies the additive fields; it is not a live browser screenshot.
- Inconsistency observed: the first implementation hid refund controls but did not explain that BEpusdt refunds must be handled manually on-chain.

## Prototype

- Prototype design image: `docs/visual-reviews/assets/bepusdt-provider-integration/prototype-bepusdt-provider-dialog.png`.
- Approval status: follows the already agreed dedicated-provider boundary and existing administration dialog.
- Scope boundary: fixed CNY settlement, sensitive API token, deterministic callback paths, USDT TRC20 only, and no automatic refund controls.

## Reuse Decision

- Reused `BaseDialog`, `Select`, `ToggleSwitch`, existing provider field rendering, callback URL controls, provider cards and payment-method selector.
- The API token uses the existing sensitive-input masking and keep-on-empty update behavior.
- No new shared visual pattern or design-system exception was required.

## State Coverage

- Default: BEpusdt appears as a provider and as USDT (TRC20) in checkout ordering.
- Enabled and disabled: existing provider enablement behavior remains unchanged.
- Editing: the API token is omitted by the backend, shown empty, and preserved when the administrator submits it empty.
- Validation error: required configuration and backend provider validation continue through the existing field and toast flow.
- Refund state: automatic and user refund toggles are absent for BEpusdt, with a visible manual on-chain refund explanation.
- Focus-visible and keyboard: all interactive elements remain existing shared controls with their established behavior.

## Viewport Coverage

- Mobile 360x800: existing two-column name/key row remains the inherited dialog risk; no new horizontal layout was introduced by BEpusdt.
- Tablet 768x900: configuration fields and callback controls remain in the existing single-column stack.
- Desktop 1280x900 and 1920x1080: the wide dialog keeps its existing density and width.
- 200% zoom and reduced motion: no motion or fixed-size board was added to runtime; final browser verification remains required.

## Evidence

- Prototype board: `docs/visual-reviews/assets/bepusdt-provider-integration/prototype-bepusdt-provider-dialog.png`.
- Updated static review board: `docs/visual-reviews/assets/bepusdt-provider-integration/updated-bepusdt-provider-dialog.png`.
- Both PNG files decode as 1280x900 RGB images.
- Automated design, lint, type, test and build commands are recorded in the manifest and must pass before release.

## Residual Risk

- Static boards do not prove actual browser wrapping, focus order, dark-theme contrast or mobile dialog overflow.
- The user must complete production browser acceptance after the reviewed change is merged and deployed.
