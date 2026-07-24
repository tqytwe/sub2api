# Mobile App Feedback And Download Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/public/downloads/android-version.json",
    "frontend/public/downloads/jisudengchat-android.apk",
    "frontend/src/components/admin/payment/AdminOrderDetail.vue",
    "frontend/src/i18n/locales/en/admin/playOps.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/misc.ts",
    "frontend/src/i18n/locales/zh/admin/playOps.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/misc.ts",
    "frontend/src/types/payment.ts",
    "frontend/src/views/admin/PlayOpsView.vue",
    "frontend/src/views/admin/orders/AdminOrdersView.vue",
    "frontend/src/views/public/AndroidDownloadView.vue"
  ],
  "routes_or_surfaces": [
    "/download/android",
    "/admin/play",
    "/admin/orders",
    "JisudengChat Android app account feedback"
  ],
  "languages_and_themes": [
    "zh-CN light static review board",
    "zh-CN dark static review board",
    "en-US localized fallback strings",
    "Android follows system light and dark mode"
  ],
  "states": [
    "download page default",
    "download manifest loaded",
    "feedback panel closed",
    "feedback panel opened",
    "feedback list loading",
    "feedback list empty",
    "feedback detail selected",
    "feedback status update",
    "admin order failure reason visible",
    "Android app chat, image, account and settings tabs"
  ],
  "viewports": [
    "390x844",
    "768x900",
    "1280x800",
    "1600x3200"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/mobile-app-feedback-download/prototype-app-light.png",
    "docs/visual-reviews/assets/mobile-app-feedback-download/prototype-app-dark.png",
    "docs/visual-reviews/assets/mobile-app-feedback-download/prototype-android-download-entry.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/mobile-app-feedback-download/baseline-android-download-entry.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/mobile-app-feedback-download/updated-app-shell-light.png",
    "docs/visual-reviews/assets/mobile-app-feedback-download/updated-app-shell-dark.png",
    "docs/visual-reviews/assets/mobile-app-feedback-download/updated-android-download-entry.png"
  ],
  "commands": [
    "cp /home/dell/.codex/visualizations/2026/07/22/019f8b24-6162-7762-9e38-13ef96028856/prototype-light.png docs/visual-reviews/assets/mobile-app-feedback-download/prototype-app-light.png",
    "cp /home/dell/.codex/visualizations/2026/07/22/019f8b24-6162-7762-9e38-13ef96028856/prototype-dark.png docs/visual-reviews/assets/mobile-app-feedback-download/prototype-app-dark.png",
    "cp docs/visual-reviews/assets/android-download-entry/updated-android-download-entry.png docs/visual-reviews/assets/mobile-app-feedback-download/updated-android-download-entry.png",
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend build",
    "pnpm --dir frontend design:check"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "The web changes reuse existing buttons, inputs, selects and table rows; the feedback panel exposes direct button and row activation without hover-only actions."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "The Play Ops refresh icons no longer add continuous spin animation, and the Android app shell follows system theme without adding decorative motion."
    }
  },
  "residual_risks": [
    "The APP shell artifacts are approved static prototype boards, not live authenticated mobile screenshots.",
    "The download page artifact is reused static review evidence because the headless Firefox preview capture produced a blank page in this environment.",
    "Final acceptance still requires browser screenshot verification after deployment and Android installation testing on the user's phone."
  ]
}
-->

## Scope

- Routes: `/download/android`, `/admin/play`, `/admin/orders`.
- Roles: public visitor for APK download; administrator for Play Ops feedback and order failure reason review; signed-in Android user for feedback submission.
- Languages and themes: Chinese primary copy, English fallback strings, existing light and dark tokens, and Android app system theme following.

## Baseline

The download page already had a public APK route and QR/download controls, but its package metadata pointed at the older Android build. Play Ops did not expose APP feedback collection, and failed payment orders lacked an obvious failure reason in the administrator review surface.

The baseline artifact records the existing Android download entry layout before this version manifest update.

## Prototype

- APP light prototype: `docs/visual-reviews/assets/mobile-app-feedback-download/prototype-app-light.png`.
- APP dark prototype: `docs/visual-reviews/assets/mobile-app-feedback-download/prototype-app-dark.png`.
- Public download entry prototype: `docs/visual-reviews/assets/mobile-app-feedback-download/prototype-android-download-entry.png`.
- Approval status: the user required the APP implementation to follow the July 22 prototype and support night mode following the phone setting.
- Scope boundary: Android only, web plus Android download, no iOS/macOS/Windows client.

## Reuse Decision

- Reuse `AppLayout`, the existing card/table/input/button treatments, `Icon`, locale namespaces and payment order detail components.
- Keep the download page on `PublicContentLayout` and update only its manifest fallback version URL.
- Keep APP feedback under Play Ops as a page-level panel opened by an explicit button instead of adding a new top-level admin module.
- No new route frame, nested page shell, raw color, large-radius or continuous-motion exception is introduced.

## State Coverage

- Default: Play Ops starts with the existing summary cards and feedback entry button.
- Feedback opened: administrators can filter by status, search by title/content/user/device, select a row and inspect device/app/error details.
- Empty and loading: the panel keeps existing disabled controls and table empty state without motion-dependent cues.
- Status update: administrators can mark feedback as viewed, handled, deferred or ignored and save an internal note.
- Payment order failure: admin order list and order detail expose `failed_reason` when the backend recorded one.
- Download manifest: `/downloads/android-version.json` points to `2.0.18-predeploy-fixes` with the release APK hash and cache-busted URL.

## Viewport Coverage

- Mobile: the APP prototype covers the bottom tab shell, chat/image/account/settings flow and system light/dark themes.
- Tablet: Play Ops controls wrap through existing flex/grid behavior.
- Desktop: Play Ops table and detail rail use the current operational page frame and avoid owning route width.
- Wide or short screen: download page stays in the existing public content frame; QR and package information remain in their current responsive bands.
- 200% zoom and reduced motion: no hover-only action is required, no new continuous animation remains after the refresh icon adjustment.

## Evidence

- Updated APP shell boards mirror the approved light/dark prototype.
- Updated download entry board records the public APK download affordance and QR entry pattern.
- Automated checks cover frontend typecheck/build, backend tests/build and fork integrity after the design-governance fixes.
- APK metadata was verified from the release build: `com.jisudeng.chat`, `versionCode=218`, `versionName=2.0.18-predeploy-fixes`, and signer `CN=JisudengChat`.

## Residual Risk

- Static review boards cannot prove the authenticated administrator panel in production.
- The environment's direct Firefox preview screenshot of `/download/android` returned blank output, so live browser screenshot proof remains a deployment verification task.
- Follow-up owner: engineering for post-deploy public route/API proof; user for final Android installation and phone-side acceptance.
