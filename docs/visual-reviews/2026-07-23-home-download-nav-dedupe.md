# Home Download Navigation Dedupe Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/router/publicNavigation.ts",
    "frontend/src/router/__tests__/androidDownloadRouting.spec.ts"
  ],
  "routes_or_surfaces": [
    "public root home route",
    "public home navigation",
    "desktop header CTA cluster",
    "mobile sticky CTA"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "zh-CN/dark",
    "en-US/light",
    "en-US/dark"
  ],
  "states": [
    "guest header default",
    "authenticated header default",
    "download CTA visible",
    "primary nav without duplicate download item",
    "mobile sticky download action retained"
  ],
  "viewports": [
    "390x844",
    "1280x720"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/home-download-nav-dedupe/prototype-home-download-nav-single-entry.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/home-download-nav-dedupe/baseline-home-download-nav-duplicate.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/home-download-nav-dedupe/updated-home-download-nav-single-entry.png"
  ],
  "commands": [
    "python3 generated static home download navigation review boards with PIL",
    "pnpm --dir frontend test:run src/router/__tests__/androidDownloadRouting.spec.ts",
    "pnpm --dir frontend build",
    "git diff --check"
  ],
  "checks": {
    "keyboard": {
      "status": "passed"
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "The change removes one navigation item and adds no animation."
    }
  },
  "residual_risks": [
    "Static review boards do not replace a final production browser screenshot.",
    "After deployment, the public homepage should be checked on www.jisudeng.com to confirm the header has only one download action."
  ]
}
-->

## Scope

This review covers the public homepage header after adding Android app distribution. The Android download route remains available, but the first desktop header row must not show both a text navigation item and a CTA button for the same download action.

## Baseline

The live header showed `下载 APP` inside the primary navigation and another `下载 APP` button in the right-side action cluster. That duplicated one destination and made the homepage header feel crowded.

Baseline artifact: `docs/visual-reviews/assets/home-download-nav-dedupe/baseline-home-download-nav-duplicate.png`.

## Prototype

The approved correction keeps the right-side download CTA as the single desktop header entry. The primary navigation continues to focus on content and product sections: models, docs, AI creation, prompt square, API keys, about, and contact.

Prototype artifact: `docs/visual-reviews/assets/home-download-nav-dedupe/prototype-home-download-nav-single-entry.png`.

## Reuse Decision

The implementation reuses the existing `nav-download` CTA in `HomeView.vue`, the existing `/download/android` route, and the existing mobile sticky download action. No new component or layout ownership is introduced.

## State Coverage

Guest and authenticated desktop headers keep one download entry in the CTA cluster. The Android route name remains exported for the header button and sticky mobile CTA. The removed navigation key cannot be reintroduced unnoticed because the route integration test now asserts that `androidApp` is absent from the primary navigation list.

## Viewport Coverage

Desktop coverage is represented at 1280px, where the duplicated header was visible. Mobile coverage remains structurally unchanged because mobile uses the sticky download CTA rather than the full desktop primary navigation row.

## Evidence

Updated artifact: `docs/visual-reviews/assets/home-download-nav-dedupe/updated-home-download-nav-single-entry.png`.

The fix removes only the primary navigation `androidApp` item. The download button, `/download/android` route, APK manifest, QR download page, and mobile sticky download action remain intact.

## Residual Risk

This record uses static review boards rather than a production browser capture. After deployment, the homepage should be opened on `https://www.jisudeng.com/` to confirm the header shows only one download action.
