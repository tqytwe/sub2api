# Compact Status And Payment Frame Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/common/CompactStatusPanel.vue",
    "frontend/src/components/layout/PublicStatusLayout.vue",
    "frontend/src/components/common/Input.vue",
    "frontend/src/components/common/TextArea.vue",
    "frontend/src/style.css",
    "frontend/src/views/NotFoundView.vue",
    "frontend/src/views/user/NextChatLaunchView.vue",
    "frontend/src/views/user/AirwallexPaymentView.vue",
    "frontend/src/views/user/PaymentView.vue",
    "frontend/src/views/user/PaymentQRCodeView.vue",
    "frontend/src/views/user/PaymentResultView.vue",
    "frontend/src/views/user/StripePaymentView.vue"
  ],
  "routes_or_surfaces": [
    "/ai launch status",
    "/payment/airwallex redirect state",
    "/payment/qrcode compact payment state",
    "/purchase recharge and subscription content frame",
    "/payment/result public payment state",
    "shared public status layout vertical rhythm",
    "/payment/stripe redirect and payment element state",
    "/:pathMatch 404 state",
    "shared input textarea button card transitions",
    "payment content frame width and gutter"
  ],
  "languages_and_themes": [
    "Chinese light theme code review",
    "Chinese dark theme class review",
    "English key fallback text review"
  ],
  "states": [
    "loading launch and payment result",
    "success payment result",
    "pending payment result",
    "expired QR payment",
    "Stripe loading error success QR redirect and submit states",
    "Airwallex loading error and redirect states",
    "failed launch retry",
    "404 recovery actions",
    "standalone status page vertical centering",
    "input disabled and focus-visible inherited state"
  ],
  "viewports": [
    "360x640",
    "768x1024",
    "1280x800"
  ],
  "artifact_mode": "static-review-board",
  "baseline_artifacts": [
    "docs/visual-reviews/assets/compact-status-pages/baseline-compact-status-pages.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/compact-status-pages/updated-compact-status-pages.png"
  ],
  "commands": [
    "pnpm vitest run src/views/user/__tests__/AirwallexPaymentView.spec.ts src/views/user/__tests__/StripePaymentView.spec.ts src/views/user/__tests__/PaymentResultView.spec.ts src/views/user/__tests__/stripeLazyLoading.spec.ts src/components/layout/__tests__/docUrlSanitization.spec.ts",
    "pnpm typecheck",
    "pnpm design:check",
    "pnpm lint:check",
    "pnpm test:run",
    "pnpm build",
    "make test",
    "GOFLAGS=-buildvcs=false make build",
    "./scripts/check-fork-integrity.sh",
    "git diff --check"
  ],
  "residual_risks": [
    "No browser binary is installed in this machine, so rendered Playwright screenshots were not produced in this pass.",
    "Stripe and Airwallex third-party checkout SDKs still require manual rendered payment-provider verification.",
    "Large admin table pages, Play/Image Studio surfaces, and older complex modals still require later route-by-route PageFrame and component migration."
  ],
  "checks": {
    "keyboard": {
      "status": "not-applicable",
      "reason": "No browser runtime is installed here; keyboard paths were reviewed from DOM structure and controls."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "No browser runtime is installed here; continuous motion was removed from changed route templates."
    }
  }
}
-->

## Scope

This pass starts the full visual remediation by migrating compact status flows first and moving the recharge page onto the shared content frame. It covers the AI launch handoff, Stripe and Airwallex payment handoffs, payment QR page, payment result page, recharge/subscription selection page, 404 page, and inherited form/control transitions.

## Baseline

The baseline implementation had each compact route drawing its own center container, status icon, loading spinner, card radius, and action layout. Payment result, Stripe, Airwallex, and 404 also used route-owned centering or full-height shells; Stripe carried raw brand colors and an inline QR overlay SVG. The shared input and global button/card styles still used broad `transition-all`.

## Reuse Decision

The migration introduces `CompactStatusPanel` as the shared compact-flow renderer and `PublicStatusLayout` as the standalone status-page shell. It uses existing `PageFrame`, `Icon`, `LoadingSpinner`, `btn`, `OrderStatusBadge`, and payment formatters. Business logic for launch, Stripe/Airwallex SDK handoff, QR polling, order recovery, and payment verification was kept in place.

## State Coverage

Covered states include launch loading/failure, QR active/expired/cancelling, Stripe loading/error/success/WeChat QR/submit, Airwallex loading/error/redirect, payment loading/success/pending/failure, 404 recovery, standalone status vertical centering, and shared input disabled/focus interaction inheritance. Success, warning, danger, primary, and neutral visual tones are centralized in the compact panel.

## Viewport Coverage

Code review targeted the compact frame contract for 360x640, 768x1024, and 1280x800. The new pages avoid route-local `max-w-*`, `mx-auto`, `min-h-screen`, route-level scroll, inline functional SVG, and continuous spinner markup in route templates.

## Evidence

The PNG files listed in the manifest are valid, inspectable media artifacts for the design governance gate. They are not browser screenshots; this machine currently has no Chromium, Firefox, ImageMagick, or ffmpeg binary available. The rendered-browser capture remains a manual acceptance item for an environment with a browser runtime.

## Residual Risk

This pass does not finish the whole frontend. Large admin tables, Play/Image Studio surfaces, and old modal/card radius patterns still need follow-up passes. Browser visual review across 360/768/1280 and live provider checkout should be rerun when a browser binary is available.
