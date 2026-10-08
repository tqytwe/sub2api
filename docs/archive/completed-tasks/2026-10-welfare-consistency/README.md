# Welfare consistency review fixtures

These archived Python/Playwright harnesses render the real production frontend bundle with synthetic API fixtures. All non-loopback requests are aborted; every API request is intercepted. They do not call production, charge an account or generate paid content.

Build with the repository pinned toolchain, then run `python docs/archive/completed-tasks/2026-10-welfare-consistency/matrix.py` and `python docs/archive/completed-tasks/2026-10-welfare-consistency/interactions.py`. Python Playwright and `/usr/bin/chromium` are required. Output screenshots are reviewed in `docs/visual-reviews/2026-10-08-welfare-consistency.md`. Fixtures illustrate branches and are not live reward configuration.

`add-p0-i18n.js` is the original PR329 one-off insertion script, archived unchanged for provenance. Do not rerun it: it contains superseded copy and positional edits. Runtime translations are maintained in source and tested through the lazy route loader.

Baseline and prototype images were captured before UI implementation. The prototype was a DOM-only design preview of the baseline, not implemented or production evidence. Final images use the normal production build.
