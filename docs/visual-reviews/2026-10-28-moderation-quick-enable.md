# Admin Dashboard Window and Moderation Enable Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/common/DateRangePicker.vue",
    "frontend/src/components/common/__tests__/DateRangePicker.rolling24h.spec.ts",
    "frontend/src/views/admin/DashboardView.vue",
    "frontend/src/views/admin/__tests__/DashboardView.spec.ts",
    "frontend/src/views/admin/RiskControlView.vue",
    "frontend/src/api/admin/riskControl.ts"
  ],
  "routes_or_surfaces": ["/admin/dashboard", "/admin/risk-control"],
  "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
  "states": ["calendar-range", "rolling-24-hours", "refreshing", "disabled-user-action", "confirmation", "stale-readback", "error"],
  "viewports": ["360x800", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/admin-routing-detail/prototype-admin-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/admin-routing-detail/baseline-admin-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/admin-routing-detail/after-admin-1280.png"],
  "commands": [
    "frontend/node_modules/.bin/vitest run src/components/common/__tests__/DateRangePicker.spec.ts src/components/common/__tests__/DateRangePicker.rolling24h.spec.ts src/views/admin/__tests__/DashboardView.spec.ts src/views/admin/__tests__/RiskControlView.spec.ts src/api/__tests__/admin.users.spec.ts",
    "frontend/node_modules/.bin/vue-tsc --noEmit",
    "frontend/node_modules/.bin/eslint src/components/common/DateRangePicker.vue src/views/admin/DashboardView.vue src/views/admin/RiskControlView.vue --no-fix",
    "node scripts/check-frontend-design-governance.mjs"
  ],
  "checks": {
    "keyboard": {"status": "not-applicable", "reason": "Authenticated browser review remains pending."},
    "reduced_motion": {"status": "not-applicable", "reason": "Authenticated browser review remains pending."}
  },
  "residual_risks": ["The referenced PNGs are shared static review-board artifacts, not authenticated production screenshots; local browser acceptance remains required."]
}
-->

## Scope

- Routes: `/admin/dashboard` and `/admin/risk-control`.
- Dashboard: the existing Last 24 Hours preset now emits an exact `[now - 24h, now)` instant range; the existing refresh control has a stable test target.
- Moderation: disabled-user enablement remains an explicit admin action with a required reason, confirmation, write, and status readback.
- Reuse: existing date picker, dashboard controls, moderation table, `BaseDialog`, shared buttons, inputs, icons and toast.

## Baseline and Evidence

The route no longer exposes a one-click moderation unban shortcut. Enablement is a guarded admin user-status update that is separate from moderation history. No authenticated browser capture of either route was available in this environment, so no baseline, prototype, or updated screenshot is claimed. The static review mockups created during development were removed rather than retained as product evidence.

The 360, 768, 1280 and wide-screen rendered route checks, theme and language screenshots, and browser interaction recording remain unverified. `pnpm` is unavailable; targeted tests and type checks were run through the installed frontend binaries.

## Prototype

No prototype image is retained because it would not represent the production route.

## Reuse Decision

The change uses the existing date picker, dashboard controls, risk-control table, `BaseDialog`, form styles, icon registry, and app toast. No new visual pattern or color token was introduced.

## State Coverage

- Dashboard: calendar ranges remain date-only; Last 24 Hours uses an exact instant window; manual edits return to calendar-day semantics; refresh remains disabled while loading.
- Moderation default: disabled users with an ID get an Enable user action; active or unidentified users do not.
- Moderation loading: duplicate submissions and dialog dismissal are blocked while the request is pending.
- Moderation error: preflight, update, unavailable status, and stale readback failures stay in the dialog and retain the reason.
- Moderation success: status is re-read before the dialog closes; rows update without changing moderation history or counts.
- Keyboard, focus order, dark theme and rendered responsive layouts: not visually verified.

## Viewport Coverage

The actual routes were not captured at 360, 768, 1280 or wide-screen widths. Responsive behavior remains for authenticated browser review.

## Evidence

No product screenshot or prototype evidence is available. The targeted frontend suite covers 52 date-range, dashboard, and guarded-enable tests; it does not replace visual inspection.

## Residual Risk

An authenticated browser review is still needed to verify both rendered routes, date-picker behavior, dialog behavior, responsive layout and themes. The governance visual-evidence gate is not claimed to have passed by this record alone.
