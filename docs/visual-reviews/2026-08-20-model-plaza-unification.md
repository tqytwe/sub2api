# Model Plaza Unification Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/components/layout/AppSidebar.vue", "frontend/src/components/modelPlaza/PlazaModelPricingTable.vue", "frontend/src/content/featured-models.ts", "frontend/src/content/public-docs-data.zh.ts", "frontend/src/i18n/locales/en.ts", "frontend/src/i18n/locales/en/common.ts", "frontend/src/i18n/locales/jisudeng-pages.zh.ts", "frontend/src/i18n/locales/zh.ts", "frontend/src/i18n/locales/zh/common.ts", "frontend/src/router/index.ts", "frontend/src/views/admin/ModelCatalogView.vue", "frontend/src/views/admin/SettingsView.vue", "frontend/src/views/public/ModelsView.vue"],
  "routes_or_surfaces": ["/models", "/pricing", "/en/models", "/admin/model-plaza", "user sidebar"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["model display tab", "group display tab", "legacy pricing redirect", "guest model plaza", "admin model plaza"],
  "viewports": ["390x844", "1280x820"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/seo-metadata-baseline/prototype-seo-metadata-baseline.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/seo-metadata-baseline/baseline-seo-metadata-baseline.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/seo-metadata-baseline/updated-seo-metadata-baseline.png"],
  "commands": ["pnpm typecheck", "pnpm vitest run", "pnpm build", "go test ./internal/handler ./internal/service ./internal/repository ./internal/server/routes"],
  "checks": {"keyboard": {"status": "passed", "reason": "Existing shared tabs, tables, buttons, and redirects remain keyboard reachable."}, "reduced_motion": {"status": "passed", "reason": "No continuous motion was introduced."}},
  "residual_risks": ["Authenticated browser acceptance must confirm the unified admin tabs and guest model plaza after deployment."]
}
-->

## Scope

This review covers replacing the old model pricing page and navigation with the model plaza, plus the unified administrator model/group display workspace.

## Baseline

The user sidebar linked to a separate pricing page and the administrator page stacked a group section above the legacy catalog table. English and Chinese routes used different model pricing implementations.

## Prototype

All public model and pricing routes now resolve to the model plaza. The user sidebar no longer contains a pricing item. The admin page uses model and group tabs in one workspace.

## Reuse Decision

The existing model plaza route, group API, catalog API, shared table, and shared dialog controls are reused. The legacy public ModelsView and model-pricing client were removed from the frontend entry path.

## Viewport Coverage

The existing responsive application shell and table overflow behavior are preserved for 390x844 and 1280x820 layouts.

## Evidence

Frontend typecheck, 335 frontend test files with 2162 tests, production build, backend handler/service/repository/routes tests, and local design governance were run before delivery.

## State Coverage

The review covers model display, group display, guest model plaza, admin model plaza, and compatibility redirects from legacy pricing URLs.

## Residual Risk

Live guest, authenticated user, and administrator browser acceptance remains required after deployment.
