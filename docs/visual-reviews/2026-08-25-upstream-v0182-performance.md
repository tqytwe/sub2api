# Visual Review: upstream-v0182-performance

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/home/HeroSphere.vue",
    "frontend/src/components/account/CNProviderBalanceCell.vue",
    "frontend/src/components/account/CNProviderQuotaCell.vue",
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/components/account/OpenAIQuotaResetCell.vue",
    "frontend/src/components/admin/channel/TimePricingSection.vue",
    "frontend/src/components/admin/channel/types.ts",
    "frontend/src/components/admin/user/UserEditModal.vue",
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/components/modelPlaza/PlazaGroupSection.vue",
    "frontend/src/components/modelPlaza/PlazaModelPricingTable.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/admin/accounts.ts",
    "frontend/src/i18n/locales/en/admin/channels.ts",
    "frontend/src/i18n/locales/en/admin/index.ts",
    "frontend/src/i18n/locales/en/admin/ops.ts",
    "frontend/src/i18n/locales/en/admin/overview.ts",
    "frontend/src/i18n/locales/en/admin/plugins.ts",
    "frontend/src/i18n/locales/en/admin/settings.ts",
    "frontend/src/i18n/locales/en/common.ts",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/admin/accounts.ts",
    "frontend/src/i18n/locales/zh/admin/channels.ts",
    "frontend/src/i18n/locales/zh/admin/index.ts",
    "frontend/src/i18n/locales/zh/admin/ops.ts",
    "frontend/src/i18n/locales/zh/admin/overview.ts",
    "frontend/src/i18n/locales/zh/admin/plugins.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts",
    "frontend/src/i18n/locales/zh/common.ts",
    "frontend/src/i18n/locales/zh/dashboard.ts",
    "frontend/src/router/index.ts",
    "frontend/src/router/meta.d.ts",
    "frontend/src/views/HomeView.vue",
    "frontend/src/views/admin/AccountsView.vue",
    "frontend/src/views/admin/GroupsView.vue",
    "frontend/src/views/admin/PluginsView.vue",
    "frontend/src/views/admin/ProxiesView.vue",
    "frontend/src/views/admin/SettingsView.vue",
    "frontend/src/views/admin/groupsMessagesDispatch.ts",
    "frontend/src/views/admin/ops/OpsDashboard.vue",
    "frontend/src/views/admin/ops/components/OpsErrorDetailModal.vue",
    "frontend/src/views/admin/ops/components/OpsErrorDetailsModal.vue",
    "frontend/src/views/admin/ops/components/OpsRequestDetailsModal.vue",
    "frontend/src/views/user/PaymentResultView.vue",
    "frontend/src/views/user/DashboardView.vue",
    "frontend/src/i18n/locales/en/wallet.ts",
    "frontend/src/i18n/locales/zh/wallet.ts"
  ],
  "routes_or_surfaces": ["public home route /", "/model-plaza", "/admin/dashboard", "/admin/accounts", "/admin/channels", "/admin/plugins", "/admin/settings", "/payment/result"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "hover", "focus-visible", "loading", "disabled", "empty", "error", "success", "normal-motion", "reduced-motion", "save-data", "hidden", "offscreen"],
  "viewports": ["360x800", "768x900", "1280x800", "1600x900", "1920x1080"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/upstream-v0182-performance/prototype.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/upstream-v0182-performance/baseline.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/upstream-v0182-performance/prototype.png"],
  "commands": ["firefox --headless --window-size 1280,720 --screenshot <png> <svg>", "pnpm design:check", "pnpm lint:check", "pnpm typecheck"],
  "checks": {
    "keyboard": {"status": "passed", "notes": "Existing buttons, links, toggles and dialogs retain visible focus; route intent prefetch also responds to focus."},
    "reduced_motion": {"status": "passed", "notes": "The target state starts no persistent sphere loop and disables user-triggered spinner motion under reduced motion."}
  },
  "residual_risks": ["These are deterministic static review boards, not browser captures. Final production acceptance remains with the user's local browser after separately authorized deployment."]
}
-->

## Scope

The review covers the v0.1.182 upstream UI reconciliation and the performance implementation boundary for the public home, route prefetch, account dialogs, route-scoped locale loading and Dashboard requests. The homepage composition, copy, navigation, themes and responsive visual hierarchy remain unchanged.

## Baseline

The baseline has intro/sphere work gating content, continuous sphere RAF, blocking geography and Chinese web-font requests, automatic heavy-route prefetch, complete locale loading and serial Dashboard requests.

Baseline artifact: `assets/upstream-v0182-performance/baseline.png`.

## Prototype

The target board preserves appearance while making content immediate, animation finite, static modes explicit, geographic detail progressive, below-fold demos lazy with stable dimensions, route prefetch intent-driven and route data independently loadable.

Prototype artifact: `assets/upstream-v0182-performance/prototype.png`.

## Reuse Decision

The work reuses the current Home composition and CSS, shared `Icon`, button, toggle, dialog, table and layout patterns. The plugin page remains in the shared application layout. No new visual component language is introduced.

## State Coverage

Default, hover, focus-visible, loading, disabled, empty, error and success states remain owned by existing shared components. Performance-specific states are normal finite motion, reduced-motion/static, Save-Data/static, hidden, offscreen and unmounted. GeoJSON failure must leave primary content usable.

## Viewport Coverage

The boundary covers 360px, 768px, 1280px and wide desktop, plus Chinese/English, light/dark and 200% zoom. Stable placeholders reserve the current demo footprints and must not introduce layout shift.

## Evidence

The SVG sources are deterministic review-board inputs. Firefox renders them to real decodable PNG files. Design governance, lint, typecheck, targeted tests and production build provide implementation evidence; the PNG files do not claim browser acceptance.

## Residual Risk

No server screenshot or static board replaces production acceptance. After a separately authorized deployment, the user must verify guest, ordinary-user and administrator surfaces in a local browser, including light/dark and reduced-motion behavior.
