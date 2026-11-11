# Coupon claim and issuance closed loop

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/coupon.ts",
    "frontend/src/api/coupon.ts",
    "frontend/src/components/admin/AdminCouponOperations.vue",
    "frontend/src/components/admin/CouponAudienceForm.vue",
    "frontend/src/components/admin/CouponCampaignPanel.vue",
    "frontend/src/components/admin/__tests__/AdminCouponOperations.spec.ts",
    "frontend/src/components/admin/__tests__/CouponAudienceForm.spec.ts",
    "frontend/src/components/admin/__tests__/CouponCampaignPanel.spec.ts",
    "frontend/src/components/coupon/CouponCodeClaim.vue",
    "frontend/src/components/coupon/CouponSelector.vue",
    "frontend/src/components/coupon/CouponWalletPanel.vue",
    "frontend/src/components/coupon/__tests__/CouponCodeClaim.spec.ts",
    "frontend/src/components/coupon/__tests__/CouponSelector.spec.ts",
    "frontend/src/components/coupon/__tests__/CouponWalletPanel.spec.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/core.ts",
    "frontend/src/i18n/locales/en/couponCampaign.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/core.ts",
    "frontend/src/i18n/locales/zh/couponCampaign.ts",
    "frontend/src/types/coupon.ts",
    "frontend/src/types/couponCampaign.ts",
    "frontend/src/utils/couponErrors.ts",
    "frontend/src/views/user/PaymentView.vue"
  ],
  "routes_or_surfaces": [
    "/admin/promo-codes templates and issuance tabs; existing coupon policy dialog",
    "/wallet coupon panel; /payment recharge and subscription selectors"
  ],
  "languages_and_themes": [
    "zh/light",
    "zh/dark",
    "en/light",
    "en/dark"
  ],
  "states": [
    "default",
    "hover via shared controls",
    "keyboard focus and Enter claim",
    "loading/disabled via async component tests",
    "empty wallet",
    "code error",
    "claim and batch success",
    "fixed preview recipient change",
    "used/locked coupon replay"
  ],
  "viewports": [
    "360x1000",
    "768x1000",
    "1280x1000",
    "1920x1000"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-11-coupon-closed-loop/prototype.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-11-coupon-closed-loop/baseline-admin.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-11-coupon-closed-loop/templates-360-dark.png",
    "docs/visual-reviews/assets/2026-11-11-coupon-closed-loop/issue-1280-light.png",
    "docs/visual-reviews/assets/2026-11-11-coupon-closed-loop/policy-360-light.png",
    "docs/visual-reviews/assets/2026-11-11-coupon-closed-loop/wallet-1280-dark.png",
    "docs/visual-reviews/assets/2026-11-11-coupon-closed-loop/checkout-success.png",
    "docs/visual-reviews/assets/2026-11-11-coupon-closed-loop/checkout-error.png",
    "docs/visual-reviews/assets/2026-11-11-coupon-closed-loop/issue-preview.png",
    "docs/visual-reviews/assets/2026-11-11-coupon-closed-loop/wallet-empty.png"
  ],
  "commands": [
    "node /data/tmp/coupon-render-review.mjs # Chromium browser; compiled Vue/Tailwind; synthetic APIs",
    "pnpm --dir frontend exec vitest run src/components/coupon/__tests__ src/components/admin/__tests__/AdminCouponOperations.spec.ts src/components/admin/__tests__/CouponCampaignPanel.spec.ts src/components/admin/__tests__/CouponAudienceForm.spec.ts",
    "make test",
    "make build",
    "./scripts/check-fork-integrity.sh"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "Actual Chromium keyboard focus and Enter submitted claim; labels, fieldsets and existing dialog controls reused."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "All 80 browser scenarios emulate reduced motion; no new animation."
    }
  },
  "residual_risks": [
    "Compiled component screenshots use synthetic fixtures and API stubs, never production data. Server browser evidence does not replace user-local production acceptance after deployment. Full test/build/release state is recorded separately below."
  ]
}
-->

## Scope

User approved the reviewed existing-system plan. Reuse PromoCodesView tabs, AdminCouponOperations, CouponSelector, CouponWalletPanel, PaymentView, DataTable, BaseDialog, Icon and existing button/input styles. No new route or marketing platform. Review adjacent UsersView, existing user search/filters, wallet and checkout.

## Baseline

The supplied real administrator screenshot is cropped to remove account identity, balances and user emails. Current issuance requires pasted IDs; code claim is absent; wallet jump does not carry coupon ID; statistics sum only the current page.

## Prototype

assets/2026-11-11-coupon-closed-loop/prototype.png is a static annotated review board produced before any frontend changes. It records the approved boundary and component reuse, not a functioning page or production evidence.

## Reuse Decision

Extend existing management and payment surfaces. Shared claim control, audience form and error mapping avoid duplicated rules. Server performs authoritative eligibility, stock, quote and accounting calculations.

## State Coverage

80 actual compiled Vue/Tailwind browser cases cover five surfaces, four widths, two themes and two languages, without page overflow or runtime errors. Browser interactions cover fixed preview/confirm, keyboard claim, success, error and empty wallet. Component tests cover async loading/disabled, audience invalidation, invalid pasted IDs, account switching, replay states, quote rejection, and the explicit “discount X%, pay Y%” percentage wording. Existing shared controls retain hover/focus/dialog behavior.

## Viewport Coverage

Actual component captures: 360, 768, 1280 and 1920px; light/dark and zh/en. browser-checks.json records geometry and interaction checks. These are isolated component captures with synthetic APIs, not production route screenshots.

## Specification and Code Review

Specification review covers the existing management → fixed-recipient preview → confirmation → wallet/code claim → recharge/subscription quote → existing order lifecycle → full-filter reconciliation and event-history flow. Every template gets a stable closed-by-default code; opening requires finite limits and an end time. Manual and code limits are shared while Play rewards retain existing rules. No new route or marketing subsystem.

Code-quality review covers template/policy locks, transaction rollback, fixed preview recipients, actor ownership, concurrent claim/replay, additional API-key group bindings, expiry events, refund rounding and currency separation, guest/user authorization, and stale frontend responses across sessions. Regressions found during review were reproduced with failing tests and fixed. Percentage labels state both reduction and payable percentages without changing calculations.

## Evidence

- Targeted frontend coupon/payment regressions: 78 tests pass.
- PostgreSQL integration: 10 campaign cases pass, including migration backfill/repeat, existing-coupon preservation, preview/replay, concurrency, stock rollback, audience filters with additional API-key bindings, expiry events and refund/currency reconciliation.
- Fork integrity: passed against current task tree; protected backend/frontend behaviors and documentation checks pass.
- Full ordinary backend tests and `make test-backend-unit` pass. The initial aggregate `make test` stopped on formatting errors; those were corrected. The complete backend test command and subsequent full `golangci-lint run ./...` pass (`0 issues`), and the full frontend target passes separately.
- Full frontend lint, design governance and typecheck pass. `make test-frontend`: 550 test files / 4,271 tests pass; 3 files / 8 tests are skipped by the existing suite. `make build` passes for backend and frontend. An additional production-equivalent backend build with `-tags=embed` passes against the newly built frontend assets; the binary build metadata confirms `-tags=embed` and `CGO_ENABLED=0`.
- Browser component evidence: 80 scenarios and five interactions, no overflow/runtime errors. Synthetic fixtures only.
- Refetched `origin/play/main` remains `ecbdfcb33c47de9122e566848491cab61acf5469`; migration 353 is unused there. No historical migration SQL is modified.
- Resource note: parallel compilation caused memory pressure; one static-analysis run was killed by the system and the competing build was deliberately terminated. Static analysis and both ordinary/embedded builds were rerun successfully with sequential lower-resource settings. Interrupted attempts are not counted as passes.
- Publication authorized by the user on 2026-11-11. Pre-deployment read-only check: production deployment `6af3134b03a5d9114765ce94` runs base `ecbdfcb33c47de9122e566848491cab61acf5469`; health HTTP 200; 445 applied migrations with latest 352; no 353 collision; 21 templates and 3,137 coupons; no campaign tables; UUID generation available. Transaction ended with ROLLBACK. Codes remain closed by default after the additive migration.
- No production coupons have been distributed by this task. PR/SHA/CI/deployment outcomes are maintained in the associated PR to avoid another production push for status-only edits.

## Residual Risk

Prototype is a static review board; updated artifacts are real Chromium captures of compiled components. Server gates are verified on this isolated publication candidate. The local preview/DB data and generated build outputs are removed after checks. PR CI, deployment, production reconciliation and user-local three-role production acceptance remain pending at commit time; no production completion is claimed.

## Publication Gate Follow-up

The first PR security scan on candidate `c33a74119ace2433206217e52a965c39def8a3e6` found standard-library advisory `GO-2026-6859` in Go 1.27.3 (`net/http`), fixed by Go 1.27.4. That candidate is not merged or deployed. Obsolete in-progress CI runs are canceled to avoid duplicated work. The same PR upgrades only the Go module minimum, Docker Go base and security version assertion to 1.27.4; no business scope changes. Toolchain verification, affected backend gates and production-equivalent build are rerun before publishing the correction. Final commit/CI/deployment evidence remains in the PR.

Go 1.27.4 follow-up gates passed before the correction commit: `govulncheck ./...` reports 0 callable vulnerabilities; complete ordinary backend tests and golangci-lint (`0 issues`), full `-tags=unit` suite, Fork integrity, `make build`, and the `CGO_ENABLED=0 -tags=embed` production-equivalent build all pass. Existing full frontend and PostgreSQL integration results above remain applicable because business, frontend and migration sources did not change. Official Docker library catalog SHA `8fd07b0ea5154a2ce4653a5d60033ac8230063c8` lists `1.27.4-alpine` including amd64/arm64; direct registry access from this server is unavailable, so actual image pull remains a deployment check. Documentation links, handoff checks and diff whitespace checks pass.
