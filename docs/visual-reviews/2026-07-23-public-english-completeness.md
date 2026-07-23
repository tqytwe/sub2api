# Public English Completeness Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/content/public-docs.ts",
    "frontend/src/content/public-docs-data.en.ts",
    "frontend/src/content/public-docs-tree.ts",
    "frontend/src/content/__tests__/publicDocsEnglishCompleteness.spec.ts",
    "frontend/src/i18n/locales/jisudeng-home.en.ts",
    "frontend/src/i18n/locales/jisudeng-home.zh.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/components/home/LmspeedBadge.vue",
    "frontend/src/components/home/TerminalDemo.vue",
    "frontend/src/utils/localizedPublicSettings.ts",
    "frontend/src/utils/__tests__/localizedPublicSettings.spec.ts",
    "frontend/src/views/HomeView.vue",
    "frontend/src/views/KeyUsageView.vue",
    "frontend/src/views/__tests__/KeyUsageView.spec.ts",
    "frontend/src/views/public/AboutView.vue",
    "frontend/src/views/public/DocsView.vue",
    "frontend/src/views/public/PromptSquareView.vue",
    "frontend/src/views/public/__tests__/AboutView.localized.spec.ts",
    "frontend/src/views/public/__tests__/PromptSquareView.spec.ts",
    "frontend/src/components/layout/__tests__/AuthLayout.localizedSettings.spec.ts",
    "frontend/src/components/public/DocsVipTiersTable.vue"
  ],
  "routes_or_surfaces": [
    "/en public home",
    "/en/docs public documentation index and reader",
    "/about with English language selected",
    "/key-usage with English language selected",
    "/prompts with English language selected",
    "shared public support-contact presentation"
  ],
  "languages_and_themes": [
    "en-US light static board",
    "zh-CN light existing behavior review",
    "en-US dark inherited token review",
    "zh-CN dark inherited token review"
  ],
  "states": [
    "English home hero with long brand copy constrained",
    "English docs index with mirrored category tree",
    "English docs reader with translated and pending-translation pages",
    "English docs content loaded from the active locale payload",
    "English public shell receiving default-language public settings",
    "English prompt library pending state",
    "Default-language public routes retaining default-language content"
  ],
  "viewports": [
    "360x740",
    "768x900",
    "1280x900",
    "1800x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/public-english-completeness/prototype-public-english-completeness.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/public-english-completeness/baseline-public-english-completeness.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/public-english-completeness/updated-public-english-completeness.png"
  ],
  "commands": [
    "python3 generated static public English completeness review boards with PIL",
    "file docs/visual-reviews/assets/public-english-completeness/*.png",
    "pnpm --dir frontend exec vitest run src/content/__tests__/publicDocsEnglishCompleteness.spec.ts src/views/public/__tests__/AboutView.localized.spec.ts src/views/__tests__/KeyUsageView.spec.ts src/utils/__tests__/localizedPublicSettings.spec.ts src/views/public/__tests__/PromptSquareView.spec.ts",
    "pnpm --dir frontend exec vitest run src/i18n/__tests__/englishBrandNoCountryFraming.spec.ts src/i18n/__tests__/bilingualProductUi.spec.ts src/components/layout/__tests__/AuthLayout.localizedSettings.spec.ts"
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
    "The artifacts are static review boards, not browser screenshots; final production browser acceptance is still required.",
    "Most English documentation pages are structurally present with reviewed English placeholders, not full article translations.",
    "The English prompt library intentionally shows a pending state until real English prompt assets are prepared."
  ]
}
-->

## Scope

This review covers the first public-English completeness pass for the reported production issues: the overflowing English home hero, the thin `/en/docs` index, default-language leakage in English public pages, and default-language prompt cards appearing inside an English language shell.

## Baseline

The reported screenshots showed the English home H1 overflowing horizontally, `/en/docs` exposing only three cards, About rendering default-language paragraphs and commitments, Key Usage rendering default-language branding and QR images inside an English shell, and Prompt Square rendering default-language prompt cards while the toolbar showed English.

Baseline artifact: `docs/visual-reviews/assets/public-english-completeness/baseline-public-english-completeness.png`.

## Prototype

The first-phase contract keeps the default-language routes and content intact while making public English surfaces coherent. English home uses a shorter H1 with fixed breakpoint sizes. English docs mirror the default docs tree, preserving existing translated pages and using reviewed English placeholders for articles pending translation. English docs now use a neutral category/page tree and dynamically load the active locale payload. English public settings localize presentation without mutating backend settings. English Prompt Square shows a pending state instead of rendering default-language content.

Prototype artifact: `docs/visual-reviews/assets/public-english-completeness/prototype-public-english-completeness.png`.

## Reuse Decision

The implementation reuses the existing HomeView, DocsView, AboutView, KeyUsageView, PromptSquareView, PublicPageToolbar, SupportContactPanel, and public-settings localization helper. No new page shell or design system primitive was introduced.

## State Coverage

Covered states include English language with default-language public settings, default-language content retained on default routes, English docs index and reader, pending English prompt-library content, and support contacts when QR images contain embedded default-language labels.

## Viewport Coverage

The static board records 360px, 768px, 1280px, and wide-desktop intent. The Home hero adds explicit breakpoint font sizes and max-width constraints for the English H1. Final browser screenshots remain required because static boards cannot prove rendered layout under production assets.

## Evidence

Updated artifact: `docs/visual-reviews/assets/public-english-completeness/updated-public-english-completeness.png`.

Automated evidence so far:

```bash
python3 generated static public English completeness review boards with PIL
file docs/visual-reviews/assets/public-english-completeness/*.png
pnpm --dir frontend exec vitest run src/content/__tests__/publicDocsEnglishCompleteness.spec.ts src/views/public/__tests__/AboutView.localized.spec.ts src/views/__tests__/KeyUsageView.spec.ts src/utils/__tests__/localizedPublicSettings.spec.ts src/views/public/__tests__/PromptSquareView.spec.ts
pnpm --dir frontend exec vitest run src/i18n/__tests__/englishBrandNoCountryFraming.spec.ts src/i18n/__tests__/bilingualProductUi.spec.ts src/components/layout/__tests__/AuthLayout.localizedSettings.spec.ts
```

Current results: targeted public-English tests passed with 27 tests; brand/no-country and bilingual-product UI tests passed; PNG artifacts decode as 1800x1080 RGB images.

## Residual Risk

This record uses static review boards rather than browser screenshots. Before release acceptance, production should still be checked at `/en`, `/en/docs`, `/about`, `/key-usage`, and `/prompts` in English and default-language states. Full article translations and a real English prompt library are intentionally left as follow-up content work.
