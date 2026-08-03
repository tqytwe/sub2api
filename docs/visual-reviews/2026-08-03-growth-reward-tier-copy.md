# Visual Review: configurable growth reward copy

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh/admin/playOps.ts",
    "frontend/src/i18n/locales/en/admin/playOps.ts"
  ],
  "routes_or_surfaces": ["/affiliate", "/admin/play-ops?tab=campaigns"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "validation-error", "claimable", "claimed"],
  "viewports": ["360x800", "768x900", "1280x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-08-03-new-user-growth/prototype-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-08-03-new-user-growth/prototype-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-08-03-new-user-growth/updated-1280.png"],
  "commands": ["pnpm design:check", "pnpm typecheck", "git diff --check"],
  "checks": {
    "keyboard": {"status": "not-applicable", "reason": "Only localized copy changed; controls and focus order are unchanged."},
    "reduced_motion": {"status": "not-applicable", "reason": "No motion or layout behavior changed."},
    "copy_locale": {"status": "passed", "notes": "Chinese and English copy now describe configurable tier totals without a fixed CNY 500 claim."}
  },
  "residual_risks": ["Authenticated browser acceptance remains required for final production verification."]
}
-->

## Scope

This release removes the fixed CNY 500 wording from the new-user growth activity and explains that the total reward is the sum of the configured tier rewards. The admin JSON editor, existing `/affiliate` surface, layout, controls, and states remain unchanged.

## Baseline

The baseline is the existing new-user growth admin editor and `/affiliate` activity card. The previous copy described a fixed CNY 500 maximum even though the tier JSON is the configurable source of the per-user total.

## Prototype

The existing `docs/visual-reviews/assets/2026-08-03-new-user-growth/prototype-1280.png` board is reused. The updated copy fits the existing hint and activity-description containers without changing their geometry.

## Reuse Decision

The existing new-user growth visual review board is reused because this is a locale-copy-only change. No new route, component, spacing, color, interaction, or motion pattern is introduced.

## State Coverage

Default, validation-error, claimable, and claimed states keep the same controls and layout. Chinese and English now state that the total comes from the configured tier rewards; no fixed 500 value is presented.

## Viewport Coverage

The existing 360px, 768px, and 1280px static artifacts cover the affected user and admin surfaces. The longer Chinese and English sentences wrap inside the existing text containers; no fixed-width element changed.

## Evidence

The existing static review artifacts cover the same admin and user surfaces across mobile and desktop viewports. Focused `PlayOpsView` tests and backend validation tests pass locally.

## Residual Risk

This copy change does not enable effective paid-consumption qualification. Production acceptance must still verify that no growth activity is enabled with an unverified metric.
