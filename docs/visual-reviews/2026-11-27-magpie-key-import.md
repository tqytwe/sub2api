# API 密钥导入 Magpie

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/user/KeysView.vue",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/zh/dashboard.ts"
  ],
  "routes_or_surfaces": ["/keys — per-key actions"],
  "languages_and_themes": ["zh-CN/light", "en-US/light"],
  "states": [
    "masked key with assigned supported group",
    "Magpie import action visible alongside existing per-key actions",
    "import remains disabled while public settings load",
    "missing settings, unsupported group, invalid endpoint and invalid key errors",
    "keyboard focus and activation",
    "custom-protocol handoff with only the selected key"
  ],
  "viewports": ["390x800", "1280x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-27-magpie-import/prototype-390.png",
    "docs/visual-reviews/assets/2026-11-27-magpie-import/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-27-magpie-import/baseline-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-27-magpie-import/updated-390.png",
    "docs/visual-reviews/assets/2026-11-27-magpie-import/updated-1280.png",
    "docs/visual-reviews/assets/2026-11-27-magpie-import/updated-en-1280.png"
  ],
  "commands": [
    "Playwright Chromium rendered the isolated static review board at 390x800 and 1280x900; no application or production data was loaded.",
    "pnpm --dir frontend exec vitest run src/views/user/__tests__/KeysView.spec.ts src/utils/__tests__/magpieImport.spec.ts"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "KeysView renders a native button; keyboard activation follows button semantics and the focused state uses the shared action styling."},
    "reduced_motion": {"status": "not-applicable", "reason": "The new action adds no animation or motion."}
  },
  "residual_risks": [
    "The rendered boards are static review artifacts, not screenshots of the running application. Verify the deployed key-actions row and custom-protocol handoff in a local browser before release acceptance."
  ]
}
-->

## Scope

- Surface: one action per API key on `/keys`.
- Roles: signed-in user viewing their own keys.
- Languages/themes: Chinese and English, light theme.

## Baseline

- Existing actions include Use Key and Import CCS; there is no Magpie action.
- Baseline board: `assets/2026-11-27-magpie-import/baseline-1280.png`.

## Prototype

- Proposal: add an Import Magpie action to the existing per-key action row without changing the key or group layout.
- Prototype boards: `assets/2026-11-27-magpie-import/prototype-390.png` and `prototype-1280.png`.
- The prototype reuses the current compact icon-and-label action pattern and keeps the key value masked.

## Reuse Decision

- Reuse the existing key action row, shared `Icon`, localization and app notification store.
- No new shared UI pattern or design-system exception.

## State Coverage

- The action is available only after public settings load; missing settings and unsupported groups produce localized errors.
- Endpoint validation rejects unsafe public HTTP origins and malformed bases. Keys are normalized and length/control-character checked before URL construction.
- The URL contains only the selected key and derived endpoints. No network request sends the key to Magpie; activation delegates to the OS custom-protocol handler.
- The action is a native button and supports keyboard focus and activation.

## Viewport Coverage

- Static boards cover 390px mobile and 1280px desktop in Chinese; the desktop board also covers English text.
- The action row follows the existing wrapping behavior on narrow screens.
- Dark theme, 200% zoom and the actual browser custom-protocol confirmation remain for local browser acceptance.

## Evidence

- Updated static boards: `assets/2026-11-27-magpie-import/updated-390.png`, `updated-1280.png` and `updated-en-1280.png`.
- Component and URL-builder regressions cover selection, loading, unsupported groups and endpoint/key validation.
- These images are review boards rather than running-app captures; no production keys or API calls were used.

## Residual Risk

- Confirm Magpie is installed and the browser/OS protocol prompt behaves as expected on a user workstation before release acceptance.
