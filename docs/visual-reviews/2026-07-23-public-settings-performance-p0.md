# Public Settings Performance P0 Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/App.vue",
    "frontend/src/components/admin/AdminComplianceDialog.vue",
    "frontend/src/components/layout/AuthLayout.vue"
  ],
  "routes_or_surfaces": [
    "global app shell",
    "login and register auth shell",
    "admin compliance dialog",
    "announcement popup"
  ],
  "languages_and_themes": [
    "zh-CN light static board",
    "zh-CN dark static board",
    "en-US light static board",
    "en-US dark static board"
  ],
  "states": [
    "default first load",
    "settings fallback loading",
    "support contact QR image load",
    "admin compliance required",
    "announcement popup visible"
  ],
  "viewports": [
    "390x844",
    "1280x820",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/public-settings-performance-p0/prototype-public-settings-performance-p0.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/public-settings-performance-p0/baseline-public-settings-performance-p0.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/public-settings-performance-p0/updated-public-settings-performance-p0.png"
  ],
  "commands": [
    "python3 generated static performance review boards with PIL",
    "pnpm --dir frontend run build",
    "pnpm --dir frontend run lint:check",
    "go test -tags=unit -count=1 ./internal/service ./internal/handler ./internal/handler/dto ./internal/server/routes",
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
    "The artifacts are static review boards because this batch is a payload and lazy-loading change; final browser screenshots on home, login, contact, dashboard, admin compliance and announcement popup are still required.",
    "Large locale chunks remain outside this P0 and should be handled by the next i18n route-splitting phase."
  ]
}
-->

## Scope

This review covers the visible shells touched by the P0 performance work: the global app shell, auth layout, admin compliance dialog loading state, and announcement popup lazy loading. It does not redesign layouts, colors, spacing, content hierarchy, or business flows.

## Baseline

The baseline behavior rendered the same UI but loaded heavy public settings and markdown-related code as part of the first screen. The support-contact QR images could enter the HTML/settings JSON as base64 data, and the app forced a second public-settings request after mount.

Baseline artifact: `docs/visual-reviews/assets/public-settings-performance-p0/baseline-public-settings-performance-p0.png`.

## Prototype

The prototype keeps the UI visually stable while moving weight out of the first load: support QR images become normal image URLs, announcement and admin compliance surfaces load only when visible, and low-frequency vendor libraries are split into route-specific chunks.

Prototype artifact: `docs/visual-reviews/assets/public-settings-performance-p0/prototype-public-settings-performance-p0.png`.

## Reuse Decision

The implementation reuses existing components: `Toast`, `NavigationProgress`, `SupportContactPanel`, `AnnouncementPopup`, `AdminComplianceDialog`, and the auth layout remain the same user-facing components. No new visual component family or CSS pattern is introduced.

## State Coverage

Default first load keeps the same title, favicon, theme, router and locale behavior. Settings fallback remains non-blocking. Support QR images use the same `<img>` rendering path after their `src` changes to a backend URL. Admin compliance has a localized loading state while markdown and documents are fetched. Announcement popup keeps the same focus management and dismissal flow.

## Viewport Coverage

The static board covers mobile, desktop, and wide-screen intent because this batch does not change layout dimensions. Existing responsive CSS continues to own auth layout, popup, and dialog sizing. Dark mode and language switching are unchanged by the payload split.

## Evidence

Updated artifact: `docs/visual-reviews/assets/public-settings-performance-p0/updated-public-settings-performance-p0.png`.

Automated evidence is provided by design governance, frontend build, backend unit tests, targeted frontend tests, and whitespace checks.

Commands run:

```bash
python3 generated static performance review boards with PIL
pnpm --dir frontend run build
pnpm --dir frontend run lint:check
go test -tags=unit -count=1 ./internal/service ./internal/handler ./internal/handler/dto ./internal/server/routes
git diff --check
```

## Residual Risk

Browser acceptance is still needed after deployment for home, login, contact/support QR, normal dashboard entry, admin compliance required state, and announcement popup. The next performance phase should target route-level i18n chunking and the largest admin/account chunks.
