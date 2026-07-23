# English Auth Settings I18n Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/common/SupportContactPanel.vue",
    "frontend/src/components/layout/AuthLayout.vue",
    "frontend/src/i18n/locales/en/common.ts",
    "frontend/src/i18n/locales/zh/common.ts",
    "frontend/src/views/HomeView.vue",
    "frontend/src/views/auth/EmailVerifyView.vue",
    "frontend/src/views/auth/RegisterView.vue"
  ],
  "routes_or_surfaces": [
    "/login public auth shell",
    "/register public auth shell",
    "/en public home shell",
    "shared SupportContactPanel in public and authenticated support surfaces"
  ],
  "languages_and_themes": [
    "en-US light static board",
    "zh-CN light static board",
    "en-US dark inherited token review",
    "zh-CN dark inherited token review"
  ],
  "states": [
    "English locale with Chinese public settings injected",
    "Chinese locale with Chinese public settings injected",
    "primary QR support contacts",
    "secondary support contact actions",
    "copy success and failure toast copy"
  ],
  "viewports": [
    "390x844",
    "1280x760",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/english-auth-settings-i18n/prototype-english-auth-settings-i18n.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/english-auth-settings-i18n/baseline-english-auth-settings-i18n.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/english-auth-settings-i18n/updated-english-auth-settings-i18n.png"
  ],
  "commands": [
    "python3 generated static bilingual auth settings review boards with PIL",
    "pnpm --dir frontend exec vitest run src/components/layout/__tests__/AuthLayout.localizedSettings.spec.ts src/utils/__tests__/localizedPublicSettings.spec.ts src/i18n/__tests__/lazyLocaleScope.spec.ts src/i18n/__tests__/englishBrandNoCountryFraming.spec.ts",
    "pnpm --dir frontend run typecheck",
    "pnpm --dir frontend run test:run",
    "pnpm --dir frontend run build",
    "git diff --check"
  ],
  "checks": {
    "keyboard": {
      "status": "passed"
    },
    "reduced_motion": {
      "status": "passed"
    }
  },
  "residual_risks": [
    "The artifacts are static review boards, not browser screenshots, so final browser acceptance should still inspect login, register, and /en with production public settings.",
    "This change localizes presentation for known public settings text; future admin-entered Chinese custom values in contact account identifiers may still need content governance."
  ]
}
-->

## Scope

This review covers the bilingual presentation of public settings inside the authentication shell, public home shell, and shared support contact panel. The change is text presentation only: no spacing, layout, color, radius, or component hierarchy was redesigned.

## Baseline

The reported English login screenshot showed raw Chinese public settings in an English UI: `极速蹬`, the Chinese site subtitle, `联系客服`, `微信服务群`, `QQ 客服群`, and Chinese support actions. The root cause was that `AuthLayout` and `SupportContactPanel` rendered the app store public settings directly.

Baseline artifact: `docs/visual-reviews/assets/english-auth-settings-i18n/baseline-english-auth-settings-i18n.png`.

## Prototype

The target presentation keeps the store and backend configuration untouched, then localizes at display time. English UI shows `Jisudeng`, the English subtitle fallback, English support labels, English type badges, and English action/toast copy. Chinese UI continues to display the configured Chinese brand, subtitle, and support text.

Prototype artifact: `docs/visual-reviews/assets/english-auth-settings-i18n/prototype-english-auth-settings-i18n.png`.

## Reuse Decision

The implementation reuses the existing `AuthLayout`, `HomeView`, `SupportContactPanel`, common locale messages, and app store. A small helper centralizes public-settings presentation rules instead of mutating the store or duplicating per-page text transformations.

## State Coverage

The review covers English locale with Chinese public settings, Chinese locale with the same settings, QR-card support contacts, secondary support actions, type badges, copy/open buttons, and copy success/failure toast messages. Existing empty and hidden support-contact states are unchanged because contact filtering still uses the current support-contact utilities.

## Viewport Coverage

The static boards cover mobile, desktop, and wide desktop intent. Since the implementation changes text selection only, the existing responsive CSS continues to own wrapping and layout. Dark mode inherits the same token classes; no new color, shadow, radius, or motion rule was added.

## Evidence

Updated artifact: `docs/visual-reviews/assets/english-auth-settings-i18n/updated-english-auth-settings-i18n.png`.

Automated evidence:

```bash
python3 generated static bilingual auth settings review boards with PIL
pnpm --dir frontend exec vitest run src/components/layout/__tests__/AuthLayout.localizedSettings.spec.ts src/utils/__tests__/localizedPublicSettings.spec.ts src/i18n/__tests__/lazyLocaleScope.spec.ts src/i18n/__tests__/englishBrandNoCountryFraming.spec.ts
pnpm --dir frontend run typecheck
pnpm --dir frontend run test:run
pnpm --dir frontend run build
git diff --check
```

Results: targeted bilingual tests passed with 31 tests, full frontend test run passed with 262 files and 1680 tests, typecheck passed, build passed, and `git diff --check` passed.

## Residual Risk

The static board is not a browser screenshot. Before deployment acceptance, login, register, `/en`, and Chinese public routes should be inspected with production public settings to confirm English surfaces have no raw Chinese UI/config labels and Chinese routes remain Chinese.
