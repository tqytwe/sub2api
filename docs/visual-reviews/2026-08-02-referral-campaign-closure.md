# Visual Review: referral campaign closure

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/play/AdminInviteGrowthOperations.vue",
    "frontend/src/i18n/locales/en/common.ts",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/zh/common.ts",
    "frontend/src/i18n/locales/zh/dashboard.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/views/auth/RegisterView.vue",
    "frontend/src/views/user/AffiliateView.vue",
    "frontend/src/views/user/PlayHubView.vue"
  ],
  "routes_or_surfaces": ["/affiliate", "/register", "/admin/play-ops?tab=invite-growth"],
  "languages_and_themes": ["zh-CN/light", "en-US/dark"],
  "states": ["empty", "scheduled", "enrolled", "claimable", "invalid invite link", "loading"],
  "viewports": ["360x800", "1280x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.png"],
  "commands": ["pnpm typecheck", "pnpm vitest run src/views/user/__tests__/AffiliateView.spec.ts", "go test ./..."],
  "checks": {
    "keyboard": {"status": "passed", "notes": "All new actions use native buttons and the existing dialog controls."},
    "reduced_motion": {"status": "passed", "notes": "No new animation is introduced."}
  },
  "residual_risks": ["Static review record covers the approved information hierarchy; browser screenshots of authenticated campaign states remain required before production acceptance."]
}
-->

## Scope

The existing `/affiliate` entry remains the only user destination. The campaign block appears before the standard rebate block; registration explains a valid campaign invite before account creation; the existing Play Hub affiliate card can signal an active or unread campaign; the admin workbench retains its existing layout.

## Baseline

The previous activity surface was subordinate to the ordinary affiliate summary and did not show public rules, rebate policy, version, or registration-side explanation.

## Prototype

The existing invite-growth operations board remains the approved admin pattern. User-facing activity content reuses the growth workspace, panels, native buttons, and safe Markdown renderer.

## Reuse Decision

No new route, navigation entry, card system, palette, or decorative treatment was introduced. `AnnouncementContent`, `AppLayout`, `BaseDialog`, and shared controls are reused.

## State Coverage

- Empty: a concise no-campaign state remains in the existing activity surface.
- Scheduled, running, settling: status and all three deadlines remain visible.
- Enrolled: the page exposes the version-bound activity link and progress.
- Claimable: an attention label and per-tier claim action are visible.
- Invalid registration link: registration shows a clear unavailable state instead of silently accepting a legacy-only referral.

## Viewport Coverage

The activity header and action buttons stack at small widths. Rules and time windows use a responsive grid; tables retain horizontal scrolling on constrained screens.

## Evidence

`pnpm typecheck` and the focused Affiliate view test pass. `go test ./...` validates backend route/service/repository integration. A browser capture of authenticated data remains part of release acceptance because it requires production-like campaign fixtures.

## Residual Risk

The visual board is not a substitute for an authenticated browser run covering all campaign statuses. Do not release until that run verifies mobile/desktop rendering and overlap behavior with real fixture data.
