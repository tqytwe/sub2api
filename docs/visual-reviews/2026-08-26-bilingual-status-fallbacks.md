# Visual Review: bilingual-status-fallbacks

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/usage/UsageCleanupDialog.vue",
    "frontend/src/components/coupon/CouponWalletPanel.vue",
    "frontend/src/components/imageStudio/ImageStudioGallery.vue",
    "frontend/src/components/payment/OrderStatusBadge.vue",
    "frontend/src/components/user/UserErrorDetailModal.vue",
    "frontend/src/components/user/UserErrorRequestsTable.vue",
    "frontend/src/features/channel-monitor-v2/MonitorSettingsPanel.vue",
    "frontend/src/features/ip-risk/IPRiskActionsView.vue",
    "frontend/src/features/ip-risk/IPRiskCaseDetail.vue",
    "frontend/src/features/ip-risk/IPRiskWorkbench.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/common.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/common.ts",
    "frontend/src/views/admin/AnnouncementsView.vue",
    "frontend/src/views/admin/RedeemView.vue",
    "frontend/src/views/admin/SubscriptionsView.vue",
    "frontend/src/views/public/AgentTeamView.vue",
    "frontend/src/views/user/SpeedTestView.vue",
    "frontend/src/views/user/KeysView.vue"
  ],
  "routes_or_surfaces": [
    "Image Studio gallery",
    "payment order status",
    "user API keys and error requests",
    "administrator usage cleanup, announcements, redeem codes, and subscriptions",
    "public agent team status",
    "coupon wallet and key speed test",
    "IP risk action history, workbench, and case detail",
    "Channel Monitor V2 ignored-error-category settings"
  ],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "loading", "empty", "error", "success"],
  "viewports": ["360x800", "768x900", "1280x800", "1920x1080"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/bilingual-status-fallbacks/prototype-1440.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/bilingual-status-fallbacks/baseline-1440.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/bilingual-status-fallbacks/updated-1440.png"],
  "commands": [
    "firefox --headless --screenshot docs/visual-reviews/assets/bilingual-status-fallbacks/baseline-1440.png --window-size 1440,980 'file:///home/codex/worktrees/sub2api-media-integration-20260826/docs/visual-reviews/assets/bilingual-status-fallbacks/static-review-board.html?mode=baseline'",
    "firefox --headless --screenshot docs/visual-reviews/assets/bilingual-status-fallbacks/prototype-1440.png --window-size 1440,980 'file:///home/codex/worktrees/sub2api-media-integration-20260826/docs/visual-reviews/assets/bilingual-status-fallbacks/static-review-board.html?mode=prototype'",
    "firefox --headless --screenshot docs/visual-reviews/assets/bilingual-status-fallbacks/updated-1440.png --window-size 1440,980 'file:///home/codex/worktrees/sub2api-media-integration-20260826/docs/visual-reviews/assets/bilingual-status-fallbacks/static-review-board.html?mode=updated'",
    "cd frontend && pnpm exec vitest run src/utils/__tests__/localizedEnum.spec.ts",
    "cd frontend && pnpm design:check"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "The change only replaces display text in existing semantic controls, dialogs, tables, and badges; no focus order, command, or control ownership changes."},
    "reduced_motion": {"status": "not-applicable", "reason": "This delivery adds no animation or continuous motion."}
  },
  "residual_risks": ["These artifacts are static review boards, not live browser captures. Final browser screenshots and local acceptance must verify every affected role, language, theme, and responsive viewport before deployment."]
}
-->

## Scope

- Surfaces: all audited server-owned status labels that can receive a newer enum value than the currently deployed frontend.
- This extension covers the coupon wallet, key speed test, IP risk action history/workbench/detail, and Channel Monitor V2 category controls. IP risk protects level, status, confidence, signal, action, and rollback labels together because they cross the same server boundary.
- Boundary: user-managed model IDs, group names, activity names, API paths, and provider messages remain raw source data. Only internal status enums use the fallback.
- Locale rule: default and explicit Chinese show `未知状态`; explicit English shows `Unknown status`. No surface renders an untranslated key or opaque backend enum value.

## Baseline

- An unknown backend status could render the raw enum token, or an untranslated `a.b.c` key, in the active UI language.
- `assets/bilingual-status-fallbacks/baseline-1440.png` is a static review board documenting that failure state; it is not presented as a live application screenshot.

## Prototype

- `assets/bilingual-status-fallbacks/prototype-1440.png` records the shared `localizedEnumOrUnknown` presentation rule: known values retain their existing localized labels and unknown values use the active locale's system label.
- The pattern reuses the existing tables, badges, dialogs, and Image Studio cards. It adds no layout, color, icon, width, or interaction pattern.

## Reuse Decision

- Reused: existing Vue I18n `t`, current locale fragments, current status badges, `BaseDialog`, tables, and the Image Studio gallery.
- New shared helper: `localizedEnumOrUnknown` centralizes the missing-key check so independently maintained backend enums cannot leak keys or tokens into Chinese or English screens.
- No design-system exception is required because component structure and styling are unchanged.

## State Coverage

- Default and success: known enum values keep their existing localized label.
- Loading and empty: existing component loading and empty states are unchanged.
- Error: an unknown, delayed, or newly introduced backend status is shown as the active locale's explicit unknown-status label rather than raw source data.
- Retry: existing retry controls remain unchanged; this change does not hide the underlying operation error message.

## Viewport Coverage

- 360px and 768px: the short fallback labels fit existing badges, rows, and dialog fields without changing control dimensions.
- 1280px and 1920px: table and gallery columns retain their existing workspace frame and density.
- Both themes: the change uses existing text and status components, so existing semantic color and contrast tokens remain authoritative.

## Evidence

- The baseline, prototype, and updated PNG files are Firefox-rendered static review boards at 1440px and are validated as decodable PNGs by `pnpm design:check`.
- `localizedEnum.spec.ts` checks the real Chinese and English labels and guards every audited rendering surface from regressing to raw keys or tokens.
- `CouponWalletPanel`, `IPRiskWorkbench`, and `IPRiskCaseDetail` inject future server enum values and assert that the rendered UI uses `Unknown status` rather than a backend token or translation key.
- Full route-fragment symmetry, browser screenshots, lint, type checking, full frontend tests, and production-role acceptance remain release gates outside this focused audit.

## Residual Risk

- Static review boards cannot prove live route-fragment timing, actual server enum payloads, themes, or browser layout. Before release, verify deployed guest, user, and administrator flows at 360/768/1280/1920 in Chinese and English, including loading, empty, error, and retry states.
