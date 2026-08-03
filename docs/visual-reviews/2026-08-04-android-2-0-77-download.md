# Android 2.0.77 Download Release Visual Review

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
    "docs/visual-reviews/assets/android-2.0.77-release/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/android-2.0.77-release/baseline-390.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/android-2.0.77-release/updated-1280.png"
  ],
  "commands": [
    "firefox --headless --screenshot docs/visual-reviews/assets/android-2.0.77-release/*.png file://.../release-review-board.html",
    "sha256sum frontend/public/downloads/jisudengchat-android.apk",
    "pnpm --dir frontend design:check",
    "post-deployment curl https://www.jisudeng.com/downloads/android-version.json",
    "post-deployment sha256sum downloaded production APK"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "This static release keeps existing download controls and fallback links without adding an interaction."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "The release replaces package metadata and APK bytes only; it introduces no animation or motion."
    }
  },
  "residual_risks": [
    "The review-board images are static release evidence, not browser product screenshots.",
    "Live browser, manifest, APK checksum, installation, update-path, and administrator-login acceptance remain required."
  ]
}
-->

## Scope

- Existing public Android download route: `/download/android`.
- Static release files: `/downloads/android-version.json` and `/downloads/jisudengchat-android.apk`.
- Audience: Android users installing or updating `com.jisudeng.chat`.

## Baseline

Production advertised `2.0.74 / 274`. The previous package was signed correctly, but its embedded web configuration did not provide the explicit Android release metadata required to prevent a web-bundle version from being presented as the installed APK version.

The existing `PublicContentLayout`, download button, QR code, package metadata table, responsive layout, and localized copy remain unchanged.

## Prototype

The review board records the intended transition to `2.0.77 / 277`, including package identity, byte size, checksum, native metadata, embedded web configuration, signing, and monotonic version-code validation. It is deliberately a static release review board, not a fabricated product screenshot.

## Reuse Decision

Reuse the existing public download route and all controls. The release changes only the APK bytes, manifest fields, and manifest-failure fallback URL. No layout, icon, user-facing copy hierarchy, or interaction pattern is introduced.

## State Coverage

The record covers the production baseline manifest, successful manifest load, manifest-failure fallback, versioned APK URL, and QR-code URL. Keyboard behavior and reduced motion are unchanged because no controls or animation are changed.

## Viewport Coverage

The `390x844` board records compact evidence and the `1280x820` board records wide evidence. The public page layout itself is unchanged; final live browser validation remains required.

## Evidence

The release manifest, APK bytes, native package metadata, embedded web configuration, checksum, signing, and absence of a bundled download manifest were validated before this static asset synchronization. The package is `com.jisudeng.chat`, version `2.0.77`, version code `277`, with SHA-256 `70197d55918ce4eda5f878135edb3fa7f1f56d1b57ca6f1817acf460ab071cc6`.

## Residual Risk

The static review board is not a substitute for production browser or Android-device acceptance. After deployment, the live manifest and downloaded checksum must match, then an installed `2.0.74` or `2.0.76` device must confirm the upgrade and an administrator must complete login plus TOTP to verify capability-controlled access.
