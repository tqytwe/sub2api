# Visual Review: Website support tickets

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/App.vue",
    "frontend/src/api/admin/settings.ts",
    "frontend/src/api/supportTickets.ts",
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/features/support-tickets/CreateTicketDialog.vue",
    "frontend/src/features/support-tickets/TicketConversation.vue",
    "frontend/src/features/support-tickets/TicketMessageBody.vue",
    "frontend/src/features/support-tickets/TicketReplyBox.vue",
    "frontend/src/features/support-tickets/TicketStatusBadge.vue",
    "frontend/src/features/support-tickets/supportTickets.ts",
    "frontend/src/i18n/index.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/admin/settings.ts",
    "frontend/src/i18n/locales/en/supportTickets.ts",
    "frontend/src/i18n/locales/en/workspaceShell.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts",
    "frontend/src/i18n/locales/zh/supportTickets.ts",
    "frontend/src/i18n/locales/zh/workspaceShell.ts",
    "frontend/src/i18n/routeScopes.ts",
    "frontend/src/router/index.ts",
    "frontend/src/router/meta.d.ts",
    "frontend/src/stores/app.ts",
    "frontend/src/stores/index.ts",
    "frontend/src/stores/supportTickets.ts",
    "frontend/src/types/index.ts",
    "frontend/src/utils/featureFlags.ts",
    "frontend/src/views/admin/SettingsView.vue",
    "frontend/src/views/admin/SupportTicketDetailView.vue",
    "frontend/src/views/admin/SupportTicketsView.vue",
    "frontend/src/views/user/SupportTicketDetailView.vue",
    "frontend/src/views/user/SupportTicketsView.vue"
  ],
  "routes_or_surfaces": [
    "/support-tickets",
    "/support-tickets/:id",
    "/admin/support-tickets",
    "/admin/support-tickets/:id",
    "/admin/settings → Features → Support Tickets",
    "authenticated sidebar"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "en-US/dark",
    "en-US/light",
    "zh-CN/dark"
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
    "success",
    "closed",
    "feature off"
  ],
  "viewports": [
    "360x800",
    "768x900",
    "1280x900",
    "1920x1080"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-20-website-support-tickets/prototype-admin-1280.png",
    "docs/visual-reviews/assets/2026-11-20-website-support-tickets/prototype-user-mobile.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-20-website-support-tickets/before-admin-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-20-website-support-tickets/after-user-create-360.png",
    "docs/visual-reviews/assets/2026-11-20-website-support-tickets/after-user-detail-768.png",
    "docs/visual-reviews/assets/2026-11-20-website-support-tickets/after-admin-list-1280.png",
    "docs/visual-reviews/assets/2026-11-20-website-support-tickets/after-admin-detail-1920.png",
    "docs/visual-reviews/assets/2026-11-20-website-support-tickets/after-settings-1280.png"
  ],
  "commands": [
    "Playwright Chromium with Vite production assets and intercepted fixture API responses",
    "pnpm exec vitest run on support ticket components, views, sidebar, router guard, store and SettingsView"
  ],
  "checks": {
    "keyboard": {
      "status": "passed"
    },
    "reduced_motion": {
      "status": "passed"
    }
  },
  "residual_risks": [
    "Production browser acceptance uses real accounts, actual ticket storage and the deployed feature setting. The captured browser sessions use fixture data."
  ]
}
-->

## Scope

- User and administrator ticket lists, conversations, settings, and navigation badges.
- All ticket pages use the authenticated app shell and chrome heading. Lists and administrator detail use the workspace frame. The single user conversation uses the content frame. Scroll and gutter follow the existing app shell.

## Baseline

- The existing administrator settings and navigation are shown in `before-admin-1280.png`. The deployed baseline has no website ticket entry. Existing native app support remains a separate surface.

## Prototype

- `prototype-admin-1280.png` and `prototype-user-mobile.png` are actual screenshots from the fixed upstream ticket change documentation. They anchor the approved upstream interaction scope.

## Reuse Decision

- Reused AppLayout, TablePageLayout, DataTable, Pagination, Select, BaseDialog, ConfirmDialog, EmptyState, Toggle, and the shared Icon registry. The ticket view uses existing card and button tokens.

## State Coverage

- Chromium exercised keyboard opening and submitting, disabled submission until required fields are complete, successful replies, administrator processing and close/reopen, feature off, and guest redirects. Component tests additionally exercise empty and server-error rendering.
- Existing hover, active, focus, dialog, pagination and loading styles are shared with the rest of the workspace.

## Viewport Coverage

- The English create dialog and detail were exercised at 360px and 768px. The Chinese administrator list, settings and conversation were exercised at 1280px and 1920px. Screenshots include both color themes. Chromium ran with reduced motion and checked page width for overflow.

## Evidence

- The listed updated images show the actual built Vue application in Chromium. Browser test requests were intercepted and served with local fixture data.

## Residual Risk

- Final production acceptance still needs browsers on the deployed version and real PostgreSQL persistence. Follow the workspace release checklist with one test ticket.
