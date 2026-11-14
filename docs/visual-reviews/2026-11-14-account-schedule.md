# Visual Review: account scheduling tasks

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/components/account/SchedulableTimerControl.vue", "frontend/src/views/admin/AccountsView.vue", "frontend/src/api/admin/schedulableTimers.ts", "frontend/src/i18n/locales/zh/admin/accounts.ts", "frontend/src/i18n/locales/en/admin/accounts.ts"],
  "routes_or_surfaces": ["/admin/accounts existing scheduling cell and shared task dialog", "Actual components with synthetic APIs and surrounding layout; development evidence only"],
  "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
  "states": ["default", "hover", "focus", "loading", "disabled", "empty", "error/retry", "daily window", "one-shot confirmation", "pending", "history", "manual override", "success"],
  "viewports": ["360x900", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "browser-capture",
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-14-account-schedule/baseline-360-light.png", "docs/visual-reviews/assets/2026-11-14-account-schedule/baseline-1280-dark.png"],
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-14-account-schedule/prototype-360-light.png", "docs/visual-reviews/assets/2026-11-14-account-schedule/prototype-1280-dark.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-11-14-account-schedule/updated-360-light.png", "docs/visual-reviews/assets/2026-11-14-account-schedule/updated-1280-dark.png", "docs/visual-reviews/assets/2026-11-14-account-schedule/updated-entry-360-light.png", "docs/visual-reviews/assets/2026-11-14-account-schedule/updated-entry-1280-dark.png", "docs/visual-reviews/assets/2026-11-14-account-schedule/updated-once-confirmation.png", "docs/visual-reviews/assets/2026-11-14-account-schedule/updated-daily.png", "docs/visual-reviews/assets/2026-11-14-account-schedule/updated-once.png", "docs/visual-reviews/assets/2026-11-14-account-schedule/updated-history.png", "docs/visual-reviews/assets/2026-11-14-account-schedule/updated-error.png"],
  "commands": ["NODE_OPTIONS=--max-old-space-size=1024 node /data/tmp/v2212-account-schedule-visual.mjs baseline # Playwright", "NODE_OPTIONS=--max-old-space-size=1024 node /data/tmp/v2212-account-schedule-visual.mjs prototype # Playwright", "NODE_OPTIONS=--max-old-space-size=1024 node /data/tmp/v2212-account-schedule-visual.mjs updated # Playwright", "NODE_OPTIONS=--max-old-space-size=1024 node /data/tmp/v2212-account-schedule-visual.mjs updated-entry # Playwright"],
  "checks": {"keyboard": {"status": "passed"}, "reduced_motion": {"status": "passed"}},
  "residual_risks": ["Synthetic API/component captures are development evidence, not authenticated route or production acceptance.", "The existing mobile account cards do not expose the desktop scheduling column; no parallel mobile interface is added in this upstream port.", "Real database worker execution and integrated build checks belong to the combined release gate."]
}
-->

## Scope

Port upstream v2.21.2 one-shot and daily account scheduling tasks into the existing
administrator account table. Preserve the Fork manual switch, bulk result
handling, billing display and account list behavior. No route or notification
framework is added.

## Baseline

Read the current AccountsView, neighbouring UsersView table controls,
TempUnschedStatusModal and shared BaseDialog before editing. Captured the actual
AccountsView/DataTable with a synthetic account: the scheduling cell contained
only its immediate switch. The harness uses simplified surrounding layout and
contains no real account or credential.

## Prototype

Before production UI edits, rendered upstream SchedulableTimerControl against
the Fork BaseDialog, styles and synthetic timer API. The prototype PNGs show
the daily window form and its narrow-screen wrapping. The upstream component
defines the scope; the absent NewFeatureBadge framework is deliberately omitted.

## Reuse Decision

Use BaseDialog for title, scrolling, focus trapping, Escape and return focus;
existing input/button/status styles and native selects/time fields supply the
controls. The entry remains in the existing scheduling cell above its manual
switch. The API module preserves upstream preview, version, confirmation and
atomic daily-window contracts. Chinese/English strings stay in the account
route dictionaries. Manual or bulk switches invalidate displayed timer snapshots;
execution notifications coalesce one account list reconciliation and are
cancelled on unmount.

## State Coverage

Component tests cover 8-hour and absolute one-shot saves, concrete confirmation,
daily paired previews/saves, edit/cancel versions, next-day wording, pending and
legacy results, history advancement, translated errors, invalidated previews,
loading guards and failed-load retry. The immediate switch remains available
when the timer service fails. Permission tests keep administrator calls out of
observer rendering and preserve the existing nonadministrator manual switch.
Success notifications reuse the application toast; no new animation is added.
Browser interactions exercise the actual native controls and shared dialog.

## Viewport Coverage

Baseline, prototype, updated dialog and updated account entry each ran a 16-case
matrix: 360, 768, 1280 and 1920px, Chinese/English, light/dark and reduced motion.
No page overflow or browser errors occurred. Additional narrow captures show
one-shot confirmation, daily/one-shot pending tasks, history and error/retry.
Keyboard selection, checkbox confirmation, dialog focus containment and Escape
passed; the failed-load immediate action remains reachable by dialog scrolling.

## Evidence

The initial RED run failed six account-cell business assertions and three suites
for the missing upstream API/component. The focused GREEN run passes 34 tests
across five files, including the existing priority preference tests; the existing
account list suite also passes its 17 tests. Independent review then reproduced
eight API-error/lifecycle/Escape cases (plus the original error test updated to
the real client rejection shape). The follow-up GREEN run passes 59 tests across
six files. Errors now read the API client's normalized reason; account/manual
refresh and unmount invalidate pending dialog operations, and the local close
handler prevents Escape while busy. These fixes change no layout and reuse the
captured visual evidence. Targeted
ESLint and diff checks pass. Harness code and verbose logs remain under
`/data/tmp/v2212-account-schedule-*`, outside Git.

## Residual Risk

All captures compile the actual components but use synthetic APIs and simplified
surrounding layout. They establish development UI behavior, not authentication,
real worker/database execution or production acceptance. The existing mobile
account-card layout is unchanged. Combined frontend typecheck/build, backend
integration tests and user-local production acceptance remain release gates.
