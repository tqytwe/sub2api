# Visual Review: payment provider fees and Stripe subtype

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/coupon/CouponSelector.vue",
    "frontend/src/components/payment/PaymentMethodSelector.vue",
    "frontend/src/components/payment/PaymentProviderDialog.vue",
    "frontend/src/components/payment/PaymentProviderList.vue",
    "frontend/src/components/payment/ProviderCard.vue",
    "frontend/src/components/payment/StripeSubtypeSelector.vue",
    "frontend/src/components/payment/paymentFlow.ts",
    "frontend/src/components/payment/fees.ts",
    "frontend/src/views/admin/SettingsView.vue",
    "frontend/src/views/user/PaymentView.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en/admin/settings.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts",
    "frontend/src/i18n/locales/en/misc.ts",
    "frontend/src/i18n/locales/zh/misc.ts"
  ],
  "routes_or_surfaces": ["/purchase (affected route; component evidence only)", "/admin/settings (affected route; component evidence only)"],
  "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
  "states": ["fee range", "explicit zero fee", "inherited fee", "custom fee", "keyboard selection", "disabled subtype"],
  "viewports": ["360x900", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-14-payment-provider-fees/prototype-360-light.png",
    "docs/visual-reviews/assets/2026-11-14-payment-provider-fees/prototype-1280-dark.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-14-payment-provider-fees/baseline-360-light.png",
    "docs/visual-reviews/assets/2026-11-14-payment-provider-fees/baseline-1280-dark.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-14-payment-provider-fees/updated-360-light.png",
    "docs/visual-reviews/assets/2026-11-14-payment-provider-fees/updated-1280-dark.png"
  ],
  "commands": ["node /data/tmp/v2212-payment-visual.mjs baseline # Playwright browser capture", "node /data/tmp/v2212-payment-visual.mjs prototype # Playwright browser capture", "node /data/tmp/v2212-payment-visual.mjs updated # Playwright browser capture"],
  "checks": {
    "keyboard": { "status": "passed" },
    "reduced_motion": { "status": "passed" }
  },
  "residual_risks": [
    "Captures compile the actual selectors and provider card with synthetic props in Chromium; they do not show authenticated payment/settings routes or production data.",
    "Full admin dialog, coupon quote layout, real checkout and user-local production acceptance remain pending integration and deployment; unit tests are not substitutes for that acceptance.",
    "No paid order was created and no production configuration was changed by this review."
  ]
}
-->

## Scope

The existing purchase route gains Stripe subtype selection and a fee estimate range.
The existing provider configuration dialog gains inherited/default and per-method rates.
The provider list shows the saved global default, not an unsaved settings draft.
Roles remain ordinary customer and payment administrator; no route or permission is added.

## Baseline

The baseline captures the existing method selector and provider card before visible edits,
with explicit development-only props. It has no subtype control or fee-range label.
The original provider card overflows at 360px; the upstream wrapping layout addresses this
without changing its refund controls. Baseline captures are not production screenshots.

## Prototype

The prototype compiles the actual upstream v2.21.2 Stripe selector beside the local
components, before implementation. It defines the boundary: retain the current purchase
and settings surfaces, add the upstream select/fee information, preserve Fork coupon,
BEpusdt, refund and payment flows. This is component design evidence, not an approved
production route mockup or a claim of user acceptance.

## Reuse Decision

Reuse the existing method buttons, native `.input` select, provider card, shared
`BaseDialog`, `Icon`, and current form controls. Fee settings use divider-separated rows,
not nested cards. No new page shell, payment route, overlay pattern or visual token is
introduced. The large existing SettingsView is not reformatted.

## State Coverage

- Chromium component checks cover inherited/custom rates, zero and positive fee ranges,
  native select keyboard change, disabled select and the retained selection after re-enable.
- Existing method/button hover and focus styling is reused; no hover-only action is added.
- Component and page tests cover saving zero/null overrides, invalid rates, provider
  changes, coupon re-quote clearing, estimated ranges, and actual server order amounts.
- Existing loading/error/QR/resume/free-order flows remain in the payment domain suite.
  Those tests exercise behavior, not complete route screenshot coverage.
- Full dialog keyboard traversal and authenticated route error/empty/success visual checks
  remain integration/production acceptance work.

## Viewport Coverage

The same compiled selectors and card were checked at 360, 768, 1280 and 1920px,
900px high, in Chinese/English and light/dark: 16 updated scenarios have no horizontal
page overflow or page errors. The images retained in Git cover 360 and 1280px in both
themes; all use Chinese labels, while English is in the automated scenario matrix.
`prefers-reduced-motion: reduce` is active for every scenario; no new animation exists.
200% route zoom is not claimed and remains pending with real-page acceptance.

## Evidence

Artifacts are under `assets/2026-11-14-payment-provider-fees/` and contain a visible
development-only label. The temporary script builds the real Vue components in memory,
intercepts an `.invalid` origin and uses Chromium; it starts no application server and
uses no credentials or production APIs. Its JSON reports are in `/data/tmp/` rather than
adding a task-specific test harness to the product.

Technical evidence: 22 payment/coupon/page/locale test files, 240 passing tests; 4GB
`vue-tsc --noEmit` passed. The earlier 2GB heap exhaustion remains recorded separately
and is not represented as a successful check. Final lint/design/build and the batch's
independent reviews are tracked by the integrating maintainer.

## Residual Risk

This review proves actual component rendering with bounded synthetic fixtures, not a
full payment deployment. The integration maintainer must verify the existing purchase
and settings routes, admin dialog keyboard behavior, coupon amount range and actual
order amount on the deployed candidate. A user-local production check is still required;
no real payment or production setting was touched for these artifacts.
