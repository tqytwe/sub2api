# Visual Review: Home Desktop Header And Hero

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/styles/home-view.css",
    "frontend/src/components/home/HeroSphere.vue"
  ],
  "routes_or_surfaces": ["/en", "/home"],
  "languages_and_themes": ["en/light", "zh-CN/light"],
  "states": ["desktop public header", "desktop hero after intro reveal", "public language routing"],
  "viewports": ["1366x900", "1920x1080"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-07-24-home-desktop-header-hero/prototype-en-1920.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-07-24-home-desktop-header-hero/before-en-1920.png",
    "docs/visual-reviews/assets/2026-07-24-home-desktop-header-hero/before-en-1366.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-07-24-home-desktop-header-hero/after-en-1920.png",
    "docs/visual-reviews/assets/2026-07-24-home-desktop-header-hero/after-en-1366.png",
    "docs/visual-reviews/assets/2026-07-24-home-desktop-header-hero/after-home-1366.png"
  ],
  "commands": [
    "npx -y pnpm@9.15.9 dev --host 127.0.0.1 --port 5179",
    "node Playwright desktop capture script for /en and /home at 1920x1080 and 1366x900"
  ],
  "checks": {
    "keyboard": { "status": "passed" },
    "reduced_motion": { "status": "passed" }
  },
  "residual_risks": ["Production CDN rollout still needs post-deploy browser capture after merge."]
}
-->

## Scope

- Routes: `/en`, `/home`
- Roles: public guest header and hero
- Languages and themes: English light and Chinese light

## Baseline

- Current behavior: English desktop navigation wrapped because labels with spaces were allowed to break. The header expanded to 132px at 1920x1080 and 1366x900.
- The desktop hero globe sat behind the English headline and body copy, making the first viewport harder to read.
- Baseline screenshots are stored in the manifest paths.

## Prototype

- Prototype target: keep all desktop header controls on one line, keep the English hero text clear, and retain only a lower-edge globe presence in the first viewport.
- Approval status: applied as a bugfix to match the user's screenshot feedback.
- Scope boundary: no copy, SEO, route, auth, or pricing behavior changes.

## Reuse Decision

- Reused the existing `HomeView.vue` header structure, `PublicPageToolbar`, and `HeroSphere` canvas.
- No new shared component or visual pattern was introduced.
- Design-system exception: none.

## State Coverage

- Default: guest desktop header with primary navigation, locale switch, theme switch, Android CTA, sign-in link, and register CTA.
- Hover and active: existing link/button hover states are unchanged.
- Focus-visible and keyboard: DOM order and focusable controls are unchanged.
- Loading, disabled, empty, error and success: not applicable to this public static header/hero fix.

## Viewport Coverage

- Desktop: 1366x900 and 1920x1080 browser captures.
- Mobile: existing mobile rules are preserved; the desktop no-wrap rules are gated at `min-width: 768px`.
- Tablet: covered by the same desktop guard above 768px and by existing mobile behavior below 768px.
- 200% zoom and reduced motion: no animation timing or motion preference logic changed.

## Evidence

- Updated screenshots are stored in the manifest paths.
- Automated capture metrics after the fix:
  - `/en` 1920x1080: `lang=en`, header `76px`, `overflowX=0`, no visible CJK, all nav actions `white-space: nowrap`.
  - `/en` 1366x900: `lang=en`, header `76px`, `overflowX=0`, no visible CJK, all nav actions `white-space: nowrap`.
  - `/home` 1366x900: `lang=zh-CN`, header `76px`, `overflowX=0`, Chinese copy retained.

## Residual Risk

- Production CDN and Zeabur rollout still need final browser evidence after merge.
