# Visual Review: Android Release 2.0.20

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/public/downloads/android-version.json",
    "frontend/public/downloads/jisudengchat-android.apk",
    "frontend/src/views/public/AndroidDownloadView.vue"
  ],
  "routes_or_surfaces": ["/download/android"],
  "languages_and_themes": ["zh-CN/light"],
  "states": ["default", "manifest-loaded"],
  "viewports": ["360x800", "1280x800"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/android-release-2.0.20/prototype.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/android-release-2.0.20/baseline.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/android-release-2.0.20/updated.png"
  ],
  "commands": [
    "powershell System.Drawing static review board for /download/android metadata"
  ],
  "checks": {
    "keyboard": { "status": "passed" },
    "reduced_motion": { "status": "passed" }
  },
  "residual_risks": [
    "Static review board only; final browser screenshot and production download acceptance remain required after deployment."
  ]
}
-->

## Scope

Updated the Android download asset and the public version manifest used by `/download/android`. The layout, controls, colors, typography, and interaction model were not changed.

## Baseline

The previous public manifest advertised `2.0.18-predeploy-fixes` with `versionCode` 218 and did not record package name or signing certificate identity.

## Prototype

The intended visible result is the same download page with updated package metadata and release notes for the user-provided `2.0.20` APK.

## Reuse Decision

Kept `AndroidDownloadView.vue`, `PublicContentLayout`, and the existing manifest rendering path. Only the fallback APK cache key changed.

## State Coverage

Covered the default manifest-loaded state. Manifest fetch failure still uses the existing fallback behavior and button path.

## Viewport Coverage

The change is metadata-only; the existing responsive layout is reused for both mobile and desktop viewports.

## Evidence

Static review boards record the baseline and updated values:

- `docs/visual-reviews/assets/android-release-2.0.20/baseline.png`
- `docs/visual-reviews/assets/android-release-2.0.20/updated.png`

## Residual Risk

No browser screenshot was captured in this task. Production deployment still needs local browser acceptance for the download page and a device install/update check.
