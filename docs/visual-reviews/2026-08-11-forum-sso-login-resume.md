# Visual Review: archived forum SSO login resume

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/router/index.ts"],
  "routes_or_surfaces": ["/login", "archived forum SSO resume route"],
  "languages_and_themes": ["zh-CN/light", "en-US/light"],
  "states": ["archived", "removed"],
  "viewports": ["360x800", "1280x800"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/upstream-0-1-172/prototype-static-review-board.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/upstream-0-1-172/baseline-static-review-board.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/upstream-0-1-172/updated-static-review-board.png"],
  "commands": ["pnpm design:check"],
  "checks": {
    "keyboard": {"status": "not-applicable", "reason": "The forum SSO resume flow is archived and no longer rendered."},
    "reduced_motion": {"status": "not-applicable", "reason": "The forum SSO resume flow is archived and no longer rendered."}
  },
  "residual_risks": ["This archived record is retained only so historical visual evidence remains traceable; the Forum SSO feature is intentionally removed from the runtime. No browser acceptance is applicable to the removed route."]
}
-->

## Scope

- Historical route: forum SSO login resume.
- Status: archived; the feature and its runtime route are intentionally removed.

## Baseline

- Historical evidence remains represented by the upstream static review board assets.

## Prototype

- The original prototype artifacts are preserved in the upstream review asset set.

## Reuse Decision

- No runtime components are retained. This record is historical evidence only.

## State Coverage

- Archived and removed; no interactive runtime states remain.

## Viewport Coverage

- Historical 360x800 and 1280x800 review-board coverage.

## Evidence

- Static review-board artifacts are referenced in the manifest.

## Residual Risk

- The feature is not available for production acceptance; this record prevents deletion of the historical audit trail.
