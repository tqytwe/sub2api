# Visual review: ranxi update policy

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/components/common/VersionBadge.vue",
    "frontend/src/i18n/locales/en/legacy/core.ts",
    "frontend/src/i18n/locales/zh/legacy/core.ts",
    "frontend/src/i18n/locales/en/misc.ts",
    "frontend/src/i18n/locales/zh/misc.ts"
  ],
  "routes_or_surfaces": [
    "Administrator VersionBadge in the existing sidebar/header"
  ],
  "languages_and_themes": [
    "zh/light",
    "zh/dark",
    "en/light",
    "en/dark"
  ],
  "states": [
    "current pin",
    "new upstream release",
    "loading",
    "error/cache warning",
    "source build",
    "release build",
    "non-admin",
    "hover",
    "keyboard focus",
    "Escape"
  ],
  "viewports": [
    "360x900",
    "768x900",
    "1280x900",
    "1920x900"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/ranxi-update-policy/prototype-1280-zh-light.png",
    "docs/visual-reviews/assets/ranxi-update-policy/prototype-360-en-dark.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/ranxi-update-policy/baseline-1280-en-dark.png",
    "docs/visual-reviews/assets/ranxi-update-policy/baseline-1280-en-light.png",
    "docs/visual-reviews/assets/ranxi-update-policy/baseline-1280-zh-dark.png",
    "docs/visual-reviews/assets/ranxi-update-policy/baseline-1280-zh-light.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/ranxi-update-policy/updated-1280-en-dark.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-1280-en-light.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-1280-zh-dark.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-1280-zh-light.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-1920-en-dark.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-1920-en-light.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-1920-zh-dark.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-1920-zh-light.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-360-en-dark.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-360-en-light.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-360-zh-dark.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-360-zh-light.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-768-en-dark.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-768-en-light.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-768-zh-dark.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-768-zh-light.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-loading-360-en-light.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-loading-360-zh-light.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-new-upstream-360-en-light.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-new-upstream-360-zh-light.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-warning-360-en-light.png",
    "docs/visual-reviews/assets/ranxi-update-policy/updated-warning-360-zh-light.png"
  ],
  "commands": [
    "Isolated Vue component compiled with Vite, Chromium Playwright page.setContent; mocked store and no HTTP application service",
    "pnpm --dir frontend exec vitest run src/components/common/__tests__/VersionBadge.spec.ts src/stores/__tests__/app.spec.ts",
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend lint:check",
    "pnpm --dir frontend typecheck"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "notes": "Native buttons/links; refresh has accessible name; Escape closes and restores trigger focus. Component tests cover Escape."
    },
    "reduced_motion": {
      "status": "passed",
      "notes": "No added animation. Removed unsafe update/restart states and continuous badge ping; fixture uses reduced-motion."
    }
  },
  "residual_risks": [
    "Captures are isolated Vue fixtures with mocked state, not an authenticated full application or production acceptance. User local-browser acceptance remains pending; no service, real account, production database or production setting was accessed."
  ]
}
-->

## Scope

Only the existing VersionBadge update/rollback surface changes. The user authorized preserving the fork style while separating upstream discovery from verified fork installation. No mobile app, Canvas, TPS, routing, billing or migrations.

## Baseline

The actual original VersionBadge was compiled as an isolated Vue fixture and inspected in Chromium before editing. A release build offered an upstream binary replacement. The fixture used non-sensitive version data and no backend service.

## Prototype

Prototype images were captured and inspected before copying the revised component into the repository. Existing badge, compact dropdown, border, typography, colors and Icon registry are reused. The authorized boundary is source-policy copy and removal of unsafe installation/rollback controls, with no page redesign.

## Reuse Decision

Inspected VersionBadge, the adjacent BaseDialog implementation, Icon and app store. Kept the existing dropdown structure and native controls; no new layout or icon system. Removed obsolete terminal commands and install/restart panels because the backend now rejects these actions.

## State Coverage

Component tests cover source/release builds, no/new upstream releases, query error, loading with disabled refresh, non-admin access, foreign cached link rejection and Escape. The guide is visible regardless of upstream update availability. No success/restart state applies because the interface does not initiate binary mutation. Hover, active and focus use existing tokens/native controls. Both actual runtime locale fragments and legacy misc fragments agree.

## Viewport Coverage

Chromium captures cover 360, 768, 1280 and 1920 pixels, Chinese/English and light/dark. Script checked document overflow at each size; none detected. The dropdown retains compact dimensions. Integrated 200% zoom and production placement remain final browser acceptance work.

## Evidence

Artifacts are real Chromium screenshots of the isolated component with mocked stores. They are not production screenshots. The source build link points to deploy/FORK_SOURCE_BUILD.md; ranxi links are constrained to the stable release namespace. No installer, Docker image or restart action is emitted by the component.

## Residual Risk

Final local browser acceptance by the user is pending and must accompany coordinated deployment. These captures cannot prove authenticated shell placement or deployment success.
