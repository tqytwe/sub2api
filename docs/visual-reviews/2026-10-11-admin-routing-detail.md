# Visual Review: Admin routing evidence

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/requestLedger.ts",
    "frontend/src/api/requestRouting.ts",
    "frontend/src/api/__tests__/requestRouting.spec.ts",
    "frontend/src/views/shared/RequestLedgerView.vue",
    "frontend/src/views/shared/components/RequestLedgerRoutingSection.vue",
    "frontend/src/views/shared/__tests__/RequestLedgerView.routing.http.spec.ts",
    "frontend/src/i18n/locales/request-ledger.en.ts",
    "frontend/src/i18n/locales/request-ledger.zh.ts"
  ],
  "routes_or_surfaces": [
    "/admin/requests",
    "/requests"
  ],
  "languages_and_themes": [
    "zh/light",
    "zh/dark",
    "en/light"
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
    "success"
  ],
  "viewports": [
    "360x900",
    "768x900",
    "1280x900",
    "1920x900"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/admin-routing-detail/prototype-admin-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/admin-routing-detail/baseline-admin-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/admin-routing-detail/active-admin-1280.png",
    "docs/visual-reviews/assets/admin-routing-detail/after-admin-1280.png",
    "docs/visual-reviews/assets/admin-routing-detail/after-admin-1920.png",
    "docs/visual-reviews/assets/admin-routing-detail/after-admin-360.png",
    "docs/visual-reviews/assets/admin-routing-detail/after-admin-768.png",
    "docs/visual-reviews/assets/admin-routing-detail/after-user-1280.png",
    "docs/visual-reviews/assets/admin-routing-detail/after-user-1920.png",
    "docs/visual-reviews/assets/admin-routing-detail/after-user-360.png",
    "docs/visual-reviews/assets/admin-routing-detail/after-user-768.png",
    "docs/visual-reviews/assets/admin-routing-detail/dark-admin-1280.png",
    "docs/visual-reviews/assets/admin-routing-detail/error-admin-1280.png",
    "docs/visual-reviews/assets/admin-routing-detail/focus-admin-1280.png",
    "docs/visual-reviews/assets/admin-routing-detail/hover-admin-1280.png",
    "docs/visual-reviews/assets/admin-routing-detail/loading-admin-1280.png",
    "docs/visual-reviews/assets/admin-routing-detail/session-account-admin-1280.png",
    "docs/visual-reviews/assets/admin-routing-detail/state-complete-admin-1280.png",
    "docs/visual-reviews/assets/admin-routing-detail/state-incomplete-admin-1280.png",
    "docs/visual-reviews/assets/admin-routing-detail/state-not_recorded-admin-1280.png",
    "docs/visual-reviews/assets/admin-routing-detail/state-pending-admin-1280.png",
    "docs/visual-reviews/assets/admin-routing-detail/state-truncated-admin-1280.png",
    "docs/visual-reviews/assets/admin-routing-detail/state-unsupported_coverage-admin-1280.png",
    "docs/visual-reviews/assets/admin-routing-detail/zh-admin-1280.png",
    "docs/visual-reviews/assets/admin-routing-detail/zoom-admin-1280.png"
  ],
  "commands": [
    "make build",
    "pnpm exec vitest run src/views/shared/__tests__/RequestLedgerView.routing.http.spec.ts",
    "node docs/archive/completed-tasks/admin-routing-detail/capture.cjs # Playwright browser contract capture",
    "LEDGER_DETAIL_BROWSER_FIXTURE=1 LEDGER_BROWSER_DIST=/tmp/ledger-routing-ui-built go test -tags=integration ./internal/requestledger/acceptance -run ^TestRequestLedgerDetailPagingBrowserFixture$ -count=1 -v",
    "node docs/archive/completed-tasks/admin-routing-detail/core-capture.cjs /tmp/routing-ui-standard-fixture.log"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "note": "Actual compiled assets: refresh by Enter, Escape/Close, no routing region on user view."
    },
    "reduced_motion": {
      "status": "passed",
      "note": "Chromium reducedMotion=reduce; no new animations."
    }
  },
  "residual_risks": [
    "#359 was unmerged at implementation start. Contract HTTP simulation does not prove integrated backend/PG routing.",
    "Server browser evidence is not final production/user-local acceptance. No merge/deployment authorization.",
    "Existing ordinary-user English header overflows document at360px: baseline and final both489px; dialog remains360px and admin routing stays within viewport. No header source changes.",
    "CSS 200% simulation is not native browser zoom."
  ]
}
-->

## Scope

Add one read-only administrator routing section in the existing request detail dialog. Preserve all
four cursor-paged detail regions and usage drill. User view gets no routing region or API request.
No new page/layout, account editor, upstream identifiers, write action or identity protocol.
Route UI target remains app/workspace/compact/chrome/document/page/header/auto/none; no new route.

## Baseline

Inspected the actual compiled #360 1280px admin detail capture (attempt page 2 and Ops), copied as the
baseline. Read RequestLedgerView, adjacent user WalletView, RequestLedgerDetailSection, DataTable and
shared BaseDialog. Existing dialog sections use border separators, text-sm and btn-secondary controls.

## Prototype

Saved and inspected prototype-admin-1280.png **before** changing UI implementation. Rendered static HTML
with existing compiled CSS; this is an illustrative prototype, not a working app screenshot. Scope is
explicitly authorized by the parent task: existing frontend style and pagination preserved, admin-only
independent routing region. The prototype shows that exact boundary and introduces no new visual system.

## Reuse Decision

Reuse AppLayout/BaseDialog and original detail scope, spacing, headings, semantic gray/amber/red text,
btn-secondary, wrapping controls and plain detail lists. Routing is a bounded API result; its local
20-event pagination is labeled as returned events, never fake backend cursors or account totals.

## State Coverage

Six backend evidence states have distinct localized text. Initial loading/disabled, no recorded events,
error/retry and success are local to routing. Hover/active/focus use existing buttons. Unknown version
is shown without assuming veto interpretation. First recorded veto and no-veto evidence are qualified.
Automated/browser results are pending; no placeholder screenshots are counted as validation.

## Viewport Coverage

Planned actual-built checks at 360/768/1280/1920, light/dark, Chinese/English, keyboard/reduced motion.

## Evidence

See ../archive/completed-tasks/admin-routing-detail for exact backend source contract, HTTP RED, later
validation and dependency status. No generation calls or production credentials are used.

## Residual Risk

#359 API is not in this branch at implementation start. Simulation and actual resource captures must
remain labeled separately from real backend/PG integration. No historical 27-request or gpt22 recovery
claim. Production deployment and user-local acceptance remain owned by the main thread.

## Actual built verification

Sequential independent specification and code reviews passed. Added 20 actual View/Pinia/Axios HTTP
contract tests plus 20 parser/veto tests passed. Actual compiled routing capture covers all six states,
local paging without extra network reads, first veto, zero candidate scope, 503/retry preserving usage,
refresh keyboard/focus/hover/active, pending/disabled, close/reopen late error, normal renewal and
same-account login/account switch using actual Pinia. No page errors.

The contract HTTP server is synthetic. Separately, unchanged mainline real handler/PostgreSQL fixture
and the new compiled assets pass both-role core ledger pagination, permissions, empty intermediate pages,
retry/refresh/usage and real fixture 6 USD / 3 images. Routing API is absent on that mainline, so its local
error coexists with working billing; this is not proof of routing PostgreSQL persistence/authorization.

All four widths were exercised. Admin routing and both dialogs remain within their viewport. The first
English user360px document-overflow assertion failed; investigation found the untouched global header.
The identical synthetic setup with the reviewed #360 compiled assets gives document489px/dialog360px,
exactly matching the new build. Both measurements and original failure are retained. Chinese core-regression
screens and all admin widths have no document overflow. This existing English user header issue is not
claimed fixed and is not silently turned into a full-site responsive pass. No broader header/style edit.

Inspected admin360px, error1280px, dark1280px and Chinese1280px screenshots: readable scope/status/first-veto,
wrapping data and existing button/section style. Baseline and prototype remain distinct from these captures.
