# Visual Review: Play operations audience and announcement membership

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/play.ts",
    "frontend/src/components/admin/announcements/AnnouncementTargetingEditor.vue",
	"frontend/src/components/admin/play/ArenaRewardSettings.vue",
    "frontend/src/components/user/dashboard/DashboardCampaignBanner.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/admin/playOps.ts",
	"frontend/src/i18n/locales/en/admin/resources.ts",
	"frontend/src/i18n/locales/en/legacy/user-dashboard.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/admin/playOps.ts",
	"frontend/src/i18n/locales/zh/admin/resources.ts",
	"frontend/src/i18n/locales/zh/legacy/user-dashboard.ts",
    "frontend/src/types/index.ts",
    "frontend/src/views/admin/AnnouncementsView.vue",
    "frontend/src/views/admin/PlayOpsView.vue",
    "frontend/src/views/user/PlayHubView.vue"
  ],
  "routes_or_surfaces": ["/admin/play?tab=campaigns", "/admin/announcements", "/dashboard", "/play"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "loading", "empty", "validation-error", "disabled", "step-up", "step-up-cancelled", "step-up-blocked", "success", "focus-visible"],
  "viewports": ["360x800", "768x1024", "1280x960"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-09-07-play-operations-audience-prototype-360.png",
    "docs/visual-reviews/assets/2026-09-07-play-operations-audience-prototype-768.png",
    "docs/visual-reviews/assets/2026-09-07-play-operations-audience-prototype-1280.png"
  ],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.png"],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-09-07-play-operations-audience-prototype-360.png",
    "docs/visual-reviews/assets/2026-09-07-play-operations-audience-prototype-768.png",
    "docs/visual-reviews/assets/2026-09-07-play-operations-audience-prototype-1280.png"
  ],
  "commands": [
    "pnpm --dir frontend exec playwright screenshot --viewport-size='360,800' --full-page file:///tmp/sub2api-play-operations-audience-20260907-wt/docs/visual-reviews/assets/2026-09-07-play-operations-audience-prototype.html docs/visual-reviews/assets/2026-09-07-play-operations-audience-prototype-360.png",
    "pnpm --dir frontend exec playwright screenshot --viewport-size='768,1024' --full-page file:///tmp/sub2api-play-operations-audience-20260907-wt/docs/visual-reviews/assets/2026-09-07-play-operations-audience-prototype.html docs/visual-reviews/assets/2026-09-07-play-operations-audience-prototype-768.png",
    "pnpm --dir frontend exec playwright screenshot --viewport-size='1280,960' --full-page file:///tmp/sub2api-play-operations-audience-20260907-wt/docs/visual-reviews/assets/2026-09-07-play-operations-audience-prototype.html docs/visual-reviews/assets/2026-09-07-play-operations-audience-prototype-1280.png",
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend lint:check",
    "pnpm --dir frontend typecheck"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "Native radios, fields and buttons reuse existing controls; the page-level TOTP test verifies dialog entry and cancellation without losing the form."},
    "reduced_motion": {"status": "passed", "notes": "This change adds no continuous motion or transition-all utility."}
  },
  "residual_risks": ["This real PNG is a repository-local static design prototype rendered with Playwright, not an authenticated candidate or production browser capture. Final local-browser acceptance remains required for TOTP completion, dark mode, zoom and long bilingual content."]
}
-->

## Scope

The existing Play Ops campaign dialog gains a distinct operational-display type.
It stores bilingual title/body, a fixed CTA enum and a real-time Play audience,
but it cannot create referral rewards, balances, coupons, budgets or ledger
entries. The announcement editor gains the same ordinary/member vocabulary.
Existing protected Arena reward saves now share the same TOTP challenge and
single-retry behavior as campaign writes, VIP publication and growth governance.

## Baseline

The existing campaign form conflates display copy with benefit overlays and
referral reward campaign types. Its dashboard and Play Hub select the first API
row. Announcement targeting only offers balance and active-subscription rules,
which are not equivalent to the current Play membership qualification.

## Prototype

The real, decodable prototype images are
`assets/2026-09-07-play-operations-audience-prototype-{360,768,1280}.png`.
They are explicitly labelled as design prototypes rather than browser captures.
They fix the
scope: selection of the operational-display type hides referral/reward fields;
the audience selection has ordinary/member/all; CTA is a fixed route enum; the
TOTP retry preserves the operation; and the announcement editor uses the same
real-time membership meaning.

## Reuse Decision

The implementation stays in the existing full-width `PlayOpsView`,
`AnnouncementsView`, dashboard banner and Play Hub. It reuses `AppLayout`,
`BaseDialog`, `TotpStepUpDialog`, native fields, `Icon`, toast store and the
existing tab/page framing. No new route, card family, color token, icon set,
gradient, animated element or page-width behavior is introduced.

## State Coverage

- Default and empty: display activities fall back to the legacy name/benefit
  line when old records do not have new content fields.
- Validation: display activities reject reward fields and arbitrary URLs;
  announcement membership only accepts ordinary/member.
- Step-up: a first `STEP_UP_REQUIRED` opens the existing dialog and retries
  only the original operation once for campaigns and Arena reward saves. Cancel
  retains form state; unavailable step-up uses the existing localized error.
- Loading, disabled, error and success reuse existing request states and toast
  behavior.

## Viewport Coverage

The static design board is rendered for 360px, 768px and 1280px review. The
candidate components retain the existing responsive grid and dialog controls;
authenticated release checks still need Chinese/English and light/dark themes.
At 200% zoom, long bilingual bodies must wrap rather than hide the save action.

## Evidence

- Updated review artifacts: the Playwright-rendered, decodable static boards at
  360px, 768px and 1280px document the campaign form, membership targeting and
  user-facing activity CTA. They are explicitly design prototypes, not live
  browser or production screenshots.
- Automated behavior evidence: `PlayOpsView.spec.ts` exercises default
  display-only activity creation, legacy benefits, referral-only growth types
  and the real TOTP retry/cancel/blocked flow; `PlayHubView.spec.ts` verifies
  bilingual display copy, server order and named CTA routing; the announcement
  editor and backend service tests prove ordinary/member selection and
  real-time read filtering. `ArenaRewardSettings.spec.ts` confirms protected
  reward settings writes enter the shared step-up runner.
- Commands run: `git diff --check`; focused Go tests; the focused Play,
  announcement and step-up Vitest suites; `pnpm design:check`; `pnpm
  lint:check`; `pnpm typecheck`; `make test` (402 Vitest files and 2,713
  tests, plus backend test and lint); and `make build` all passed locally.
  These are non-production evidence only.

## Residual Risk

No local service was started and no production API/data/configuration was read
or modified. The static prototype cannot prove live authorization, TOTP or the
authenticated responsive shell. Those remain PR/production acceptance gates.
