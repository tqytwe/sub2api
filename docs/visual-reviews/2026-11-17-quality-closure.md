# Visual Review: Quality Operations Closure

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/AccountQualityView.vue",
    "frontend/src/i18n/locales/en/admin/resources.ts",
    "frontend/src/i18n/locales/zh/admin/resources.ts",
    "frontend/src/api/admin/accountQuality.ts",
    "frontend/src/types/index.ts",
    "frontend/src/utils/qualityRulePatch.ts"
  ],
  "routes_or_surfaces": ["/admin/account-quality existing quality operations view"],
  "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
  "states": ["fixed question", "random reasoning question", "OAuth 401 threshold", "preset create/edit", "history", "batch evidence", "loading", "empty", "error", "disabled", "observe only"],
  "viewports": ["360x900", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "browser-capture",
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-17-quality-closure/baseline-360-light.png", "docs/visual-reviews/assets/2026-11-17-quality-closure/baseline-1280-dark.png"],
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-17-quality-closure/prototype-360-light.png", "docs/visual-reviews/assets/2026-11-17-quality-closure/prototype-1280-dark.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-11-17-quality-closure/updated-360-light.png", "docs/visual-reviews/assets/2026-11-17-quality-closure/updated-1280-dark.png"],
  "commands": ["Playwright Chromium component capture with synthetic API responses", "pnpm exec vitest run src/views/admin/__tests__/AccountQualityView.spec.ts"],
  "checks": {"viewport_overflow": {"status": "passed", "scenarios": 16}, "page_errors": {"status": "passed"}, "keyboard": {"status": "passed"}, "reduced_motion": {"status": "passed"}},
  "residual_risks": ["Component captures use synthetic data and do not replace production administrator acceptance.", "Final integrated build, migration review, deployment and user-local acceptance remain required."]
}
-->

## Scope

Close the quality operations view around fixed upstream random reasoning, shared batch evidence, OAuth 401 round controls and preset editing. The existing administrator route and controls remain the product surface.

## Baseline

The baseline captures show the existing quality operations view before the random reasoning, shared batch evidence and preset controls were integrated.

## Prototype

The real quality view was rendered with synthetic API responses for fixed and random questions, history, batch evidence and error states before integration.

## Reuse Decision

The work reuses the existing quality operations view, form controls, history rows, localization bundles and shared button/input primitives. It adds no user-facing route.

## State Coverage

Covered states include fixed/random question selection, reasoning effort, multiple models, OAuth 401 threshold, preset create/edit, history, batch evidence, loading, empty, error, disabled and observe-only behavior.

## Viewport Coverage

Chinese and English light/dark scenarios were checked at 360, 768, 1280 and 1920 pixels with no horizontal overflow. Keyboard focus and disabled controls were included.

## Evidence

The screenshots and tests use the real Vue component with synthetic responses. Backend service and PostgreSQL integration tests provide the corresponding durable behavior evidence.

## Residual Risk

Production administrator acceptance, full integrated build, exact-SHA deployment and the final guest/user/admin regression remain required.
