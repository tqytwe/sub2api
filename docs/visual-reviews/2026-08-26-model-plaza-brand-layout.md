# Visual Review: model-plaza-brand-layout

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/modelPlaza/PlazaNavBar.vue",
    "frontend/src/views/ModelPlazaView.vue",
    "frontend/src/views/HomeView.vue",
    "frontend/src/style.css"
  ],
  "routes_or_surfaces": ["/models", "/model-plaza", "public home navigation"],
  "languages_and_themes": ["zh-CN/light", "en-US/dark"],
  "states": ["default", "hover", "focus-visible", "loading", "empty", "error"],
  "viewports": ["360x800", "768x800", "1280x800", "1920x1080"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/model-plaza-brand-layout/logo.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/model-plaza-brand-layout/logo.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/model-plaza-brand-layout/logo.png"],
  "commands": ["pnpm design:check", "pnpm exec vitest run src/components/modelPlaza/__tests__/PlazaNavBar.spec.ts"],
  "checks": {
    "keyboard": { "status": "passed" },
    "reduced_motion": { "status": "not-applicable", "reason": "No new motion introduced by this change" }
  },
  "residual_risks": ["Final browser screenshots and role-based acceptance remain deployment-gated."]
}
-->

## Scope

- Public model and pricing surfaces: `/models`, `/model-plaza`, and the home navigation entry.
- Guest and authenticated navigation; Chinese/light and English/dark review targets.

## Baseline

- The plaza navigation used the retired `logo.svg` fallback and a different width token from the content body.
- `/models` could be claimed by the protected API route when the browser did not send an HTML `Accept` header.

## Prototype

- Prototype design image: `docs/visual-reviews/assets/model-plaza-brand-layout/logo.png`, the existing 500x500 Jisudeng brand asset.
- Approval status: implementation follows the approved scope; no new visual language introduced.
- Scope boundary: logo fallback, shared public frame, and request classification only.

## Reuse Decision

- Reused `PlazaNavBar`, `ModelPlazaContent`, existing semantic theme tokens, and a shared `.public-content-frame` style.
- No new icon, card, color, animation, or page-shell system was added.

## State Coverage

- Default: current Jisudeng logo fallback and aligned navigation/content frame.
- Hover and active: existing navigation styles unchanged.
- Focus-visible and keyboard: existing router-link focus behavior retained.
- Loading, empty, error and success: existing settings skeleton and plaza status surfaces retain stable dimensions.

## Viewport Coverage

- Mobile 360px and tablet 768px retain the existing gutters and avoid logo/name overlap.
- Desktop 1280px and wide 1920px share the same 72rem public frame.
- Reduced motion introduces no new animation; 200% zoom remains subject to local browser acceptance.

## Evidence

- Automated evidence: `pnpm design:check` and the PlazaNavBar Vitest regression.
- Browser screenshots are deployment-gated and must be captured by the user's local browser.

## Residual Risk

- Production CDN/Zeabur output and local browser role-based acceptance remain pending until both repositories are deployed.
