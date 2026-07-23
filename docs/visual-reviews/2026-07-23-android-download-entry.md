# Android Download Entry Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/i18n/locales/jisudeng-home.en.ts",
    "frontend/src/i18n/locales/jisudeng-home.zh.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/router/index.ts",
    "frontend/src/router/publicNavigation.ts",
    "frontend/src/styles/home-view.css",
    "frontend/src/views/HomeView.vue",
    "frontend/src/views/public/AndroidDownloadView.vue",
    "frontend/public/downloads/android-version.json",
    "frontend/public/downloads/jisudengchat-android.apk"
  ],
  "routes_or_surfaces": [
    "public root home route",
    "public home route",
    "Android download route",
    "mobile home sticky CTA",
    "public home navigation",
    "APK and version static downloads"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "zh-CN/dark",
    "en-US/light",
    "en-US/dark"
  ],
  "states": [
    "default",
    "hover",
    "focus-visible",
    "manifest loading",
    "manifest failure",
    "APK download link",
    "QR code ready",
    "2.0.1 login-network hotfix manifest"
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
    "docs/visual-reviews/assets/android-download-entry/baseline-android-download-entry.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/android-download-entry/updated-android-download-entry.png"
  ],
  "commands": [
    "python3 generated static Android download review boards with PIL",
    "corepack yarn test:ci test/managed-nextchat-request.test.ts",
    "corepack yarn android:export",
    "npx cap sync android",
    "./gradlew assembleRelease",
    "corepack yarn android:package",
    "sha256sum frontend/public/downloads/jisudengchat-android.apk",
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend build",
    "git diff --check"
  ],
  "checks": {
    "keyboard": {
      "status": "passed"
    },
    "reduced_motion": {
      "status": "passed"
    }
  },
  "residual_risks": [
    "Static review boards do not replace deployed browser screenshots or user acceptance on the real device.",
    "The production deployment must still verify APK bytes, manifest JSON, homepage entry visibility, and CORS preflight on www.jisudeng.com and api.jisudeng.com."
  ]
}
-->

## Scope

- Routes: `/`, `/home`, `/download/android`.
- Roles: public guest, authenticated user header, Android app user opening the official download page.
- Languages and themes: Chinese and English copy, light and dark theme tokens.

## Baseline

The live public homepage did not expose a clear Android download action in the first screen, and `/download/android` was not backed by the new JisudengChat APK page. Users could install a local APK during development, but visitors on `www.jisudeng.com` could not reliably find or download it.

Baseline artifact: `docs/visual-reviews/assets/android-download-entry/baseline-android-download-entry.png`.

## Prototype

The prototype adds a compact `下载 APP` action in the public homepage header, a two-button mobile sticky CTA for download plus registration, and a dedicated Android page with APK download, web fallback, QR code, package metadata, SHA256, and release notes.

Prototype artifact: `docs/visual-reviews/assets/android-download-entry/prototype-android-download-entry.png`.

Approval status: user requested immediate full implementation and deployment for Android-only Web plus APP distribution.

## Reuse Decision

The implementation reuses `PublicContentLayout`, the shared `Icon` component, existing public-home navigation, route SEO utilities, and existing i18n locale files. No new shell ownership pattern is introduced in the route view.

## State Coverage

Default state shows direct APK download and QR scan entry. Hover and focus use existing color and focus behavior from links and buttons. Manifest loading shows a stable QR placeholder; manifest failure keeps the official APK link available and shows a localized warning. Dark mode uses the existing dark surface, border, and text utilities.

## Viewport Coverage

Mobile coverage uses a 390px review board with the sticky download/register strip. Desktop coverage uses a 1280px homepage header and the `/download/android` content layout. The route view avoids viewport-scaled fonts and does not own global page width or scrolling.

## Evidence

Updated artifact: `docs/visual-reviews/assets/android-download-entry/updated-android-download-entry.png`.

The 2.0.1 hotfix keeps the same download-page layout and updates only the static Android package plus version manifest. The manifest now advertises `version=2.0.1`, `versionCode=20001`, and APK SHA256 `1b56cdd1420884e6bca2628c61e5cefeea46d98c6a16a955cb8b1e9b6b6010c5`. The bundled APK index still fixes `managedBackendBaseUrl` to `https://api.jisudeng.com` and `nextchatWebUrl` to `https://www.jisudeng.com`.

Automated evidence is expected from design governance, typecheck, tests, production build, APK packaging, checksum verification, and live deployment probes.

Commands run or queued:

```bash
python3 generated static Android download review boards with PIL
corepack yarn test:ci test/managed-nextchat-request.test.ts
corepack yarn android:export
npx cap sync android
./gradlew assembleRelease
corepack yarn android:package
sha256sum frontend/public/downloads/jisudengchat-android.apk
pnpm --dir frontend design:check
pnpm --dir frontend typecheck
pnpm --dir frontend build
git diff --check
```

## Residual Risk

These artifacts are static review boards, not browser captures from production. After deployment, final acceptance still needs live checks for `https://www.jisudeng.com/download/android`, `https://www.jisudeng.com/downloads/android-version.json`, the APK byte header, the homepage download button, and Android login against `https://api.jisudeng.com`.
