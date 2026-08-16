# Visual Review: AI Creation Space Canvas launch

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/router/index.ts",
    "frontend/src/views/user/NextChatLaunchView.vue",
    "frontend/src/views/user/CanvasLaunchView.vue",
    "frontend/src/api/user.ts",
    "backend/internal/config/config.go",
    "backend/internal/server/routes/nextchat.go"
  ],
  "routes_or_surfaces": [
    "/ai-creation-space",
    "/ai",
    "/image-studio"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "zh-CN/dark",
    "en-US/light",
    "en-US/dark"
  ],
  "states": [
    "loading",
    "error",
    "retry"
  ],
  "viewports": [
    "360x800",
    "768x900",
    "1280x900",
    "1440x900",
    "1920x1039"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/ai-creation-space-web-cleanup/prototype-1440.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/ai-creation-space-web-cleanup/baseline-1440.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/ai-creation-space-web-cleanup/updated-1440.png"
  ],
  "commands": [
    "corepack pnpm --dir frontend design:check",
    "corepack pnpm --dir frontend lint:check",
    "corepack pnpm --dir frontend typecheck",
    "corepack pnpm --dir frontend exec vitest run src/router/__tests__/aiCreationSpaceRouting.spec.ts"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "evidence": "The existing shared status panel and retry controls are reused without new keyboard interactions."
    },
    "reduced_motion": {
      "status": "passed",
      "evidence": "No new animation or motion pattern was introduced."
    }
  },
  "residual_risks": [
    "The artifacts are static review boards rather than authenticated browser captures.",
    "The deployed platform must be rebuilt with AI_CREATION_SPACE_PUBLIC_URL set to the Canvas origin or use the canonical default.",
    "The NextChat session and mobile API contracts remain intentionally retained for the App and Canvas session exchange."
  ]
}
-->

## Scope

The authenticated `/ai-creation-space` entry keeps the existing platform loading and
error shell, then launches the managed Infinite Canvas origin with a one-time platform
session token. `/ai` and `/image-studio` remain compatibility redirects.

## Baseline

The prior route loaded `NextChatLaunchView.vue`; its successful launch address was
`/ai`, so authenticated users stayed in the retired NextChat web surface. The baseline
artifact is the existing 1440px review board listed in the manifest.

## Prototype

The prototype keeps the existing compact loading/error shell and changes only the
successful destination to the managed Canvas origin. The existing reviewed prototype
artifact is listed in the manifest because no new visual pattern was introduced.

## Reuse Decision

The existing `AppLayout`, `CompactStatusPanel`, `Icon` and shared retry controls are
reused. No new page width, visual language, animation or storage UI was introduced.

## State Coverage

- Loading: secure launch session is being created.
- Error: existing localized error state keeps retry and dashboard actions.
- Success: the browser is redirected to `https://canvas.jisudeng.com` with the
  one-time launch token; the token is exchanged server-side by Canvas.

## Viewport Coverage

The existing shared shell remains responsible for 360px, 768px, 1280px and wide
layouts. This change adds no fixed width, scroll container or breakpoint.

## Evidence

Baseline, prototype and updated artifacts are the existing reviewed PNGs listed in the
manifest. Automated design, route, type and build checks are recorded during validation.

## Residual Risk

The final acceptance still requires a logged-in browser check on the deployed
`www.jisudeng.com` entry and a Canvas session exchange after the platform deployment.
