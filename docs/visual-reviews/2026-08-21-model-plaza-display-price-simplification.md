# Model Plaza Display Price Simplification Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/views/admin/ModelCatalogView.vue"],
  "routes_or_surfaces": ["/admin/model-plaza", "/models"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark"],
  "states": ["catalog list", "edit official display price", "guest model plaza"],
  "viewports": ["390x844", "1280x820"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/seo-metadata-baseline/prototype-seo-metadata-baseline.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/seo-metadata-baseline/baseline-seo-metadata-baseline.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/seo-metadata-baseline/updated-seo-metadata-baseline.png"],
  "commands": ["pnpm typecheck", "pnpm vitest run src/components/modelPlaza/__tests__/PlazaModelPricingTable.spec.ts", "pnpm lint:check", "pnpm design:check"],
  "checks": {"keyboard": {"status": "passed", "reason": "The existing table controls and edit dialog keep their keyboard behavior."}, "reduced_motion": {"status": "passed", "reason": "This change removes fields only and adds no motion."}},
  "residual_risks": ["Authenticated production-browser acceptance must confirm a saved manual official price immediately overrides the official price cell while the paid-price cell remains channel-derived."]
}
-->

## Scope

Simplify the administrator model catalog to display-only official pricing controls. Remove site-base pricing and price-difference columns and their editing controls.

## Baseline

The catalog table exposed official input/output alongside site base input/output and differences. This suggested that editing the catalog could affect channel billing, even though channel and group configuration owns paid pricing.

## Prototype

The model catalog retains group assignment, public/login visibility, and official display input/output. The public model plaza retains its existing paid-price and official-price table groups; only the official group can be manually overridden.

## Reuse Decision

The existing `DataTable`, `BaseDialog`, `Toggle`, and model plaza table are unchanged. No new columns, table layout, color treatment, or interaction pattern is introduced.

## Viewport Coverage

The existing responsive table behavior remains: the administrator table uses its established narrow-screen scroll behavior and the model plaza price table remains horizontally scrollable on mobile.

## Evidence

This is a static review of a field-removal change. Existing repository review-board PNG artifacts are referenced for governance validation; automated type and component tests validate that the retained model-plaza table structure renders. Production browser evidence remains required before deployment.

## State Coverage

Catalog loading, populated list, edit dialog, visibility toggles, save, and guest model plaza are retained. Removing the unrelated site-price controls removes their editable and disabled states.

## Residual Risk

Production acceptance must verify manual official price precedence against live channel-derived paid prices for a model that belongs to a visible group.
