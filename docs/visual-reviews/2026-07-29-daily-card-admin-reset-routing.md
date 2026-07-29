# Visual Review: daily-card admin reset routing

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/types/index.ts",
    "frontend/src/views/admin/SubscriptionsView.vue"
  ],
  "routes_or_surfaces": [
    "/admin/subscriptions"
  ],
  "languages_and_themes": [
    "zh-CN light",
    "zh-CN dark",
    "en-US light",
    "en-US dark"
  ],
  "states": [
    "active recurring subscription reset button",
    "active daily-card subscription release-holds button",
    "active daily-card subscription reset quota button",
    "exhausted daily-card subscription reset quota button",
    "daily-card restore confirmation modal"
  ],
  "viewports": [
    "390x844",
    "1280x860",
    "1600x900"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/daily-card-admin-reset-routing/prototype-daily-card-admin-reset-routing.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/daily-card-admin-reset-routing/baseline-daily-card-admin-reset-routing.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/daily-card-admin-reset-routing/updated-daily-card-admin-reset-routing.png"
  ],
  "commands": [
    "pnpm --dir frontend run typecheck",
    "pnpm --dir frontend run lint:check",
    "pnpm --dir frontend run design:check"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "notes": "The change reuses the existing button and confirmation modal controls."
    },
    "reduced_motion": {
      "status": "passed",
      "notes": "No new animation or transition pattern was introduced."
    }
  },
  "residual_risks": [
    "Authenticated browser acceptance should spot-check a daily card row with daily_card present after production data backfill."
  ]
}
-->

## Scope

This review covers the admin subscriptions action column only. The reset quota action now remains available for daily-card rows, but routes to the daily-card entitlement restore flow instead of the recurring subscription window reset API.

## Baseline

Daily-card rows hid the generic reset quota button. Rows that looked like daily cards but lacked a `daily_card` entitlement object also lacked the daily-card action buttons, so administrators could not correct quota from the table.

## Updated Behavior

Recurring subscriptions still use the existing reset quota confirmation and `reset-quota` API. Daily-card rows use the existing restore daily-card confirmation and entitlement API from the reset quota button, while the separate release-holds action remains available for reserved-hold cleanup.

## Prototype

The prototype is the existing action column with one routing change: a daily-card row that can be restored shows the familiar reset quota action, and selecting it opens the current daily-card restore confirmation. No new button family, toolbar, or modal layout was introduced.

## Reuse Decision

The implementation reuses the existing `refresh` icon button, daily-card restore confirmation modal, release-holds button, and admin table action spacing. It deliberately avoids adding another daily-card-only reset visual style, so administrators use the same reset command regardless of card type.

## State Coverage

Covered states are active recurring subscriptions, active daily cards, exhausted daily cards, and daily-card rows with release-holds available. The action remains disabled while the matching reset or restore request is in flight.

## Viewport Coverage

The change stays inside the existing action-cell flex row. Mobile and desktop continue to use the same compact icon-and-label action layout already used by the subscription table.

## Evidence

No layout composition, typography, spacing, or new visual component was introduced. The change is a conditional action routing update that reuses the existing action button, icon, modal, and success/error handling.

Baseline artifact: `docs/visual-reviews/assets/daily-card-admin-reset-routing/baseline-daily-card-admin-reset-routing.png`.
Prototype artifact: `docs/visual-reviews/assets/daily-card-admin-reset-routing/prototype-daily-card-admin-reset-routing.png`.
Updated artifact: `docs/visual-reviews/assets/daily-card-admin-reset-routing/updated-daily-card-admin-reset-routing.png`.

## Residual Risk

Authenticated browser acceptance should spot-check a production row after the historical entitlement backfill has run, because rows that previously lacked `daily_card` will only show the daily-card actions after API data includes the repaired entitlement object.
