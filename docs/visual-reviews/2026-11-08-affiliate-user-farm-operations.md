# Visual review: affiliate user operations and Farm leaderboards

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/user/AffiliateView.vue",
    "frontend/src/views/user/__tests__/AffiliateView.spec.ts",
    "frontend/src/views/public/ArenaView.vue",
    "frontend/src/views/public/__tests__/ArenaView.competitive.spec.ts",
    "frontend/src/api/affiliateOperations.ts",
    "frontend/src/api/play.ts",
    "frontend/src/api/__tests__/affiliateOperations.spec.ts",
    "frontend/src/api/__tests__/play.arenaSeason.spec.ts",
    "frontend/src/composables/useLeaderboardLiveRefresh.ts",
    "frontend/src/composables/__tests__/useLeaderboardLiveRefresh.spec.ts",
    "frontend/src/i18n/locales/en/affiliateOperations.ts",
    "frontend/src/i18n/locales/zh/affiliateOperations.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en/misc.ts",
    "frontend/src/i18n/locales/zh/misc.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts"
  ],
  "routes_or_surfaces": ["/affiliate", "/arena"],
  "languages_and_themes": ["zh light static review board", "English locale parity tests", "light and dark semantic-token code review"],
  "states": ["default", "hover", "active", "keyboard tab selection", "focus-visible", "loading", "disabled", "empty", "load failure", "retry", "transfer success", "claim pending", "claim failure", "mixed currency", "historical campaign", "top ten", "own rank outside top ten", "hidden pause", "visible resume", "backoff", "late response", "unmount cleanup"],
  "viewports": ["360x800", "768x1024", "1280x800", "1920x1080"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-08-affiliate-operations-prototype.png"],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/welfare-consistency/updated-affiliate-1280-zh-light-default.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-arena-1280-zh-light-default.png"
  ],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-11-08-affiliate-user-farm-review.png"],
  "commands": [
    "pnpm exec vitest run src/views/user/__tests__/AffiliateView.spec.ts src/views/public/__tests__/ArenaView.competitive.spec.ts src/api/__tests__/affiliateOperations.spec.ts src/api/__tests__/play.arenaSeason.spec.ts src/composables/__tests__/useLeaderboardLiveRefresh.spec.ts",
    "pnpm exec vitest run src/views/public/__tests__/authenticatedPlayShellIntegration.spec.ts src/views/public/__tests__/playModulesWideLayout.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts",
    "pnpm exec vue-tsc --noEmit",
    "pnpm design:check",
    "pnpm lint:check"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "Code review: native controls retain focus styles; affiliate tabs provide roving tabindex and ArrowLeft/ArrowRight/Home/End selection. Browser focus and mobile native month/date inputs still need final acceptance."},
    "reduced_motion": {"status": "passed", "notes": "No new decorative motion; existing loading indicators retain their theme behavior. Polling updates data without layout animation."}
  },
  "residual_risks": [
    "The updated artifact is a static implementation review board, not a screenshot of the implemented Vue page. It cannot prove browser layout, responsive behavior, live data accuracy or contrast.",
    "Final acceptance requires the user's local browser at 360, 768, 1280 and wide viewports, Chinese and English, light and dark themes, 200% zoom, keyboard navigation, and guest/user/admin permissions.",
    "Production API, ledger reconciliation, exact deployment SHA and final local-browser evidence remain release gates handled by the root task. No localhost session or server browser is presented as production acceptance."
  ]
}
-->

## Scope

Preserve the existing `/affiliate` route and ordinary/campaign/team invitation behavior. The user page now has Overview, My invitations, Earnings, Leaderboard and Invitation campaigns as local tabs. The monthly leaderboard is honorary and creates no bonus, withdrawal system or balance ledger. The existing Farm page keeps its layout, personal progress, scoring and reward rules.

Target route contract remains app/workspace/document scroll with existing AppLayout for affiliate and AuthenticatedPlayShell for Farm. No private page width, gutter or new route frame is introduced.

## Baseline

Read the existing AffiliateView, ArenaView, shared DataTable and Pagination, root/frontend AGENTS, design system and governance manifest. Inspected both baseline PNGs linked above. The original affiliate layout placed long activity details before the normal share and transfer path. Its invitation list was limited to 100 records. Existing activity rankings were embedded in every campaign and fetched with the campaign payload. The Farm cached a previously visited period indefinitely.

## Prototype

`assets/2026-11-08-affiliate-operations-prototype.png` was created and displayed before visible changes. It records the agreed five-tab affiliate structure, server pagination, separate source/currency totals, Top 10 plus a separate personal rank, and shared visibility-aware refresh. The updated static board describes the implemented structure; neither board is a rendered application screenshot.

## Reuse Decision

Reuse existing AppLayout, AuthenticatedPlayShell, Icon, AnnouncementContent, DataTable, Pagination, native labelled filters, shared buttons, semantic growth-world tokens, existing Farm podium/list styles and wallet route. Ordinary code/link sharing stays in Overview; campaign sharing remains within its campaign. Full campaign rules expand in native details. History is paged by the server and campaign actions use the original enrollment, signed link and versioned claim APIs.

The shared useLeaderboardLiveRefresh composable only calls the selected lightweight ranking endpoint. It does not reload campaign details, Farm quest data or payout history. It pauses while hidden/inactive, refreshes on return, retries at 10/20/30 seconds and prevents overlapping or outdated results.

## State Coverage

- Each query region distinguishes loading, empty, failed and retry states. Failed background refresh keeps the last successful board and identifies it as stale.
- Ordinary monthly invitation and campaign boards show at most 10 rows. Personal rank remains outside the list, even when the user is below tenth place.
- Farm shows three podium users plus positions 4–10. Loaded history is retained across refreshes; a late history response only updates history and cannot overwrite newer ranking/current-user data.
- A monthly selection following the current month advances at Shanghai midnight during the existing lightweight refresh; an explicitly selected historical month remains fixed. The current-month key remains stable across automatic rollover so it does not create a second request or reload activities.
- Transfers retain the original server operation and show the completed amount inline. Earnings link to the existing wallet with the affiliate source filter.
- Summary shows this month's net earnings and lifetime detail. Mixed-currency aggregates render a currency-breakdown hint instead of an invented total or zero; source/currency rows remain separate. Frozen release is not fabricated as another earning.
- Growth and referral campaign claims reject duplicate clicks while pending, retain versioned APIs, and refresh account summary after successful claims. No public leaderboard reward currency is guessed because the honorary board has no reward column.

## Viewport Coverage

Native filters and tabs wrap, share values break long strings, tables contain only local horizontal overflow, and invitation/earning rows use the existing DataTable mobile layout. Numeric columns retain tabular figures. The semantic tokens and existing theme classes are reused. Browser overflow, focus visibility, touch targets, contrast and 200% zoom remain explicit final-browser checks rather than claims from this static board.

## Evidence

Focused tests cover server pagination, history scope, currency separation, claim concurrency, wallet linking, Top 10/personal-rank separation, cached-period revalidation, lightweight polling, delayed history, backoff, visibility, old selections and unmount. Locale completeness and authenticated-shell/wide-layout tests are also run. The root task performs the final full test/build/Fork gate and records any remaining failure honestly before submission.

## Residual Risk

No server browser or local service was used as product acceptance. This review does not claim production deployment or local user-browser sign-off. Real guest/user/admin sessions and ledger reconciliation remain necessary before completion can be declared.
