# Android 2.0.36 Download Release Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/public/downloads/android-version.json",
    "frontend/public/downloads/jisudengchat-android.apk",
    "frontend/src/router/__tests__/androidDownloadRouting.spec.ts",
    "frontend/src/views/public/AndroidDownloadView.vue"
  ],
  "routes_or_surfaces": [
    "/download/android",
    "/downloads/android-version.json",
    "/downloads/jisudengchat-android.apk"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "zh-CN/dark",
    "en-US/light",
    "en-US/dark"
  ],
  "states": [
    "download page default",
    "manifest loaded",
    "manifest failure fallback",
    "APK link with cache-busted URL",
    "QR code using current APK URL"
  ],
  "viewports": [
    "390x844",
    "1280x820"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/android-download-entry/prototype-android-download-entry.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/mobile-app-feedback-download/baseline-android-download-entry.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/mobile-app-feedback-download/updated-android-download-entry.png"
  ],
  "commands": [
    "sha256sum frontend/public/downloads/jisudengchat-android.apk",
    "git diff --check",
    "./scripts/check-fork-integrity.sh",
    "make test",
    "make build",
    "post-deploy curl https://www.jisudeng.com/downloads/android-version.json",
    "post-deploy sha256sum downloaded production APK"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "The release changes keep the existing download and web fallback links; no new interaction pattern is introduced."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "The release changes update package metadata and static APK bytes only; no new motion is introduced."
    }
  },
  "residual_risks": [
    "Static review boards are reused because this change does not alter layout or copy hierarchy.",
    "Final acceptance still requires deployed manifest, APK checksum and Android installation verification."
  ]
}
-->

## Scope

- Public Android download route: `/download/android`.
- Static release files: `/downloads/android-version.json` and `/downloads/jisudengchat-android.apk`.
- Audience: Android users updating from an already installed `com.jisudeng.chat` package.

## Baseline

The production static download directory still pointed at the older `2.0.18-predeploy-fixes / 218` APK. The visible download page layout was already reviewed, but its fallback APK URL and manifest metadata could continue to route users to stale package bytes.

## Prototype

This release follows the already approved Android download entry prototype. No new layout, hierarchy, navigation position, control style or language treatment is introduced. The intended user-visible outcome is that the existing APK button, QR code and package information render the current `2.0.36 / 236` release instead of the stale package.

## Reuse Decision

This release keeps the existing `PublicContentLayout`, download button, QR code, package metadata table and responsive layout. The only visible change is the package version, release notes, APK size and checksum rendered from the refreshed manifest.

## State Coverage

The manifest-loaded state now advertises `version=2.0.36`, `versionCode=236`, APK SHA256 `4ae903fc3266dcabb9f76b2f1c2a2cfeb14f29c9da9a47802d9195598df501f0`, package `com.jisudeng.chat`, signing certificate SHA256 `cd7abbd79daf6648a429ff34d7450b18cfb6b416e660b2f5169178e0a488627e`, and cache-busted APK URL `/downloads/jisudengchat-android.apk?v=2.0.36-236`.

The manifest-failure state keeps the same fallback download URL, also cache-busted to `2.0.36-236`, so the primary button and QR code do not fall back to the old APK.

## Viewport Coverage

The reused review board covers the existing mobile and desktop route layout. Because the release only changes manifest-driven text and static APK bytes, the responsive behavior remains the reviewed two-column desktop layout and single-column mobile layout. The SHA256 field remains in the existing `break-all` treatment so the longer package metadata cannot overflow.

## Evidence

Reused updated artifact: `docs/visual-reviews/assets/mobile-app-feedback-download/updated-android-download-entry.png`.

Local checksum verification confirmed the tracked APK matches the manifest SHA256:

```text
4ae903fc3266dcabb9f76b2f1c2a2cfeb14f29c9da9a47802d9195598df501f0  frontend/public/downloads/jisudengchat-android.apk
```

## Residual Risk

The CDN and deployment platform must publish the refreshed backend static directory before users see the new APK. Post-deploy checks must verify the live manifest, live APK content length and live APK SHA256.
