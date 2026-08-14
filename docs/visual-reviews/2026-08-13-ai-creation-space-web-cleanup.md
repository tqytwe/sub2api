# Visual Review: AI Creation Space web entry cleanup

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/router/index.ts",
    "frontend/src/router/publicNavigation.ts",
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/views/admin/PromptsView.vue",
    "frontend/src/components/prompt/prompt-admin.css",
    "frontend/src/components/prompt/PromptCard.vue",
    "frontend/src/components/prompt/PromptLibraryPanel.vue",
    "frontend/src/views/HomeView.vue",
    "frontend/src/views/public/PromptSquareView.vue",
    "frontend/src/views/public/PromptDetailView.vue",
    "frontend/src/views/user/PlayHubView.vue",
    "frontend/src/components/user/dashboard/UserDashboardQuickActions.vue",
    "frontend/src/components/user/dashboard/FirstLoginWelcomeModal.vue",
    "frontend/src/utils/promptLibrary.ts",
    "frontend/src/composables/useImageStudioWorkspace.ts",
    "frontend/src/views/user/NextChatLaunchView.vue",
    "frontend/src/api/user.ts",
    "backend/internal/server/routes/nextchat.go",
    "frontend/src/content/public-docs-data.zh.ts",
    "frontend/src/content/public-docs-data.en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/en/admin/ops.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts",
    "frontend/src/i18n/locales/en/admin/settings.ts",
    "frontend/src/i18n/locales/jisudeng-home.zh.ts",
    "frontend/src/i18n/locales/jisudeng-home.en.ts"
    ,
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/i18n/locales/zh/dashboard.ts"
    ,
    "frontend/src/i18n/locales/zh/admin/ops.ts"
  ],
  "routes_or_surfaces": [
    "/ai-creation-space",
    "/ai",
    "/image-studio",
    "retired public /prompts and /prompts/:id routes",
    "prompt-library to Canvas image-workbench handoff",
    "authenticated sidebar and public home navigation"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "zh-CN/dark",
    "en-US/light",
    "en-US/dark"
  ],
  "states": [
    "default",
    "hover",
    "active",
    "focus-visible",
    "loading",
    "disabled",
    "empty",
    "error",
    "success"
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
    "file docs/visual-reviews/assets/ai-creation-space-web-cleanup/*.png",
    "corepack pnpm --dir frontend design:check",
    "corepack pnpm --dir frontend lint:check",
    "corepack pnpm --dir frontend typecheck",
    "corepack pnpm --dir frontend exec vitest run src/router/__tests__/aiCreationSpaceRouting.spec.ts src/router/__tests__/publicNavigation.spec.ts"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "evidence": "Existing shared navigation and button controls retain keyboard behavior; this change only changes destinations and labels."
    },
    "reduced_motion": {
      "status": "passed",
      "evidence": "No new animation or motion pattern was introduced."
    }
  },
  "residual_risks": [
    "The artifacts are static review boards rather than authenticated browser captures.",
    "The user must verify the deployed AI Creation Space and legacy redirects in a local browser.",
    "NextChat mobile routes, image-generation APIs, feature flags and backend workers remain intentionally retained for the App."
  ]
}
-->

## Scope

- Consolidate web-facing creation navigation under `/ai-creation-space`.
- Keep `/ai` and `/image-studio` as compatibility redirects.
- Remove the retired public `/prompts` and `/prompts/:id` prompt-square pages; Canvas remains the user-facing prompt workspace and the former administrator prompt-governance surface is removed.
- Preserve legacy query parameters so a prompt-library launch can reach the unified entry.
- Allow only a validated image-prompt identifier in the Canvas launch URL; Canvas retrieves the prompt through the authenticated server BFF and only pre-fills the image workbench.
- Keep the internal prompt panel usable after retiring the public prompt pages: selecting a cover uses the prompt directly, while copy, favorite and use actions remain available.
- Do not delete App-facing NextChat/mobile or image-generation backend contracts.

## Baseline

- The sidebar and several public/user surfaces exposed separate NextChat and Image Studio destinations.
- Baseline artifact: `docs/visual-reviews/assets/ai-creation-space-web-cleanup/baseline-1440.png`.

## Prototype

- Prototype artifact: `docs/visual-reviews/assets/ai-creation-space-web-cleanup/prototype-1440.png`.
- Approval status: implementation follows the requested unified navigation name, “AI创作空间”.
- Scope boundary: navigation and web route cleanup only; backend APIs and mobile contracts remain.

## Reuse Decision

- Reused the existing router, AppSidebar, PageFrame route metadata, feature flags, i18n resources and shared controls.
- No new page shell, icon set, card pattern, storage behavior or queue implementation was introduced.

## State Coverage

- Default: one visible creation entry is labeled AI创作空间.
- Active and focus-visible: existing router-link, sidebar and button states remain unchanged.
- Loading/error/disabled: existing route guard and feature-flag behavior remains in force.
- Legacy routes: `/ai` and `/image-studio` redirect to the unified entry.
- Prompt handoff: the browser sends an image-prompt ID and optional version to the launch endpoint; invalid intents fail before a token is issued, and the prompt body is not put in the URL.

## Viewport Coverage

- Static review board prepared at 1440px.
- Existing responsive shell and navigation rules remain responsible for 360px, 768px, 1280px and wide layouts.
- No new fixed width, scroll container, animation or responsive breakpoint was added.

## Evidence

- Baseline: `docs/visual-reviews/assets/ai-creation-space-web-cleanup/baseline-1440.png`.
- Prototype: `docs/visual-reviews/assets/ai-creation-space-web-cleanup/prototype-1440.png`.
- Updated: `docs/visual-reviews/assets/ai-creation-space-web-cleanup/updated-1440.png`.
- Automated route, navigation and design checks are listed in the manifest.

## Residual Risk

- Static artifacts are not a substitute for deployed browser acceptance.
- Removing the backend `nextchat_enabled` or `image_studio_enabled` settings would affect the App and is deliberately out of scope.
- The unified web entry is controlled by `nextchat_enabled`; `image_studio_enabled` remains a backend capability switch used by existing App and asynchronous image workflows.
- Final prompt handoff, legacy redirect query preservation, and mobile navigation verification still require a user's deployed-browser acceptance.
