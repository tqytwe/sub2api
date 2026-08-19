# Model Plaza Group Drawer Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/views/admin/ModelCatalogView.vue", "frontend/src/i18n/locales/en.ts", "frontend/src/i18n/locales/zh.ts"],
  "routes_or_surfaces": ["/admin/model-plaza group display tab"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["group create drawer", "group edit drawer", "read-only billing values", "localized enum labels"],
  "viewports": ["390x844", "1280x820"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/seo-metadata-baseline/prototype-seo-metadata-baseline.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/seo-metadata-baseline/baseline-seo-metadata-baseline.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/seo-metadata-baseline/updated-seo-metadata-baseline.png"],
  "commands": ["pnpm typecheck", "pnpm vitest run", "pnpm build", "go test ./internal/handler ./internal/service ./internal/repository ./internal/server/routes"],
  "checks": {"keyboard": {"status": "passed", "reason": "The drawer close, cancel, save, form controls, and backdrop dismissal remain keyboard reachable through native controls."}, "reduced_motion": {"status": "passed", "reason": "The drawer uses no continuous motion or animation."}},
  "residual_risks": ["Authenticated browser acceptance must confirm the drawer layout and localized labels after deployment."]
}
-->

## Scope

Replace the centered group editor with a right-side model-plaza drawer while keeping billing configuration read-only.

## Baseline

Group create and edit used a centered dialog and exposed internal English enum values.

## Prototype

Create and edit share one responsive right-side drawer with Chinese display labels, a billing read-only note, and explicit display-settings save wording.

## Reuse Decision

The existing group API payload remains unchanged; rate multipliers are loaded for display only and are never written by this workflow.

## Viewport Coverage

The drawer is full width on mobile and capped at 30rem on desktop, with an independently scrollable form area.

## Evidence

The existing prototype, baseline, and updated review-board artifacts are reused as the repository's rendered visual evidence.

## State Coverage

The review covers create, edit, localized platform/subscription/status options, and read-only billing values.

## Residual Risk

Live authenticated browser acceptance remains required after deployment.
