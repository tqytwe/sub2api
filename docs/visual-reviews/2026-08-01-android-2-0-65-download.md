# Android 2.0.65 Download Release Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/public/downloads/android-version.json",
    "frontend/public/downloads/jisudengchat-android.apk"
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
    "production baseline download page",
    "manifest loaded",
    "manifest failure fallback",
    "versioned APK download URL",
    "QR code using current APK URL"
  ],
  "viewports": [
    "390x844",
    "1280x820"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/android-2.0.65-release/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/android-2.0.65-release/baseline-390.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/android-2.0.65-release/updated-1280.png"
  ],
  "commands": [
    "sha256sum frontend/public/downloads/jisudengchat-android.apk",
    "pnpm --dir frontend design:check",
    "./scripts/check-fork-integrity.sh",
    "post-deployment curl https://www.jisudeng.com/downloads/android-version.json",
    "post-deployment sha256sum downloaded production APK"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "This static release keeps the existing download controls and fallback link without adding an interaction."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "The release replaces package metadata and APK bytes only; it introduces no animation or motion."
    }
  },
  "residual_risks": [
    "The prototype and updated artifacts are a static release review board, not browser product screenshots.",
    "Live post-deployment browser, manifest, APK checksum, installation and update-path acceptance remain required."
  ]
}
-->

## Scope

- Existing public Android download route: `/download/android`.
- Static release files: `/downloads/android-version.json` and `/downloads/jisudengchat-android.apk`.
- Audience: Android users installing or updating `com.jisudeng.chat`.

## Baseline And Reuse

The production baseline manifest was read from `/downloads/android-version.json` while it still advertised `2.0.49 / 249`. The baseline image is a static release review board that records those live fields, because this release does not change the existing download-page layout. This release preserves the existing `PublicContentLayout`, download button, QR code, package metadata table, responsive layout and localized copy. No page hierarchy, navigation or interaction changes.

## Prototype

The prototype artifact records the intended manifest-driven package transition from `2.0.49 / 249` to `2.0.65 / 265`, including the package name, byte size, signing result and SHA-256. It is explicitly a static release review board, not a fabricated browser-product capture.

## Reuse Decision

Reuse the existing public download route and all its controls. The release updates only the APK bytes and manifest data returned to the existing page, including the versioned APK URL used by its download button and QR code. No component, layout, icon, copy hierarchy or interaction is introduced.

## Release State

The refreshed manifest advertises `version=2.0.65`, `versionCode=265`, package `com.jisudeng.chat`, size `7,456,605` bytes and APK SHA-256 `18ac2104df53176d03058f5eb9ff752b70830fd3a2b37bfbc29b8f5ed189ec33`. The APK URL is versioned as `/downloads/jisudengchat-android.apk?v=2.0.65-265` so browser and CDN caches cannot reuse the old bytes.

The existing manifest-failure fallback remains unchanged. No layout needs a new browser capture because all user-facing structure and copy hierarchy are reused; the desktop prototype and updated images are deliberately labelled static review boards rather than product screenshots.

## State Coverage

The record covers the production baseline manifest, successful manifest load, existing manifest-failure fallback, versioned APK URL and QR-code download URL. The download button remains the existing control, so keyboard and reduced-motion behavior are unchanged.

## Viewport Coverage

The 390x844 artifact confirms the compact presentation of the release fields. The 1280x820 prototype and updated artifacts cover the wide review board. Since the public page layout is unchanged, its existing mobile and desktop responsive behavior is reused; live browser validation remains a release acceptance requirement.

## Evidence

Local release validation confirmed the published file and manifest checksum agree:

```text
18ac2104df53176d03058f5eb9ff752b70830fd3a2b37bfbc29b8f5ed189ec33  frontend/public/downloads/jisudengchat-android.apk
```

The signed APK was previously verified as `com.jisudeng.chat`, `2.0.65 / 265`, using the existing release signing key and APK Signature Scheme v1 and v2.

## Residual Risk

The static assets still need normal Sub2API deployment. After merge, acceptance must verify the live manifest fields, download the public APK to compare its checksum, and install/update it on Android. The review-board images are not a substitute for those production checks.
