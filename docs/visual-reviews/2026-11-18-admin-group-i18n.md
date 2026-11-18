# Visual Review: Admin Group Translations

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/group/GroupI18nField.vue",
    "frontend/src/components/common/LocaleSwitcher.vue",
    "frontend/src/i18n/locales/en/admin/overview.ts",
    "frontend/src/i18n/locales/zh/admin/overview.ts",
    "frontend/src/views/admin/GroupsView.vue"
  ],
  "routes_or_surfaces": ["Admin group create and edit forms"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en/light"],
  "states": ["default locale", "locale menu open", "focused translation field", "empty translation", "update submitted"],
  "viewports": ["360x900", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/group-i18n/prototype-390.png",
    "docs/visual-reviews/assets/group-i18n/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/group-i18n/baseline-form-360.png",
    "docs/visual-reviews/assets/group-i18n/baseline-form-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/group-i18n/updated-form-360.png",
    "docs/visual-reviews/assets/group-i18n/updated-form-768.png",
    "docs/visual-reviews/assets/group-i18n/updated-form-1280.png",
    "docs/visual-reviews/assets/group-i18n/updated-form-1920.png",
    "docs/visual-reviews/assets/group-i18n/updated-form-menu-1280.png",
    "docs/visual-reviews/assets/group-i18n/updated-edit-en-1280.png"
  ],
  "commands": [
    "pnpm typecheck",
    "pnpm lint:check",
    "pnpm exec vitest run src/components/admin/group/__tests__/GroupI18nField.spec.ts src/components/common/__tests__/LocaleSwitcher.spec.ts",
    "Playwright browser capture of the baseline and updated GroupsView with intercepted API fixtures"
  ],
  "checks": {
    "keyboard": { "status": "passed" },
    "reduced_motion": { "status": "passed" }
  },
  "residual_risks": [
    "Playwright captured the actual application view with local API fixtures. Final browser acceptance still needs to verify saved translations in the production user view."
  ]
}
-->

## Scope

Administrators can edit per-language names and descriptions in the existing create and edit group dialogs. The default fields keep the original text. Selecting a site language reloads the current route so that its API responses use the new language.

## Baseline

The baseline images show the actual create group dialog from the pre-change worktree at mobile and desktop widths. The dialog previously showed one name input and one description input.

## Prototype

The prototype images show the proposed language picker alongside each field label. The implemented scope covers the existing name and description fields.

## Reuse Decision

The implementation uses the existing group dialog and form classes, dropdown styling, Icon component, and locale catalog.

## State Coverage

Playwright opened both dialogs, switched to English, verified focus moved to the translation input, checked that switching back restored the draft default name, and submitted an edit against a controlled API fixture. The submitted edit kept the original name and the existing English description. Vitest covers opening and dismissing the selector, editing translations, and keeping the active language in the current route. The parent form continues to handle validation, submitting, and save errors.

## Viewport Coverage

Chromium captured the Chinese create dialog at 360, 768, 1280, and 1920 pixels. The 768 and 1920 pixel cases used dark theme. It also captured the opened language menu and the English edit dialog at 1280 pixels. The viewport checks verified the edited input remained inside the available width. The browser ran with reduced motion enabled.

## Evidence

The manifest lists the before and after PNGs. All updated screenshots use the actual `GroupsView.vue` with an isolated administrator fixture. The Go tests independently verify the request language, fallback text, localization without exporting the translation map, and database persistence.

## Residual Risk

The final three-role acceptance uses the production site after deployment. Verify the updated group appears in the selected language on the user side, and that reverting the language displays the original value for an untranslated locale.
