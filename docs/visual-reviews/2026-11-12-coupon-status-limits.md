# Coupon status and issuance-limit clarification

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/AdminCouponOperations.vue",
    "frontend/src/components/admin/CouponCampaignPanel.vue",
    "frontend/src/i18n/locales/zh/couponCampaign.ts",
    "frontend/src/i18n/locales/zh/legacy/admin-resources.ts",
    "frontend/src/i18n/locales/zh/legacy/user-misc.ts",
    "frontend/src/i18n/locales/en/couponCampaign.ts",
    "frontend/src/i18n/locales/en/legacy/admin-resources.ts",
    "frontend/src/i18n/locales/en/legacy/user-misc.ts",
    "frontend/src/i18n/__tests__/couponColdLocale.spec.ts"
  ],
  "routes_or_surfaces": [
    "/wallet",
    "/purchase",
    "/admin/promo-codes"
  ],
  "languages_and_themes": [
    "zh/light",
    "zh/dark",
    "en/light",
    "en/dark"
  ],
  "states": [
    "valid coupon status",
    "unknown future status",
    "limit help text",
    "keyboard input focus",
    "existing async/empty/error states via regression tests"
  ],
  "viewports": [
    "360x1000",
    "768x1000",
    "1280x1000",
    "1920x1000"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-12-coupon-status-limits/prototype.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-12-coupon-status-limits/baseline-wallet.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-12-coupon-status-limits/wallet-360-light.png",
    "docs/visual-reviews/assets/2026-11-12-coupon-status-limits/templates-360-light.png",
    "docs/visual-reviews/assets/2026-11-12-coupon-status-limits/policy-1280-dark.png",
    "docs/visual-reviews/assets/2026-11-12-coupon-status-limits/checkout-1280-dark.png"
  ],
  "commands": [
    "node /data/tmp/coupon-followup-render.mjs # Chromium browser, compiled Vue/Tailwind with synthetic APIs",
    "pnpm --dir frontend exec vitest run src/i18n/__tests__/couponColdLocale.spec.ts src/i18n/__tests__/lazyLocaleScope.spec.ts src/components/coupon/__tests__ src/components/admin/__tests__/CouponCampaignPanel.spec.ts",
    "make test",
    "make build",
    "./scripts/check-fork-integrity.sh"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "Existing form inputs retain keyboard focus; no new controls."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "Browser matrix uses reduced motion; no animation changes."
    }
  },
  "residual_risks": [
    "Compiled-component browser checks use synthetic APIs and separate user/admin locale scopes. They do not replace user-local production acceptance. Release evidence belongs to the associated PR."
  ]
}
-->

## Scope

Restore existing coupon status labels and clarify issuance limits. No gameplay or financial policy change.

## Baseline

Base: production merge `871c5a81c506b843c76b5753f55dd9f34f9396d6` (#425). Isolated branch: `fix/coupon-status-play-audit-20261112`. Supplied wallet screenshot shows a valid coupon as unknown. Only the sanitized wallet crop is retained. Existing wallet, selector, campaign form and shared dialog/input styles are reused; no routes, financial rules, API contracts or schema change.

## Prototype

The static prototype was produced before editing. It clarifies all-source template issuance versus code-only claims and restores existing user coupon state labels. User approval covers this reported bugfix scope. The two backend limits remain independent: code claims consume both quotas; manual and Play issuance consume template stock. No new visual pattern or design exception.

## State Coverage

Cold `/wallet` and `/purchase` tests use the real lazy locale loader, without administrator messages. Removing the fix produces four missing-key failures; adding it restores all five existing status labels in both languages. An unknown future enum remains unknown. Existing coupon tests cover loading, disabled, empty, errors, success and session changes.

Compiled Vue/Tailwind browser checks separately load user and admin locale fragments, preventing admin text leakage from masking the defect. Matrix: four surfaces × four widths × two languages × two themes. Five valid states and unknown fallback, input keyboard focus, reduced motion, overflow and runtime errors are checked. Browser fixtures are synthetic; the prototype is a static board, not production evidence. Shared hover/active/dialog behavior is unchanged; 200% zoom is not separately exercised for this text-only correction.

## Reuse Decision

Keep the existing wallet/selector and campaign/template forms, shared input classes and dialog. No new component, page, shell or visual exception.

## Viewport Coverage

64 compiled-component checks: 360, 768, 1280 and 1920 pixels; zh/en; light/dark. No horizontal page overflow or runtime errors. Mobile template screenshots scroll the existing dialog to show the affected limit field.

## Evidence

`assets/2026-11-12-coupon-status-limits/browser-checks.json` records the matrix and interactions. The manifest links the sanitized baseline, static prototype and four actual component captures. Focused tests: 6 files / 57 tests pass. Design governance, full `make test` (backend tests, golangci-lint: 0 issues; frontend lint/typecheck and 551 files / 4,275 tests), and full `make build` pass. Existing suite skips: 3 files / 8 tests. Fork integrity, the complete `make test-backend-unit` suite and the production-equivalent `CGO_ENABLED=0 go build -tags=embed` build also pass. One embed invocation from the repository root failed because go.mod lives in backend; the same invocation from backend passed. The first aggregate test process was interrupted (143); it was not counted as a pass, and the complete command was rerun successfully.

## Production reward investigation

Read-only PostgreSQL reconciliation on 2026-11-12 found the reported 7-unit reward consists of a 2-unit configured random pool draw and a 5-unit day-30 milestone. Thirty consecutive daily records and the immutable rule snapshot support the streak. The two wallet entries match their reward-ledger idempotency keys exactly once. The current milestone settings were changed after that reward (00:05 on Nov 12 versus 22:39 on Nov 11, Asia/Shanghai). No duplicate daily/streak reward was found for the reported account. All 23 quiz attempt amounts match their corresponding reward entries; this is a targeted account check, not a whole-site audit. No production balances, settings or history were changed; transactions ended with ROLLBACK. No account identifiers or raw production data are retained here.

## Review and delivery

Specification review: restore valid statuses on cold user routes, preserve genuine unknown fallback, explain both caps, investigate sign-in/quiz against historical configuration and ledger. Code review: locale keys are placed in the user scope already loaded by consumers; admin data is not added to user payloads; source counters and monetary calculations are untouched. Required test/build/Fork results and exact PR/deployment SHA are recorded in the associated PR after gates finish. Final user-local wallet/admin acceptance remains pending; server screenshots are supplementary evidence only.

## Residual Risk

User-local production wallet, checkout and administrator acceptance remain pending. Server screenshots and targeted production ledger reconciliation cannot establish whole-site gameplay correctness. There is no change to payment/reward algorithms or production data.
