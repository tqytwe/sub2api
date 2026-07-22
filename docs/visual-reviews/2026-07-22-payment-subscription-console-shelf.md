# Visual Review: payment-subscription-console-shelf

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/payment/SubscriptionPlanCard.vue",
    "frontend/src/components/payment/SubscriptionPlanDecisionShelf.vue"
  ],
  "routes_or_surfaces": [
    "/purchase subscription tab",
    "/purchase renewal plan modal"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "zh-CN/dark",
    "en-US/light"
  ],
  "states": [
    "configured shelf with four plans",
    "default recommended plan",
    "plan with cover thumbnail",
    "plan without cover placeholder",
    "hover border feedback",
    "focus-visible controls",
    "subscribe action",
    "details action"
  ],
  "viewports": [
    "360x800",
    "768x1024",
    "1280x760",
    "1920x1039"
  ],
  "artifact_mode": "static-review-board",
  "baseline_artifacts": [
    "docs/visual-reviews/assets/payment-subscription-console-shelf/before-payment-subscription-current.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/payment-subscription-console-shelf/after-payment-subscription-console-grid.png"
  ],
  "commands": [
    "python3 static PNG artifact generation for payment subscription console shelf review",
    "pnpm --dir frontend exec vitest run src/components/payment/__tests__/SubscriptionPlanDecisionShelf.spec.ts src/components/payment/__tests__/SubscriptionPlanCard.spec.ts --run"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "Details and subscribe remain real buttons or keyboard-triggerable regions, and focus-visible rings are preserved through card focus-within."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "The revised components remove hover translation, image scale, shimmer, and transition-all; feedback is limited to border, color, and shadow changes."
    }
  },
  "residual_risks": [
    "Static review board still needs browser screenshot and final acceptance with an authenticated account after deployment.",
    "Live plan names and cover assets may be longer than the fixture data, so deployed /purchase should be checked at 360, 768, 1280, and 1920 widths."
  ]
}
-->

## Scope

- Routes: `/purchase` subscription tab and the renewal plan modal inside the same payment flow.
- Roles: authenticated user.
- Languages and themes: Chinese light mode from the reported production screenshot; Chinese dark and English light are covered by shared classes and i18n keys but still need browser acceptance.

## Baseline

- Current behavior: the subscription tab used a two-column decision shelf with a left-side `spotlight` card. On a full-width console page, that card expanded into a giant product cover image, while the actual selectable plans were compressed into a narrow right rail.
- Baseline screenshot or recording: `before-payment-subscription-current.png`, captured from the reported production page.
- Inconsistencies observed: the page looked like a marketing landing section instead of a console purchase workflow; hover image scale and card translation also conflicted with the operational interface rules.

## Reuse Decision

- Shared layouts and components reused: existing `/purchase` `PaymentView`, existing shelf filters, platform color helpers, payment i18n keys, `Icon.vue`, and the existing subscribe/details event contract.
- New shared pattern, if any: the subscription decision shelf is now a console plan matrix with small thumbnails, stable data cards, compact metrics, and explicit action buttons.
- Design-system exception, if any: none.

## State Coverage

- Default: all plans in the selected shelf render as equal plan cards; the configured default plan becomes a recommended badge instead of a large hero.
- Hover and active: hover changes border or button color only; no layout movement, image zoom, or decorative shimmer remains.
- Focus-visible and keyboard: details remain keyboard-triggerable through the card detail region, subscribe remains a button, and cards use `focus-within` rings.
- Loading, disabled, empty, error and success: no new loading or error path was introduced; existing `/purchase` empty shelf, payment creation, toast, and payment status flows remain owned by `PaymentView`.

## Viewport Coverage

- Mobile: the grid collapses to one column and keeps 44px-class action targets through the existing button padding.
- Tablet: the grid moves to two columns and keeps plan metrics inside stable card dimensions.
- Desktop: the grid uses three columns by default and can expand to four columns on very wide screens, avoiding the old left hero and right rail split.
- Wide or short screen: the plan list is page-flow content rather than an inner 720px scroll rail.
- 200% zoom and reduced motion: text is not viewport-scaled, and motion is limited to color, border, and shadow transitions.

## Evidence

- Updated screenshot or recording: `after-payment-subscription-console-grid.png`, a static review board showing the intended console matrix structure.
- Automated visual or overlap checks: component tests assert the old `plan-spotlight` and `plan-list-item` hooks are gone, the old `aspect-[16/9]` and two-column hero classes are absent, plan cards render as a grid, the default plan is badged, and buttons still emit details/select events.
- Commands run: static PNG generation and targeted Vitest commands listed in the manifest.

## Residual Risk

- Known limitations: this record uses a static review board rather than an authenticated browser screenshot, so final visual acceptance still needs the deployed `/purchase` page with live data.
- Follow-up owner: release owner should verify after deployment in a real browser for light/dark mode, mobile/tablet/desktop widths, hover/focus states, plan details, and payment order creation.
