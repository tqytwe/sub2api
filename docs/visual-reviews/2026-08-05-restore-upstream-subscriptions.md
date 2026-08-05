# Visual Review: restore upstream subscription surfaces

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/usage/UsageTable.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/views/admin/SubscriptionsView.vue",
    "frontend/src/views/admin/UsageView.vue",
    "frontend/src/views/admin/orders/PlanEditDialog.vue",
    "frontend/src/views/user/SubscriptionsView.vue"
  ],
  "routes_or_surfaces": ["/admin/subscriptions", "/admin/usage", "/admin/orders", "/subscriptions"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["monthly subscription", "one-day subscription", "quota progress", "reset quota", "empty and expired"],
  "viewports": ["360x800", "768x900", "1280x800", "1600x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/daily-card-entitlements/prototype-admin-plan-quota.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/daily-card-lifecycle/baseline-admin-subscriptions.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/daily-card-admin-reset-routing/updated-daily-card-admin-reset-routing.png"
  ],
  "commands": [
    "git diff origin/play/main..HEAD -- frontend/src/views/admin/SubscriptionsView.vue frontend/src/views/user/SubscriptionsView.vue",
    "corepack pnpm typecheck",
    "corepack pnpm build"
  ],
  "checks": {
    "keyboard": { "status": "passed", "notes": "Existing buttons, selects, dialogs, and table controls remain shared native controls." },
    "reduced_motion": { "status": "passed", "notes": "The cleanup removes daily-card controls and does not add motion." }
  },
  "residual_risks": [
    "This is a static review board using preserved historical artifacts; local browser acceptance of the administrator and user subscription surfaces remains required after deployment."
  ]
}
-->

## Scope

- Routes: administrator subscriptions, administrator usage, plan editor, and user subscriptions.
- Roles: administrator and regular user.
- Change type: remove fork-only daily-card controls and restore the upstream subscription surfaces.

## Baseline

- The fork exposed daily-card quota, exhausted status, and compensation controls in subscription management.
- Historical baseline evidence is preserved in the daily-card review artifacts.

## Prototype

- The reviewed target removes daily-card-only controls and leaves the shared subscription quota and reset controls.
- Existing upstream one-day subscriptions continue to show a single non-resetting daily window.

## Reuse Decision

- Reused the existing subscription table, quota progress bar, dialog, locale, and shared subscription quota helpers.
- No new navigation, modal, or control system was introduced.

## State Coverage

- Default: recurring and one-day subscriptions use the upstream fields.
- Hover and active: existing shared action styles remain unchanged.
- Focus-visible and keyboard: existing native controls and shared dialogs remain in use.
- Loading, disabled, empty, error and success: existing subscription request states remain unchanged.

## Viewport Coverage

- Mobile: subscription rows and quota blocks retain their existing responsive layout.
- Tablet and desktop: removal of daily-card-only controls leaves the existing table actions aligned.
- Wide or short screen: no new fixed-width content was added.
- 200% zoom and reduced motion: no new motion or overflow was introduced.

## Evidence

- Historical prototype, baseline, and updated artifacts are preserved under `docs/visual-reviews/assets/`.
- Automated evidence: design governance, typecheck, frontend build, and backend tests.
- The static board deliberately records browser acceptance as a residual risk rather than claiming it was performed here.

## Residual Risk

- Production browser acceptance by guest, regular user, and administrator remains required after deployment.
