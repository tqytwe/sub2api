# Visual Review: seo-catalog-route-separation

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/router/index.ts",
    "frontend/src/router/publicNavigation.ts",
    "frontend/src/components/layout/AppHeader.vue",
    "frontend/src/components/modelPlaza/PlazaNavBar.vue",
    "frontend/src/content/public-docs-data.zh.ts",
    "frontend/src/content/featured-models.ts",
    "frontend/src/main.ts",
    "frontend/src/utils/publicContentFallback.ts",
    "frontend/src/utils/routeSeo.ts",
    "frontend/src/utils/public-route-seo-contract.json",
    "backend/internal/web/embed_on.go",
    "backend/internal/web/embed_test.go",
    "backend/internal/handler/prompt_library_handler.go",
    "backend/internal/handler/prompt_library_seo_test.go",
    "frontend/src/i18n/locales/zh/admin/settings.ts",
    "frontend/src/i18n/locales/en/admin/settings.ts"
  ],
  "routes_or_surfaces": ["/catalog", "/catalog/:family", "/en/catalog", "/en/catalog/:family", "legacy /models HTML redirect"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "hover", "focus-visible", "loading", "empty", "error", "legacy redirect", "javascript-disabled semantic fallback"],
  "viewports": ["360x800", "768x900", "1280x800", "1920x1080"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/model-family-seo/prototype-model-family-seo.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/model-family-seo/baseline-model-family-seo.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/model-family-seo/updated-model-family-seo.png"],
  "commands": ["pnpm exec vitest run src/utils/__tests__/routeSeo.spec.ts src/utils/__tests__/publicContentFallback.spec.ts src/utils/__tests__/publicLocaleRoute.spec.ts src/router/__tests__/publicNavigation.spec.ts src/components/layout/__tests__/AppHeader.modelPlaza.spec.ts", "go test -tags embed ./internal/web -run 'TestInjectRouteSEO|TestSSRPublicRouteSEOAlignsWithCrossLayerContract|TestFrontendServer|TestServeEmbeddedFrontend' -count=1", "go test ./internal/handler -run 'TestBuildExtendedAIReferenceFilesExposePublicTextOnlyGuidance|TestBuildPromptLibrarySitemapContainsOnlyExistingPublicPages' -count=1", "pnpm design:check"],
  "checks": {
    "keyboard": {"status": "passed", "notes": "The existing router-link focus-visible behavior and keyboard navigation are retained because the catalog reuses ModelPlazaView."},
    "reduced_motion": {"status": "not-applicable", "reason": "This route and metadata separation introduces no visual motion."}
  },
  "residual_risks": ["The evidence is a reusable static review board, not a browser or production capture. Final local-browser acceptance must inspect the redirect and catalog states after deployment."]
}
-->

## Scope

- The public ModelPlaza surface moves from browser-facing `/models` to canonical `/catalog` and
  `/en/catalog`; `ModelPlazaView` itself, shared public frame, and visual density are reused.
- A non-credentialed browser navigation to legacy `/models` or its former public family paths
  becomes a 308 redirect to the matching catalog path. API requests remain on the protected root
  endpoint and do not render a UI surface.
- Indexable public routes also provide route-specific semantic text inside `#app` before JavaScript
  mounts. Vue replaces that container during normal startup, so the interactive catalog does not
  gain a second visible content layer; crawlers and JavaScript-disabled clients receive a meaningful
  document rather than an empty SPA root.

## Baseline

- The same string `/models` previously represented both the protected OpenAI-compatible endpoint
  and a public model page. Caches and crawlers could therefore reach an API authentication response
  instead of a page document.
- The reusable baseline board records the existing ModelPlaza family layout before the route
  canonicalization; no shell, component, color, typography, card, or motion change is intended.

## Prototype

- `prototype-model-family-seo.png` is a valid 1440x1040 static review board for the existing
  ModelPlaza information hierarchy and family navigation.
- The design boundary is deliberately limited to URL ownership, localized navigation, and metadata.
  `/catalog` reuses the approved page rather than introducing a new public visual pattern.

## Reuse Decision

- Reused `ModelPlazaView`, the public route frame, existing navigation links, semantic theme tokens,
  loading placeholder, empty state, error state, and keyboard focus treatment.
- No icon, button, card, color token, viewport breakpoint, layout transition, or animation was added.

## State Coverage

- Default, hover, and focus-visible states remain those of the existing ModelPlaza controls.
- Loading, empty, and error states remain bound to the unchanged public model catalog request.
- Chinese and English family paths retain the same content surface; only URL, canonical, and locale
  destination differ. Legacy HTML navigation ends before a page is painted, so it has no transient
  blank catalog frame.
- The JavaScript-disabled state uses the same title, route description, catalog, and documentation
  destinations as the interactive route. Its semantic fallback is inside `#app`, which Vue clears
  before normal interactive rendering.

## Viewport Coverage

- Static review covers 360px, 768px, 1280px, and 1920px targets in Chinese/English and light/dark.
- The route-only change adds no motion. Reduced-motion behavior is unchanged; 200% zoom and all
  listed states remain required local-browser review items after deployment.

## Evidence

- The three referenced PNGs are real, decodable static review assets for the same ModelPlaza family
  surface. `artifact_mode` intentionally remains `static-review-board` and does not claim a browser
  capture.
- Automated evidence is the focused SEO/locale/navigation test set, the SSR/hydration cross-layer
  metadata contract test, the sitemap/`llms-full.txt` inventory test, and the design-governance check.

## Residual Risk

- This does not prove production routing, CDN cache variation, crawler rendering, or external
  webmaster ownership. After deployment, a local browser must verify `/catalog`, `/en/catalog`, a
  legacy `/models` document navigation, dark/light themes, keyboard focus, 200% zoom, and all three
  roles before acceptance is recorded.
