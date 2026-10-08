# Production acceptance fixes

## Scope

Repair the missing checkin tab translation and video pricing units without changing layout, prices, multipliers or reward rules. API route registration preserves administrator authentication and PUT step-up. Existing Icon, table and tab components are reused.

## Baseline

Captured the deployed production bundle with synthetic isolated API fixtures. Catalog video rows display per-request labels; checkin tab displays a raw key and the fixture reproduces the missing route404. No production credentials or mutations.

## Prototype

Before implementation, captured the same baseline build with DOM text substitutions for the approved labels and a synthetic successful read response. These images are proposals, not implemented screenshots.

## Reuse Decision

Updated images render the new production build with all external requests blocked. Admin data is synthetic. Desktop/mobile and both languages/themes retain existing components. The code change is text-only on frontend; existing loading, disabled, success and error behavior is unchanged. Real administrator acceptance remains pending and must not be claimed from these screenshots.

## State Coverage

| State | Coverage |
|---|---|
| default/active | selected checkin tab and catalog video rows |
| loading/error | baseline404 fixture; existing asynchronous rendering unchanged |
| disabled/success | synthetic loaded form; production writes blocked |
| hover/focus-visible | existing shared button/tab styles, no style changes |
| reduced-motion | all browser contexts use reduced motion |
| empty | no new empty-state behavior; no price/record mutations |

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/modelPlaza/PlazaModelPricingTable.vue",
    "frontend/src/i18n/locales/zh/admin/playOps.ts",
    "frontend/src/i18n/locales/en/admin/playOps.ts",
    "frontend/src/i18n/locales/zh/dashboard.ts",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts"
  ],
  "routes_or_surfaces": [
    "/catalog",
    "/admin/play-ops?tab=checkin"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "zh-CN/dark",
    "en-US/light",
    "en-US/dark"
  ],
  "states": [
    "default",
    "active",
    "focus-visible",
    "loading",
    "error",
    "disabled",
    "success",
    "reduced-motion"
  ],
  "viewports": [
    "360x900",
    "768x900",
    "1280x900",
    "1920x900"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/production-acceptance/prototype-admin-play-ops-1280-zh-light.png",
    "docs/visual-reviews/assets/production-acceptance/prototype-catalog-1280-zh-light.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/production-acceptance/baseline-admin-play-ops-1280-zh-light.png",
    "docs/visual-reviews/assets/production-acceptance/baseline-catalog-1280-zh-light.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/production-acceptance/updated-admin-play-ops-1280-zh-dark.png",
    "docs/visual-reviews/assets/production-acceptance/updated-admin-play-ops-1920-en-light.png",
    "docs/visual-reviews/assets/production-acceptance/updated-admin-play-ops-360-zh-light.png",
    "docs/visual-reviews/assets/production-acceptance/updated-admin-play-ops-768-en-dark.png",
    "docs/visual-reviews/assets/production-acceptance/updated-catalog-1280-zh-dark.png",
    "docs/visual-reviews/assets/production-acceptance/updated-catalog-1920-en-light.png",
    "docs/visual-reviews/assets/production-acceptance/updated-catalog-360-zh-light.png",
    "docs/visual-reviews/assets/production-acceptance/updated-catalog-768-en-dark.png"
  ],
  "checks": {
    "keyboard": {
      "status": "passed"
    },
    "reduced_motion": {
      "status": "passed"
    }
  },
  "residual_risks": [
    "Offline production bundles use synthetic admin/API fixtures. Not real production administrator acceptance.",
    "Do not modify rates, group description, rewards or production settings.",
    "Group description says flat0.3/S while tier prices differ; needs separate business confirmation."
  ],
  "commands": [
    "pnpm build",
    "python /workspace/acceptance-evidence/capture-sub.py --dist /workspace/acceptance-sub2api-20261008/backend/internal/web/dist --phase updated # Playwright browser screenshots",
    "pnpm exec vitest run src/i18n/__tests__/adminRuntimeTabs.spec.ts src/components/modelPlaza/__tests__/PlazaModelPricingTable.spec.ts"
  ]
}
-->

## Viewport Coverage

360,768,1280,1920 CSS pixels; Chinese/English and light/dark paired across four captures per route.

## Evidence

Production-bundle browser captures enumerated in manifest. API fixtures are synthetic and external networking and all mutation methods are blocked.

## Residual Risk

Not real production administrator acceptance. No reward or pricing configuration changes. No paid generation.
