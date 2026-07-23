# English SEO Layer Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/App.vue",
    "frontend/src/content/public-docs.ts",
    "frontend/src/content/public-docs-data.en.ts",
    "frontend/src/i18n/index.ts",
    "frontend/src/i18n/locales/en/core.ts",
    "frontend/src/i18n/locales/jisudeng-auth-aside.en.ts",
    "frontend/src/i18n/locales/jisudeng-auth-aside.zh.ts",
    "frontend/src/i18n/locales/jisudeng-home.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/i18n/locales/zh/core.ts",
    "frontend/src/router/index.ts",
    "frontend/src/router/meta.d.ts",
    "frontend/src/router/publicNavigation.ts",
    "frontend/src/router/title.ts",
    "frontend/src/utils/routeSeo.ts",
    "frontend/src/views/HomeView.vue",
    "frontend/src/views/public/DocsView.vue"
  ],
  "routes_or_surfaces": [
    "/en public home route",
    "/en/models public models and pricing route",
    "/en/docs public documentation route",
    "/home /models /docs Chinese public routes",
    "/login and /register auth aside lightweight locale shell",
    "crawler-visible title description canonical hreflang metadata"
  ],
  "languages_and_themes": [
    "zh-CN light public route review",
    "en light public route review",
    "zh-CN auth aside core locale review",
    "en auth aside core locale review"
  ],
  "states": [
    "Chinese route default locale",
    "English /en forced locale",
    "guest home navigation logo and docs links",
    "models pricing public copy",
    "docs index and reader query state",
    "auth aside login and register text",
    "server-rendered public HTML head and ETag states"
  ],
  "viewports": [
    "390x844",
    "768x1024",
    "1280x860",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/english-seo-layer/prototype-english-seo-layer.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/english-seo-layer/baseline-english-seo-layer.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/english-seo-layer/updated-english-seo-layer.png"
  ],
  "commands": [
    "python3 generated static English SEO layer review boards with PIL",
    "pnpm --dir frontend exec vitest run src/i18n/__tests__/lazyLocaleScope.spec.ts src/i18n/__tests__/englishBrandNoCountryFraming.spec.ts src/utils/__tests__/routeSeo.spec.ts src/i18n/__tests__/homeTruthfulness.spec.ts src/i18n/locales/__tests__/jisudeng-pages.spec.ts",
    "pnpm --dir frontend run typecheck",
    "pnpm --dir frontend run build",
    "go test -tags embed ./internal/web",
    "go test ./internal/handler -run TestBuildPromptLibrarySitemapContainsOnlyProvidedPublishedPrompts|TestPromptRequestOriginUsesCanonicalProductionHost|TestBuildRobotsTxtAdvertisesSitemapAndKeepsPrivateAPIsOut",
    "make test",
    "git diff --check"
  ],
  "checks": {
    "keyboard": {
      "status": "not-applicable",
      "reason": "No new keyboard interaction pattern was introduced; existing router-link and button controls remain in place."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "No new animation, transition, or motion behavior was introduced."
    }
  },
  "residual_risks": [
    "Artifacts are static review boards, so final browser acceptance should still inspect /home, /en, /models, /en/models, /docs, /en/docs, /login, and /register before production acceptance.",
    "Search result snippets and sitelinks depend on crawler recrawl timing even after sitemap, canonical, and hreflang are deployed."
  ]
}
-->

## Scope

This review covers the English brand layer added on top of existing public product pages, not a separate landing site. The changed visible behavior is limited to locale selection, English public copy, English docs content, public route navigation, and crawler-visible metadata for `/en`, `/en/models`, and `/en/docs`. Chinese routes remain the default product experience.

## Baseline

The baseline public surface was Chinese-first: `/home`, `/models`, and `/docs` existed, but there was no official English URL layer for search engines to index. Public HTML metadata was not path-aware for English pages, and the auth shell could expose raw `authAside.*` keys when the lightweight locale bundle missed those strings.

Baseline artifact: `docs/visual-reviews/assets/english-seo-layer/baseline-english-seo-layer.png`.

## Prototype

The prototype keeps the existing product surfaces and adds English URL coverage only where it matches real functionality. The public English positioning is `Jisudeng: One OpenAI-Compatible API for Frontier AI Models` with model-family copy that names DeepSeek, Qwen, Kimi, GLM, GPT, Claude, and Gemini without national framing.

Prototype artifact: `docs/visual-reviews/assets/english-seo-layer/prototype-english-seo-layer.png`.

## Reuse Decision

The implementation reuses `HomeView`, `ModelsView`, `DocsView`, the existing public navigation helpers, the existing i18n lazy-loading system, and the embedded frontend server. The new `routeSeo` helper centralizes public route metadata so App title updates, router title handling, and server-rendered HTML stay aligned. No new page layout, card system, toolbar, or visual component family was introduced.

## State Coverage

Chinese route states cover `/home`, `/models`, `/docs`, `/login`, and `/register` with `zh-CN` copy and no raw locale keys. English route states cover `/en`, `/en/models`, and `/en/docs`, including direct navigation, docs query states, canonical URLs, hreflang alternates, and path-specific ETags. Auth aside state is covered in both core locale bundles so login and register can render before the full locale payload loads.

## Viewport Coverage

The changed files do not introduce new responsive layout primitives. The static boards record the route and language states across mobile, tablet, desktop, and wide desktop viewport targets. Final browser acceptance should still inspect the public pages on real devices or browser emulation before production acceptance.

## Evidence

Updated artifact: `docs/visual-reviews/assets/english-seo-layer/updated-english-seo-layer.png`.

Commands run:

```bash
python3 generated static English SEO layer review boards with PIL
pnpm --dir frontend exec vitest run src/i18n/__tests__/lazyLocaleScope.spec.ts src/i18n/__tests__/englishBrandNoCountryFraming.spec.ts src/utils/__tests__/routeSeo.spec.ts src/i18n/__tests__/homeTruthfulness.spec.ts src/i18n/locales/__tests__/jisudeng-pages.spec.ts
pnpm --dir frontend run typecheck
pnpm --dir frontend run build
go test -tags embed ./internal/web
go test ./internal/handler -run 'TestBuildPromptLibrarySitemapContainsOnlyProvidedPublishedPrompts|TestPromptRequestOriginUsesCanonicalProductionHost|TestBuildRobotsTxtAdvertisesSitemapAndKeepsPrivateAPIsOut'
make test
git diff --check
```

## Residual Risk

Static boards satisfy the local governance gate but do not replace final browser acceptance. Before production acceptance, inspect `/home`, `/en`, `/models`, `/en/models`, `/docs`, `/en/docs`, `/login`, and `/register` for correct language, no raw i18n keys, stable navigation, and non-overlapping text. Search snippets may still lag until Google recrawls the updated routes and sitemap.
