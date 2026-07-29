# Visual Review: daily-card-admin-actions

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/SubscriptionsView.vue",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts"
  ],
  "routes_or_surfaces": [
    "/admin/subscriptions"
  ],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light"],
  "states": [
    "daily card active row",
    "daily card exhausted row",
    "release holds confirmation",
    "compensation restore confirmation",
    "generic reset quota hidden for daily cards"
  ],
  "viewports": ["360x800", "768x1024", "1280x900", "1644x650"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/daily-card-lifecycle/prototype-admin-subscriptions-effective-status.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/daily-card-lifecycle/baseline-admin-subscriptions.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/daily-card-lifecycle/prototype-admin-subscriptions-effective-status.png"
  ],
  "commands": [
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend exec vitest run src/utils/__tests__/subscriptionQuota.spec.ts"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "The new actions reuse existing table buttons and ConfirmDialog focus handling."},
    "reduced_motion": {"status": "passed", "notes": "No new motion, animation, or viewport-scaled typography was added."}
  },
  "residual_risks": [
    "The evidence uses the existing static admin subscriptions review board rather than an authenticated production browser capture.",
    "Production acceptance must verify the new protected admin action routes return authentication errors instead of 404 before an admin token is used."
  ]
}
-->

## Scope

- Routes: `/admin/subscriptions`.
- Roles: administrators operating on daily-card subscription rows.
- Languages and themes: Chinese and English labels in the existing light and dark admin table.

## Baseline

- Current behavior: daily-card rows expose no dedicated stale-hold cleanup action.
- Baseline screenshot or recording: `docs/visual-reviews/assets/daily-card-lifecycle/baseline-admin-subscriptions.png`.
- Inconsistencies observed: the old quota reset action is intentionally not applicable to daily cards because it clears legacy subscription counters instead of entitlement quota.

## Prototype

- Prototype design image: `docs/visual-reviews/assets/daily-card-lifecycle/prototype-admin-subscriptions-effective-status.png`.
- Approval status: follows the prior daily-card lifecycle table review and keeps the dense operations layout.
- Scope boundary: add two explicit row actions only; no page shell, table track, filter, modal component, or navigation redesign.

## Reuse Decision

- Shared layouts and components reused: existing admin table row actions, `Icon`, button utility classes, notification toasts, and `ConfirmDialog`.
- New shared pattern, if any: none.
- Design-system exception, if any: none.

## State Coverage

- Default: daily-card rows show `释放预留` / `Release Holds`; active and exhausted daily cards also show `补偿恢复` / `Compensate`.
- Hover and active: the actions use the same compact icon-and-label button behavior as adjacent row actions.
- Focus-visible and keyboard: confirmation dialogs reuse the existing keyboard path and cancel/confirm event handlers.
- Loading, disabled, empty, error and success: each action disables itself while submitting, closes on success, reloads subscriptions, and shows a localized success or failure toast.

## Viewport Coverage

- Mobile: horizontal table access and compact button labels remain within the existing action column.
- Tablet: no breakpoint, page gutter, or row-height contract changes were introduced.
- Desktop: the two actions sit beside existing row operations without altering table tracks.
- Wide or short screen: the prior wide admin-subscriptions review board remains representative because this change is row-action-only.
- 200% zoom and reduced motion: no viewport-scaled text or new animation was introduced.

## Evidence

- Updated screenshot or recording: static review board `docs/visual-reviews/assets/daily-card-lifecycle/prototype-admin-subscriptions-effective-status.png`.
- Automated visual or overlap checks: design governance validates artifact signatures and manifest coverage for the visible changed files.
- Commands run: `pnpm --dir frontend design:check`; `pnpm --dir frontend typecheck`; `pnpm --dir frontend exec vitest run src/utils/__tests__/subscriptionQuota.spec.ts`.

## Residual Risk

- Known limitations: the static artifact does not include an authenticated browser capture of the two new confirmation dialogs.
- Follow-up owner: release verification should confirm the production admin routes are deployed and protected before any customer-service use.
