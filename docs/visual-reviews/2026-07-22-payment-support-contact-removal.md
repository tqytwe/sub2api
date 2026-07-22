# Visual Review: payment-support-contact-removal

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/views/user/PaymentView.vue"],
  "routes_or_surfaces": ["/payment recharge subscription"],
  "languages_and_themes": ["zh-CN/light", "en-US/dark"],
  "states": ["select phase", "plan shelf", "help text", "floating support button"],
  "viewports": ["360x800", "1280x800"],
  "artifact_mode": "static-review-board",
  "baseline_artifacts": [
    "docs/visual-reviews/assets/payment-support-contact-removal/baseline-payment-support-contact.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/payment-support-contact-removal/updated-payment-support-removed.png"
  ],
  "commands": [
    "node generated static review board PNGs",
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend test:run frontend/src/views/user/__tests__/PaymentView.spec.ts"
  ],
  "checks": {
    "keyboard": { "status": "passed" },
    "reduced_motion": { "status": "passed" }
  },
  "residual_risks": [
    "Static review boards are not browser screenshots; final browser acceptance should confirm the live recharge/subscription page."
  ]
}
-->

## Scope

- Routes: authenticated recharge/subscription page.
- Roles: normal signed-in user selecting a top-up amount or subscription plan.
- Languages and themes: zh-CN light is the reported screen; en-US dark keeps the same component removal because no copy is added.

## Baseline

- Current behavior: after the plan shelf and optional checkout help block, the page renders an inline `SupportContactPanel` with customer-service methods.
- Baseline screenshot or recording: `docs/visual-reviews/assets/payment-support-contact-removal/baseline-payment-support-contact.png` is a static review board showing the previous inline support area.
- Inconsistencies observed: the inline support panel duplicates the customer-service floating button in the lower-right corner and makes the recharge/subscription page unnecessarily long.

## Reuse Decision

- Shared layouts and components reused: existing payment page layout, plan shelf, help text/image block and floating support button remain unchanged.
- New shared pattern, if any: none.
- Design-system exception, if any: static board evidence is used as development evidence; browser acceptance remains a rollout check.

## State Coverage

- Default: select phase without a selected plan no longer renders the inline support panel.
- Hover and active: plan cards, payment controls and the floating support button keep their existing hover/active behavior.
- Focus-visible and keyboard: no focus handling is changed; removing the inline panel removes its copy/action controls from tab order.
- Loading, disabled, empty, error and success: payment loading, checkout, QR, result and error panels are untouched.

## Viewport Coverage

- Mobile: removal frees vertical space below the selectable payment content.
- Tablet: optional help text/image remains the only bottom helper content on the page.
- Desktop: the large two-column QR support display is removed from the main page body.
- Wide or short screen: the lower-right floating support button remains the customer-service entry point.
- 200% zoom and reduced motion: no animation or zoom-dependent layout code changes in this patch.

## Evidence

- Updated screenshot or recording: `docs/visual-reviews/assets/payment-support-contact-removal/updated-payment-support-removed.png` is a static review board showing the payment page without the inline support section.
- Automated visual or overlap checks: `pnpm --dir frontend design:check` validates the structured evidence and artifact files.
- Commands run: `pnpm --dir frontend design:check`; `pnpm --dir frontend test:run frontend/src/views/user/__tests__/PaymentView.spec.ts`.

## Residual Risk

- Known limitations: static review boards are not rendered browser proof; final browser screenshot or user acceptance should confirm production after deployment.
- Follow-up owner: release owner during Sub2API frontend deployment verification.
