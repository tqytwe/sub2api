# Dashboard statistic readability correction

Baseline: `origin/play/main@5312264073d3775ca4b2a5d4c88ac223ae62621f`.
Branch: `fix/stat-card-readability-20261109`.
Scope: isolate shared-card typography from homepage selectors only; shared dashboard data and financial rules are unchanged.

The user supplied a production screenshot with clipped overview values. Homepage `.stat-value` and `.stat-label` rules were global. After visiting the homepage they remained loaded and changed the dashboard number to 58px serif text in a 32px clipped line box. Give shared-card numbers and labels dedicated classes mapped to the existing shared typography. Keep the legacy style selectors and homepage CSS unchanged to preserve all other consumers.

TDD RED: the new mounted-component style-isolation tests fail for all four screenshot values, finding two matching external number selectors. GREEN: five isolation/home-style cases and nineteen existing homepage/overview cases pass (24 total). Real Chromium component fixtures reproduce 58/32 before and verify 24/32 after at 360, 768, 1280 and 1920px, plus light/dark variants. The prototype precedes the stylesheet change. See [visual review](../../../visual-reviews/2026-11-09-stat-card-readability.md).

Specification review: only shared-card number/label classes and the existing shared CSS mappings change; homepage CSS and desktop/mobile artwork remain unchanged; data, icons, layout, API, migrations and reward policies are unchanged. Passed.
Code quality review: neither desktop nor comma-separated mobile home selectors match the shared card classes; mounted StatCard tests check both number and label matching under all media rules. Existing component and homepage behaviors remain tested. Passed for this scope.

## Reproduce component evidence

The archived `component-browser-check.cjs` needs the project's installed frontend dependencies and an available Playwright/Chromium installation. Set `STAT_CARD_PLAYWRIGHT_MODULE` and `STAT_CARD_CHROMIUM_PATH` to installed locations when they are not available by default. Run from the repository root with argument `updated`. It uses the real component template and compiled repository styles; it does not start a web server, log in, or substitute for final local production browser acceptance. `prototype` applies typography isolation in memory; the baseline capture was generated before editing the stylesheet.

Complete test/build/Fork gates: passed. `make test` includes backend default tests, golangci-lint (0 issues), frontend lint/typecheck and 547 passing files / 4,249 tests (3 files / 8 tests skipped as in baseline). `make test-backend-unit`, `make build`, `./scripts/check-fork-integrity.sh`, design governance, document links and handoff checks passed. No backend or financial source changes. The homepage stylesheet is byte-for-byte unchanged. Initial governance checks rejected a minified home-stylesheet change and an incomplete evidence manifest; the implementation was narrowed to dedicated shared-card classes and the completed manifest passed without adding exceptions or relaxing the checker.
Commit/PR/CI: pending.
Exact production deployment and health: pending.
Local production browser acceptance: pending.
