# Visual Review: growth effective paid consumption

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/play.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh/admin/playOps.ts",
    "frontend/src/i18n/locales/en/admin/playOps.ts",
    "frontend/src/views/user/AffiliateView.vue"
  ],
  "routes_or_surfaces": ["/affiliate", "/admin/play-ops?tab=campaigns"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "funding-conflict", "zero-progress", "claimable"],
  "viewports": ["360x800", "768x900", "1280x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-08-03-new-user-growth/prototype-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-08-03-new-user-growth/updated-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-08-03-new-user-growth/updated-1280.png"],
  "commands": ["pnpm design:check", "pnpm typecheck", "pnpm exec vitest run src/views/user/__tests__/AffiliateView.spec.ts"],
  "checks": {
    "keyboard": {"status": "passed", "reason": "The existing select, reward claim buttons, and focus order are unchanged; the conflict explanation is a non-interactive status."},
    "reduced_motion": {"status": "not-applicable", "reason": "This change introduces no motion behavior."},
    "copy_locale": {"status": "passed", "notes": "Chinese and English name the paid-consumption rule and explain the conflict state."}
  },
  "residual_risks": ["Authenticated final browser acceptance remains required for the funding-conflict state."]
}
-->

## Scope

The existing new-user-growth option is renamed from actual consumption to effective paid consumption. The existing activity panel gains one compact status line only when a non-recharge balance credit makes the current period ineligible.

## Baseline

The existing activity card already presents the metric, rebate policy, deadline, progress bar, and tier state in a vertical stack. The approved growth review board is the baseline for both this user card and the compact admin metric selector.

## Prototype

The accepted new-user-growth prototype is reused because the activity panel, route, controls, density, and responsive layout are unchanged. The warning uses the existing inline status treatment and fits between the activity metadata and progress bar.

## Reuse Decision

The change reuses the existing panel, inline text treatment, progress bar, select, and locale architecture. It adds no card, icon, route, color token, or new interaction pattern.

## State Coverage

- Default and zero-progress: existing layout, now labels the metric as effective paid consumption.
- Funding conflict: one text status announces that this period cannot accrue rewards; the progress remains zero.
- Claimable: unchanged.

## Viewport Coverage

At 360px the status text wraps below the metadata; at 768px and 1280px it remains in the existing card column and does not affect the tier grid or primary action width. Chinese and English use the same container and semantic warning token.

## Evidence

`AffiliateView.spec.ts` covers the conflict-status rendering and the existing user activity layout. The manifest uses the validated growth prototype and updated review board as static evidence; authenticated browser capture is reserved for production acceptance.

## Residual Risk

An authenticated production account with a deliberately injected non-recharge balance credit is still required to verify the final rendered warning and reward revocation.

## Review Notes

The status line wraps naturally at 360px, does not change button geometry, and exposes `role=status`. No new color token, card, icon, route, or interaction is introduced. Browser verification remains required after deployment.
