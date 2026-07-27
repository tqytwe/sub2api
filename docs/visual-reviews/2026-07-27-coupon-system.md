# Visual Review: coupon system

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/views/auth/WechatPaymentCallbackView.vue",
    "frontend/src/views/admin/PromoCodesView.vue",
    "frontend/src/views/public/BlindboxView.vue",
    "frontend/src/views/public/QuizQuestView.vue",
    "frontend/src/views/user/PaymentView.vue",
    "frontend/src/views/user/PlayHubView.vue",
    "frontend/src/views/user/paymentWechatResume.ts",
    "frontend/src/components/payment/paymentFlow.ts",
    "frontend/src/components/payment/PaymentMethodSelector.vue",
    "frontend/src/views/user/WalletView.vue",
    "frontend/src/components/admin/AdminCouponOperations.vue",
    "frontend/src/components/coupon/CouponSelector.vue",
    "frontend/src/components/coupon/CouponWalletPanel.vue",
    "frontend/src/components/play/CouponRewardCard.vue"
  ],
  "routes_or_surfaces": ["/purchase", "/wallet#coupons", "/play/blindbox", "/play/quiz", "/admin/promo-codes"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "loading", "empty", "quote error", "discounted order", "zero-pay completed order", "coupon reward", "order-in-progress", "used and expired", "disabled", "focus-visible"],
  "viewports": ["360x800", "768x900", "1280x800", "1600x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/coupon-system/prototype-coupon-operations.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/coupon-system/baseline-coupon-operations.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/coupon-system/updated-coupon-operations.png"
  ],
  "commands": [
    "firefox --headless --no-remote --profile <temporary-profile> --window-size 1440,1100 --screenshot docs/visual-reviews/assets/coupon-system/baseline-coupon-operations.png file://.../prototype-coupon-operations.html?mode=baseline",
    "firefox --headless --no-remote --profile <temporary-profile> --window-size 1440,1100 --screenshot docs/visual-reviews/assets/coupon-system/prototype-coupon-operations.png file://.../prototype-coupon-operations.html?mode=prototype",
    "firefox --headless --no-remote --profile <temporary-profile> --window-size 1440,1100 --screenshot docs/visual-reviews/assets/coupon-system/updated-coupon-operations.png file://.../prototype-coupon-operations.html?mode=updated",
    "cd frontend && pnpm vitest run src/components/coupon/__tests__ src/components/payment/__tests__/paymentFlow.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/views/public/__tests__/BlindboxView.spec.ts",
    "cd frontend && pnpm typecheck",
    "cd frontend && pnpm design:check",
    "cd frontend && pnpm lint:check"
  ],
  "checks": {
    "keyboard": { "status": "passed", "notes": "Tabs use native buttons and roles; coupon selection uses radio inputs; links remain keyboard reachable." },
    "reduced_motion": { "status": "passed", "notes": "No new continuous animation was added to coupon operations, wallet, payment, blindbox, or quiz surfaces." }
  },
  "residual_risks": [
    "The review artifacts are static review boards rather than authenticated browser captures because the local API has no coupon fixtures. Release acceptance must verify actual coupon, quote, payment callback, and administrator data in a user browser after deployment."
  ]
}
-->

## Scope

- Routes: payment recharge/subscription, wallet coupon package, blindbox and quiz result states, and the existing administrator promo-code route.
- Roles: signed-in user and administrator; guest play pages retain their existing register call to action.
- Languages and themes: Chinese and English copy with existing light/dark semantic class patterns.

## Baseline

- Current behavior: payment had no coupon selection, wallet had no coupon package, game rewards only described balance, and administrator promo codes only managed registration bonus codes.
- Baseline screenshot or recording: `baseline-coupon-operations.png` is a Firefox-rendered static board for the former split registration-code, checkout, blindbox, and quiz operations. It explicitly records the missing checkout coupon entry and balance-only activity rewards.
- Inconsistencies observed: there was no visible path from a coupon reward to a checkout quote, and no single administrator entry for templates, issuance, and prize pools.

## Prototype

- Prototype design image: `prototype-coupon-operations.png`, rendered from `prototype-coupon-operations.html?mode=prototype`, shows the proposed single operations surface, coupon pool weighting, and checkout loop.
- Approval status: implementation follows the requested unified `/admin/promo-codes` entry, fixed outer reward splits, and an account-bound coupon payment loop.
- Scope boundary: preserve registration promo-code behavior; add order coupons without a new navigation item or a parallel admin page.

## Reuse Decision

- Reused `AppLayout`, `TablePageLayout`, `DataTable`, `BaseDialog`, `ConfirmDialog`, existing payment method controls, existing wallet card structure, `Icon.vue`, public play panels, and `RewardCelebrationOverlay`.
- Added focused shared components for coupon selection, coupon wallet records, reward terms, and admin operations instead of duplicating coupon formatting across pages.
- No design-system exception is required. Refresh controls remain static while loading, so no continuous animation was introduced.

## State Coverage

- Default: users select a coupon in recharge or subscription checkout and see the server-provided discount, fee, and final payment amount.
- Loading and disabled: coupon package, candidate selection, quote, admin tables, and batch/pool actions retain loading or disabled states without layout changes.
- Pool safeguards: the selected fallback coupon remains an enabled pool entry, while its weight is fixed at 1 and excluded from the ordinary 10000-weight draw, and its stock, per-user cap, and time-window controls are cleared and disabled. It is issued only when every ordinary coupon is unavailable. Only draft pool versions expose a destructive action and require confirmation; published and retired versions never expose deletion.
- Empty and error: no available coupon, coupon list load failure, and order-ineligible quote errors stay in the same region with a next action.
- Success: blindbox and quiz coupon outcomes display name, benefit, scopes, order threshold, absolute expiry, and applicable checkout links. Quiz answers are submitted as one full set; on its 20% balance branch, any nonzero score receives the complete original reward (default $0.50), never a per-correct-answer amount. A completed full-discount order bypasses the payment provider and payment-status panel, refreshes account data, and opens the existing result route.
- Order in progress: a coupon locked by a payment order appears in a separate amber "order pending" wallet tab, explains that an unpaid or cancelled order returns it to available, and offers no checkout action.
- Used, expired, and voided: wallet tabs distinguish terminal coupon states from available and order-pending coupons.
- Hover, focus-visible and keyboard: tabs, radio controls, native form controls, dialog actions, tables, links, and buttons use existing focus and disabled conventions.

## Viewport Coverage

- Mobile: tabs scroll horizontally; coupon controls and action links wrap without hiding expiry or threshold details.
- Tablet: payment selectors remain within existing stacked checkout cards; wallet records preserve two action targets.
- Desktop: payment summary stays in the existing sticky side column and admin operations preserve the existing full-width table workspace.
- Wide or short screen: no new page width, centering, full-height, or page-scroll ownership was added.
- 200% zoom and reduced motion: text wraps in coupon rows and no new continuous motion is present.

## Evidence

- Updated screenshot or recording: `updated-coupon-operations.png`, rendered from `prototype-coupon-operations.html?mode=updated`, highlights the implemented 60% coupon / 40% original blindbox split, 80% coupon / 20% original quiz balance split, coupon template management, and guarded checkout loop.
- Automated visual or overlap checks: design governance validates the prototype, baseline, updated artifact, changed-file mapping, motion rule, and required review sections; the three PNGs have distinct SHA-256 hashes and were manually inspected at 1440x1100.
- Commands run: three Firefox headless static-board captures, targeted coupon/payment/blindbox tests, `pnpm typecheck`, `pnpm design:check`, and `pnpm lint:check`.

## Residual Risk

- Static review boards do not prove authenticated API data, payment-provider callback behavior, or production browser rendering.
- Follow-up owner: release owner verifies a real user coupon, recharge coupon, subscription coupon, blindbox coupon, quiz coupon, and administrator pool publish flow in the user's local browser after deployment.
