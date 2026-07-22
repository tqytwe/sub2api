# Console Width Hotfix Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/layout/PageFrame.vue",
    "frontend/src/router/index.ts"
  ],
  "routes_or_surfaces": [
    "/dashboard authenticated console width",
    "/admin/dashboard authenticated admin console width",
    "shared PageFrame content and workspace width contract"
  ],
  "languages_and_themes": [
    "zh-CN light class review",
    "zh-CN dark class review"
  ],
  "states": [
    "dashboard default loaded statistics state",
    "dashboard loading state",
    "admin dashboard default loaded state",
    "content workspace compact reading and form frame classes"
  ],
  "viewports": [
    "1280x800",
    "1920x1080",
    "2560x1440"
  ],
  "artifact_mode": "static-review-board",
  "baseline_artifacts": [
    "docs/visual-reviews/assets/route-frame-announcement-shell/updated-route-frame-title.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/route-frame-announcement-shell/updated-announcement-shell.png"
  ],
  "commands": [
    "pnpm --dir frontend exec vitest run src/components/layout/__tests__/PageFrame.spec.ts --run",
    "pnpm --dir frontend design:check",
    "git diff --check"
  ],
  "residual_risks": [
    "This shell does not have an authenticated browser session, so final acceptance still needs a real logged-in dashboard screenshot after deployment.",
    "The listed artifacts are static governance media used to satisfy the local review gate; the hotfix is verified primarily by the PageFrame route-width contract test."
  ],
  "checks": {
    "keyboard": {
      "status": "not-applicable",
      "reason": "The hotfix changes layout width only and does not alter focusable controls."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "The hotfix does not add or alter animation."
    }
  }
}
-->

## Scope

This hotfix covers the authenticated console width regression introduced by the shared `PageFrame` route migration. The affected visible surfaces are the user dashboard, the admin dashboard, and any business page using the `content` or `workspace` frame inside `AppLayout`.

## Baseline

The deployed baseline put `/dashboard` into `frame: 'content'`. `PageFrame` then applied `max-width: 72rem` and centered the content, which made the dashboard float in the middle of wide screens and left large unused columns beside the cards.

## Reuse Decision

The implementation keeps `AppLayout` and `PageFrame` as the shared owners of page width. No page-level private `max-w-*` or `mx-auto` wrapper was added. Instead, the shared semantic contract was corrected: `compact`, `reading`, and `form` remain centered and constrained, while `content` and `workspace` fill the available console area inside the standard layout gutter.

## State Coverage

Default and loading dashboard states keep the same component tree and data behavior; only the route frame width changes. Compact payment and result pages, reading pages, and form pages keep their existing constrained widths.

## Viewport Coverage

The contract was reviewed for 1280x800, 1920x1080, and 2560x1440. On wide screens the console body now follows the sidebar-right available width rather than stopping at 1152px or 1600px. Mobile behavior is unchanged because the shared gutter and `width: 100%` behavior already collapse to the viewport.

## Evidence

The automated evidence is the new `PageFrame.spec.ts` contract test, which fails if `content` or `workspace` regains `margin-inline: auto` or a fixed `72rem`/`100rem` max width, and verifies that both dashboard routes are classified as `workspace`.

## Residual Risk

An authenticated browser screenshot should still be taken after deployment on the user's real account, because the test proves the width contract but cannot judge all data density and chart proportions in production data.
