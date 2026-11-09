# Visual review: restore growth governance tab

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/components/admin/play/GrowthGovernanceOperations.vue"],
  "routes_or_surfaces": ["/admin/play: growth-governance tab"],
  "languages_and_themes": ["zh light and dark; existing en strings retained"],
  "states": ["default", "loading", "disabled", "empty", "error", "success"],
  "viewports": ["360x900", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-09-governance-prototype-1280-light.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-09-governance-baseline-1280-light.png"],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-09-governance-updated-360-light.png",
    "docs/visual-reviews/assets/2026-11-09-governance-updated-768-light.png",
    "docs/visual-reviews/assets/2026-11-09-governance-updated-1280-light.png",
    "docs/visual-reviews/assets/2026-11-09-governance-updated-1920-light.png",
    "docs/visual-reviews/assets/2026-11-09-governance-updated-360-dark.png",
    "docs/visual-reviews/assets/2026-11-09-governance-updated-1280-dark.png"
  ],
  "commands": ["pnpm --dir frontend design:check", "pnpm --dir frontend exec vitest run src/components/admin/play/__tests__/GrowthGovernanceOperations.spec.ts src/views/admin/__tests__/PlayOpsView.spec.ts", "node docs/archive/completed-tasks/2026-11-09-growth-governance/component-browser-check.cjs updated"],
  "checks": {
    "keyboard": {"status": "passed", "notes": "Existing refresh, retry and protected approval controls retain native button semantics. No focus or dialog changes."},
    "reduced_motion": {"status": "passed", "notes": "No animation, transition or motion behavior changed."}
  },
  "residual_risks": ["Isolated real Vue component and repository CSS with synthetic API data, not an authenticated production route. Local production browser acceptance remains pending. Server fallback fonts and stubbed dialogs/icons do not prove production typography or dialog behavior."]
}
-->

## Scope

Restore the existing growth-governance tab and independent reward readiness display. Reward issuance policy is outside this patch.

## Baseline

The supplied production screenshot shows an empty selected growth-governance tab. It contains account details and is not copied into Git. The isolated Chromium fixture runs the real pre-change component with an older server's `blocking_reasons: null`: the component renders no readiness statuses. Vitest also reproduces the exact null `.length` exception. Adjacent AdminCheckinRecords and shared BaseDialog/Icon/button patterns were inspected.

## Prototype

Saved before editing the component. Restore the existing page and guard null blocking reasons. Move the independently loaded reward-readiness section outside the cohort condition so an absent cohort cannot hide reward diagnostics. The prototype uses the actual template with only these proposed changes; no new navigation, theme, reward policy or configuration is introduced.

## Reuse Decision

Reuse existing cards, metrics, semantic badges, buttons, dialogs and translations. Keep reward configuration in the existing coupon menu. No new component, icon, color, width rule or visual exception. Removing accidental card nesting preserves the intended section hierarchy.

## State Coverage

Component tests cover ready/null and blocked/nonempty reasons in the same response, absent cohort with approvals disabled, error/retry recovery, incomplete metrics, approval and revocation. Loading retains aria-busy and disabled controls. Refresh and retry retain hover/active/focus styles from shared buttons; dialogs and step-up are unchanged. Empty readiness uses the existing empty-state text. No animation changes; reduced-motion remains inherited.

## Viewport Coverage

Real Chromium captures at 360, 768, 1280 and 1920px, plus dark at 360 and 1280px. Both reward statuses render with no page overflow. The fixture executes the component script and synthetic asynchronous API calls without starting a server. It is development evidence, not local production acceptance. Chinese strings come from the repository; interpolation and dialogs are stubbed. Existing English text is retained, with no new copy.

## Evidence

The archived component-browser-check.cjs generates the baseline, prototype and post-change captures above. Targeted Vitest passes 38 tests across the governance component, PlayOps route wiring and API boundary. The backend regression confirms readiness JSON uses an empty list, never null, when no reasons exist.

## Residual Risk

After exact-SHA deployment, the local administrator browser must verify this tab, then existing check-in records and the coupon/check-in-pool entry. The current task does not change qualification, growth energy issuance, pool probabilities or historical rewards; a pending user decision governs any reward-policy change.
