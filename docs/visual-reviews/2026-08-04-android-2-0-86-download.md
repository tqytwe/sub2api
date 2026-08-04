# Android 2.0.86 Download Release Visual Review

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
    "versioned APK download URL",
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
    "docs/visual-reviews/assets/android-2.0.77-release/baseline-390.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/android-2.0.86-release/updated-1280.png"
  ],
  "commands": [
    "firefox --headless --window-size 1280,820 --screenshot docs/visual-reviews/assets/android-2.0.86-release/prototype-1280.png file://.../release-review-board.html",
    "sha256sum frontend/public/downloads/jisudengchat-android.apk",
    "pnpm --dir frontend design:check",
    "post-deployment curl https://www.jisudeng.com/downloads/android-version.json",
    "post-deployment sha256sum downloaded production APK"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "The existing controls and focus behavior are unchanged."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "The release changes metadata, bytes, and a fallback cache key only."
    }
  },
  "residual_risks": [
    "Static review board evidence is not a browser product capture.",
    "Live manifest, APK checksum, installation, overlay upgrade, and administrator TOTP acceptance remain required."
  ]
}
-->

## Scope

- Existing public Android download route: `/download/android`.
- Static release files: `/downloads/android-version.json` and `/downloads/jisudengchat-android.apk`.
- Audience: Android users installing or updating `com.jisudeng.chat`.

## Baseline

Production advertises `2.0.77 / 277`. Its cache-safe fallback URL therefore still points to the previous CDN cache key when the manifest request fails.

The existing `PublicContentLayout`, controls, QR code, package metadata table, responsive layout, and localized copy remain unchanged.

## Prototype

The review board records the intended `2.0.86 / 286` static artifact transition: package identity, byte size, checksum, signing continuity, and the `?v=2.0.86-286-r1` cache key shared by the manifest and fallback URL. It is a release metadata board, not a fabricated product screenshot.

## Reuse Decision

Reuse the existing public route, buttons, icons, layout, and localization. The only source behavior change is the manifest-failure fallback URL's cache key (`?v=2.0.86-286-r1`). No layout, hierarchy, copy, icon, or interaction pattern is introduced.

## State Coverage

The record covers manifest loaded and failed states, the versioned download URL, and QR-code input. Keyboard and reduced-motion behavior are unchanged because no control or animation changes.

## Viewport Coverage

The previous `390x844` baseline is retained and the `1280x820` release board records the updated metadata. The public page layout itself is unchanged; final live browser validation remains required.

## Evidence

The signed APK is `com.jisudeng.chat`, version `2.0.86`, version code `286`, size `7,532,618` bytes, SHA-256 `87e36b169e2ea0568445c270999c0c0b1c58016a21c1b422dd13bec0e0c60613`, and uses the already released signing certificate. The canonical download file remains unchanged; only its versioned cache query changes to `?v=2.0.86-286-r1` so stale CDN objects cannot be selected.

## Residual Risk

This static board is not a substitute for production browser or Android-device acceptance. After deployment, the live manifest and downloaded checksum must match, and an installed prior version must confirm upgrade with preserved chat, project, and local-image state. Administrator login and a TOTP-protected action still require authenticated production acceptance.
