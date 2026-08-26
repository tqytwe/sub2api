# Visual Review: v0182-i18n-account-test-regression

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/i18n/index.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/admin/accounts.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/admin/accounts.ts",
    "frontend/src/components/common/LocaleSwitcher.vue",
    "frontend/src/router/index.ts",
    "frontend/src/views/HomeView.vue",
    "frontend/src/components/admin/account/AccountTestModal.vue",
    "frontend/src/components/admin/account/AccountStatsModal.vue",
    "frontend/src/components/admin/account/ScheduledTestsPanel.vue",
    "frontend/src/components/admin/account/ImportDataModal.vue",
    "frontend/src/components/admin/account/ReAuthAccountModal.vue",
    "frontend/src/components/account/CreateAccountModal.vue",
    "frontend/src/components/account/BulkEditAccountModal.vue",
    "frontend/src/components/account/SyncFromCrsModal.vue",
    "frontend/src/components/account/TempUnschedStatusModal.vue",
    "frontend/src/components/admin/ErrorPassthroughRulesModal.vue",
    "frontend/src/components/admin/TLSFingerprintProfilesModal.vue"
  ],
  "routes_or_surfaces": ["/dashboard", "/wallet", "/admin/accounts", "workspace locale switcher", "account test dialog"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "hover", "focus-visible", "loading", "disabled", "empty", "error", "success"],
  "viewports": ["360x800", "768x900", "1280x800", "1920x1080"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/i18n-p1-lazy-locale/prototype-i18n-p1-lazy-locale.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/i18n-p1-lazy-locale/baseline-i18n-p1-lazy-locale.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/i18n-p1-lazy-locale/updated-i18n-p1-lazy-locale.png"],
  "commands": ["pnpm design:check", "pnpm lint:check", "pnpm typecheck", "pnpm exec vitest run src/i18n/__tests__/lazyLocaleScope.spec.ts src/components/common/__tests__/LocaleSwitcher.spec.ts src/components/admin/account/__tests__/AccountTestModal.spec.ts"],
  "checks": {
    "keyboard": {"status": "passed", "notes": "Locale menu and model retry remain regular buttons with existing focus behavior."},
    "reduced_motion": {"status": "not-applicable", "reason": "The change adds no animation or continuous motion."}
  },
  "residual_risks": ["Static review artifacts are not a production browser capture; local role-based acceptance remains required after deployment."]
}
-->

## Scope

- Routes and surfaces: user workspace locale selection, `/dashboard`, `/wallet`, `/admin/accounts`, and account-related lazy dialogs.
- Roles: authenticated users and administrators.
- Languages and themes: Chinese and English in light and dark themes.

## Baseline

- An old `sub2api_locale=en` value could leak into unprefixed workspace routes.
- First-visible lazy dialogs could run their watcher before initialization functions were available, and the account test dialog could show an empty model selector after a failed request.
- Baseline artifact: `docs/visual-reviews/assets/i18n-p1-lazy-locale/baseline-i18n-p1-lazy-locale.png`.

## Prototype

- Prototype design image: `docs/visual-reviews/assets/i18n-p1-lazy-locale/prototype-i18n-p1-lazy-locale.png`.
- Approval status: implementation keeps the existing workspace and dialog visual language; only route language state and an inline error/retry state are changed.
- Scope boundary: no new page frame, icon system, card treatment, or modal layout.

## Reuse Decision

- Reused the existing `LocaleSwitcher`, `BaseDialog`, `Select`, semantic status text, and account modal layouts.
- No design-system exception or new shared visual pattern was added.

## State Coverage

- Default: unprefixed workspace routes resolve to Chinese; explicit `/en/*` and `?lang=en` resolve to English.
- Hover, active, focus-visible and keyboard: existing menu/button states remain in place.
- Loading and disabled: model select and retry button remain disabled while the request is active.
- Empty and error: empty model responses stay non-selectable; request failure displays localized text and a retry action.
- Success: a successful retry restores the model options and default selection.

## Viewport Coverage

- 360px and 768px: the existing compact menu and dialog controls retain their touch targets.
- 1280px and wide desktop: route content continues to use the existing workspace frame.
- 200% zoom, reduced motion, and theme contrast remain subject to local browser acceptance; no new motion was introduced.

## Evidence

- Updated artifact: `docs/visual-reviews/assets/i18n-p1-lazy-locale/updated-i18n-p1-lazy-locale.png`.
- Automated evidence: locale routing, locale switcher, initial model loading, localized model error, retry, and full design/lint/type checks.
- Browser screenshots are deployment-gated and are not represented by the static board artifacts.

## Residual Risk

- Production CDN output and administrator browser acceptance still need to verify the full lazy chunk lifecycle and Chinese/English switching after the authorized deployment.
