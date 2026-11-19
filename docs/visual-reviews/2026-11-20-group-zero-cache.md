# OpenAI group zero-cache protection

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/GroupsView.vue",
    "frontend/src/components/admin/group/GroupAntiDistillationSettings.vue",
    "frontend/src/api/admin/groups.ts",
    "frontend/src/i18n/locales/en/admin/overview.ts",
    "frontend/src/i18n/locales/zh/admin/overview.ts"
  ],
  "routes_or_surfaces": [
    "/admin/groups existing OpenAI group edit dialog"
  ],
  "languages_and_themes": [
    "zh/light",
    "zh/dark"
  ],
  "states": [
    "disabled by default",
    "enabled",
    "pause interval",
    "saved and reopened",
    "loading and save failures covered by component tests"
  ],
  "viewports": [
    "360x900",
    "768x900",
    "1280x900",
    "1920x900"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/group-anti-distillation-v2212/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/group-anti-distillation-v2212/baseline-actual-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/group-anti-distillation-v2212/updated-actual-360.png",
    "docs/visual-reviews/assets/group-anti-distillation-v2212/updated-actual-768.png",
    "docs/visual-reviews/assets/group-anti-distillation-v2212/updated-actual-1280.png",
    "docs/visual-reviews/assets/group-anti-distillation-v2212/updated-actual-1920.png"
  ],
  "commands": [
    "Playwright Chromium against the built admin group route with isolated API fixture responses",
    "pnpm design:check",
    "pnpm lint:check",
    "pnpm typecheck"
  ],
  "checks": {
    "keyboard": {
      "status": "passed"
    },
    "reduced_motion": {
      "status": "passed"
    },
    "viewport_overflow": {
      "status": "passed",
      "scenarios": 4
    }
  },
  "residual_risks": [
    "Production browser acceptance still pending. Fixture saves do not change production group settings."
  ]
}
-->

## Scope

Add the zero-cache protection setting to the existing OpenAI group edit dialog.

## Baseline

The baseline shows that dialog before the protection controls were added.

## Prototype

The prototype shows the controls below the RPM limit.

## Reuse Decision

The existing dialog, button, inputs and toggle stay in use. Save a new group before opening the protection setting.

## State Coverage

Chromium enabled the setting with the keyboard, saved an eight-minute pause and reopened the dialog. The interval and enabled state were restored. Component tests cover loading and save failures.

## Viewport Coverage

The actual built page showed no horizontal overflow or page errors at 360, 768, 1280 and 1920 pixels. The browser included both themes and reduced motion.

## Evidence

The baseline and updated images were captured in Chromium with local API fixtures on the actual group-management route.

## Residual Risk

Production browser acceptance is pending. Fixture saves did not change production group settings.
