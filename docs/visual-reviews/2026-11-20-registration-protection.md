# Registration and login controls

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/settings.ts",
    "frontend/src/i18n/locales/en/admin/settings.ts",
    "frontend/src/i18n/locales/en/common.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts",
    "frontend/src/i18n/locales/zh/common.ts",
    "frontend/src/views/admin/SettingsView.vue",
    "frontend/src/views/auth/EmailVerifyView.vue",
    "frontend/src/views/auth/RegisterView.vue"
  ],
  "routes_or_surfaces": ["Administrator settings, Security, Registration Settings", "Existing registration and verification error messages"],
  "languages_and_themes": ["zh/light", "en/light", "zh/dark", "en/dark"],
  "states": ["Default controls enabled", "One independently disabled", "Existing registration form reports the specific server rejection"],
  "viewports": ["1280x900", "360x800", "768x900", "1920x1080"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-20-registration-protection/prototype-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-20-registration-protection/baseline-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-11-20-registration-protection/updated-1280.png", "docs/visual-reviews/assets/2026-11-20-registration-protection/updated-360.png"],
  "commands": ["Playwright Chromium with compiled site assets and fixture API responses", "Vitest settings, registration and email verification suites"],
  "checks": {
    "keyboard": {"status": "passed", "reason": "Opened Security with Enter. Focused the email alias setting and changed it using Space."},
    "reduced_motion": {"status": "not-applicable", "reason": "The task reuses the shared toggle and existing form messages."}
  },
  "residual_risks": ["Screenshots use a fixture administrator. Production acceptance requires the real site and a confirmed trusted ingress configuration."]
}
-->

## Scope

Administrators can control the client IP registration quota, email alias blocking and login attempt limits from the Security tab. The registration and verification pages use the existing error area.

## Baseline

The previous production build shows registration, email verification and domain controls in the same settings card.

## Prototype

The capture shows the built Security page with the new settings enabled before interaction.

## Reuse Decision

The controls use the shared Toggle with its built-in accessible label and description. Rejected registrations use the existing form feedback.

## State Coverage

Chromium checked saved values and keyboard interaction. Vue tests cover the settings payload and the registration and verification flows.

## Viewport Coverage

The registration settings card was checked at 360, 768, 1280 and 1920 pixels. The captures show desktop light and mobile dark.

## Evidence

The manifest links the actual built page captures. Browser requests used fixtures.

## Residual Risk

Confirm the production reverse proxy settings before enabling IP policy on the public service.
