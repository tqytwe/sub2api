# V2 recent-sample color and privacy regression

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/features/channel-monitor-v2/monitorCards.ts"],
  "routes_or_surfaces": ["/monitor V2 cards; administrator shared cards"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en/light", "en/dark"],
  "states": ["healthy", "warning", "critical", "unknown", "low request volume with scored cache evidence", "no traffic", "redacted volume", "samples/windows"],
  "viewports": ["360x1000", "768x1000", "1280x1000", "1920x1000"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-10-monitor-user-visibility/prototype-server-health.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-10-monitor-user-visibility/baseline-card.png", "docs/visual-reviews/assets/2026-11-10-monitor-user-visibility/baseline-1280-light.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-11-10-monitor-user-visibility/updated-360-light.png", "docs/visual-reviews/assets/2026-11-10-monitor-user-visibility/updated-360-dark.png", "docs/visual-reviews/assets/2026-11-10-monitor-user-visibility/updated-1280-light.png", "docs/visual-reviews/assets/2026-11-10-monitor-user-visibility/updated-1280-dark.png"],
  "commands": ["pnpm --dir frontend exec vitest run src/features/channel-monitor-v2/__tests__/monitorCards.spec.ts", "node /data/tmp/monitor-user-visibility-render.mjs # browser --baseline", "node /data/tmp/monitor-user-visibility-render.mjs # browser", "make test", "make build", "./scripts/check-fork-integrity.sh"],
  "checks": {
    "keyboard": {"status": "not-applicable", "reason": "Read-only state mapping; no control, navigation or dialog change."},
    "reduced_motion": {"status": "passed", "reason": "32 compiled-component browser scenarios use reduced motion; no animation added."}
  },
  "residual_risks": ["Actual compiled Vue/Tailwind with synthetic data and closed dialog stub; server component captures do not replace user-local production browser acceptance. Specific missing group name is pending; existing authorization is preserved."]
}
-->

## Scope

Restore server-scored recent-sample colors in the existing card. No layout, palette, navigation or probe change. Backend recent samples receive the same volume redaction as matrix buckets.

## Baseline

Reviewed the user screenshot (cropped to exclude account and balance), MonitorStatusCards, adjacent RelayPulseMatrix, monitorFormat, shared status colors and service health calculation. Fixed baseline 2ce32e25b88cf491d1b2df946811af46cce1636b grays all four states when request_count=4 and minimum_sample=50 despite cache evidence. Chromium reproduced this before validating the repair.

## Prototype

prototype-server-health.png was generated before implementation from a cropped production card, explicitly labeled synthetic colors. Authorized boundary: correct existing indicators and reuse the current layout.

## Reuse Decision

Reuse backend health.overall and existing healthy/warning/critical/unknown classes. Server health considers request, latency and cache evidence independently; request count alone cannot override it. Genuine server unknown remains gray.

## State Coverage

32 Chromium cases cover four widths, two themes, two languages and samples/windows. Colors are distinct, bars are 20px high, with no page overflow or runtime errors. Tests exercise sufficient cache evidence with low request count and redacted zero. Empty traffic and server unknown remain. Hover title unchanged; no changes to active/focus/loading/disabled/error controls.

## Viewport Coverage

360, 768, 1280 and 1920px, reduced motion. Actual compiled component plus repository CSS; synthetic metrics and closed Pelican dialog stub. These component captures are not production route screenshots.

## Evidence

Baseline/updated geometry JSON summarize browser checks. Frontend RED: healthy/warning/critical became unknown. Backend RED: recent-sample request count 4 leaked to user. GREEN: server health retained, samples redacted, admin metrics untouched. Group regression proves authorized new configured groups are seeded without traffic and disappear when authorization is removed.

## Residual Risk

User-local ordinary-user/admin/light/dark acceptance after exact-SHA deployment is pending. Specific new group name remains pending; selected production subscription groups require valid subscriptions. No permission changes in this patch.

## Review and release gate

Specification review passed: preserve backend health (including unknown), recent-sample volume privacy, unchanged administrator metrics, authorized empty-group seeding and denied-group exclusion. The reported missing group remains under investigation pending its name; this patch does not claim that report resolved.

Code-quality review passed after specification review: reuse existing metric redaction and health classes; no schema, threshold, configuration, authorization or billing changes. No remaining references to the removed frontend minimum helper.

Server validation passed: focused monitor tests; make test (backend suite and lint: 0 issues; frontend lint/typecheck, 547 files and 4257 tests passed, 3 files/8 cases skipped); make build; make test-backend-unit; check-fork-integrity.sh. Initial parallel lint was killed and initial integrity invocation lacked pnpm on PATH; both were rerun successfully with the configured toolchain, serial resource-heavy checks and bounded Go memory. Browser component evidence is synthetic, not final production acceptance. PR/merge/deployment and user-local acceptance are recorded in the associated PR.
