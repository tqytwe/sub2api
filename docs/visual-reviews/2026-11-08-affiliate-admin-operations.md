# Visual review: administrator affiliate operations

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/router/index.ts",
    "frontend/src/i18n/routeScopes.ts",
    "frontend/src/i18n/locales/en/admin/overview.ts",
    "frontend/src/i18n/locales/zh/admin/overview.ts",
    "frontend/src/i18n/locales/en/admin/affiliateOperations.ts",
    "frontend/src/i18n/locales/zh/admin/affiliateOperations.ts",
    "frontend/src/i18n/locales/en/workspaceShell.ts",
    "frontend/src/i18n/locales/zh/workspaceShell.ts",
    "frontend/src/api/admin/affiliates.ts",
    "frontend/src/api/admin/affiliateOperations.ts",
    "frontend/src/api/admin/play.ts",
    "frontend/src/views/admin/affiliates/AdminAffiliateOverviewView.vue",
    "frontend/src/views/admin/affiliates/AffiliateTrendChart.vue",
    "frontend/src/views/admin/affiliates/AdminAffiliatePromoters.vue",
    "frontend/src/views/admin/affiliates/AdminAffiliateUserDialog.vue",
    "frontend/src/views/admin/affiliates/AdminAffiliateRecordsTable.vue",
    "frontend/src/views/admin/affiliates/AffiliateOfflineWithdrawDialog.vue",
    "frontend/src/views/admin/affiliates/affiliateWithdrawOperation.ts",
    "frontend/src/components/admin/play/AdminInviteGrowthOperations.vue",
    "frontend/src/views/admin/SettingsView.vue",
    "frontend/src/views/admin/orders/AdminOrdersView.vue",
    "frontend/src/features/ip-risk/IPRiskWorkbench.vue"
  ],
  "routes_or_surfaces": ["/admin/affiliates/overview", "/admin/affiliates/invites", "/admin/affiliates/rebates", "/admin/affiliates/transfers", "/admin/play-ops?tab=invite-growth", "/admin/settings", "/admin/orders?order_id=", "/admin/proxies/risk?case_id="],
  "languages_and_themes": ["zh light and dark static review boards", "en locale parity and shared-theme code review"],
  "states": ["default", "hover", "active", "focus-visible", "loading", "disabled", "empty", "partial error", "retry", "success", "exact linked user", "mixed currencies", "inherited global rate", "step-up cancellation", "export limit", "uncertain payout replay", "hidden leaderboard pause"],
  "viewports": ["360x1000", "768x1000", "1280x1000", "1920x1000"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-08-affiliate-operations-prototype.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.png"],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-08-affiliate-admin-360-light.png", "docs/visual-reviews/assets/2026-11-08-affiliate-admin-360-dark.png",
    "docs/visual-reviews/assets/2026-11-08-affiliate-admin-768-light.png", "docs/visual-reviews/assets/2026-11-08-affiliate-admin-768-dark.png",
    "docs/visual-reviews/assets/2026-11-08-affiliate-admin-1280-light.png", "docs/visual-reviews/assets/2026-11-08-affiliate-admin-1280-dark.png",
    "docs/visual-reviews/assets/2026-11-08-affiliate-admin-1920-light.png", "docs/visual-reviews/assets/2026-11-08-affiliate-admin-1920-dark.png"
  ],
  "commands": [
    "node docs/archive/completed-tasks/2026-11-08-affiliate-admin/render-review-board.mjs",
    "pnpm exec vitest run src/views/admin/affiliates/__tests__ src/components/admin/play/__tests__/AdminInviteGrowthOperations.spec.ts src/views/admin/__tests__/SettingsView.spec.ts src/views/admin/__tests__/AdminOrdersView.manualConfirm.spec.ts src/__tests__/ipRiskWorkbench.spec.ts src/api/__tests__/admin.affiliateOperations.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts",
    "pnpm exec vue-tsc --noEmit", "pnpm design:check", "pnpm lint:check"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "Native buttons, selects and form fields retain shared focus styles. BaseDialog supplies focus trapping and return. Tests exercise real selection and form submission; final full-shell keyboard validation remains local-browser acceptance."},
    "reduced_motion": {"status": "passed", "notes": "New chart animation is disabled. No decorative motion was introduced; ranking refresh changes data only. Existing shared DataTable loading visuals are unchanged."}
  },
  "residual_risks": [
    "Updated artifacts are explicitly static review boards, not screenshots of the implemented Vue routes; they do not prove real browser geometry or live data correctness.",
    "Baseline is an existing operations prototype, supported by inspection of the current three affiliate record pages, adjacent RiskControl and AdminOrders pages, AppLayout, TablePageLayout, DataTable, StatCard, BaseDialog and step-up components. No new production baseline was captured in this task.",
    "Authenticated local-browser final acceptance must cover guest/user/admin permissions, 360/768/1280/wide widths, Chinese/English, light/dark, 200% zoom, actual Chart.js rendering, modal focus, exports and source/currency reconciliation.",
    "No localhost or server-browser artifact is claimed as production acceptance. Deployment SHA, production health and ledger verification remain root-task delivery gates."
  ]
}
-->

## Scope

Preserve the existing invitation/rebate sidebar tree and its three record URLs. Add one operations overview child. Activities and their approvals remain in Play Operations; existing custom-setting endpoints and accounting rules are reused. Linking exact order and risk-case IDs opens their existing detail flows instead of introducing new editors.

The new overview route declares shell `app`, frame `workspace`, compact operational density, chrome heading, document scrolling and page surface. AppLayout owns width, gutter and scrolling; no private page max-width, centering or viewport height was added.

## Baseline

Reviewed the existing record table, offline withdrawal dialog, special-setting panel and campaign manager. Existing table filters, modal focus infrastructure, layout and sort behavior are retained. The historical operations prototype documents the established compact administrative visual language. This is historical design evidence, not a fresh production capture.

## Prototype

The root task generated and displayed `docs/visual-reviews/assets/2026-11-08-affiliate-operations-prototype.png` before visible implementation. The accepted boundary preserves administrator invitation navigation, adds overview/promoter investigation, enriches the three existing records and keeps activities in Play Operations. No gameplay rules, payouts or reward values were redesigned.

## Reuse Decision

Reuse AppLayout, TablePageLayout, StatCard, DataTable, Pagination, Icon, BaseDialog, ConfirmDialog, TotpStepUpDialog, existing button/input classes and existing Chart.js line-chart conventions. The new chart adds a numeric table alternative and disables animation. All data tables inherit shared mobile card rendering. No new shell, drawer, icon or payment system was introduced.

## State Coverage

- Default/hover/active/focus: shared native controls and semantic text colors; view switches expose `aria-pressed`.
- Loading: section `aria-busy`, independent overview/leaderboard/pending failures, disabled mutation/export controls.
- Empty: shared DataTable empty state and chart text alternative.
- Error/retry: visible failed-section state; exact-filter list errors clear stale rows; summaries and exports preserve filters and explain limits.
- Success: existing toast and refresh/readback; exclusive settings retain inherited global rate; clearing settings explicitly warns that invitation code changes.
- Payout ambiguity: original amount, external reference, timestamp and note are stored with the idempotency key. Uncertain outcomes lock inputs and retry the same request; step-up cancellation does not create a payment.
- Linked workflows: exact IDs are carried from monthly rank or pending item to user investigation, then rebate records, order detail or risk-case detail.

## Viewport Coverage

Static design boards at 360, 768, 1280 and 1920 px in light/dark show wrapping toolbar and local table scroll intent. Implemented tables reuse DataTable mobile rendering, and dialogs use shared responsive widths. These boards are not implementation-layout screenshots. Actual English long strings, 200% zoom, native month/date controls and full-shell focus order remain final local-browser checks.

## Evidence

Focused API/component tests cover exact filters, same-filter export and summaries, mixed currencies, per-section retry, eligibility/risk separation, step-up cancellation, global-rate inheritance, payout evidence/ambiguous retries and exact order/risk-case deep links. Review boards are generated reproducibly by the archived script above. The root task records final complete checks and deployment evidence.

## Residual Risk

No real production identity was used for visual acceptance in this child task. Production source/currency totals, real exports and administrator permissions must be reconciled before calling the overall delivery complete. The static review artifact must not be described as a screenshot of the running application.
