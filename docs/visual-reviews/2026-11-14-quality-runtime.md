# Visual Review: quality rule runtime and reuse

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/views/admin/AccountQualityView.vue", "frontend/src/i18n/locales/zh/admin/resources.ts", "frontend/src/i18n/locales/en/admin/resources.ts"],
  "routes_or_surfaces": ["/admin/account-quality existing rule, template and preset forms", "Actual component with synthetic APIs/layout; development evidence only"],
  "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
  "states": ["default", "hover", "focus", "loading", "disabled", "empty", "error", "success", "clone", "selected-field bulk", "partial failure", "unconfirmed readback"],
  "viewports": ["360x900", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "browser-capture",
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-14-quality-runtime/baseline-360-light.png", "docs/visual-reviews/assets/2026-11-14-quality-runtime/baseline-1280-dark.png"],
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-14-quality-runtime/prototype-360-light.png", "docs/visual-reviews/assets/2026-11-14-quality-runtime/prototype-1280-dark.png", "docs/visual-reviews/assets/2026-11-14-quality-runtime/prototype-reuse-360.png", "docs/visual-reviews/assets/2026-11-14-quality-runtime/prototype-reuse-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-11-14-quality-runtime/updated-360-light.png", "docs/visual-reviews/assets/2026-11-14-quality-runtime/updated-1280-dark.png", "docs/visual-reviews/assets/2026-11-14-quality-runtime/updated-keyboard-360.png", "docs/visual-reviews/assets/2026-11-14-quality-runtime/updated-template-360.png", "docs/visual-reviews/assets/2026-11-14-quality-runtime/updated-bulk-360.png", "docs/visual-reviews/assets/2026-11-14-quality-runtime/updated-clone-360.png"],
  "commands": ["NODE_OPTIONS=--max-old-space-size=1024 node /data/tmp/v2212-quality-runtime-visual.mjs baseline # Playwright", "NODE_OPTIONS=--max-old-space-size=1024 node /data/tmp/v2212-quality-runtime-visual.mjs prototype # Playwright", "NODE_OPTIONS=--max-old-space-size=1024 node /data/tmp/v2212-quality-runtime-visual.mjs updated # Playwright"],
  "checks": {"keyboard": {"status": "passed"}, "reduced_motion": {"status": "passed"}},
  "residual_risks": ["Synthetic API/component captures are development evidence, not authentication or production acceptance.", "Combined frontend typecheck/build, backend queue/database integration and user-local production verification are release gates."]
}
-->

## Scope

Port upstream v2.21.2 OAuth 5xx triggering and retest rest into the existing Fork
quality rule, group template and reusable preset forms. Add minimal clone and
explicit selected-field bulk editing on the current rule list. Preserve Fork
multi-model probes, ownership operations and the existing automatic-restore
choice. No route, page redesign, BPS runtime or new dependencies are added.

## Baseline

Read the current quality page, scheduled-test controls, shared Toggle and
QualityIsolationGroups. Captured the actual compiled page before visible edits,
using synthetic accounts and API responses in an in-memory browser harness.
Existing forms had no 5xx/rest controls or clone/bulk rule entry.

## Prototype

Before editing production UI, rendered both runtime controls, clone entry and
selected-field bulk controls by transforming the current component in memory.
The PNGs show the proposed fields and narrow-screen wrapping. This defines an
incremental extension of the existing operational page, with no new shell.

## Reuse Decision

Keep existing page sections, native labels/inputs/selects, buttons, Icon, Toggle
and isolation group picker. Place rest beside failure action and keep 5xx trigger
independent. Reuse the same editor for bulk drafts; no fields are checked by
default. Clone requires an explicit destination account and excludes result and
ownership snapshots. Bulk refreshes each target, merges checked fields into its
own nested configuration, then performs version-checked updates and readback.

## State Coverage

Tests cover rule save/reopen, false triggers, legacy defaults, template/preset
roundtrips, invalid rest and observation cleanup. Reuse tests cover explicit
clone target, preserved multi-model and reusable fields, no ownership copying,
empty bulk selection, selected-field validation, current CAS versions, partial
failure and unconfirmed committed readback. Failed rules retain selection and
draft. Existing empty/loading/history/ownership states remain covered by the
page suite. New controls use existing focus and disabled styles.

## Viewport Coverage

Baseline and prototype each ran 360, 768, 1280 and 1920px in Chinese/English,
light/dark and reduced motion, with no horizontal overflow or browser error.
Updated captures ran the same 16 cases without overflow or browser errors.
Keyboard Space changes the native 5xx and bulk field checkboxes; rest is editable,
the template includes both fields, and clone starts with an unselected account.
Additional 360px captures show those real component interactions.

## Evidence

The runtime RED run failed seven page business assertions and the missing helper
suite. Its initial GREEN passed 48 tests. Five separate clone/bulk business tests
then failed for the absent controls before implementation. The combined GREEN
passes 53 tests across two files. Harness source, verbose logs and scenario
reports stay outside Git under `/data/tmp/v2212-quality-*`. Targeted ESLint,
design governance and `git diff --check` pass. Twelve real PNGs total under 1 MB.

## Residual Risk

All captures compile the actual component but use synthetic APIs and simplified
surrounding layout. They do not replace authenticated routes or user-local
production acceptance. Combined frontend typecheck/build, backend integration
tests, Fork checks and production verification remain release gates.
