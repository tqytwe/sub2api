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
  "routes_or_surfaces": ["/admin/pelican-tests", "/pelican-showcase", "/channel-status-v2 group card history dialog"],
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
    "docs/visual-reviews/assets/pelican-group-showcase/showcase-user-1280.png",
    "docs/visual-reviews/assets/pelican-group-showcase/showcase-user-390.png",
    "docs/visual-reviews/assets/pelican-group-showcase/updated-monitor-pelican-modal-1280.png"
  ],
  "commands": [
    "Playwright browser screenshot on Vite preview with fixed mock API responses; baseline from b0f52cda5 and updated routes from this worktree",
    "cd frontend && pnpm typecheck",
    "cd frontend && pnpm vitest run src/views/admin/__tests__/PelicanTestsView.spec.ts src/views/user/__tests__/PelicanShowcaseView.spec.ts src/features/channel-monitor-v2/__tests__/MonitorStatusCards.pelican.spec.ts"
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

- Routes: `/admin/pelican-tests`, `/pelican-showcase`, and the Pelican history dialog opened from a V2 channel group card.
- Role: administrator configures scheduled tests; users see permitted groups and published artwork.
- Screenshots use Chinese light theme at 390px and 1280px.

## Baseline

- `baseline-monitor-v2-1280.png` is the prior V2 Candy card implementation from `b0f52cda5`, before the Pelican history entry was added.
- The admin scheduler and user artwork gallery did not exist in that baseline.

## Prototype

- The two prototype images are the supplied V2 group-card and artwork-preview references.
- They define the feature boundary; the screenshots below show rendered implementation, not prototype mockups.

## Reuse Decision

- Reuses the existing application layout, sidebar, shared icons, buttons, dialogs, empty states, and semantic color tokens.
- Artwork remains data and is rendered in a restricted iframe; it is not inserted into the application DOM.

## State Coverage

- Default and success: admin group plan/history, user gallery cards, and V2 group-card history dialog are captured.
- Loading, empty, error, disabled, and save/run states are covered by focused Vue tests.
- Keyboard: native buttons plus the gallery row's tested arrow/Home/End controls; focus indicators are present.
- Reduced motion: horizontal gallery movement respects `prefers-reduced-motion`.

## Viewport Coverage

- Mobile: the user gallery is captured at 390px; admin configuration is covered by responsive component tests.
- Desktop: 1280px admin, user, and V2 card-dialog views are captured.
- The gallery row keeps a visible keyboard/pointer slider when content overflows; the card preview is constrained to its viewport.

## Evidence

- Baseline: `baseline-monitor-v2-1280.png`.
- Updated: admin, user gallery, and V2 card-dialog captures listed in the manifest.
- Browser captures use fixed mock API responses and do not access production accounts or data.

## Residual Risk

- Production deployment and user-local browser acceptance remain outstanding; mock screenshots do not verify live permissions, migration execution, or real model output.
