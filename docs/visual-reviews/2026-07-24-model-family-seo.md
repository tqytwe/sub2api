# Model Family SEO Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/router/index.ts",
    "frontend/src/views/public/ModelsView.vue"
  ],
  "routes_or_surfaces": [
    "/models/deepseek",
    "/models/qwen",
    "/models/kimi",
    "/models/glm",
    "/en/models/deepseek",
    "/en/models/qwen",
    "/en/models/kimi",
    "/en/models/glm",
    "model catalog family note",
    "crawler-visible title description canonical hreflang metadata"
  ],
  "languages_and_themes": [
    "zh-CN light public model family route",
    "en light public model family route",
    "zh-CN dark inherited model table state",
    "en dark inherited model table state"
  ],
  "states": [
    "guest public model catalog",
    "signed-in model pricing table",
    "family-filtered result list",
    "empty filtered family result",
    "pricing API loading",
    "pricing API error",
    "Chinese default route locale",
    "English path-forced route locale"
  ],
  "viewports": [
    "390x844",
    "768x1024",
    "1366x900",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/model-family-seo/prototype-model-family-seo.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/model-family-seo/baseline-model-family-seo.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/model-family-seo/updated-model-family-seo.png"
  ],
  "commands": [
    "python3 generated static model family SEO review boards with PIL",
    "pnpm --dir frontend run design:check",
    "go test -tags embed ./internal/web ./internal/server/routes",
    "go test ./internal/handler",
    "pnpm --dir frontend exec vitest run src/i18n/__tests__/englishBrandNoCountryFraming.spec.ts src/i18n/__tests__/lazyLocaleScope.spec.ts",
    "pnpm --dir frontend run typecheck",
    "pnpm --dir frontend run build",
    "git diff --check"
  ],
  "checks": {
    "keyboard": {
      "status": "not-applicable",
      "reason": "The change adds route-level copy, filtering, and metadata; it does not add a new keyboard interaction or alter focus order."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "The change does not add animation, transition, or timed motion behavior."
    }
  },
  "residual_risks": [
    "Artifacts are static review boards, so final browser screenshots and production curl checks remain required before release acceptance.",
    "Search engine visibility depends on crawler recrawl timing after sitemap, canonical, hreflang, and llms.txt updates are deployed."
  ]
}
-->

## Scope

This review covers model-family SEO landing pages for DeepSeek, Qwen, Kimi, and GLM. The visible change is intentionally small: the existing public model catalog gains focused family routes, route-specific intro copy, and family-filtered pricing rows while keeping the current `/models` layout and table behavior.

Chinese public routes stay Chinese by default, including `/models/deepseek`, `/models/qwen`, `/models/kimi`, and `/models/glm`. English model-family coverage is path-forced only below `/en/models/...`.

## Baseline

Before this phase, `/models` and `/en/models` gave search engines a broad model catalog but no dedicated long-tail pricing URLs for DeepSeek, Qwen, Kimi, or GLM. Users searching a specific model family could still reach the catalog, but the crawler-visible title, description, canonical URL, and AI reference files did not have a focused destination for each family.

Baseline artifact: `docs/visual-reviews/assets/model-family-seo/baseline-model-family-seo.png`.

## Prototype

The prototype keeps the existing product surface and adds crawlable family pages rather than creating a separate marketing shell. Each family route should answer pricing and access intent in the first screen, then reuse the live model table for the actual catalog and rates.

Prototype artifact: `docs/visual-reviews/assets/model-family-seo/prototype-model-family-seo.png`.

## Reuse Decision

The implementation reuses `ModelsView.vue`, the existing public router, the existing locale bundles, and the existing SEO metadata helper. The new family note does not introduce a new page shell, nested card system, raw colors, custom max width, or separate pricing data source.

No design-system exception is required. The family note inherits the existing public page typography and page frame so the header and model table keep their current responsive constraints.

## State Coverage

Covered states include guest public pricing, signed-in pricing, loading, API error, empty family result, and family-filtered result lists. Chinese pages use Chinese copy for the family heading and description. English pages use English copy and avoid country or national framing in the brand positioning.

The route locale guard is part of the functional coverage: `/en/models/{family}` resolves English, while `/models/{family}` resolves Chinese and does not inherit a previous `/en` visit.

## Viewport Coverage

Mobile coverage targets `390x844`, where the existing model hero and table wrapping rules remain responsible for preventing overflow. Tablet coverage targets `768x1024`. Desktop coverage targets `1366x900`, and wide desktop coverage targets `1920x1080`.

The change does not add a new fixed header, overlay, viewport-sized shell, or page-level max-width. Final acceptance should still inspect the live homepage and model-family routes before production sign-off, because the user has already caught header overflow and blocked contact text issues in this area.

## Evidence

Updated artifact: `docs/visual-reviews/assets/model-family-seo/updated-model-family-seo.png`.

Commands run or scheduled for this PR:

```bash
python3 generated static model family SEO review boards with PIL
pnpm --dir frontend run design:check
go test -tags embed ./internal/web ./internal/server/routes
go test ./internal/handler
pnpm --dir frontend exec vitest run src/i18n/__tests__/englishBrandNoCountryFraming.spec.ts src/i18n/__tests__/lazyLocaleScope.spec.ts
pnpm --dir frontend run typecheck
pnpm --dir frontend run build
git diff --check
```

## Residual Risk

Static boards satisfy the design-governance record but do not replace production browser acceptance. Before merge and after deploy, inspect `/`, `/en`, `/models/deepseek`, `/models/qwen`, `/models/kimi`, `/models/glm`, `/en/models/deepseek`, `/en/models/qwen`, `/en/models/kimi`, and `/en/models/glm` for correct language, no raw locale keys, no CJK in English HTML, and no header or content overflow.

Search snippet improvements may lag until Google, Bing, Baidu, and AI answer engines recrawl the sitemap, canonical links, hreflang alternates, and `llms.txt`.
