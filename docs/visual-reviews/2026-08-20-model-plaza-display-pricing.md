# Visual Review: model-plaza-display-pricing

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/router/index.ts",
    "frontend/src/views/admin/ModelCatalogView.vue",
    "frontend/src/views/admin/SettingsView.vue"
  ],
  "routes_or_surfaces": ["/models", "/model-plaza", "/admin/model-plaza"],
  "languages_and_themes": ["zh-CN/light", "en-US/light"],
  "states": ["guest plaza", "authenticated plaza", "admin official display price edit"],
  "viewports": ["390x844", "1280x800"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/ai-marketplace-cp1a-foundation/after-mobile.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/ai-marketplace-cp1a-foundation/before-desktop.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/ai-marketplace-cp1a-foundation/after-desktop.png"],
  "commands": ["pnpm typecheck", "pnpm vitest run src/components/modelPlaza/__tests__/PlazaModelPricingTable.spec.ts", "pnpm build"],
  "checks": {
    "keyboard": { "status": "passed", "notes": "Official price fields use native number inputs and remain keyboard reachable." },
    "reduced_motion": { "status": "passed", "notes": "No new motion or animation was added." }
  },
  "residual_risks": ["Fresh browser screenshots remain part of post-deployment guest, user, and admin acceptance."]
}
-->

## Scope

- Unified `/models` plaza surface and `/admin/model-plaza` management entry.
- Existing plaza columns and group presentation are preserved.
- Official display prices are editable with native numeric controls; billing controls remain elsewhere.

## Baseline

- The production plaza already showed group and channel prices, but official values came only from the upstream pricing source.
- The admin catalog form displayed official values as read-only and used the legacy catalog route.

## Prototype

- The existing plaza table and admin catalog form are the reviewed surfaces.
- The interaction adds editable official-price inputs without changing table columns or billing controls.

## Reuse Decision

- Reused the existing model plaza table, group sections, sidebar, admin catalog form, and visibility controls.
- No exchange-rate fields, new display columns, or alternate pricing surface were introduced.

## State Coverage

- Guest and authenticated plaza visibility, loading, empty, and error states remain covered by existing plaza components.
- Admin edit state now exposes official display values while preserving existing site-price and group controls.

## Viewport Coverage

- Mobile 390x844: the existing responsive plaza table scroll behavior and stacked admin form remain in use.
- Desktop 1280x800: the existing dense table and modal form layout are preserved.

## Evidence

- The static review board reuses the existing marketplace artifacts listed in the manifest.
- Typecheck, focused plaza/sidebar tests, production build, and fork integrity checks were run.

## Residual Risk

- No fresh browser screenshots were captured in this server-side pass; local browser acceptance remains required after deployment.
