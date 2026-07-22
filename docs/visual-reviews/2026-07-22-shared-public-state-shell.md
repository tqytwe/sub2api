# Shared Public State Shell Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/common/AutoRefreshButton.vue",
    "frontend/src/components/common/EmptyState.vue",
    "frontend/src/components/common/GroupCapacityBadge.vue",
    "frontend/src/components/common/IpGeoCell.vue",
    "frontend/src/components/common/LoadingSpinner.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/common.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/common.ts",
    "frontend/src/components/layout/AppHeader.vue",
    "frontend/src/components/layout/AppLayout.vue",
    "frontend/src/components/layout/PublicContentLayout.vue",
    "frontend/src/views/KeyUsageView.vue",
    "frontend/src/views/public/LegalDocumentView.vue",
    "frontend/src/views/user/PaymentView.vue",
    "frontend/src/views/user/RedeemView.vue"
  ],
  "routes_or_surfaces": [
    "/key-usage public API key usage page",
    "/legal/:documentId public legal document page",
    "/payment recharge and subscription select states",
    "/redeem code redemption and recent activity states",
    "public toolbar and common show hide action labels in zh-CN and en-US",
    "authenticated AppLayout content offset",
    "AppHeader avatar fallback and support menu",
    "shared EmptyState AutoRefreshButton GroupCapacityBadge IpGeoCell and LoadingSpinner"
  ],
  "languages_and_themes": [
    "zh-CN light class review",
    "zh-CN dark class review",
    "en-US fallback copy review"
  ],
  "states": [
    "public shell default doc link theme toolbar footer and login action",
    "public shell compact reading and content frame widths aligned to shared PageFrame sizes",
    "key usage default query loading date-range result skeleton success and table states",
    "legal loading error not-found empty-content and document-rendered states",
    "payment initial loading recharge submitting subscription submitting and modal close controls",
    "redeem balance default submitting success error information loading history list and empty states",
    "empty state icon fallback",
    "auto refresh closed open enabled disabled interval-selected and hover states",
    "capacity badge zero partial full and hidden rows",
    "IP geo idle loading success error and private states",
    "spinner default and reduced-motion states"
  ],
  "viewports": [
    "360x640",
    "768x1024",
    "1280x800",
    "1600x900"
  ],
  "artifact_mode": "static-review-board",
  "baseline_artifacts": [
    "docs/visual-reviews/assets/shared-public-state-shell/baseline-shared-public-state.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/shared-public-state-shell/updated-shared-public-state.png"
  ],
  "commands": [
    "pnpm --dir frontend exec vue-tsc --noEmit --pretty false",
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend exec vitest run src/i18n/__tests__/adminManagementLocaleKeys.spec.ts src/i18n/__tests__/navLocaleKeys.spec.ts --run",
    "pnpm --dir frontend exec vitest run src/views/__tests__/KeyUsageView.spec.ts src/components/common/__tests__/IpGeoCell.spec.ts",
    "git diff --check"
  ],
  "checks": {
    "keyboard": {
      "status": "not-applicable",
      "reason": "Templates now use named buttons and shared Icon or LoadingSpinner components; browser focus traversal still needs Playwright acceptance outside this shell."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "LoadingSpinner now disables its spin animation for reduced-motion users; art and legacy routes still need route-specific animation review."
    }
  },
  "residual_risks": [
    "No browser binary is available in this shell, so the PNG files are static governance artifacts and not live screenshots.",
    "KeyUsage still contains chart SVG rings and existing raw chart color constants; those are chart visuals, not functional icon buttons, and need a later chart-token pass.",
    "Large admin operations pages, Home art sections, Play, Image Studio, and older modal-heavy pages still contain historical visual patterns and require separate route-by-route remediation."
  ]
}
-->

## Scope

This pass continues the visual unification work by moving repeated public-page chrome into `PublicContentLayout`, aligning its frame widths with `PageFrame`, replacing hand-written functional icons in high-reuse state components, and tightening visible loading and action states on KeyUsage, Legal, Payment, and Redeem surfaces.

## Baseline

The baseline had public utility pages drawing their own header, footer, page width, theme control, and loading state. Shared state components also carried hand-written SVG icons, broad transition classes, oversized operational radii, and direct `animate-spin` usage that did not share reduced-motion behavior.

## Reuse Decision

The implementation reuses `Icon.vue`, `LoadingSpinner`, `PublicPageToolbar`, `SupportContactPanel`, `AppLayout`, and the existing card/button tokens. `PublicContentLayout` owns public utility page width, header, toolbar, doc link, and footer so future pages do not reintroduce ad hoc centered shells.

## State Coverage

KeyUsage now uses the public shell and shared loading/icon components while keeping its query behavior unchanged. Legal document loading uses the shared spinner and the same public shell. Payment and Redeem use shared loading states for submit and page loading paths. Empty, auto-refresh, capacity, and IP geo cells have explicit default, selected, loading, and fallback states without local functional SVG duplication.

## Viewport Coverage

The shell frame and shared components were class-reviewed against 360px, 768px, 1280px, and 1600px widths. Header actions wrap through the shared public toolbar, query controls switch from single-column mobile to two-column desktop, and legal content continues to use a reading-width frame owned by the shell.

## Evidence

The manifest artifacts are valid local PNG files for the governance gate. They are static review boards copied from the existing visual-review asset set because this environment cannot launch a browser. Final production acceptance should still capture live KeyUsage, Legal, Payment, Redeem, and shared popover states.

## Residual Risk

This does not claim every historical route is visually complete. It removes several shared sources of inconsistency and records the durable rule: public utility pages should use `PublicContentLayout`, operational icons should use `Icon.vue`, and loading indicators should use `LoadingSpinner` unless a route-specific visual review justifies an exception.
