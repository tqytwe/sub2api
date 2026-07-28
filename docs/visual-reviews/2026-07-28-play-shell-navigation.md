# Visual Review: Play Shell Navigation

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/layout/AuthenticatedPlayShell.vue",
    "frontend/src/views/public/ArenaView.vue",
    "frontend/src/views/public/AgentTeamView.vue",
    "frontend/src/views/public/BlindboxView.vue",
    "frontend/src/views/public/QuizQuestView.vue",
    "frontend/src/router/index.ts"
  ],
  "routes_or_surfaces": ["/arena", "/blindbox", "/quiz-quest", "/agent-team"],
  "languages_and_themes": ["zh-CN/light"],
  "states": ["default", "guest", "authenticated", "loading", "disabled", "error", "success", "focus-visible"],
  "viewports": ["360x800", "1280x800", "1920x1039"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/play-shell-navigation/prototype-play-shell-quiz-1920.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/play-shell-navigation/baseline-checkin-app-shell-1920.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/play-shell-navigation/prototype-play-shell-quiz-1920.png"
  ],
  "commands": [
    "Read frontend/AGENTS.md and docs/FRONTEND_DESIGN_SYSTEM.md before implementation.",
    "pnpm exec vitest run src/components/layout/__tests__/AuthenticatedPlayShell.spec.ts src/views/public/__tests__/authenticatedPlayShellIntegration.spec.ts src/views/public/__tests__/ArenaView.competitive.spec.ts src/views/public/__tests__/AgentTeamView.competitive.spec.ts src/views/public/__tests__/BlindboxView.spec.ts src/views/public/__tests__/QuizQuestView.spec.ts --run"
  ],
  "checks": {
    "keyboard": { "status": "passed", "reason": "The shared shell only changes page ownership; existing page controls and AppLayout navigation retain their keyboard behavior." },
    "reduced_motion": { "status": "passed", "reason": "The shell adds no animation or motion behavior." }
  },
  "residual_risks": [
    "This is a static review board, not a browser capture of the implementation.",
    "Final acceptance must inspect authenticated routes at 360, 768, 1280 and wide desktop, including the existing sidebar drawer, focus-visible states and light/dark themes."
  ]
}
-->

## Scope

- Routes: `/arena`, `/blindbox`, `/quiz-quest` and `/agent-team`.
- Roles: authenticated regular users receive the established console navigation; guests keep the existing public pages and registration CTA.
- Boundary: no changes to reward, pool, team, quiz or guest-entry behavior.

## Baseline

- Current authenticated reference: the daily check-in route already uses `AppLayout`, with the persistent "玩法福利" sidebar group and the normal account header.
- Current mismatch: the four public gameplay routes own a public header, a single back link to the play hub and a duplicate support floating card, so authenticated users lose their console navigation.

## Prototype

- Approved static design: `docs/visual-reviews/assets/play-shell-navigation/prototype-play-shell-quiz-1920.png`.
- Direction: reuse the existing sidebar and header exactly; highlight the current game in the "玩法福利" tree. Do not add a second game switcher or a required back link inside the page.
- Approval status: approved by the user on 2026-07-28.

## Reuse Decision

- Reuse `AppLayout`, `AppSidebar`, `AppHeader`, `PageFrame` and the existing route-aware sidebar selection.
- Add `AuthenticatedPlayShell` only as a conditional ownership bridge: authenticated routes render inside `AppLayout`, while guests retain their public page shell.
- Do not create a new navigation, toolbar, support card or page-width system.

## State Coverage

- Guest: public page header, back link, toolbar, support card and registration CTA remain available.
- Authenticated: `AppLayout` supplies the sidebar, account header and single support floating card; the public header and duplicate support card are absent.
- Loading, disabled, empty, error and success: existing gameplay components retain their existing states unchanged.
- Hover, active and focus-visible: existing sidebar and page control behavior is reused; the shell adds no new interactive control.

## Viewport Coverage

- Mobile: `AppLayout` retains its existing drawer navigation rather than pinning a desktop sidebar.
- Tablet, desktop and wide: the `workspace` frame uses the sidebar-right width, while the shell removes duplicated gameplay page gutter.
- Dark theme and English: required as final browser acceptance, not represented in the approved zh-CN light static board.

## Evidence

- Baseline reference: `baseline-checkin-app-shell-1920.png`.
- Approved static prototype: `prototype-play-shell-quiz-1920.png`.
- Automated behavioral coverage: shared shell plus all four existing gameplay view suites.

## Residual Risk

- A real authenticated browser capture remains required before delivery because the static board cannot validate live sidebar expansion, API data density or responsive layout.
