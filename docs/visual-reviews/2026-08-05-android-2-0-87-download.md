# Android 2.0.87 Download Release Visual Review

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

The prior release was `2.0.86 / 286`. The existing download page uses
`PublicContentLayout`, shared buttons and icons, a manifest-driven QR code,
and localized package metadata. No visual layout, copy, icon or interaction
changes are introduced in this release.

## Prototype

The reviewed prototype and static board from the unchanged `2.0.86` download
surface are retained because this change only replaces release metadata. The
new contract is that both the manifest and the hard-coded fallback URL carry
the exact content-addressed key for the APK bytes.

## Reuse Decision

The release reuses the existing public route, `PublicContentLayout`, shared
controls, QR generator and bilingual copy. No new visual system or interaction
is added: only the package bytes, manifest values and fallback cache key move
from `2.0.86 / 286` to `2.0.87 / 287`.

## State Coverage

Manifest-loaded and manifest-failure fallback states both resolve the same
content-addressed APK URL. The QR code receives that URL, so a scan cannot
silently fall back to a stale, same-name APK. Keyboard, focus-visible and
reduced-motion behavior remain supplied by the unchanged shared controls.

## Viewport Coverage

The retained review artifacts cover `390x844` and `1280x820`. Since there is
no layout change, no new breakpoint behavior is introduced; final live browser
checks remain part of the production rollout.

## Evidence

The signed APK is `com.jisudeng.chat`, version `2.0.87`, version code `287`,
size `7,533,669` bytes and SHA-256
`86bd20988ed232c3b7b87a3c0a360eb8a5928518494c322173058acc21e94392`.
The signing certificate remains the released certificate. The canonical file
name is unchanged; the manifest and manifest-failure fallback now share
`?v=2.0.87-287-86bd20988ed232c3b7b87a3c0a360eb8a5928518494c322173058acc21e94392`
so an edge cache cannot silently serve the prior package.

## Residual Risk

This record does not replace deployment validation. After production rollout,
the live manifest, downloaded hash, Android overlay upgrade with preserved
local data, and an administrator TOTP-protected action must be checked on a
real device.
