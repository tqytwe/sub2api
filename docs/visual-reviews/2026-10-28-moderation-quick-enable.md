# Moderation Quick-Enable Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/RiskControlView.vue",
    "frontend/src/views/admin/__tests__/RiskControlView.spec.ts",
    "frontend/src/api/__tests__/admin.users.spec.ts",
    "frontend/src/types/index.ts"
  ],
  "routes_or_surfaces": ["/admin/risk-control"],
  "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
  "states": ["default", "loading", "error", "success", "disabled", "focus-visible"],
  "viewports": ["360x800", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/admin-routing-detail/prototype-admin-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/admin-routing-detail/baseline-admin-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/admin-routing-detail/after-admin-1280.png"],
  "commands": [
    "npm exec --yes pnpm@9.15.4 -- exec vitest run src/views/admin/__tests__/RiskControlView.spec.ts src/api/__tests__/admin.users.spec.ts",
    "npm exec --yes pnpm@9.15.4 -- exec vue-tsc --noEmit",
    "npm exec --yes pnpm@9.15.4 -- exec eslint src/views/admin/RiskControlView.vue src/views/admin/__tests__/RiskControlView.spec.ts src/api/__tests__/admin.users.spec.ts --no-fix"
  ],
  "checks": {
    "keyboard": {"status": "not-applicable", "reason": "Authenticated browser review remains pending."},
    "reduced_motion": {"status": "not-applicable", "reason": "Authenticated browser review remains pending."}
  },
  "residual_risks": ["The referenced PNGs are shared static review-board artifacts, not authenticated production screenshots; local browser acceptance remains required."]
}
-->

## Scope

- Route: `/admin/risk-control`
- Surface: disabled-user action in the existing moderation log and its confirmation dialog.
- Reuse: existing table row actions, `BaseDialog`, shared buttons, inputs, icons and toast.

## Baseline and Evidence

The existing route exposes a moderation-specific Unban action for auto-banned accounts. Quick enable is a separate admin user-status update. No authenticated browser capture of this route was available in this environment, so no baseline, prototype, or updated screenshot is claimed. The static review mockups created during development were removed rather than retained as product evidence.

The 360, 768, 1280 and wide-screen rendered route checks, theme and language screenshots, and browser interaction recording remain unverified. `pnpm` is unavailable; targeted tests and type checks were run through the installed frontend binaries.

## Prototype

No prototype image is retained because it would not represent the production route.

## Reuse Decision

The change uses the existing risk-control table, `BaseDialog`, form styles, icon registry, and app toast. No new visual pattern or color token was introduced.

## State Coverage

- Default: disabled users with an ID get an Enable user action; active or unidentified users do not.
- Loading: duplicate submissions and dialog dismissal are blocked while the request is pending.
- Error: preflight, update, unavailable status and readback failures stay in the dialog and retain the reason.
- Success: status is re-read before the dialog closes; the rows update without changing moderation history or counts.
- Keyboard, focus order, dark theme and rendered responsive layouts: not visually verified.

## Viewport Coverage

The actual route was not captured at 360, 768, 1280 or wide-screen widths. Responsive behavior remains for authenticated browser review.

## Evidence

No product screenshot or prototype evidence is available. Tests cover the interaction contract only; they do not replace visual inspection.

## Residual Risk

An authenticated browser review is still needed to verify the rendered route, dialog behavior, responsive layout and themes. The governance visual-evidence gate is not claimed to have passed.
