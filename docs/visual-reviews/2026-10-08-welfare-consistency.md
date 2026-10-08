# Welfare consistency review — PR329

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/user/PlayHubView.vue",
    "frontend/src/views/user/AffiliateView.vue",
    "frontend/src/views/public/ArenaView.vue",
    "frontend/src/views/public/BlindboxView.vue",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh/dashboard.ts",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/zh/legacy/user-misc.ts",
    "frontend/src/i18n/locales/en/legacy/user-misc.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts"
  ],
  "routes_or_surfaces": [
    "/play",
    "/arena",
    "/blindbox",
    "/affiliate"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "zh-CN/dark",
    "en-US/light",
    "en-US/dark"
  ],
  "states": [
    "default",
    "hover",
    "active",
    "focus-visible",
    "loading",
    "disabled",
    "empty",
    "error",
    "success",
    "dialog-dismiss"
  ],
  "viewports": [
    "360x900",
    "768x900",
    "1280x900",
    "1920x900",
    "640x900"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/welfare-consistency/prototype-play-1280.png",
    "docs/visual-reviews/assets/welfare-consistency/prototype-arena-1280.png",
    "docs/visual-reviews/assets/welfare-consistency/prototype-blindbox-1280.png",
    "docs/visual-reviews/assets/welfare-consistency/prototype-affiliate-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/welfare-consistency/baseline-play-1280.png",
    "docs/visual-reviews/assets/welfare-consistency/baseline-arena-1280.png",
    "docs/visual-reviews/assets/welfare-consistency/baseline-blindbox-1280.png",
    "docs/visual-reviews/assets/welfare-consistency/baseline-affiliate-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/welfare-consistency/updated-affiliate-1280-zh-light-default.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-affiliate-1920-en-dark-default.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-affiliate-360-zh-dark-default.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-affiliate-360-zh-light-empty.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-affiliate-360-zh-light-error.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-affiliate-360-zh-light-loading.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-affiliate-640-en-light-zoom.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-affiliate-768-en-light-default.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-affiliate-focus-hover.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-affiliate-interaction-success.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-arena-1280-zh-light-default.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-arena-1920-en-dark-default.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-arena-360-zh-dark-default.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-arena-360-zh-light-empty.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-arena-360-zh-light-error.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-arena-360-zh-light-loading.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-arena-640-en-light-zoom.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-arena-768-en-light-default.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-arena-focus-hover.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-arena-interaction-success.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-blindbox-1280-zh-light-default.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-blindbox-1920-en-dark-default.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-blindbox-360-zh-dark-default.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-blindbox-360-zh-light-empty.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-blindbox-360-zh-light-error.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-blindbox-360-zh-light-loading.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-blindbox-640-en-light-zoom.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-blindbox-768-en-light-default.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-blindbox-focus-hover.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-blindbox-interaction-success.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-blindbox-result-dialog.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-play-1280-zh-light-default.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-play-1920-en-dark-default.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-play-360-zh-dark-default.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-play-360-zh-light-empty.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-play-360-zh-light-error.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-play-360-zh-light-loading.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-play-640-en-light-zoom.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-play-768-en-light-default.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-play-focus-hover.png",
    "docs/visual-reviews/assets/welfare-consistency/updated-play-interaction-success.png"
  ],
  "commands": [
    "pnpm --dir frontend build",
    "Python Playwright Chromium: python docs/archive/completed-tasks/2026-10-welfare-consistency/matrix.py",
    "Python Playwright Chromium: python docs/archive/completed-tasks/2026-10-welfare-consistency/interactions.py"
  ],
  "checks": {
    "keyboard": {
      "status": "passed"
    },
    "reduced_motion": {
      "status": "passed"
    }
  },
  "residual_risks": [
    "Synthetic browser fixtures validate rendering and interactions, not production settlement or final local-browser acceptance. No production account, paid draw, merge or deployment was used.",
    "640 CSS px at device scale 2 checks the reflow equivalent of a 1280px display at 200%; native desktop browser zoom remains part of final user acceptance.",
    "Existing separate growth-claim actions can overlap; the mutual exclusion changed here covers referral-campaign actions, not every action on the page."
  ]
}
-->

## Scope

Four existing user-facing routes, with no new route or backend reward rule. Reuse the current workspace/page contracts through AppLayout and AuthenticatedPlayShell. The user authorized this correction scope and review-branch delivery; production acceptance remains unapproved.

## Baseline

Real production-bundle Chromium captures using synthetic API data were taken before implementation. The initial PR had a type error referencing a nonexistent funding_source property; baseline capture alone omitted vite-plugin-checker to render that existing state. This capture-only workaround was never used for final validation: final pnpm build includes locale tests, vue-tsc and Vite. Baseline and prototype source frontend was unchanged by main 8e88e4b.

Observed: invite-count thresholds incorrectly multiplied into recharge amounts, fabricated personal zero progress, USD formatting for CNY rewards, fixed 10% copy, hard-coded Top10, overlapping rank ranges, unknown balance branch interpreted as 40%, and failed loads conflated with empty content. Existing daily-quest cards were squeezed into three columns inside the sidebar.

## Prototype

The four prototype_artifacts were captured before implementation from a DOM-only preview of the baseline. They were presented in task commentary before visible code edits. Scope was already explicitly authorized: retain shared layouts and server-owned amounts, qualification, probability and VIP rules; correct explanations and states. This is not a claim that the user approved a separate new visual identity. The prototype images are design previews, not implementation or production evidence.

## Reuse Decision

Reuse AppLayout, AuthenticatedPlayShell, Icon, growth-world/public page buttons and state styles, AnnouncementContent, CouponRewardCard and RewardCelebrationOverlay. Use semantic description lists for rule and branch facts. No parallel design system, raw colors, new SVGs or page shell added. Adjacent CheckInView and CouponRewardCard informed the layout. Arena's existing quest grid now fits its sidebar in one column.

## State Coverage

- Default: all four routes with published synthetic configuration, including CNY campaign rewards and three reward branches.
- Hover/active/focus-visible: real Tab navigation reaches each route's primary tested action; Enter activates it. Hover/focus screenshots and computed outline/shadow are in interaction-report.json.
- Loading/disabled/empty/error: 360px fixture cases for all routes. Blindbox's empty case is feature-disabled; Arena has no rows/tiers/history; Affiliate has no campaigns. Unit tests additionally cover unknown/invalid/zero branch weights, Explorer, guest redaction, governance, double draw, pending claim and identity changes.
- Success/recovery: hub error→keyboard retry; Arena lazy historical empty response; invitation claim failure→re-enable→claimed-frozen progress; blindbox settled result survives same-ID user refresh.
- Cancel/dismiss: shared reward result dialog closes with its accessible close button. No pre-transaction cancellation mechanism was added or promised; invitation claims remain server-owned.
- A real-browser failure exposed legacy locale overrides restoring 10%; a route-loader RED test now covers the final zh/en message tree. Another exposed a same-user refresh clearing a settled result; a RED test and multi-source watch fix preserve true logout/switch isolation.

## Viewport Coverage

32 real captures: each route at 360/zh/dark, 768/en/light, 1280/zh/light, 1920/en/dark, plus 640/en/light at device scale 2 and 360/zh/light loading/error/empty. All use prefers-reduced-motion: reduce. Final matrix reports zero page exceptions and zero document horizontal overflow. The 640px case is a 200% reflow-equivalent check, not claimed as native-browser zoom acceptance.

## Evidence

- matrix-report.json and interaction-report.json are archived next to the reproducible offline fixtures.
- Baseline/prototype and final images are in assets/welfare-consistency; manifest enumerates actual PNGs.
- Final production build and browser matrix/interactions have explicit exit code 0. Full repository gates are recorded in the PR after completion.
- Specification review then quality review passed; follow-up reviews covered locale overrides, the local SDK test fixture and same-user refresh.

## Residual Risk

No production settlement, deployment, paid generation, real reward claim or user-local browser acceptance was performed. All API responses and identities in captures are synthetic. Existing growth-claim concurrency outside the referral-campaign action lock is not newly fixed here. Repository AGENTS references PROJECT_HYGIENE.md and ZEABUR_POSTGRES_RUNBOOK.md, but neither file exists on the current main snapshot; explicit AGENTS hygiene rules were followed and no production operation required the missing runbook.
