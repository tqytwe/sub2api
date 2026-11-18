# Visual Review: account diagnostics

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/account/AccountTestModal.vue",
    "frontend/src/components/admin/account/ModelTracePanel.vue",
    "frontend/src/components/account/AccountTestTiming.vue",
    "frontend/src/i18n/index.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en/index.ts",
    "frontend/src/i18n/locales/zh/index.ts",
    "frontend/src/i18n/locales/en/accountTestTiming.ts",
    "frontend/src/i18n/locales/en/modelTrace.ts",
    "frontend/src/i18n/locales/zh/accountTestTiming.ts",
    "frontend/src/i18n/locales/zh/modelTrace.ts"
  ],
  "routes_or_surfaces": [
    "The existing administrator account connection-test modal, rendered from the real AccountTestModal Vue component with a synthetic OAuth account and local API adapter."
  ],
  "languages_and_themes": [
    "en/light",
    "zh/light"
  ],
  "states": [
    "Baseline connection test mode in the existing modal",
    "ModelTrace setup, limitations and controls",
    "Codex ticket-probe verdict, ticket lengths, HTTP statuses and latency using synthetic values",
    "Timing summary with local timestamps, measured durations and collapsed detail disclosure",
    "360px mobile Codex ticket-probe result"
  ],
  "viewports": [
    "360x900",
    "1280x900"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-18-account-diagnostics/prototype-modeltrace-1280-en.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-18-account-diagnostics/baseline-1280-en.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-18-account-diagnostics/updated-state-probe-1280-en.png",
    "docs/visual-reviews/assets/2026-11-18-account-diagnostics/updated-state-probe-360-zh.png",
    "docs/visual-reviews/assets/2026-11-18-account-diagnostics/updated-timing-1280-zh.png"
  ],
  "commands": [
    "pnpm exec vite --host 127.0.0.1 --port 5174 --strictPort # temporary local component capture server; stopped after capture",
    "Playwright Chromium screenshot of the actual AccountTestModal component at 360x900 and 1280x900 with synthetic account data and no provider requests"
  ],
  "checks": {
    "keyboard": {
      "status": "not-applicable",
      "reason": "This review records component appearance; modal interaction is covered by component tests."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "The captured diagnostic panels contain no motion-dependent state or animation."
    }
  },
  "residual_risks": [
    "Screenshots render the real modal component in an isolated local harness with synthetic account and API values, not the authenticated application route.",
    "The browser capture process exhausted its Vite worker memory after the requested screenshots had been written; no server remains running.",
    "No real model or account requests were made, and deployed permissions and model availability still require normal application verification."
  ]
}
-->

## Scope

Add ModelTrace, the read-only Codex state probe and server-reported connection timing to the existing administrator account-test modal. No diagnostic route or navigation entry is added.

## Baseline

The baseline capture shows the existing connection-test view before selecting a diagnostic mode.

## Prototype

The prototype capture shows the ModelTrace setup and its evidence limits in the same real modal. It reuses the current account dialog rather than introducing a separate diagnostic page.

## Reuse Decision

The work uses the existing administrator test modal, account header, selects, dialog actions and localization system. New panels use the existing border, spacing, color and typography tokens.

## State Coverage

The review covers connection mode, ModelTrace controls, a Codex ticket-probe result with synthetic lengths and HTTP statuses, and the server timing summary with details collapsed. The state-probe response displays lengths and statuses; no ticket value, authorization value or account credential is present.

## Viewport Coverage

At 360px, the probe result wraps within the modal and remains vertically readable. At 1280px, the modal keeps account controls and diagnostic content in a single bounded dialog. Screenshots cover English and Chinese in light theme.

## Evidence

The images are Playwright Chromium screenshots of the real Vue modal component using a local adapter and synthetic account/model values. The adapter makes no provider requests. Targeted component tests and the cold route locale test cover behavior; the visual snapshots cover 360px and 1280px widths.

## Residual Risk

The isolated harness does not prove authenticated route permissions, production configuration or live model availability. No real model or account requests were made. The capture Vite process exhausted its worker memory after writing the screenshots and was stopped.

The isolated harness loaded the account-test locale scope but omitted the shared core scope, so the captured close-action label remains `common.close`; the real administrator route loads both scopes.
