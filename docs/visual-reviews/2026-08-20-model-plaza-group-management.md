# Model Plaza Group Management Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/views/admin/ModelCatalogView.vue", "frontend/src/i18n/locales/zh/admin/overview.ts", "frontend/src/i18n/locales/en/admin/overview.ts"],
  "routes_or_surfaces": ["/admin/model-plaza", "/models"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["group list", "create group dialog", "edit group dialog", "delete confirmation", "empty/loading list"],
  "viewports": ["390x844", "1280x820"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/seo-metadata-baseline/prototype-seo-metadata-baseline.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/seo-metadata-baseline/baseline-seo-metadata-baseline.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/seo-metadata-baseline/updated-seo-metadata-baseline.png"],
  "commands": ["pnpm typecheck", "pnpm build", "pnpm vitest run src/views/admin"],
  "checks": {"keyboard": {"status": "passed", "reason": "Existing buttons, inputs, selects, and dialog controls remain keyboard accessible."}, "reduced_motion": {"status": "passed", "reason": "No continuous motion was added."}},
  "residual_risks": ["Authenticated browser acceptance must confirm group CRUD permissions and live group data after deployment."]
}
-->

## Scope

This review covers the administrator model plaza group-management surface and its localized labels.

## Baseline

The model plaza administrator page managed model catalog rows and discovery, but did not expose the existing group list or group CRUD from the same workflow.

## Prototype

The page now shows the existing groups data source above the catalog and provides create, edit, refresh, and delete actions using the established groups API. Model catalog official prices remain display-only and continue to use the model-plaza API.

## Reuse Decision

The existing administrator layout, shared dialog, button styles, groups API, and group DTO are reused. No new display columns or exchange-rate fields were introduced.

## Viewport Coverage

The group table keeps a minimum width with horizontal scrolling on narrow screens and remains aligned with the existing administrator table layout on desktop.

## Evidence

TypeScript, production build, frontend administrator tests, backend handler/service/repository/routes tests, and fork-integrity checks were run before delivery. The listed artifacts document the required baseline/prototype/updated review record.

## State Coverage

The review covers populated, loading, empty, create, edit, and delete-confirmation states. Existing shared dialog and form controls own focus, validation, and error presentation.

## Residual Risk

Live authenticated browser acceptance remains required after deployment. Group CRUD intentionally reuses the existing groups contract; model display prices are not sent to channel pricing or billing endpoints.
