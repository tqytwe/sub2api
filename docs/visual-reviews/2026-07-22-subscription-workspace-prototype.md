# Visual Review: Subscription Storefront Prototype

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/payment/SubscriptionPlanDecisionShelf.vue"
  ],
  "routes_or_surfaces": [
    "/purchase?tab=subscription",
    "/admin/payment/plans storefront configuration reference"
  ],
  "languages_and_themes": ["zh-CN/light"],
  "states": [
    "default",
    "selected/default preview",
    "hover",
    "focus-visible",
    "disabled",
    "loading"
  ],
  "viewports": ["390x844", "1920x1080"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/subscription-workspace-prototype/prototype-subscription-storefront-v3-1920.png",
    "docs/visual-reviews/assets/subscription-workspace-prototype/prototype-subscription-storefront-v3-390.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/subscription-workspace-prototype/user-bad-subscription-1920.png",
    "docs/visual-reviews/assets/subscription-workspace-prototype/user-previous-subscription-style-1920.png",
    "docs/visual-reviews/assets/subscription-workspace-prototype/user-admin-subscription-config-1920.png",
    "docs/visual-reviews/assets/payment-shelf-display/after-payment-storefront-user-desktop.png",
    "docs/visual-reviews/assets/payment-subscription-console-shelf/after-payment-subscription-console-grid.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/subscription-workspace-prototype/prototype-subscription-storefront-v3-1920.png",
    "docs/visual-reviews/assets/subscription-workspace-prototype/prototype-subscription-storefront-v3-390.png"
  ],
  "commands": [
    "Read frontend/AGENTS.md and docs/FRONTEND_DESIGN_SYSTEM.md before any visible implementation.",
    "Inspected merged payment shelf history: PR #109 / commit 1ebe095cc and later layout override bba21d5a7.",
    "Copied the user-provided admin storefront configuration screenshot into the review assets.",
    "Generated static PNG prototype boards with PIL for 1920x1080 and 390x844.",
    "Implemented the approved v3 direction in SubscriptionPlanDecisionShelf.vue without changing PaymentView or admin storefront settings.",
    "pnpm --dir frontend exec vitest run src/components/payment/__tests__/SubscriptionPlanDecisionShelf.spec.ts --run"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "Shelf plan rows are real buttons, spotlight detail and subscribe remain buttons, and the component test covers preview plus emit behavior."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "The implemented shelf removes grid-card hover motion and oversized aspect poster behavior; feedback is limited to color, border and shadow classes."
    }
  },
  "residual_risks": [
    "Static review boards do not replace final browser screenshots or deployed acceptance.",
    "A real authenticated browser capture is still needed because this environment does not currently have Playwright/browser binaries available.",
    "Final browser acceptance must verify live admin storefront configuration, real plan cover images, mobile width and 1920px workspace width."
  ]
}
-->

## Scope

- Routes: `/purchase?tab=subscription`, using `/admin/payment/plans` storefront configuration only as the source-of-truth reference.
- Roles: logged-in regular user, with administrator configuration already represented by the existing storefront settings.
- Languages and themes: zh-CN light for this prototype gate. Dark theme and English are implementation acceptance items.
- Boundary: the user approved v3 before implementation. The actual code change is limited to `SubscriptionPlanDecisionShelf.vue`; backend storefront configuration and the surrounding payment flow are intentionally unchanged.

## Baseline

- Current bad behavior reported by the user: the subscription page became a flat console/grid style and lost the previously intended package storefront format.
- Historical source of truth: PR #109 / commit `1ebe095cc` added configurable payment storefront shelves, backend-admin shelf settings, and the user-facing left spotlight plus right scan-list display.
- Later regression source: `bba21d5a7` / `01912cfd8` changed `SubscriptionPlanDecisionShelf.vue` and `SubscriptionPlanCard.vue` into a console matrix to solve layout problems, but it overcorrected by replacing the desired storefront format.
- Important correction: `user-previous-subscription-style-1920.png` is an external-site format reference only. It must not be copied for brand, color, product metaphor, typography or image style.
- Admin source reference: `user-admin-subscription-config-1920.png` shows the already designed backend settings for shelves, tags, default recommendation and ordering.

## Prototype

- Current candidate v3:
  - Desktop: `docs/visual-reviews/assets/subscription-workspace-prototype/prototype-subscription-storefront-v3-1920.png`
  - Mobile: `docs/visual-reviews/assets/subscription-workspace-prototype/prototype-subscription-storefront-v3-390.png`
- v3 direction: keep the configured storefront structure from PR #109, with shelf chips at the top, a left default/recommended package spotlight, and a right selectable package list.
- v3 width rule: use the available logged-in app workspace width with normal layout gutter, not a centered narrow shell and not a giant poster image.
- Implementation result: the approved structure has been restored in `SubscriptionPlanDecisionShelf.vue`; the left spotlight column now has bounded responsive width and fixed responsive cover height instead of the old `aspect-[16/9]` behavior that could grow into a giant poster.
- Superseded:
  - v1 `prototype-subscription-workspace-*` is not approved because it turns the page into a pure grid workspace.
  - v2 `prototype-subscription-legacy-wide-*` is not approved because it leaned into the external site's visual style instead of the 极速蹬 style.

## Reuse Decision

- Reuse the existing payment flow in `PaymentView.vue`, including tab switching, order creation, detail modal and configured storefront shelves.
- Reuse the PR #109 storefront model: `storefront_config.shelves`, shelf order, `default_plan_id`, custom tags, plan cover image, product display name, badge and enabled state.
- Restore the useful parts of the original `SubscriptionPlanDecisionShelf.vue`: selected spotlight, right-side list, selected state and no separate full-page detail poster.
- Keep the current design-system constraints: operational card radius around 8px, subtle border/shadow, no external decorative style, no card nesting and no layout movement on hover.

## State Coverage

- Default: first enabled configured shelf is selected, and that shelf's default plan becomes the left spotlight.
- Selected: clicking a right-side plan updates the spotlight and selected border without route layout changes.
- Hover and active: shelf chips, plan list rows and primary actions may change border/color/shadow only; no scaling, image zoom, shimmer or layout shift.
- Focus-visible: shelf chips, list rows, detail actions and primary action must remain keyboard reachable with visible focus rings.
- Loading, disabled, empty, error and success: keep the existing payment flow states, but render them inside the same workspace width without collapsing to a centered unrelated card.

## Viewport Coverage

- Mobile 390px: shelf chips scroll horizontally; spotlight appears first; selected list item remains visible near the start of the list.
- Desktop 1920px: content fills the sidebar-right workspace with normal gutter; the left spotlight and right list use the available width without large side blanks.
- Tablet and 1280px: implementation must preserve the same structure, reducing the spotlight/list widths before stacking.
- 200% zoom and long content: implementation must avoid clipped price, long package name overlap and hidden primary actions.
- Dark theme and English: not represented in the static board, required after implementation.

## Evidence

- Updated prototype images: `prototype-subscription-storefront-v3-1920.png` and `prototype-subscription-storefront-v3-390.png`.
- Historical evidence reviewed: `after-payment-storefront-user-desktop.png`, `after-payment-storefront-admin-config.png`, and `after-payment-subscription-console-grid.png`.
- Commands run before this record:
  - `git log --oneline --decorate --all --grep='subscription' --grep='storefront' --grep='shelf' --grep='套餐'`
  - `git show --stat 1ebe095cc`
  - `git show --stat bba21d5a7`
  - `rg -n "storefront|shelf|订阅套餐|cover_image|SubscriptionPlanDecisionShelf" frontend/src backend docs`
  - static PNG generation with PIL.

## Residual Risk

- Known limitation: this record still uses static review boards for visual evidence because browser screenshot tooling is unavailable in this worktree.
- Required after implementation: browser screenshots for 390, 768, 1280 and 1920 widths, plus hover/focus-visible/payment-state checks against live or mocked API data.
- Deployment acceptance must verify the configured admin shelves still control the same user-facing shelf order, default plan and tags.
