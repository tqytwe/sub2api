# Visual Review: LMSpeed speed-test bilingual locale

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/keys/UseKeyModal.vue"
  ],
  "routes_or_surfaces": ["/keys", "Use API Key dialog"],
  "languages_and_themes": ["zh-CN/light", "en-US/light", "zh-CN/dark", "en-US/dark"],
  "states": ["Chinese app locale opens LMSpeed Chinese route", "English app locale opens LMSpeed English root", "active key", "inactive key hidden"],
  "viewports": ["360x800", "768x800", "1280x858"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/lmspeed-key-speedtest/prototype-use-key-modal-lmspeed.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/lmspeed-key-speedtest/baseline-api-key-page-user.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/lmspeed-key-speedtest/updated-static-use-key-modal-lmspeed.png"
  ],
  "commands": [
    "pnpm --dir frontend exec vitest run src/utils/__tests__/lmspeed.spec.ts src/components/keys/__tests__/UseKeyModal.spec.ts",
    "pnpm --dir frontend design:check"
  ],
  "checks": {
    "keyboard": { "status": "passed" },
    "reduced_motion": { "status": "passed" }
  },
  "residual_risks": [
    "The artifacts are reused static review boards because this hotfix changes the external LMSpeed locale target only; final browser acceptance should still verify both Chinese and English app locales."
  ]
}
-->

## Scope

This review covers the LMSpeed action in the authenticated Use API Key dialog. The hotfix changes only the external LMSpeed URL locale selection: Chinese app locale opens the Chinese LMSpeed route, while English app locale opens the English LMSpeed root.

## Baseline

The visible action strip from the prior LMSpeed implementation was already approved and deployed, but its click handler always passed the Chinese LMSpeed locale. That made the English UI open a Chinese third-party speed-test page.

Baseline artifact: `docs/visual-reviews/assets/lmspeed-key-speedtest/baseline-api-key-page-user.png`.

## Prototype

No layout or copy prototype changed. The intended behavior is the same visible LMSpeed action strip, with locale-aware click behavior tied to the current `vue-i18n` locale.

Prototype artifact: `docs/visual-reviews/assets/lmspeed-key-speedtest/prototype-use-key-modal-lmspeed.png`.

## Reuse Decision

The implementation reuses the existing Use API Key dialog, action strip, i18n messages, and LMSpeed URL utility. A small locale resolver keeps the third-party route mapping centralized.

## State Coverage

Automated tests cover active key visibility, inactive key hidden state, no API key rendered in LMSpeed href attributes, Chinese app locale opening `/zh`, English app locale opening the LMSpeed root, and locale normalization for `zh`, `en`, and fallback values.

## Viewport Coverage

The visible UI has no spacing, sizing, color, layout, or text changes. The existing static review board remains representative for mobile, tablet, and desktop viewports.

## Evidence

Updated artifact: `docs/visual-reviews/assets/lmspeed-key-speedtest/updated-static-use-key-modal-lmspeed.png`.

Automated evidence:

```bash
pnpm --dir frontend exec vitest run src/utils/__tests__/lmspeed.spec.ts src/components/keys/__tests__/UseKeyModal.spec.ts
```

Result: targeted LMSpeed tests passed with 2 files and 28 tests.

## Residual Risk

The static board is not a live browser screenshot. Final browser acceptance should open the Use API Key dialog in both Chinese and English locales and confirm the LMSpeed page target language follows the app language.
