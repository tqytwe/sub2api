# Visual Review: Codex credit snapshot synchronization

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/account/OpenAIQuotaResetCell.vue"
  ],
  "routes_or_surfaces": [
    "Admin account list: OpenAIQuotaResetCell, isolated real-component fixture"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "en-US/dark"
  ],
  "states": [
    "default",
    "snapshot-update",
    "zero",
    "unknown",
    "stale-response",
    "hover",
    "focus-visible",
    "keydown-observed",
    "loading",
    "disabled",
    "error",
    "success"
  ],
  "viewports": [
    "360x1000",
    "768x1000",
    "1280x1000",
    "1920x1000"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/codex-credits-snapshot/prototype-zh-light-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/codex-credits-snapshot/before-en-dark-1280.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/before-en-dark-1920.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/before-en-dark-360.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/before-en-dark-768.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/before-zh-light-1280.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/before-zh-light-1920.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/before-zh-light-360.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/before-zh-light-768.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/codex-credits-snapshot/after-en-dark-1280-focus.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/after-en-dark-1280-hover.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/after-en-dark-1280-keydown.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/after-en-dark-1280-zoom200.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/after-en-dark-1280.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/after-en-dark-1920.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/after-en-dark-360.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/after-en-dark-768.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/after-zh-light-1280-focus.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/after-zh-light-1280-hover.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/after-zh-light-1280-keydown.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/after-zh-light-1280-zoom200.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/after-zh-light-1280.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/after-zh-light-1920.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/after-zh-light-360.png",
    "docs/visual-reviews/assets/codex-credits-snapshot/after-zh-light-768.png"
  ],
  "commands": [
    "PLAYWRIGHT_MODULE=<installed playwright/index.mjs> node docs/archive/completed-tasks/2026-10-11-codex-credits-snapshot/capture.mjs /workspace/quota-frontend /workspace/quota-frontend-baseline",
    "PLAYWRIGHT_MODULE=<installed playwright/index.mjs> node docs/archive/completed-tasks/2026-10-11-codex-credits-snapshot/check-interactions.mjs <capture scratch>/after/dist docs/visual-reviews/assets/codex-credits-snapshot"
  ],
  "checks": {
    "keyboard": {
      "status": "passed"
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "This script-only synchronization change adds no animation. Existing query spinner remains spin 1s under reduced motion; recorded inherited risk, not a claim of reduced-motion compliance."
    }
  },
  "residual_risks": [
    "Isolated Chromium screenshots are not production or final user-local-browser acceptance. Whole account list integration, all identities and cross-product themes remain for final acceptance.",
    "Prototype was captured during explicitly authorized evidence follow-up, after initial script implementation; it is not evidence of an earlier prototype or earlier approval.",
    "Existing 10px controls and reduced-motion spinner are unchanged. Native :active was not confirmed in this harness; keydown capture is not an active-state pass."
  ]
}
-->

## Scope

- Surface: the administrator account list's existing `OpenAIQuotaResetCell`.
- Business changes remain limited to its script and targeted tests. No template, style, shared UI, locale, route, API contract or database changes.
- Roles: synthetic administrator-view component fixture only; no authenticated or real account sessions.
- Actual repository component, child components, Tailwind configuration, styles and Chinese/English translation modules are bundled by Vite production build. The surrounding labeled board is a test fixture, not a screenshot of the whole account list.

## Baseline

- Baseline worktree: `/workspace/quota-frontend-baseline`, detached at `8364b462feda15f3bf8f3bb2029314d2193b9dba`.
- `before-*` captures perform the same updates as `after-*`: the baseline incorrectly retains 1234.50 after a same-ID update to 42.50, zero or unknown; a delayed response also restores 1234.50.
- Review checked target, adjacent `AccountUsageCell` and existing `ConfirmDialog`/`BaseDialog`; production page geometry is outside this isolated fixture.
- [Mobile English/dark baseline](./assets/codex-credits-snapshot/before-en-dark-360.png), [desktop Chinese/light baseline](./assets/codex-credits-snapshot/before-zh-light-1280.png).

## Prototype

- [Baseline component rendered with desired initial states](./assets/codex-credits-snapshot/prototype-zh-light-1280.png).
- This is a real browser capture of the baseline component initialized with each target value, preserving its existing UI. It establishes the approved data-only visual boundary; it does not implement a new design.
- Chronology: the initial code work occurred before this image was available. The parent subsequently explicitly authorized and required completing visual artifacts. This image was captured in that follow-up before the final after-captures. It is **not** presented as a coding-before prototype. The earlier evidence-gate failure is retained in the archive.
- Scope approval: parent authorized component synchronization and later these acceptance artifacts. No redesign or production access was authorized or performed.

## Reuse Decision

- Preserve all existing buttons, tooltip, referral cell, ConfirmDialog, icon markup, typography, spacing and wrapping. No new visual pattern and no design-governance exceptions.
- Snapshot time continues to use the existing tooltip. Unknown dated observations keep the timestamp and display an em dash; explicit no-credit observations display zero.

## State Coverage

| State | Evidence and result |
| --- | --- |
| Default | Synthetic 1234.50 and full-precision decimal balance retain existing rendering. |
| Same-ID success | 42.50 follows persisted snapshot updates without issuing an API call. |
| Zero / unknown | Separate rows show `0` versus `—` in both languages; neither is inferred from a missing balance. |
| Old pending result | A 1234.50 local result cannot overwrite the newer 42.50 snapshot. |
| Hover | Real browser hover captures at 1280 in both language/theme combinations; no new layout behavior. |
| Focus / keyboard | Two Tab presses reach credits button and `:focus-visible` is true in browser JSON; screenshots retained. |
| Keydown / active | Actual keydown captures retained. Native `:active` was observed false in this harness; this is **not** an active-state pass. No new active styling exists in this change. Observation failure retained. |
| Loading / disabled | Controlled unresolved local read leaves query controls disabled; reset stays disabled throughout. |
| Error | Controlled local rejection retains prior balance and displays localized fixture error. |
| Reset success | Not exercised: reset invocation is prohibited by this task; mock reset throws and its call count stays zero. |
| Reduced motion | Browser emulates reduce. The existing spinner still computes `spin`, `1s`; unchanged inherited limitation, not a passing accessibility claim. |

## Viewport Coverage

- Real before/after captures: 360, 768, 1280 and 1920 CSS pixels wide, each 1000 pixels viewport height with full-page capture.
- Chinese/light and English/dark captured at each width. Responsive wrapping, zero/unknown, long balance and local error are visible; no horizontal overflow in the recorded fixture.
- Real 200% CSS zoom captures at 1280 in both combinations; supplemental DOM check records no horizontal overflow. CSS zoom is a component-level size stress check, not a claim about user browser chrome zoom.
- The component's inherited 10px controls and whole admin table width remain unchanged; fixture does not establish whole-page touch target or responsive compliance.

## Evidence

- [Desktop updated Chinese/light](./assets/codex-credits-snapshot/after-zh-light-1280.png), [mobile updated English/dark](./assets/codex-credits-snapshot/after-en-dark-360.png).
- [Main browser observations](./assets/codex-credits-snapshot/browser-checks.json), [supplemental interactions](./assets/codex-credits-snapshot/interaction-checks.json).
- [Capture scripts and technical evidence](../archive/completed-tasks/2026-10-11-codex-credits-snapshot/README.md).
- Browser uses Playwright routing to fulfill built HTML/JS/CSS from disk at `quota.fixture`; no HTTP server, dev service or production access. All other origins abort. API module is a local stub, never a credentialed request.
- After-board call accounting is exactly fixture-initiated `[5,6,7]`: pending-old, loading, error. Normal snapshot updates issue no query. Reset calls are zero. Page errors and external requests are empty.
- PNGs are direct Chromium screenshots, manually inspected for language, theme and balance semantics. No generated, composited or retouched screenshots.

## Residual Risk

- Prototype timing is a documented workflow deviation from the repository's pre-code prototype rule; completing the image now cannot retroactively meet that chronology. The parent explicitly accepted this authorized follow-up for the narrow repair after independent visual review. No additional user approval is pending; the chronology deviation remains disclosed.
- Legacy reduced-motion behavior, native active-state observation and small touch targets are not repaired by this data-only task. They are stated limitations, not ignored passes.
- Full account-list browser integration, user-local-browser verification, visitor/user/admin identities, production deployment and production account behavior have not been validated. Owner of final acceptance: parent thread/user after authorized deployment.

## Root-review reset-feedback follow-up

The actual updated component was rebuilt after the root review identified stale points reviving through reset-result parent feedback. The isolated harness clicks the existing reset button and confirmation dialog; its build-time fake reset result contains no quota and the old account snapshot. The parent replaces the same-ID row through the real emitted event. No upstream reset or production call occurs.

- [Chinese light 1280](./assets/codex-credits-snapshot/reset-feedback-zh-light-1280.png)
- [English dark 360](./assets/codex-credits-snapshot/reset-feedback-en-dark-360.png)
- [Browser checks](./assets/codex-credits-snapshot/reset-feedback-checks.json): points remain unknown (`—`), one local fake reset/one feedback per capture, zero query calls/external requests/page errors/overflow.

The existing warning, disabled reset state and unknown point typography are preserved. Existing 25 screenshots are reused only for unchanged visual design; these two new screenshots document the reset-result feedback behavior. Earlier prototype chronology and active/reduced-motion limitations remain unchanged.
