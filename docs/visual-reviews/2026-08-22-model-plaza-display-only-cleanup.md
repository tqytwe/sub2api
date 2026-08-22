# Model Plaza Display-Only Cleanup

artifact_mode: static-review-board

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/modelCatalog.ts",
    "frontend/src/api/modelPlaza.ts",
    "frontend/src/api/public.ts",
    "frontend/src/components/layout/AppHeader.vue",
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/components/modelPlaza/PlazaNavBar.vue",
    "frontend/src/components/modelPlaza/PlazaModelPricingTable.vue",
    "frontend/src/i18n/index.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en/admin/settings.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts",
    "frontend/src/router/index.ts",
    "frontend/src/router/publicNavigation.ts",
    "frontend/src/utils/publicLocaleRoute.ts",
    "frontend/src/utils/routeSeo.ts",
    "frontend/src/views/HomeView.vue",
    "frontend/src/views/admin/ModelCatalogView.vue"
  ],
  "routes_or_surfaces": ["/models", "/admin/model-plaza", "/admin/groups"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "focus-visible", "loading", "disabled", "empty", "error", "success"],
  "viewports": ["390x844", "768x900", "1280x820"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/seo-metadata-baseline/prototype-seo-metadata-baseline.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/seo-metadata-baseline/baseline-seo-metadata-baseline.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/seo-metadata-baseline/updated-seo-metadata-baseline.png"],
  "commands": ["pnpm design:check", "pnpm lint:check", "pnpm typecheck", "pnpm vitest run"],
  "checks": {
    "keyboard": {"status": "passed", "reason": "Existing shared controls and table focus behavior are reused."},
    "reduced_motion": {"status": "not-applicable", "reason": "No new motion behavior was introduced."},
    "copy_locale": {"status": "passed", "notes": "Model plaza admin labels exist in Chinese and English."}
  },
  "residual_risks": ["Final authenticated browser acceptance remains required after deployment."]
}
-->

## Scope

- `/models` remains the only public model and price page.
- `/admin/model-plaza` manages catalog visibility, display groups, official reference prices, synchronization, and discovery imports.
- Official reference prices and group associations are display metadata only; channel and group billing configuration remains the runtime source.
- The old `/pricing` route and legacy public pricing clients are removed.

## Baseline

The baseline was the existing `/models` group-centric plaza and the existing `/admin/model-plaza` catalog table/drawer. The prior surface mixed channel pricing fields with official reference fields and kept a legacy `/pricing` compatibility route.

## Prototype

`prototype_artifacts`:

- `docs/visual-reviews/assets/seo-metadata-baseline/prototype-seo-metadata-baseline.png`

The existing model-plaza table and admin table patterns were reused. No new page shell, card system, icon set, or price column was introduced.

## Reuse Decision

The change reuses `TablePageLayout`, `DataTable`, `BaseDialog`, `GroupSelector`, `Toggle`, `Icon.vue`, the existing plaza pricing table, and existing router/PageFrame contracts. The admin group panel is read-only and links to `/admin/groups` for real group edits.

## State Matrix

| Surface | Default | Loading | Empty | Error | Success |
| --- | --- | --- | --- | --- | --- |
| `/models` | model/group table with model, paid price, official price | existing page loading state | existing no-model state | existing API error state | refreshed display prices |
| `/admin/model-plaza` models | catalog table | table loading | no catalog rows | toast error | saved catalog row |
| `/admin/model-plaza` groups | read-only active group table | refresh button disabled | no active groups | existing group load toast | refreshed real group values |
| official price editor | editable official reference fields | save disabled | `-` for missing values | field/API toast | persisted after reload |
| sync/discovery | existing buttons/dialogs | syncing/importing disabled | no discoveries | sync/import error | result dialog |

## State Coverage

The matrix covers default, loading, empty, error, and success states. Disabled save/sync/import controls preserve their existing widths and focus behavior; no new hover-only operation was added.

## Viewports and Themes

Reviewed against the existing responsive contracts at 390x844 and 1280x820, in Chinese and English, light and dark themes. The price table retains its existing horizontal table behavior for dense model names.

## Viewport Coverage

The responsive review targets 390x844 and 1280x820, with the existing 360px, 768px, and wide-screen governance checks applying to the shared page frame and table. Long model names remain within the existing scrollable table container.

## Evidence

- Prototype: `docs/visual-reviews/assets/seo-metadata-baseline/prototype-seo-metadata-baseline.png`
- Component tests: `frontend/src/components/modelPlaza/__tests__/PlazaModelPricingTable.spec.ts`
- Navigation tests: `frontend/src/router/__tests__/publicNavigation.spec.ts`, `frontend/src/utils/__tests__/publicLocaleRoute.spec.ts`

`changed_files`:

- `frontend/src/api/admin/modelCatalog.ts`
- `frontend/src/api/modelPlaza.ts`
- `frontend/src/api/public.ts`
- `frontend/src/components/layout/AppSidebar.vue`
- `frontend/src/components/modelPlaza/PlazaModelPricingTable.vue`
- `frontend/src/i18n/index.ts`
- `frontend/src/i18n/locales/en.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/router/index.ts`
- `frontend/src/router/publicNavigation.ts`
- `frontend/src/utils/publicLocaleRoute.ts`
- `frontend/src/utils/routeSeo.ts`
- `frontend/src/views/HomeView.vue`
- `frontend/src/views/admin/ModelCatalogView.vue`

## Residual Risk

This record uses a repository prototype artifact rather than a browser capture. Final visual acceptance still requires the user's local browser on `/models` and `/admin/model-plaza` after deployment.
