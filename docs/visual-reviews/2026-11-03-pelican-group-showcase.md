# Visual Review: Pelican Group Tests and Showcase

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/router/index.ts",
    "frontend/src/features/channel-monitor-v2/MonitorStatusCards.vue",
    "frontend/src/components/user/pelican/PelicanArtworkPreview.vue",
    "frontend/src/components/user/pelican/PelicanGroupHistoryDialog.vue",
    "frontend/src/components/user/pelican/PelicanShowcaseApiDialog.vue",
    "frontend/src/components/user/pelican/PelicanShowcaseCard.vue",
    "frontend/src/components/user/pelican/PelicanShowcaseRow.vue",
    "frontend/src/components/user/pelican/pelicanShowcaseFormat.ts",
    "frontend/src/i18n/locales/en/pelicanShowcase.ts",
    "frontend/src/i18n/locales/en/pelicanTests.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/channelMonitorV2.ts",
    "frontend/src/i18n/locales/en/index.ts",
    "frontend/src/i18n/locales/en/workspaceShell.ts",
    "frontend/src/i18n/locales/zh/pelicanShowcase.ts",
    "frontend/src/i18n/locales/zh/pelicanTests.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/channelMonitorV2.ts",
    "frontend/src/i18n/locales/zh/index.ts",
    "frontend/src/i18n/locales/zh/workspaceShell.ts",
    "frontend/src/views/admin/PelicanTestsView.vue",
    "frontend/src/views/user/PelicanShowcaseView.vue",
    "frontend/src/views/user/ChannelStatusV2View.vue"
  ],
  "routes_or_surfaces": ["/admin/pelican-tests", "/monitor group cards and history dialog"],
  "languages_and_themes": ["zh-CN/light"],
  "states": ["default", "loading", "empty", "error", "success", "disabled", "focus-visible", "keyboard navigation", "reduced motion"],
  "viewports": ["390x844", "1280x1360"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/pelican-group-showcase/prototype-v2-group-card.png",
    "docs/visual-reviews/assets/pelican-group-showcase/prototype-result-preview.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/pelican-group-showcase/baseline-monitor-v2-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/pelican-group-showcase/updated-admin-1280.png",
    "docs/visual-reviews/assets/pelican-group-showcase/updated-monitor-pelican-modal-1280.png"
  ],
  "commands": [
    "Playwright browser screenshot on Vite preview with fixed mock API responses; baseline from b0f52cda5 and updated routes from this worktree",
    "cd frontend && pnpm typecheck",
    "cd frontend && pnpm vitest run src/router/__tests__/pelicanChannelStatusRouting.spec.ts src/views/admin/__tests__/PelicanTestsView.spec.ts src/features/channel-monitor-v2/__tests__/MonitorStatusCards.pelican.spec.ts"
  ],
  "checks": {
    "keyboard": { "status": "passed" },
    "reduced_motion": { "status": "passed" }
  },
  "residual_risks": [
    "Screenshots use fixed mock data and verify implementation rendering only; final production behavior and real account permissions still require the user's local browser after deployment."
  ]
}
-->

## Scope

- Admin route: `/admin/pelican-tests`. User results stay in the existing `/monitor` group cards; Candy summaries are inline and Pelican artwork history opens from its group card.
- Role: administrator configures scheduled tests; users see permitted group results without a separate showcase route or sidebar item.
- Screenshots use Chinese light theme at 1280px. The earlier standalone gallery screenshots document the discarded intermediate implementation and are not part of the final surface.

## Scope Correction

- The standalone `/pelican-showcase` user page and its sidebar entry were removed after review. They split the result from Channel Status and were not the requested user workflow.
- The administrator's view action opens `/monitor`; scheduled configuration and the per-group history dialog remain.

## Baseline

- `baseline-monitor-v2-1280.png` is the prior V2 Candy card implementation from `b0f52cda5`, before the Pelican history entry was added.
- The admin scheduler and inline group-card history dialog did not exist in that baseline.

## Prototype

- The group-card prototype defines the user-facing result location; the artwork-preview prototype defines the history dialog content.
- They define the feature boundary; the screenshots below show rendered implementation, not prototype mockups.

## Reuse Decision

- Reuses the existing application layout, sidebar, shared icons, buttons, dialogs, empty states, and semantic color tokens.
- Artwork remains data and is rendered in a restricted iframe; it is not inserted into the application DOM.

## State Coverage

- Default and success: admin group plan/history and V2 group-card history dialog are captured.
- Loading, empty, error, disabled, and save/run states are covered by focused Vue tests.
- Keyboard: native buttons and dialog controls retain visible focus indicators.

## Viewport Coverage

- Desktop: 1280px admin and V2 card-dialog views are captured.
- Artwork remains in the selected group history dialog and sandboxed preview.

## Evidence

- Baseline: `baseline-monitor-v2-1280.png`.
- Updated: admin and V2 card-dialog captures listed in the manifest.
- Browser captures use fixed mock API responses and do not access production accounts or data.

## Residual Risk

- Production deployment and user-local browser acceptance remain outstanding; mock screenshots do not verify live permissions, migration execution, or real model output.
