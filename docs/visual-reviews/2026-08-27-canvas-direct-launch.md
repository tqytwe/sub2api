# Canvas Direct Launch Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/views/user/CanvasLaunchView.vue"],
  "routes_or_surfaces": ["/ai-creation-space", "/ai", "/image-studio"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["loading", "redirect success", "redirect failure", "retry", "keyboard focus"],
  "viewports": ["360x800", "1280x720"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/canvas-direct-launch/prototype-direct-canvas-entry.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/canvas-direct-launch/baseline-direct-canvas-entry.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/canvas-direct-launch/updated-direct-canvas-entry.png"],
  "commands": ["playwright screenshot --device=Desktop Chrome --full-page file:///tmp/canvas-direct-launch-prototype.html docs/visual-reviews/assets/canvas-direct-launch/prototype-direct-canvas-entry.png", "pnpm design:check", "pnpm test -- --run src/router/__tests__/aiCreationSpaceRouting.spec.ts"],
  "checks": {
    "keyboard": {"status": "passed", "reason": "The existing retry and dashboard actions remain native buttons and router links."},
    "reduced_motion": {"status": "passed", "reason": "No motion, layout, or visual component behavior changed."}
  },
  "residual_risks": ["Final browser acceptance must verify the production redirect at all supported desktop and mobile widths after deployment."]
}
-->

## Scope

The protected main-site entry keeps the existing `AppLayout` and `CompactStatusPanel`. Only its target changes from a tokenized Canvas session to the Canvas root URL, with optional non-authentication prompt context.

## Baseline

The prior loading state rendered the shared status panel and then requested a one-time launch token. That authentication query parameter was the source of the unnecessary cross-service dependency.

## Prototype

The prototype preserves the shared compact loading panel, spacing, title hierarchy, and retry behavior. It documents the direct Canvas target and intentionally introduces no new button, card, icon, or page-frame pattern.

## Reuse Decision

`AppLayout`, `CompactStatusPanel`, `Icon`, existing button classes, route protection, and compatibility redirects are reused. No new visual primitive or design-system exception is needed.

## State Coverage

Loading continues to show the existing panel; a successful redirect replaces the document. The existing failure state retains retry and dashboard actions. No input, hover, disabled, or animated state was added.

## Viewport Coverage

The shared compact panel is already responsible for the 360px and 1280px layouts. This change adds no container, width, gutter, text, or theme rule.

## Evidence

The static review board is a real Playwright PNG. Automated route assertions verify that no `launch_token` or launch API call remains in the Canvas entry while legacy URLs and mobile session routes remain intact.

## Residual Risk

Production browser acceptance remains required because this local review cannot prove deployed Canvas domain availability or browser navigation behavior.
