# Visual Review: video per-second billing display

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/channel/IntervalRow.vue",
    "frontend/src/components/admin/usage/UsageTable.vue",
    "frontend/src/views/admin/ChannelsView.vue",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/admin/channels.ts",
    "frontend/src/i18n/locales/zh/admin/channels.ts",
    "frontend/src/i18n/locales/zh/admin/resources.ts"
  ],
  "routes_or_surfaces": ["/admin/channels", "/admin/usage"],
  "languages_and_themes": ["zh-CN light", "zh-CN dark", "en-US light", "en-US dark"],
  "states": ["video pricing row", "video usage row", "cost tooltip", "loading", "empty", "error"],
  "viewports": ["390x844", "1280x860", "1600x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/admin-dashboard-billing-surcharge/prototype-admin-dashboard-surcharge-module.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/admin-dashboard-billing-surcharge/baseline-admin-dashboard-usage-cards.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/admin-dashboard-billing-surcharge/updated-admin-dashboard-surcharge-module.png"],
  "commands": ["pnpm --dir frontend exec vitest run src/components/admin/usage/__tests__/UsageTable.spec.ts src/utils/__tests__/billingMode.spec.ts", "pnpm --dir frontend run typecheck", "pnpm --dir frontend run design:check"],
  "checks": {
    "keyboard": {"status": "not-applicable", "reason": "Existing table, tooltip, select, and number-input focus paths are reused."},
    "reduced_motion": {"status": "passed", "notes": "No animation or transition was added."}
  },
  "residual_risks": ["Artifacts are static review boards, not authenticated production screenshots. Final local admin browser acceptance remains required."]
}
-->

## Scope

The change keeps the existing dense admin surfaces and clarifies that channel `video` pricing is per second. Usage cost details now expose count, resolution, duration, unit price, and total price. No route shell, table layout, colors, or interaction pattern is redesigned.

## Baseline

The baseline reuses the existing admin billing review board because the product surface is unchanged. The relevant visual contract is the existing table and tooltip density; only semantic labels and read-only fields are added.

## Prototype

The prototype retains the baseline's dense operational table and tooltip. It replaces the ambiguous video billing label with `视频（按秒）` / `Video (per second)` and uses the existing field-row pattern for count, resolution, duration, unit price, and total.

## Reuse Decision

The implementation reuses the existing `UsageTable`, channel interval row, semantic billing-mode badge, locale catalog, and native form controls. No new component or icon pattern was introduced.

## State Coverage

- Default: `视频（按秒）` / `Video (per second)` label and `$ / s` channel input.
- Cost tooltip: count, resolution, duration, per-second unit price, and total.
- Loading, empty, error, disabled, hover, active, and focus-visible behavior remain owned by the existing table/form components.
- Long model names and missing historical video fields retain the existing `-` fallback.

## Viewport Coverage

- Mobile uses the existing table horizontal-scroll behavior; no new fixed-width surface is introduced.
- Tablet, desktop, and wide desktop retain the existing usage table columns and tooltip placement.
- At 200% zoom, the field labels wrap within the existing tooltip; no interactive control or layout-sized text is added.

## Evidence

The referenced PNGs are real, decodable static review artifacts and are not represented as browser screenshots. Final acceptance must inspect `/admin/channels` and `/admin/usage` in the user's local browser in Chinese and English, light and dark themes, and reconcile one real 720p/6-second record against the API response.

## Residual Risk

Static review boards cannot prove authenticated production data rendering. Local administrator browser acceptance remains required after deployment.
