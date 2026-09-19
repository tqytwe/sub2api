# Canvas Official Direct Entry Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/router/index.ts", "frontend/src/router/publicNavigation.ts", "frontend/src/views/user/CanvasLaunchView.vue"],
  "routes_or_surfaces": ["/ai-creation-space", "/ai", "/image-studio", "Home creation navigation"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["loading", "redirect success", "redirect failure", "retry", "keyboard focus"],
  "viewports": ["360x800", "1280x720"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/canvas-direct-launch/prototype-direct-canvas-entry.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/canvas-direct-launch/baseline-direct-canvas-entry.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/canvas-direct-launch/updated-direct-canvas-entry.png"],
  "commands": ["pnpm design:check", "pnpm lint:check", "pnpm typecheck", "pnpm vitest run src/router/__tests__/aiCreationSpaceRouting.spec.ts src/router/__tests__/publicNavigation.spec.ts"],
  "checks": {
    "keyboard": {"status": "passed", "reason": "The existing retry button and dashboard link remain unchanged."},
    "reduced_motion": {"status": "passed", "reason": "This release changes only navigation metadata and the emitted URL."}
  },
  "residual_risks": ["Final browser acceptance must verify that a guest reaches the Canvas registration or login screen after the production deployment."]
}
-->

## Scope

The Jisudeng entry now forwards every visitor to Canvas without requiring a Jisudeng session or an old NextChat feature flag. It only passes the public `baseUrl` value; the Canvas application owns registration, login, protocol selection, API Key entry, models, and all local configuration.

## Baseline

The existing direct-launch route rendered the shared loading panel and then redirected only after platform-authentication and the `nextchat_enabled` gate allowed it. Legacy URLs also preserved old query values. The visible status-panel presentation was already recorded in the linked baseline artifact.

## Prototype

The existing direct-launch prototype is retained because the visible loading, retry, and failure panel do not change. This is a navigation-only change with no new visual primitive, layout, colors, typography, icon, or motion.

## Reuse Decision

`AppLayout`, `CompactStatusPanel`, `Icon`, the retry button, and the dashboard link are unchanged. The compatible legacy routes remain named redirects and do not render an additional page.

## State Coverage

Loading, redirect success, redirect failure, retry, and keyboard focus retain their existing behavior. On success the target URL is `https://canvas.jisudeng.com?baseUrl=https%3A%2F%2Fapi.jisudeng.com`; no API Key, launch token, prompt context, or Jisudeng login state is forwarded.

## Viewport Coverage

The loading and failure panel uses the unchanged shared `CompactStatusPanel`; the linked 360x800 and 1280x720 artifacts remain representative because this release changes no rendered element, spacing, text, or state control.

## Evidence

The focused router tests verify guest and signed-in home navigation converge on the same direct route, legacy paths discard their old query context, the route has no platform-authentication or NextChat gate, and the emitted Canvas URL contains only the public Base URL. The production CORS preflight for `https://canvas.jisudeng.com` against `https://api.jisudeng.com/v1/models` returned `204` with the Canvas origin allowed; an invalid key returned `401`, confirming the address is the OpenAI-compatible API boundary rather than an unauthenticated data endpoint.

## Residual Risk

The static review board is prior evidence for the unchanged presentation. Production browser acceptance is still required to confirm the deployed guest redirect and the official Canvas registration/login screens.
