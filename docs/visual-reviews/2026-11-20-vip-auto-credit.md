# VIP automatic credit

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/UsersView.vue",
    "frontend/src/components/admin/user/UserAutoCreditModal.vue",
    "frontend/src/components/admin/user/UserBalanceHistoryModal.vue",
    "frontend/src/views/admin/settings/EmailTemplateEditor.vue",
    "frontend/src/views/user/WalletView.vue",
    "frontend/src/i18n/locales/zh/autoCredit.ts",
    "frontend/src/i18n/locales/en/autoCredit.ts",
    "frontend/src/i18n/locales/zh/wallet.ts",
    "frontend/src/i18n/locales/en/wallet.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts"
  ],
  "routes_or_surfaces": [
    "/admin/users existing user action menu, balance history and automatic credit dialog",
    "/wallet existing balance history",
    "existing email template event selector"
  ],
  "languages_and_themes": [
    "zh/light",
    "zh/dark",
    "en component checks"
  ],
  "states": [
    "configuration and outstanding balance",
    "cancel and confirm full collection",
    "filtered wallet credit history",
    "empty, validation and failed request states covered by component tests"
  ],
  "viewports": [
    "360x900",
    "768x900",
    "1280x900",
    "1920x900"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-20-vip-auto-credit/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-20-vip-auto-credit/baseline-actual-1280.png",
    "docs/visual-reviews/assets/2026-11-20-vip-auto-credit/baseline-wallet-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-20-vip-auto-credit/updated-actual-360.png",
    "docs/visual-reviews/assets/2026-11-20-vip-auto-credit/updated-actual-768.png",
    "docs/visual-reviews/assets/2026-11-20-vip-auto-credit/updated-actual-1280.png",
    "docs/visual-reviews/assets/2026-11-20-vip-auto-credit/updated-actual-1920.png",
    "docs/visual-reviews/assets/2026-11-20-vip-auto-credit/updated-wallet-1280.png"
  ],
  "commands": [
    "Playwright Chromium against built routes with disposable fixture responses",
    "pnpm design:check",
    "pnpm lint:check",
    "pnpm typecheck"
  ],
  "checks": {
    "viewport_overflow": {
      "status": "passed",
      "scenarios": 4
    },
    "reduced_motion": {
      "status": "passed"
    },
    "keyboard": {
      "status": "passed"
    }
  },
  "residual_risks": [
    "Production browser acceptance remains pending. Fixture settlement did not change a live account."
  ]
}
-->

## Scope

Add automatic credit and collection to existing user management and show its separate entry in the wallet ledger.

## Baseline

The existing user action menu and balance history provide the account entry points. The wallet already filters the shared balance ledger by source.

## Prototype

The annotated baseline marks the location of the action and the outstanding balance shortcut.

## Reuse Decision

Use the existing user details dialog, controls, and balance history. The wallet gains a source choice, and the email editor gains an event label.

## State Coverage

Chromium opened a receivable, canceled the confirmation, then confirmed it on an isolated fixture at four widths. The wallet applied the credit filter and showed the entry. Component tests cover loading and configuration validation.

## Viewport Coverage

The existing user list and confirmation were exercised at 360, 768, 1280 and 1920 pixels. The wallet filter was checked at 1280 pixels.

## Evidence

The baseline and updated images use production builds with local, disposable fixture responses. The prototype annotates the existing list layout.

## Residual Risk

Authenticated production acceptance is pending. The review exercised settlement against a fixture account.
