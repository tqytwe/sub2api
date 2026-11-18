# Visual Review: OpenAI Authorization Connections

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/components/account/OpenAIAuthConnections.vue",
    "frontend/src/i18n/index.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/openaiAuth.ts",
    "frontend/src/i18n/locales/en/openaiAuth.ts"
  ],
  "routes_or_surfaces": [
    "/admin/accounts existing EditAccountModal authorization section; browser captures compile the real component with synthetic APIs and are not production acceptance"
  ],
  "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
  "states": [
    "existing legacy connection status",
    "managed connection pending/active/disabled",
    "add and authorize",
    "refresh uncertain authorization",
    "revocation confirmation",
    "loading and retry error",
    "keyboard focus and Enter activation"
  ],
  "viewports": ["360x1000", "768x1000", "1280x1000", "1920x1000"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-17-auth-connections/prototype-360-light.png",
    "docs/visual-reviews/assets/2026-11-17-auth-connections/prototype-1280-dark.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-17-auth-connections/baseline-360-light.png",
    "docs/visual-reviews/assets/2026-11-17-auth-connections/baseline-1280-dark.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-17-auth-connections/updated-360-light.png",
    "docs/visual-reviews/assets/2026-11-17-auth-connections/updated-1280-dark.png",
    "docs/visual-reviews/assets/2026-11-17-auth-connections/updated-error-1920-dark.png"
  ],
  "commands": [
    "Playwright browser capture: NODE_OPTIONS=--max-old-space-size=1024 node /data/tmp/v2212-auth-connections-visual.mjs baseline",
    "Playwright browser capture: NODE_OPTIONS=--max-old-space-size=1024 node /data/tmp/v2212-auth-connections-visual.mjs prototype",
    "Playwright browser capture: NODE_OPTIONS=--max-old-space-size=1024 node /data/tmp/v2212-auth-connections-visual.mjs updated"
  ],
  "checks": {
    "viewport_overflow": {"status": "passed", "scenarios": 16},
    "keyboard": {"status": "passed", "details": "Focused label, typed text, tabbed to add, Enter activated the action"},
    "error_state": {"status": "passed", "details": "API failure renders an alert and page remains usable"},
    "page_errors": {"status": "passed"},
    "reduced_motion": {"status": "passed"}
  },
  "residual_risks": [
    "The browser harness uses the real Vue connection component but synthetic API responses and a small host shell; it does not prove the authenticated production modal or permissions.",
    "The baseline capture shows the existing ExcelOAuthBinding component; its locale is intentionally not a comparison for the new messages.",
    "Final integrated build, independent review and user-local production acceptance remain required."
  ]
}
-->

## Scope

Add independently managed Codex, BPS and SIWC authorization connections within the existing administrator account editor. The section reuses the account modal and established form controls; it adds no route or navigation. The localized controls are included in the cold `/admin/accounts` lazy locale bundle.

## Prototype

The prototype used the real connection component inside the existing account modal shell with synthetic API responses. It covered empty, pending, active and error states before the integrated implementation was reviewed.

## Reuse Decision

The connection section reuses the existing account edit modal, button, input, details and alert primitives. It does not add a separate management page or navigation entry.

## State Coverage

Covered states include legacy and managed connections, pending/active/disabled status, add/authorize, uncertain refresh, revoke confirmation, loading, API error, retry, keyboard focus and Enter activation.

## Viewport Coverage

The review covered 360, 768, 1280 and 1920 pixel viewports in Chinese and English, light and dark themes, including the modal narrow layout and keyboard operation.

## Baseline and Prototype

Before implementation, the existing account editor and Excel OAuth binding were reviewed. Baseline captures render the existing Excel OAuth binding component. Prototype captures use the real new Vue component with synthetic API responses and are design evidence only. The final updated captures cover the existing/pending connection states, keyboard activation and a recoverable load error.

## Review

The connection section uses the existing modal surface, divider, input and button styles. At 360, 768, 1280 and 1920 pixels, both locales and themes rendered without horizontal overflow. Keyboard review focused the connection label, entered text, moved to Add with Tab and activated it with Enter. Reduced-motion emulation was enabled. The browser reported no page errors.

## Evidence Limits

Captures use a real compiled Vue component, synthetic APIs and a small host shell; they do not establish production behavior, authentication, authorization or service integration. The cold-route locale test and component tests validate the application’s lazy translation and interaction contracts separately. No production account was changed and no upstream paid call was made.

## Residual Risk

The captures do not replace production administrator acceptance. Integrated build, migration review, exact-SHA deployment and user-local guest/user/admin acceptance remain required.
