# Home Locale SEO Fix Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/index.html",
    "frontend/src/components/common/LocaleSwitcher.vue",
    "frontend/src/router/index.ts",
    "frontend/src/utils/publicLocaleRoute.ts",
    "frontend/src/utils/__tests__/publicLocaleRoute.spec.ts",
    "frontend/src/views/HomeView.vue",
    "frontend/src/views/__tests__/HomeView.performance.spec.ts",
    "frontend/src/utils/routeSeo.ts",
    "frontend/src/utils/__tests__/routeSeo.spec.ts",
    "backend/internal/handler/prompt_library_handler.go",
    "backend/internal/handler/prompt_library_seo_test.go",
    "backend/internal/web/embed_on.go",
    "backend/internal/web/embed_test.go"
  ],
  "routes_or_surfaces": [
    "/home Chinese public home",
    "/en English public home",
    "/models and /docs public locale switch targets",
    "/sitemap.xml static public URL inventory",
    "public route SEO head metadata"
  ],
  "languages_and_themes": [
    "zh-CN light public route",
    "en light public route"
  ],
  "states": [
    "Chinese route default locale",
    "English route forced locale",
    "public language switch from zh to en",
    "public language switch from en to zh",
    "English home headline spacing",
    "sitemap canonical homepage listing"
  ],
  "viewports": [
    "390x844",
    "768x1024",
    "1280x860",
    "1366x900",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/home-locale-seo-fix/prototype-home-locale-seo-fix.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/home-locale-seo-fix/baseline-home-locale-seo-fix.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/home-locale-seo-fix/updated-home-locale-seo-fix.png"
  ],
  "commands": [
    "python3 generated static review boards with PIL",
    "pnpm --dir frontend exec vitest run src/views/__tests__/HomeView.performance.spec.ts src/utils/__tests__/publicLocaleRoute.spec.ts src/i18n/__tests__/lazyLocaleScope.spec.ts src/utils/__tests__/routeSeo.spec.ts src/i18n/__tests__/englishBrandNoCountryFraming.spec.ts",
    "go test -tags embed ./internal/web",
    "go test ./internal/handler -run TestBuildPromptLibrarySitemapContainsOnlyProvidedPublishedPrompts|TestPromptRequestOriginUsesCanonicalProductionHost|TestBuildRobotsTxtAdvertisesSitemapAndKeepsPrivateAPIsOut",
    "pnpm --dir frontend run typecheck",
    "pnpm --dir frontend run build",
    "make build",
    "git diff --check"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "LocaleSwitcher keeps the existing button/menu controls and changes only the selected public-route navigation target."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "The fix does not add animation; the existing first-run HeroSphere intro remains a separate production acceptance risk."
    }
  },
  "residual_risks": [
    "Static review boards are not browser screenshots. Firefox headless immediate screenshots captured the existing first-run intro overlay before reveal, so final production browser acceptance must still inspect /home and /en after the intro.",
    "Search result snippets and ranking will lag until the fixed branch is merged, deployed, sitemap is recrawled, and webmaster tools request indexing."
  ]
}
-->

## Scope

This review covers a bugfix, not a new landing-page design. The changed visible behavior is limited to the public language switcher and the English home headline structure. The crawler-visible behavior changes are route metadata alignment and removal of `/home` from the generated sitemap.

## Baseline

The baseline risk was that a Chinese public URL could display English after a public language switch because the switcher changed locale state without moving to `/en`. The English home headline also reused the Chinese three-part title structure, which could produce joined crawler text such as `JisudengOne APIfor AI models`. The sitemap listed both `/` and `/home`, even though `/home` canonicalizes to `/`.

Baseline artifact: `docs/visual-reviews/assets/home-locale-seo-fix/baseline-home-locale-seo-fix.png`.

## Prototype

The prototype keeps the current Home layout and public toolbar. Public language selection becomes path-based: English goes to `/en`, `/en/models`, or `/en/docs`; Chinese returns to `/`, `/models`, or `/docs`. The English headline uses a dedicated two-line structure with a real DOM text space between `Jisudeng` and `One API for AI models`.

Prototype artifact: `docs/visual-reviews/assets/home-locale-seo-fix/prototype-home-locale-seo-fix.png`.

## Reuse Decision

The implementation reuses `LocaleSwitcher`, `PublicPageToolbar`, `HomeView`, the existing public route names, and the existing `routeSeo` metadata helper. No new button, menu, card, layout, icon, or color system was introduced.

## State Coverage

Default public route state is covered by tests for `/home` and `/en`. Language-switch target state is covered by `resolvePublicLocaleRoute` tests for `/`, `/home`, `/models`, `/docs`, `/login`, `/register`, `/about`, `/contact`, `/en`, `/en/models`, and `/en/docs`. SEO state is covered by frontend route metadata tests, embedded SSR metadata tests, and sitemap/robots tests.

## Viewport Coverage

The English headline uses `clamp`, explicit width constraints, and `text-wrap: balance` with mobile overrides to prevent overflow. The changed language switch behavior does not alter menu layout dimensions.

## Evidence

Updated artifact: `docs/visual-reviews/assets/home-locale-seo-fix/updated-home-locale-seo-fix.png`.

Commands run:

```bash
python3 generated static review boards with PIL
pnpm --dir frontend exec vitest run src/views/__tests__/HomeView.performance.spec.ts src/utils/__tests__/publicLocaleRoute.spec.ts src/i18n/__tests__/lazyLocaleScope.spec.ts src/utils/__tests__/routeSeo.spec.ts src/i18n/__tests__/englishBrandNoCountryFraming.spec.ts
go test -tags embed ./internal/web
go test ./internal/handler -run 'TestBuildPromptLibrarySitemapContainsOnlyProvidedPublishedPrompts|TestPromptRequestOriginUsesCanonicalProductionHost|TestBuildRobotsTxtAdvertisesSitemapAndKeepsPrivateAPIsOut'
pnpm --dir frontend run typecheck
pnpm --dir frontend run build
make build
git diff --check
```

## Residual Risk

Firefox headless immediate screenshots showed the existing first-run Home intro overlay before reveal, so those screenshots were not used as updated visual proof. Final acceptance still needs a real browser check of `/home`, `/en`, `/models`, `/en/models`, `/docs`, `/en/docs`, `/login`, and `/register` after deployment. Search engine snippets will also lag until the deployed sitemap and head metadata are recrawled.
