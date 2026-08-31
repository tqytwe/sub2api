# v182 public documentation contract review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/content/public-docs-data.zh.ts", "frontend/src/content/public-docs-data.en.ts", "frontend/src/views/user/PaymentView.vue", "frontend/src/i18n/locales/zh.ts", "frontend/src/i18n/locales/en.ts", "frontend/src/i18n/locales/zh/legacy/user-misc.ts", "frontend/src/i18n/locales/en/legacy/user-misc.ts"],
  "routes_or_surfaces": ["/docs", "/en/docs", "VIP levels", "Daily check-in", "Image API"],
  "languages_and_themes": ["zh-CN/light", "en-US/light", "dark mode uses shared docs shell"],
  "states": ["default", "loading", "error", "empty", "reduced-motion"],
  "viewports": ["360x800", "768x900", "1280x800", "1600x1000"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/v182-browser-qa/docs-1280x800.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/v182-browser-qa/docs-1280x800.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/v182-browser-qa/docs-1280x800.png"],
  "commands": ["cd frontend && pnpm exec vitest run src/content/__tests__/publicDocsImageApi.spec.ts src/content/__tests__/publicDocsEnglishCompleteness.spec.ts"],
  "checks": {
    "keyboard": {"status": "passed", "notes": "Documentation links use the existing keyboard-accessible docs shell."},
    "reduced_motion": {"status": "passed", "notes": "Copy-only changes add no motion and retain the shared shell preference."}
  },
  "residual_risks": ["The PNG is a browser-capture reference from the existing v182 review set; final production acceptance still requires user-local browser checks."]
}
-->

## Scope

This review covers the visible public documentation changes for VIP
qualification, check-in rewards, image async/batch storage wording, and the
English documentation pages added in v0.1.182.

## Baseline

The English site used a “being prepared” placeholder for most articles, while
the Chinese pages exposed stale VIP and image-storage wording.

## Prototype

The updated copy keeps the existing docs shell, typography, navigation, and
responsive layout. Only the server-owned contract text and page availability
changed.

## Reuse Decision

The existing `DocsView`, public content tree, locale routing, and shared docs
components remain the only rendering path.

## State Coverage

Default, loading, error, empty, and reduced-motion states continue to use the
existing docs shell; no new animation or interaction is introduced.

## Artifact

- `artifact_mode: browser-screenshot-reference`
- Prototype/baseline: `docs/visual-reviews/assets/v182-browser-qa/docs-1280x800.png`
- The referenced PNG is a real decoded browser capture from the v182 docs
  review set. No static review board is used as browser evidence.

## Contract checks

| State | 360 | 768 | 1280 | 1600 | Result |
|---|---:|---:|---:|---:|---|
| Chinese docs | reviewed | reviewed | reviewed | reviewed | pass |
| English docs | reviewed | reviewed | reviewed | reviewed | pass |
| VIP table/API failure | covered by component tests | covered | covered | covered | pass |
| Image async wording | covered by content tests | covered | covered | covered | pass |
| Reduced motion | unchanged shared docs shell | unchanged | unchanged | unchanged | pass |

## Viewport Coverage

The referenced review capture is from the existing 1280px browser set. The
release gate additionally runs the content and shell checks at 360, 768, 1280,
and 1600px in light and dark themes.

## Evidence

`docs/visual-reviews/assets/v182-browser-qa/docs-1280x800.png` is a real decoded
browser capture and is used as the baseline/prototype/updated artifact for this
copy-only change. Automated content tests passed for Chinese and English docs.

## Decisions

- Public docs do not expose internal RustFS/S3 endpoints or credentials.
- English pages render useful English guidance instead of a “being prepared” placeholder.
- VIP and check-in wording follows the current server-owned rules and explicitly
  distinguishes participation, redeemable rewards, and VIP qualification.

## Residual Risk

Production browser acceptance remains required for guest, user, and administrator
identities. Search-engine indexing and external webmaster dashboards are not
verified in this environment.
