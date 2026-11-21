# Visual review: Admin version status

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/common/VersionBadge.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/misc.ts",
    "frontend/src/i18n/locales/en/legacy/core.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/misc.ts",
    "frontend/src/i18n/locales/zh/legacy/core.ts"
  ],
  "routes_or_surfaces": [
    "Admin sidebar version badge"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "en-US/dark"
  ],
  "states": [
    "closed",
    "partial",
    "complete",
    "loading",
    "disabled",
    "error",
    "focus-visible"
  ],
  "viewports": [
    "360x800",
    "768x800",
    "1280x800",
    "1920x900"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/v2231-admin-version/prototype.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/v2231-admin-version/baseline.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/v2231-admin-version/desktop.png",
    "docs/visual-reviews/assets/v2231-admin-version/mobile.png",
    "docs/visual-reviews/assets/v2231-admin-version/tablet-offline.png",
    "docs/visual-reviews/assets/v2231-admin-version/wide-loading.png"
  ],
  "commands": [
    "Vitest mounted VersionBadge with mock store and serialized the rendered markup",
    "Tailwind CLI compiled the existing utility classes",
    "Chromium headless screenshot of the rendered fixture at each viewport"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "notes": "Component tests verify Escape closes the popup and the loading state disables refresh. Existing focus styles are reused."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "The patch adds only text and does not introduce motion."
    }
  },
  "residual_risks": [
    "Browser evidence uses the actual component markup in an isolated fixture. The authenticated production sidebar still requires user browser acceptance."
  ]
}
-->

## Scope

The administrator sidebar version popup in Chinese and English.

## Baseline

The submitted admin screenshot shows the Fork build, the latest upstream release and the old pinned source.

## Prototype

The prototype adds a sync status line to the existing popup. The scope is limited to version and migration information.

## Reuse Decision

The popup, refresh control, theme tokens and navigation links retain their existing components and classes.

## State Coverage

The refresh button retains its hover and focus-visible styles. Tests cover successful and failed lookups, unavailable data on older servers, refresh disabled during loading, explicit completion, Escape, role and cache behavior.

## Viewport Coverage

Desktop and mobile captures show the partial state. The tablet capture shows a lookup error with the retained source target. The wide capture shows an in-progress refresh.

## Evidence

The manifest records Chromium screenshots from rendered component markup with mocked version data and the existing Tailwind utilities.

## Residual Risk

A production admin should verify the values in the local browser alongside guest and regular-user acceptance.
