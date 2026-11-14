# Visual Review: single-key model and context policy

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/user/ApiKeyModelPolicyDialog.vue",
    "frontend/src/components/admin/user/UserApiKeysModal.vue",
    "frontend/src/views/user/KeysView.vue",
    "frontend/src/api/admin/apiKeys.ts",
    "frontend/src/types/index.ts",
    "frontend/src/i18n/index.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en/index.ts",
    "frontend/src/i18n/locales/zh/index.ts",
    "frontend/src/i18n/locales/en/apiKeyModelPolicy.ts",
    "frontend/src/i18n/locales/zh/apiKeyModelPolicy.ts"
  ],
  "routes_or_surfaces": ["/admin/users user-key dialog (affected route)", "/keys administrator edit dialog (affected route)", "Actual components with synthetic fixtures; authenticated route acceptance pending"],
  "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
  "states": ["saved mapping and effort", "long-context disabled", "custom context ceiling", "administrator entry", "keyboard navigation", "empty/error/disabled and stale-save behavior tested in Vitest"],
  "viewports": ["360x900", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-14-key-model-policy/prototype-360-light.png", "docs/visual-reviews/assets/2026-11-14-key-model-policy/prototype-1280-dark.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-14-key-model-policy/baseline-360-light.png", "docs/visual-reviews/assets/2026-11-14-key-model-policy/baseline-1280-dark.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-11-14-key-model-policy/updated-360-light.png", "docs/visual-reviews/assets/2026-11-14-key-model-policy/updated-1280-dark.png", "docs/visual-reviews/assets/2026-11-14-key-model-policy/updated-360-context-controls.png", "docs/visual-reviews/assets/2026-11-14-key-model-policy/updated-entry-360-light.png"],
  "commands": ["node /data/tmp/v2212-key-policy-visual.mjs baseline # Playwright browser capture", "node /data/tmp/v2212-key-policy-visual.mjs prototype # Playwright browser capture", "node /data/tmp/v2212-key-policy-visual.mjs updated # Playwright browser capture", "node /data/tmp/v2212-key-policy-visual.mjs updated-entry # Playwright browser capture"],
  "checks": {
    "keyboard": { "status": "passed" },
    "reduced_motion": { "status": "passed" }
  },
  "residual_risks": [
    "The browser compiles actual Vue components with synthetic key/API/store fixtures; captures are not authenticated route or production screenshots.",
    "Administrator entry in the full Keys route is covered by component tests, not a full route browser capture. Final real-route and user-local production acceptance remain pending.",
    "Frontend evidence does not establish backend authorization, database persistence or gateway policy enforcement; the backend work package and integrated release gates must verify these separately."
  ]
}
-->

## Scope

Port the upstream v2.21.2 dialog and its existing administrator entry points: the
user-key list under `/admin/users` and the edit dialog under `/keys` when the viewer
is an administrator. Only model mapping, fixed effort, long-context permission and
the per-key context ceiling are included. Ordinary key create/update types omit all
four fields and ordinary users receive no model-policy editor. No page, group-access
workflow, daily-spend module, cross-IP rule or feature-promotion system is added.

## Baseline

Before production UI edits, Chromium compiled the existing `UserApiKeysModal` and
loaded an explicitly synthetic key through a bounded API fixture. Its normal
false-to-true open transition is exercised so the baseline contains the key, rather
than accidentally showing the empty state. There is no model-policy entry.

The existing key form, neighbouring admin bulk-action dialog, shared `BaseDialog`,
`Toggle` and `Icon` were inspected before implementation.

## Prototype

The prototype captures the actual upstream `ApiKeyModelPolicyDialog.vue`, before
local implementation. The scope was reviewed by the integrating maintainer before
business changes. Local adaptation removes nested framed sections and feature
promotion badges, uses existing action icons, and wraps the entry below the key on
mobile. Policy controls, validation and separate save semantics remain upstream's.
These are component design images, not a claim that production has this screen.

## Reuse Decision

Reuse shared `BaseDialog` for scrolling, footer, Escape and focus containment;
reuse the native input/select, `Toggle`, existing buttons and `Icon`. The new dialog
stays on top of its existing parent. Existing key names, groups, quotas, multi-group
bindings and other unsaved form fields keep their current editing flow.

## State Coverage

- Browser: saved mapping, fixed effort, disabled long-context permission, positive
  ceiling, administrator user-key entry, theme/language changes and reachable footer.
- Keyboard: native effort selection, editing the context ceiling and Tab containment
  in the shared dialog. A real shared-dialog unit test verifies Escape closes only
  the nested dialog.
- Vitest: legacy defaults, explicit false/zero, clearing mapping/effort/ceiling,
  UTF-8/control/duplicate validation, complete effort options, saving/failed retry,
  key changes, closing/unmounting during an in-flight save, and preserved main-form
  drafts. These states are behavior tests, not separate browser screenshots.
- Successful saves replace the selected saved key from the server response; the
  user's ordinary save still explicitly omits all four administrator policy fields.

## Viewport Coverage

Baseline, upstream prototype, updated dialog and updated entry each run at 360,
768, 1280 and 1920px by 900px in Chinese/English and light/dark. The updated surfaces
have no horizontal page overflow or page errors in their 16-case matrices. The
360px lower-controls capture verifies the long context ceiling is reachable by
scrolling. Every scenario requests reduced motion; no new animation is added.
Full-page 200% zoom and authenticated route checks remain release acceptance work.

## Evidence

Artifacts are in `assets/2026-11-14-key-model-policy/`. The temporary Playwright
script compiles actual components in memory, uses an intercepted `.invalid` origin,
and provides synthetic environment fixtures. It starts no application server and
contacts no production API. It deliberately substitutes the runtime locale singleton
with the review's real i18n instance to avoid bundling unrelated route dictionaries.

Six relevant API, dialog, key-page and locale test files pass 75 tests. Cold-start
locale tests exercise both affected route scopes in Chinese and English. The first
RED run established the missing admin API and entry points; its unsupported matcher
in a separate existing-group assertion was corrected and is not a product finding.
Final lint/design/typecheck/build and independent reviews belong to the integrated
release gates; this component record does not mark them complete.

## Residual Risk

No production setting or key was changed. The integrating maintainer must verify
the real admin response, persistence after refresh, ordinary-user write restrictions,
and gateway enforcement together with the backend work package. Authenticated
routes and the final user-local production check remain pending deployment.
