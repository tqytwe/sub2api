# Home Mobile Hero Readability

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/home/HeroSphere.vue",
    "frontend/src/styles/home-view.css"
  ],
  "routes_or_surfaces": [
    "/home public home hero",
    "/en English public home hero"
  ],
  "languages_and_themes": [
    "zh-CN light public route",
    "en light public route"
  ],
  "states": [
    "mobile first-run hero after intro reveal",
    "mobile header brand and public toolbar",
    "mobile primary and secondary hero CTA readability",
    "mobile active tool list readability"
  ],
  "viewports": [
    "390x844",
    "1366x900"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/home-mobile-hero-readability/updated-home-mobile.png",
    "docs/visual-reviews/assets/home-mobile-hero-readability/updated-en-mobile.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/home-mobile-hero-readability/baseline-home-mobile.png",
    "docs/visual-reviews/assets/home-mobile-hero-readability/baseline-en-mobile.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/home-mobile-hero-readability/updated-home-mobile.png",
    "docs/visual-reviews/assets/home-mobile-hero-readability/updated-en-mobile.png"
  ],
  "commands": [
    "Playwright Chromium captured production baseline screenshots for https://www.jisudeng.com/home and https://www.jisudeng.com/en at 390x844 after the intro reveal",
    "Playwright Chromium captured local updated screenshots for http://127.0.0.1:4175/home and http://127.0.0.1:4175/en at 390x844 after the intro reveal",
    "Playwright Chromium measured desktop production /home and /en at 1366x900 during the SEO deploy check",
    "pnpm exec vitest run src/views/__tests__/HomeView.performance.spec.ts src/utils/__tests__/publicLocaleRoute.spec.ts src/utils/__tests__/routeSeo.spec.ts src/i18n/__tests__/englishBrandNoCountryFraming.spec.ts",
    "pnpm typecheck",
    "pnpm build",
    "git diff --check"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "The change repositions the decorative canvas and adjusts mobile readability styling; no focusable controls or tab order changed."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "No new animation was added. The existing HeroSphere animation keeps its timing and only changes the mobile resting center."
    }
  },
  "residual_risks": [
    "The updated screenshots are local browser captures before deployment. Production must be rechecked after merge and Zeabur rollout.",
    "The first-run intro overlay still intentionally covers the page during its animation; this review covers the revealed mobile hero state after the intro finishes."
  ]
}
-->

## Scope

This review covers the mobile public home hero on `/home` and `/en`. The goal is to keep the bilingual headline, supporting copy, primary CTA, secondary CTA and tool list readable after the first-run intro reveal, with no horizontal overflow.

## Baseline

After the SEO and locale fix deployed, the mobile hero no longer overflowed horizontally, but the decorative globe sat high enough to cross the lower hero controls and active tool list. The Chinese mobile header could also wrap the brand name into two lines.

Baseline artifacts:

- `docs/visual-reviews/assets/home-mobile-hero-readability/baseline-home-mobile.png`
- `docs/visual-reviews/assets/home-mobile-hero-readability/baseline-en-mobile.png`

## Prototype

The correction keeps the existing visual system and home hero structure. On mobile only, the resting globe center moves lower in the hero canvas, while secondary CTA and active tool text receive a translucent page-background readability layer. The brand name is prevented from wrapping on mobile.

Prototype artifacts:

- `docs/visual-reviews/assets/home-mobile-hero-readability/updated-home-mobile.png`
- `docs/visual-reviews/assets/home-mobile-hero-readability/updated-en-mobile.png`

## Reuse Decision

The implementation reuses `HeroSphere`, the existing hero CTA classes, the existing `active-on` list, and existing `--bg` design token. No new component, card, color palette, or alternate mobile header pattern was introduced.

## State Coverage

Covered states include `/home` Chinese mobile after intro reveal, `/en` English mobile after intro reveal, mobile brand rendering, mobile H1 bounds, primary CTA readability, secondary CTA readability and active tool list readability.

## Viewport Coverage

Browser verification used a `390 x 844` viewport, matching the reported mobile layout class, and the earlier production deploy check covered `1366 x 900` desktop. Both mobile `/home` and `/en` reported `scrollWidth=390` and `bodyScrollWidth=390`; the H1 bounding boxes stayed inside the viewport with `x=4`, `right=386`, and `width=382`. Desktop rendering is not changed because the globe center helper returns the previous center value outside mobile.

## Evidence

Updated artifacts:

- `docs/visual-reviews/assets/home-mobile-hero-readability/updated-home-mobile.png`
- `docs/visual-reviews/assets/home-mobile-hero-readability/updated-en-mobile.png`

Commands run:

```bash
pnpm exec vitest run src/views/__tests__/HomeView.performance.spec.ts src/utils/__tests__/publicLocaleRoute.spec.ts src/utils/__tests__/routeSeo.spec.ts src/i18n/__tests__/englishBrandNoCountryFraming.spec.ts
pnpm typecheck
pnpm build
git diff --check
```

## Residual Risk

The updated screenshots are local browser captures. After this patch is merged, production still needs the same `/home` and `/en` mobile route check against `https://www.jisudeng.com/` after Zeabur rollout. The first-run intro overlay remains a deliberate animation state before the hero is revealed.
