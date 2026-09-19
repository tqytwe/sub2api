# Visual Review: v0.2.7 Billing and Support Compatibility

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/views/user/PaymentView.vue", "frontend/src/components/common/SupportContactPanel.vue"],
  "routes_or_surfaces": ["/purchase", "support contact panel"],
  "languages_and_themes": ["zh-CN/light", "en-US/light", "zh-CN/dark", "en-US/dark"],
  "states": ["default", "loading", "disabled", "empty", "error", "success"],
  "viewports": ["360x800", "768x1024", "1280x820"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png", "docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png", "docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"],
  "commands": ["pnpm --dir frontend design:check", "pnpm --dir frontend typecheck", "pnpm --dir frontend vitest run src/views/user/__tests__/PaymentView.spec.ts"],
  "checks": {"keyboard": {"status": "not-applicable", "reason": "Static review board has no live focus traversal."}, "reduced_motion": {"status": "passed", "notes": "No new continuous motion was introduced."}},
  "residual_risks": ["Static artifacts are non-production evidence; browser payment acceptance remains required before release.", "No payment order or balance was changed by this compatibility work."]
}
-->

## Scope

- Routes: `/purchase`; shared support contact panel.
- Roles: authenticated user and guest-visible layout consumers.
- Languages and themes: Chinese default and explicit English routes, light and dark themes.

## Baseline

- Current behavior: purchase tabs now follow the server public `subscription_enabled` flag; an unavailable billing combination shows a localized empty state. Support contact rendering tolerates a minimal i18n test/runtime context.
- Baseline screenshot or recording: `docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png`.
- Inconsistencies observed: the merge had a subscription flag in backend settings but the purchase view always rendered the subscription tab.

## Prototype

- Prototype design image: `docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png` and `docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png`.
- Approval status: static engineering review only; no deployment approval.
- Scope boundary: no gateway payload, payment callback or account balance behavior was changed.

## Reuse Decision

- Shared layouts and components reused: existing AppLayout, PaymentMethodSelector, SubscriptionPlanCard, LoadingSpinner, i18n and support-contact helpers.
- New shared pattern, if any: no new visual component.
- Design-system exception, if any: none added.

## State Coverage

- Default: recharge and subscription tabs when enabled.
- Hover and active: existing shared buttons and tab styles.
- Focus-visible and keyboard: static evidence cannot execute traversal; browser pass remains required.
- Loading, disabled, empty, error and success: existing payment and support states remain in place; the zero-tab state is localized through `payment.billingUnavailable`.

## Viewport Coverage

- Mobile: 360x800.
- Tablet: 768x1024.
- Desktop: 1280x820.
- Wide or short screen: existing AppLayout owns the page shell.
- 200% zoom and reduced motion: reduced motion passes because no new continuous motion was introduced; zoom remains a browser follow-up.

## Evidence

- Updated screenshot or recording: the two static review-board PNGs listed in the manifest.
- Automated visual or overlap checks: `pnpm --dir frontend design:check`.
- Commands run: `pnpm --dir frontend typecheck`, `pnpm --dir frontend lint:check`, and targeted payment tests where dependencies are present.

## Residual Risk

- Known limitations: no browser, localhost service, container or production payment data was used; evidence is non-production only.
- Follow-up owner: release reviewer must verify recharge, subscription-only and disabled-billing states in both Chinese and English using the local browser acceptance procedure.
