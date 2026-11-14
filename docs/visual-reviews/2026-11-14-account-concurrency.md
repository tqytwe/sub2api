# Visual Review: account current-concurrency sorting

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/views/admin/AccountsView.vue", "frontend/src/i18n/locales/zh/admin/accounts.ts", "frontend/src/i18n/locales/en/admin/accounts.ts"],
  "routes_or_surfaces": ["/admin/accounts existing capacity column", "Actual AccountsView, DataTable and AccountCapacityCell; synthetic APIs/layout; not authenticated route acceptance"],
  "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
  "states": ["default", "sortable header", "ascending", "descending", "keyboard focus", "persisted preference", "automatic refresh", "read failure retains previous rows"],
  "viewports": ["360x900", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-14-account-concurrency/prototype-360-light.png", "docs/visual-reviews/assets/2026-11-14-account-concurrency/prototype-1280-dark.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-14-account-concurrency/baseline-360-light.png", "docs/visual-reviews/assets/2026-11-14-account-concurrency/baseline-1280-dark.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-11-14-account-concurrency/updated-360-light.png", "docs/visual-reviews/assets/2026-11-14-account-concurrency/updated-1280-dark.png", "docs/visual-reviews/assets/2026-11-14-account-concurrency/updated-keyboard-descending.png"],
  "commands": ["NODE_OPTIONS=--max-old-space-size=1024 node /data/tmp/v2212-account-concurrency-visual.mjs baseline # Playwright", "NODE_OPTIONS=--max-old-space-size=1024 node /data/tmp/v2212-account-concurrency-visual.mjs prototype # Playwright", "NODE_OPTIONS=--max-old-space-size=1024 node /data/tmp/v2212-account-concurrency-visual.mjs updated # Playwright"],
  "checks": {"keyboard": {"status": "passed"}, "reduced_motion": {"status": "passed"}},
  "residual_risks": ["Component captures use synthetic APIs and reduced surrounding layout, and cannot establish real-route or production correctness.", "Mobile keeps the existing card layout, which has no table header; this change does not add a new mobile sort interface.", "Integrated frontend build/typecheck, backend snapshot tests and final user-local production acceptance are separate release gates."]
}
-->

## Scope

Port v2.21.2 current-concurrency sorting into the existing administrator account
list, preserving Fork behavior and the existing table/mobile presentation.

## Baseline

Only the existing administrator account table's capacity column is changed. The
current AccountsView, neighbouring UsersView sortable columns, DataTable,
AccountCapacityCell and HelpTooltip were read first. Before implementation,
Playwright rendered the actual components with synthetic account APIs: capacity
was a fixed header and had no sorting affordance. Existing table density, account
cells, layout, pagination and mobile cards are preserved.

## Prototype

Before editing production UI, an in-memory render of the existing AccountsView
enabled the capacity column's shared sort affordance and upstream help tooltip.
Its real PNGs show the intended narrow change. The upstream v2.21.2 mapping and
current-count behavior define the implementation scope; no new page, controls or
local sort policy is introduced.

## Reuse Decision

DataTable owns the sort button, arrows, keyboard handling, `aria-sort` and stored
column key. AccountsView maps its `capacity` key to the backend
`current_concurrency` field on both clicks and preference restoration. The existing
ETag refresh keeps server order and count values together. Billing-probe refreshes
also reload the list when current concurrency is the active sort because compact
billing snapshots have no counts. The hint retains upstream Chinese and English
wording: sort the numerator across all filtered accounts, with server-side ID ties.

## State Coverage

Real component tests cover ascending/descending requests, persistence, invalid
stored aliases, automatic reordered counts, billing refresh with unchanged IDs,
retaining rows on refresh error, and late polls after a newer sort, filter, page
or same-condition manual reload. A request generation and condition check reject
the old response before it can change rows, pagination or ETag. This follow-up
changes no layout and reuses the captured visuals. Loading/empty/error layouts are existing
DataTable/useTableLoader behavior, covered by adjacent tests. No new disabled,
success notification or animation state is introduced.

## Viewport Coverage

Baseline, prototype and updated components each ran at 360, 768, 1280 and 1920px,
in Chinese/English and light/dark, with reduced motion. No page overflow or page
errors occurred in their 16-case matrices. Enter and Space activate the real sort
button, update `aria-sort`, and issue `current_concurrency` requests. Seven PNGs
are retained; harness and verbose results stay under `/data/tmp`, outside Git.

## Evidence

The four relevant component test files pass 43 tests. The first valid RED run had
four business failures: absent sort button, wrong restored API sort, wrong automatic
refresh sort, and stale counts after a billing snapshot. Targeted lint and diff
checks pass; the manifest is checked by the repository design gate.
The independent-review follow-up added four deferred-response cases, all of
which failed before the poll guard and passed after it. Their RED/GREEN logs are
under `/data/tmp/sub2api-v2212-account-concurrency-poll-race-*.log`.

## Residual Risk

These are actual component captures with synthetic APIs and simplified surrounding
layout, not authenticated route or production evidence. The mobile card layout
remains unchanged. Real Redis counts, page-boundary moves, final integrated build
and user-local production verification remain release acceptance work.
