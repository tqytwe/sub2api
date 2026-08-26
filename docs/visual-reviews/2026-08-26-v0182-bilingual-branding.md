# Visual Review: v0182-bilingual-branding

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/App.vue",
    "frontend/src/api/client.ts",
    "frontend/src/api/imageStudio.ts",
    "frontend/src/components/account/AccountStatusIndicator.vue",
    "frontend/src/components/account/AccountTestModal.vue",
    "frontend/src/components/account/CreateAccountModal.vue",
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/components/admin/account/AccountTestModal.vue",
    "frontend/src/components/admin/play/AdminInviteGrowthOperations.vue",
    "frontend/src/components/admin/user/BulkUserActionDialog.vue",
    "frontend/src/components/common/HelpTooltip.vue",
    "frontend/src/components/common/NavigationProgress.vue",
    "frontend/src/components/common/Pagination.vue",
    "frontend/src/components/common/ProxySelector.vue",
    "frontend/src/components/common/Select.vue",
    "frontend/src/components/imageStudio/ImageStudioSizePicker.vue",
    "frontend/src/components/home/WhyHoverCard.vue",
    "frontend/src/components/layout/AppHeader.vue",
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/components/layout/AuthLayout.vue",
    "frontend/src/components/layout/PublicContentLayout.vue",
    "frontend/src/components/modelPlaza/PlazaNavBar.vue",
    "frontend/src/components/prompt/PromptCard.vue",
    "frontend/src/components/prompt/PromptFilters.vue",
    "frontend/src/components/prompt/PromptGeneratedCover.vue",
    "frontend/src/components/prompt/PromptLibraryPanel.vue",
    "frontend/src/components/play/RewardCelebrationOverlay.vue",
    "frontend/src/composables/useImageStudioCapabilities.ts",
    "frontend/src/composables/useImageStudioWorkspace.ts",
    "frontend/src/i18n/index.ts",
    "frontend/src/i18n/routeScopes.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en/admin/accounts.ts",
    "frontend/src/i18n/locales/en/admin/overview.ts",
    "frontend/src/i18n/locales/en/admin/playOps.ts",
    "frontend/src/i18n/locales/en/adminShell.ts",
    "frontend/src/i18n/locales/en/common.ts",
    "frontend/src/i18n/locales/en/landing.ts",
    "frontend/src/i18n/locales/en/legacy/admin-accounts.ts",
    "frontend/src/i18n/locales/en/legacy/admin-channels.ts",
    "frontend/src/i18n/locales/en/legacy/admin-ops.ts",
    "frontend/src/i18n/locales/en/legacy/admin-play.ts",
    "frontend/src/i18n/locales/en/legacy/admin-resources.ts",
    "frontend/src/i18n/locales/en/legacy/admin-settings.ts",
    "frontend/src/i18n/locales/en/legacy/core.ts",
    "frontend/src/i18n/locales/en/legacy/user-dashboard.ts",
    "frontend/src/i18n/locales/en/legacy/user-misc.ts",
    "frontend/src/i18n/locales/en/workspaceShell.ts",
    "frontend/src/i18n/locales/jisudeng-home.en.ts",
    "frontend/src/i18n/locales/jisudeng-home.zh.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/i18n/locales/zh/admin/accounts.ts",
    "frontend/src/i18n/locales/zh/adminShell.ts",
    "frontend/src/i18n/locales/zh/admin/playOps.ts",
    "frontend/src/i18n/locales/zh/common.ts",
    "frontend/src/i18n/locales/zh/landing.ts",
    "frontend/src/i18n/locales/zh/legacy/admin-accounts.ts",
    "frontend/src/i18n/locales/zh/legacy/admin-channels.ts",
    "frontend/src/i18n/locales/zh/legacy/admin-ops.ts",
    "frontend/src/i18n/locales/zh/legacy/admin-play.ts",
    "frontend/src/i18n/locales/zh/legacy/admin-resources.ts",
    "frontend/src/i18n/locales/zh/legacy/admin-settings.ts",
    "frontend/src/i18n/locales/zh/legacy/core.ts",
    "frontend/src/i18n/locales/zh/legacy/user-dashboard.ts",
    "frontend/src/i18n/locales/zh/legacy/user-misc.ts",
    "frontend/src/i18n/locales/zh/workspaceShell.ts",
    "frontend/src/router/index.ts",
    "frontend/src/style.css",
    "frontend/src/styles/public-pages.css",
    "frontend/src/utils/branding.ts",
    "frontend/src/utils/accountStatus.ts",
    "frontend/src/utils/promptCover.ts",
    "frontend/src/utils/promptLibrary.ts",
    "frontend/src/views/HomeView.vue",
    "frontend/src/views/KeyUsageView.vue",
    "frontend/src/views/admin/AccountsView.vue",
    "frontend/src/views/admin/GroupsView.vue",
    "frontend/src/views/admin/ProxiesView.vue",
    "frontend/src/views/public/DocsView.vue",
    "frontend/src/views/public/LegalDocumentView.vue",
    "frontend/src/views/user/ImageStudioView.vue",
    "frontend/src/views/user/UsageView.vue",
    "public/logo.png"
  ],
  "routes_or_surfaces": [
    "/home", "/en", "/models", "/en/models", "/docs", "/en/docs", "/login", "/dashboard", "/wallet", "/admin/dashboard", "/admin/accounts", "/admin/groups", "/admin/proxies", "/ai-creation-space", "Image Studio size picker", "prompt library", "favicon"
  ],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "hover", "active", "focus-visible", "loading", "disabled", "empty", "error", "success"],
  "viewports": ["360x800", "768x900", "1280x800", "1920x1080"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/v0182-bilingual-branding/prototype-1440.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/v0182-bilingual-branding/baseline-1440.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/v0182-bilingual-branding/updated-1440.png"],
  "commands": [
    "firefox --headless --screenshot --window-size=1440,1280 file:///home/codex/worktrees/sub2api-v182-bilingual-sensenova-20260826/docs/visual-reviews/assets/v0182-bilingual-branding/static-review-board.html",
    "pnpm exec vitest run src/utils/__tests__/branding.spec.ts src/components/modelPlaza/__tests__/PlazaNavBar.spec.ts src/components/layout/__tests__/PublicContentLayout.localized.spec.ts src/components/layout/__tests__/siteLogoSanitization.spec.ts",
    "pnpm exec vitest run src/components/prompt/__tests__/PromptCard.spec.ts src/components/prompt/__tests__/PromptFilters.spec.ts src/components/prompt/__tests__/PromptLibraryPanel.spec.ts src/components/layout/__tests__/PublicContentLayout.localized.spec.ts src/components/modelPlaza/__tests__/PlazaNavBar.spec.ts src/components/common/__tests__/NavigationProgress.spec.ts src/components/common/__tests__/HelpTooltip.spec.ts src/components/common/__tests__/Select.spec.ts src/components/layout/__tests__/AppSidebar.spec.ts src/components/layout/__tests__/siteLogoSanitization.spec.ts",
    "pnpm typecheck",
    "pnpm design:check"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "Existing semantic links, buttons, select controls and labelled inputs remain in use; focused component regressions cover the newly added brand and size-control branches."},
    "reduced_motion": {"status": "not-applicable", "reason": "This delivery adds no animation or continuous motion."}
  },
  "residual_risks": ["This is a static-review-board, not a live application browser screenshot. Real browser screenshots and final local guest, user and administrator acceptance remain required before production deployment."]
}
-->

## Scope

- Routes and surfaces: public home and its feature-hover card, public models and documentation, login shell, user/admin workspace shell, account-status displays, group/proxy account tables, prompt library, Image Studio recipe/model-size controls, invitation-growth operations, and the runtime favicon.
- Roles: guest, authenticated user, and administrator. User-configured group, model, campaign, and site-logo values remain raw data; only system-owned copy is localized.
- Languages and themes: default Chinese without a route/query language marker; explicit English through `/en/*` or `?lang=en`; light and dark themes.

## Baseline

- Current behavior: the previous fallback used a retired image on some public paths, and system-owned labels could remain in the opposite language when a route fragment had not been loaded.
- Baseline screenshot or recording: `docs/visual-reviews/assets/v0182-bilingual-branding/baseline-1440.png` is the frozen pre-change static board. It shows the retired fallback and mixed-language failure state deliberately.
- Inconsistencies observed: an empty configured Logo could render the retired asset; `site_logo` fallback, favicon handling, page title branding, prompts, navigation accessibility labels, and account status labels did not all share the same localized/fallback contract.

## Prototype

- Prototype design image: `docs/visual-reviews/assets/v0182-bilingual-branding/prototype-1440.png` sets the approved two-column Chinese/English workspace treatment, DENG fallback asset, shared content gutters, and retained raw model identifiers.
- Approval status: implementation stays within the requested bilingual-completeness, DENG-brand, and SenseNova image-size scope. No new page frame, card family, color system, or interactive visual language is introduced.
- Scope boundary: this review covers rendering, capability presentation, and accessibility labels. It does not translate administrator-configured names, model IDs, API paths, code examples, or retained historical onboarding HTML, and it does not add a database migration.

## Reuse Decision

- Shared layouts and components reused: `AppSidebar`, `AppHeader`, `AuthLayout`, `PublicContentLayout`, `PlazaNavBar`, existing prompt cards/filters/panel, existing Image Studio workspace and `ImageStudioSizePicker`, existing `Select`, buttons, and semantic theme tokens.
- New shared pattern, if any: `.brand-logo-asset--deng` is only attached when `site_logo` is empty or unsafe. It crops transparent padding from the approved DENG PNG without scaling a configured customer Logo. `updateFavicon` uses the same `/logo.png` fallback and `image/png` MIME type.
- Design-system exception, if any: none. Existing Home art and transient skeleton exceptions are unchanged.

## State Coverage

- Default: unprefixed routes resolve Chinese; `/en/*` and `?lang=en` resolve English; system labels do not read from a persisted locale fallback. Empty Logo paths render DENG `/logo.png`; configured Logos remain `object-contain` at native scale.
- Hover and active: existing public/workspace navigation, prompt actions, and Image Studio controls retain their established styles and layout dimensions.
- Focus-visible and keyboard: logo links, prompt actions, the locale controls, native size select, and custom width/height inputs keep labelled keyboard access. The model plaza regression verifies Chinese/English `alt` text and the configured-Logo branch.
- Loading and disabled: public setting skeletons, prompt loading grid, and Image Studio generating controls preserve their existing dimensions; disabled size inputs/select remain non-interactive.
- Empty and error: an empty/unsafe `site_logo` and favicon resolve to DENG; prompt empty/error/retry messages, Image Studio recipe labels, feature-hover labels, and unknown dynamic status labels come from the active locale rather than rendering an untranslated key.
- Success: successful prompt copy/use, model selection, account-status display, and valid SenseNova dimensions use localized status text. U1.5 accepts a valid `512-4096`, 32-step, at-most-3:1 pair; U1 Fast presents its dedicated fixed sizes and does not expose an unsupported edit control.

## Viewport Coverage

- Mobile: 360x800 checks the existing compact navigation and Logo shell with Chinese labels; long model IDs remain code-like data rather than translated labels.
- Tablet: 768x900 keeps the two dimension inputs as a stable pair and keeps the U1 Fast fixed-size selector reachable.
- Desktop: 1280x800 retains the established public content frame and workspace gutters; logo/name pairs do not create a separate width contract.
- Wide or short screen: 1920x1080 retains the public-content frame and stable Image Studio controls without page-level max-width changes.
- 200% zoom and reduced motion: no new continuous motion was added. 200% zoom, actual theme contrast, and interactive focus rings require the final real-browser acceptance noted below.

## Evidence

- Updated screenshot or recording: `docs/visual-reviews/assets/v0182-bilingual-branding/updated-1440.png` is a 1440x1280 Firefox-rendered static review board. It records separate Chinese and English system copy, the DENG-only fallback crop, a configured-Logo unscaled comparison, U1.5 custom dimensions, and U1 Fast fixed-size controls.
- Automated visual or overlap checks: targeted component regressions cover prompt text, public branding, tooltip/select/loading labels, sidebar navigation, favicon fallback, DENG-vs-configured Logo classes, and Image Studio size branches. `pnpm design:check` validates visual-file-to-manifest coverage and decodable evidence assets.
- Commands run: see the manifest; frontend type checking, focused Vitest suites, lint/design/full suites, and backend gates remain part of the parent delivery gate.

## Residual Risk

- Known limitations: the artifacts are a static-review-board, not production or live-application browser screenshots. A real browser screenshot matrix at 360/768/1280/1920, 200% zoom, both themes, and guest/user/admin sessions must be completed after the candidate is deployed to an authorized review environment; the user must perform final local acceptance before production deployment.
- Follow-up owner: release owner validates the deployed SHA, public settings, `image_studio_enabled`, full bilingual navigation, and SenseNova user flows before the production window.
