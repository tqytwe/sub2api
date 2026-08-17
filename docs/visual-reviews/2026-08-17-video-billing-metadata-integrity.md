# Visual Review: video billing metadata integrity

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/usage/UsageTable.vue",
    "frontend/src/components/admin/usage/__tests__/UsageTable.spec.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts"
  ],
  "routes_or_surfaces": ["/admin/usage"],
  "languages_and_themes": ["zh-CN light", "zh-CN dark", "en-US light", "en-US dark"],
  "states": ["video metadata complete", "video metadata missing", "cost tooltip", "loading", "empty", "error"],
  "viewports": ["390x844", "1280x860", "1600x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/admin-dashboard-billing-surcharge/prototype-admin-dashboard-surcharge-module.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/admin-dashboard-billing-surcharge/baseline-admin-dashboard-usage-cards.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/admin-dashboard-billing-surcharge/updated-admin-dashboard-surcharge-module.png"],
  "commands": ["pnpm --dir frontend exec vitest run src/components/admin/usage/__tests__/UsageTable.spec.ts", "pnpm --dir frontend run typecheck", "pnpm --dir frontend run lint:check", "pnpm --dir frontend run design:check"],
  "checks": {
    "keyboard": {"status": "not-applicable", "reason": "The existing hover-only informational tooltip and its trigger remain unchanged."},
    "reduced_motion": {"status": "passed", "notes": "No animation or transition changed."}
  },
  "residual_risks": ["Static artifacts preserve the existing table and tooltip density only. Final authenticated administrator-browser verification must check both a new six-second record and a historical record with missing metadata."]
}
-->

## Scope

The existing UsageTable cost tooltip remains unchanged in layout, color roles, trigger, and density. A missing video duration now displays `未记录` / `not recorded` in the existing per-second-price value position instead of presenting the total request cost as a fabricated per-second rate.

## Baseline

The reviewed baseline is the existing dense administrator usage-table contract. The supplied production example exposed the error state: an absent duration was displayed as `-s` while its total cost was simultaneously shown as a per-second price.

## Prototype

The prototype retains the existing table and floating cost-detail pattern. No new panel, control, icon, color system, or interaction is introduced. The sole state change is a text value in the established right-hand detail column.

## Reuse Decision

The change reuses `UsageTable`, its existing Teleport tooltip, typography, spacing, and semantic warning text color. It deliberately does not redesign the table or add a special historical-record component.

## State Coverage

- Complete metadata: count, resolution, duration, calculated per-second price, and total remain visible.
- Missing historical metadata: resolution and duration now both show `未记录` / `not recorded`; per-second price is explicitly unavailable rather than inferred.
- Loading, empty, error, hover, active, focus-visible, and disabled behavior remain owned by existing shared table and tooltip code.

## Viewport Coverage

The tooltip continues using its existing compact field-row layout at mobile, desktop, and wide desktop widths. The new localized value is shorter than the replaced numeric price and cannot expand the surface.

## Evidence

`UsageTable.spec.ts` covers both a complete six-second record and a missing-duration record. The static artifacts are only layout evidence; they are not authenticated browser captures.

## Residual Risk

Production acceptance still requires an administrator to verify a newly generated 720p six-second video and a historical missing-metadata row in the local browser.
