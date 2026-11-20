# Gemini account setup

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/account/CreateAccountModal.vue",
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/components/account/__tests__/CreateAccountModal.spec.ts",
    "frontend/src/components/account/__tests__/EditAccountModal.spec.ts",
    "frontend/src/i18n/locales/en/admin/accounts.ts",
    "frontend/src/i18n/locales/zh/admin/accounts.ts",
    "frontend/src/utils/geminiExplicitCache.ts",
    "frontend/src/utils/__tests__/geminiExplicitCache.spec.ts"
  ],
  "routes_or_surfaces": ["/admin/accounts Gemini create and edit dialogs"],
  "languages_and_themes": ["zh/light", "en/dark", "en/light", "zh/dark"],
  "states": ["service account default", "Vertex API key", "project selection", "existing masked key", "explicit cache enabled", "explicit cache disabled", "official URL"],
  "viewports": ["360x800", "768x900", "1280x900", "1920x1080"],
  "artifact_mode": "browser-capture",
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-20-gemini-account/baseline-create-vertex-1280.png", "docs/visual-reviews/assets/2026-11-20-gemini-account/baseline-edit-cache-1280.png"],
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-20-gemini-account/prototype-create-vertex-1280.png", "docs/visual-reviews/assets/2026-11-20-gemini-account/prototype-edit-cache-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-11-20-gemini-account/updated-create-vertex-1280.png", "docs/visual-reviews/assets/2026-11-20-gemini-account/updated-edit-cache-1280.png", "docs/visual-reviews/assets/2026-11-20-gemini-account/updated-create-vertex-360.png", "docs/visual-reviews/assets/2026-11-20-gemini-account/updated-edit-cache-360.png"],
  "commands": ["Vite production build and Playwright Chromium with intercepted fixture APIs", "Vitest Gemini account dialogs and explicit cache credentials"],
  "checks": {"keyboard": {"status": "passed", "notes": "Space switches the shared cache toggle and Escape closes the dialog."}, "reduced_motion": {"status": "passed", "notes": "Chromium emulated reduced motion in all four updated scenarios."}},
  "residual_risks": ["The rendered fixtures use synthetic accounts. Production browser acceptance and live credentials remain independent of the visual review."]
}
-->

## Scope

Add the fixed upstream Gemini cache and Vertex authentication controls to the administrator account dialogs.

## Baseline

Playwright opened the account list and dialogs from the #448 production build with fixture responses. Vertex offered JSON authentication. The Gemini gateway credential form had no explicit cache controls.

## Prototype

Playwright rendered the upstream changes in a detached copy of the Fork account dialogs before applying them to the release worktree. The Vertex authentication picker and custom gateway cache settings follow the layout shown in the prototype captures.

## Reuse Decision

The account dialogs keep BaseDialog, shared Toggle, standard inputs and existing theme styles. The Gemini account type choices stack on small screens after the prototype exposed cramped cards.

## State Coverage

Chromium covered selecting Vertex API key authentication, entering the optional project ID, loading a cache setting, and toggling the TTL field with the keyboard. Component tests cover official URL visibility, submitted credentials, rotation and blank-key preservation. Existing dialog submission handles request loading and errors.

## Viewport Coverage

The four viewports in the manifest rendered without page errors or horizontal viewport overflow. The fixtures exercised both languages, both themes, keyboard controls and reduced motion.

## Evidence

The attached captures show the actual account dialogs from the baseline, proposal and updated builds. The browser intercepted API requests and used synthetic account metadata.

## Residual Risk

Production browser acceptance remains pending.
