# Android 2.0.89 Download Release Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/public/downloads/android-version.json",
    "frontend/public/downloads/jisudengchat-android.apk",
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
    "manifest loaded",
    "manifest failure fallback",
    "content-addressed APK download URL",
    "QR code using current APK URL"
  ],
  "viewports": [
    "390x844",
    "1280x820"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/android-2.0.86-release/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/android-2.0.86-release/updated-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/android-2.0.86-release/updated-1280.png"
  ],
  "commands": [
    "sha256sum frontend/public/downloads/jisudengchat-android.apk",
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend typecheck",
    "post-deployment curl https://www.jisudeng.com/downloads/android-version.json",
    "post-deployment sha256sum downloaded production APK"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "Existing controls and focus behavior are unchanged."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "The release changes package bytes and release metadata only."
    }
  },
  "residual_risks": [
    "Static review board evidence is not a browser product capture.",
    "Live manifest, APK checksum, overlay upgrade, and authenticated administrator acceptance remain required after deployment."
  ]
}
-->

## Scope

- Existing public Android download route: `/download/android`.
- Static release files: `/downloads/android-version.json` and `/downloads/jisudengchat-android.apk`.
- Audience: Android users installing or upgrading `com.jisudeng.chat`.

## Baseline

The prior production release was `2.0.87 / 287`. The existing download page
continues to use `PublicContentLayout`, shared controls and icons, a
manifest-driven QR code, and localized package metadata. No layout, hierarchy,
copy, icon, or interaction change is introduced in this release.

## Prototype

The existing 2.0.86 static review board is retained as the rendered prototype
and baseline because this release only changes package metadata and bytes. It
is a release evidence board, not a fabricated browser capture.

## Reuse Decision

Reuse the existing public route, `PublicContentLayout`, buttons, icons, QR
generator, and bilingual copy. The only source behavior change is the
manifest-failure fallback cache key moving from 2.0.87 / 287 to 2.0.89 / 289.

## Viewport Coverage

The retained review artifacts cover `390x844` and `1280x820`. Since there is no
layout change, no new breakpoint behavior is introduced; final live browser
checks remain part of the production rollout.

## State Coverage

Manifest-loaded and manifest-failure fallback states both resolve the
content-addressed 2.0.89 APK URL. The QR code receives that URL, so a scan
cannot silently fall back to a stale same-name package. Keyboard, focus-visible
and reduced-motion behavior remain supplied by the unchanged shared controls.

## Evidence

The signed APK is `com.jisudeng.chat`, version `2.0.89`, version code `289`,
size `7,637,136` bytes, and SHA-256
`664b8287a5cfd370534a48d1531205786ed23186bb621808498e3bc76353d3fe`.
The signing certificate remains the released certificate. The manifest and
fallback use the same content-addressed URL:
`?v=2.0.89-289-664b8287a5cfd370534a48d1531205786ed23186bb621808498e3bc76353d3fe`.

## Residual Risk

This record does not replace deployment validation. After production rollout,
the live manifest, downloaded hash, Android overlay upgrade with preserved
local data, and an administrator TOTP-protected action must be checked on a
real device.
