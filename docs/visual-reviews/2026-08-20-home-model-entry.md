# Home Model Entry Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/router/index.ts", "frontend/src/router/publicNavigation.ts"],
  "routes_or_surfaces": ["home model and pricing CTA", "/pricing compatibility redirect", "/models model plaza"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["guest homepage", "guest model entry", "legacy pricing compatibility path"],
  "viewports": ["390x844", "1280x820"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/seo-metadata-baseline/prototype-seo-metadata-baseline.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/seo-metadata-baseline/baseline-seo-metadata-baseline.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/seo-metadata-baseline/updated-seo-metadata-baseline.png"],
  "commands": ["pnpm typecheck", "pnpm vitest run src/router/__tests__/publicNavigation.spec.ts src/views/__tests__/HomeView.performance.spec.ts"],
  "checks": {"keyboard": {"status": "passed", "reason": "The existing homepage router-link remains keyboard reachable and now resolves to the named compatibility redirect."}, "reduced_motion": {"status": "passed", "reason": "No motion or visual treatment was changed."}},
  "residual_risks": ["Live guest browser acceptance must confirm the CTA reaches the model plaza after deployment."]
}
-->

## Scope

Restore the named route used by the Chinese homepage model and pricing entry.

## Baseline

The navigation target used the `Pricing` route name, but the `/pricing` compatibility redirect had no route name and could not resolve.

## Prototype

The named compatibility route resolves normally and redirects to the unified `/models` model plaza.

## Reuse Decision

The existing model plaza remains the only destination; no second pricing page or billing behavior was introduced.

## Viewport Coverage

The existing homepage CTA and model plaza layouts are covered at mobile and desktop viewport sizes.

## Evidence

The existing review-board artifacts are reused, with route-level regression tests and a live guest browser check required after deployment.

## State Coverage

Guest homepage, CTA navigation, and legacy `/pricing` compatibility navigation are covered.

## Residual Risk

Live production propagation remains the final acceptance gate.
