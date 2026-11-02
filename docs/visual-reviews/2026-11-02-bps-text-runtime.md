# Visual Review: BPS text Responses account control

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/i18n/locales/en/admin/accounts.ts",
    "frontend/src/i18n/locales/zh/admin/accounts.ts"
  ],
  "routes_or_surfaces": ["/admin/accounts account edit modal", "OpenAI OAuth account settings"],
  "languages_and_themes": ["zh/light", "en/dark"],
  "states": ["default off", "enabled with text-only warning", "focus-visible", "save and reload"],
  "viewports": ["750x2121", "1080x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/bps-text-runtime-20261102/prototype-bps-text-toggle.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/bps-text-runtime-20261102/baseline-account-edit-production.png"],
  "updated_artifacts": [
    "docs/visual-reviews/assets/bps-text-runtime-20261102/updated-bps-text-toggle-enabled.png",
    "docs/visual-reviews/assets/bps-text-runtime-20261102/updated-bps-text-toggle-focus.png"
  ],
  "commands": [
    "PYTHONPATH=/data/cache/quality-visual-tools python3.9 - (Pillow static review renders)",
    "frontend/node_modules/.bin/vitest run src/components/account/__tests__/EditAccountModal.spec.ts",
    "frontend/node_modules/.bin/eslint src/components/account/EditAccountModal.vue src/components/account/__tests__/EditAccountModal.spec.ts src/i18n/locales/en/admin/accounts.ts src/i18n/locales/zh/admin/accounts.ts",
    "node scripts/check-frontend-design-governance.mjs"
  ],
  "checks": {
    "keyboard": {"status": "passed", "note": "The switch keeps a visible focus ring and supports the native button keyboard interaction."},
    "reduced_motion": {"status": "passed", "note": "No continuous or layout motion is introduced; the switch uses the existing short transition pattern."}
  },
  "residual_risks": [
    "The PNGs are static design review renders, not captures of the running application. Verify the production account editor after deployment; no local development server or browser was used as production evidence."
  ]
}
-->

## Scope

- Surface: existing administrator account edit modal at `/admin/accounts`.
- Roles: administrators only; the switch is part of account configuration.
- Languages and themes: Chinese light and English dark strings use the existing locale entries and modal styling.
- Boundary: one opt-in account switch for text Responses. It does not add a page, navigation entry, or media controls.

## Baseline

- Current behavior: the user-provided production account-edit screenshot shows the established compact setting rows, native toggle treatment, and modal layout. It predates this BPS text switch.
- Baseline image: `assets/bps-text-runtime-20261102/baseline-account-edit-production.png`.
- Inconsistencies observed: none in the existing modal pattern; the new control reuses its divider, label, hint, and toggle conventions.

## Prototype

- Prototype design image: `assets/bps-text-runtime-20261102/prototype-bps-text-toggle.png`.
- Approval status: the task scope was already set to the BPS text Responses runtime and account-scoped opt-in.
- Scope boundary: display only for eligible OpenAI OAuth accounts; a warning appears when enabled.
- Note: this static prototype is a review artifact for the already approved small control, not a claim that a pre-implementation image was reviewed.

## Reuse Decision

- The control is placed in the existing `EditAccountModal.vue` OpenAI settings area and uses the established semantic colors, divider, text sizes, and switch shape.
- No shared layout, page shell, or navigation was changed.
- Keyboard focus uses `focus-visible` ring utilities.

## State Coverage

- Default: switch starts off unless `extra.openai_excel_bps` is explicitly true.
- Enabled: warning clarifies that the path is text-only and uses separate Excel OAuth authorization.
- Focus-visible: the existing control shows a visible primary ring.
- Save/reload: component tests verify the true flag persists, reload reflects it, and disabling removes the flag.
- Loading, error, empty, and success states do not apply to this synchronous local switch.

## Viewport Coverage

- The baseline user screenshot is 750 pixels wide; the review render is 1080 pixels wide.
- This adds one wrapping label/hint row inside the existing modal and introduces no page-level width or scrolling changes.
- The static renders show default, enabled, and keyboard-focus treatments.

## Evidence

- Updated static renders: `assets/bps-text-runtime-20261102/updated-bps-text-toggle-enabled.png` and `assets/bps-text-runtime-20261102/updated-bps-text-toggle-focus.png`.
- Automated checks cover eligibility, persistence, warning visibility, and locale strings.
- Design governance is run with the frontend checks before submission.

## Residual Risk

- Static images review the design only; they are not browser captures of the application. Production admin account-editor visual acceptance remains part of post-deployment verification.
