# Encrypted history recovery setting

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/settings.ts",
    "frontend/src/i18n/locales/en/admin/settings.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts",
    "frontend/src/views/admin/SettingsView.vue"
  ],
  "routes_or_surfaces": ["/admin/settings → Gateway → Request Forwarding"],
  "languages_and_themes": ["Chinese light and dark", "English light and dark"],
  "states": ["off by default", "enabled", "keyboard focus", "settings save and reload"],
  "viewports": ["360x800", "768x900", "1280x820", "1920x1080"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-19-encrypted-history-409/existing-toggle.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-19-encrypted-history-409/existing-toggle.png"],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-19-encrypted-history-409/default-1280.png",
    "docs/visual-reviews/assets/2026-11-19-encrypted-history-409/default-360.png",
    "docs/visual-reviews/assets/2026-11-19-encrypted-history-409/keyboard-enabled.png"
  ],
  "commands": ["pnpm build", "pnpm design:check", "Playwright on the built SettingsView with fixture HTTP responses"],
  "checks": {
    "keyboard": {"status": "passed", "reason": "Focused switch changes with Space at all reviewed widths."},
    "reduced_motion": {"status": "not-applicable", "reason": "The row uses the existing Toggle component without new animation."}
  },
  "residual_risks": ["Browser captures use fixture data. Production administrator acceptance is pending."]
}
-->

## Scope

Admin → Settings → Gateway → Request Forwarding.

## Baseline

The existing forwarding card has adjacent settings and no encrypted history switch. `existing-toggle.png` shows the adjacent forwarding control, which is unchanged in this branch.

## Prototype

Use the existing forwarding control captured in `existing-toggle.png` as the design reference for the new row.

## Reuse Decision

Use the shared `Toggle` with an accessible name and description in the existing settings form.

## State Coverage

Browser checks cover disabled-by-default, enabled, and keyboard focus. The settings test covers loading and saving. Existing form loading and error presentation apply.

## Viewport Coverage

The control stayed within the viewport at 360, 768, 1280 and 1920 px. Both languages and themes were exercised across these sizes.

## Evidence

Screenshots in the manifest capture the production build with fixture API responses. The keyboard capture shows the focused switch after pressing Space.

## Residual Risk

Production administrator acceptance remains pending.
