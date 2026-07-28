# Visual Review: Play V2 Unification

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/style.css",
    "frontend/src/styles/tokens.css",
    "frontend/src/styles/growth-world.css",
    "frontend/src/styles/public-pages.css",
    "frontend/src/styles/arena-rpg.css",
    "frontend/src/views/public/AgentTeamView.vue",
    "frontend/src/views/public/BlindboxView.vue",
    "frontend/src/views/public/QuizQuestView.vue"
  ],
  "routes_or_surfaces": ["/play", "/check-in", "/arena", "/agent-team", "/affiliate", "/blindbox", "/quiz-quest"],
  "languages_and_themes": ["zh-CN/light", "en/light", "zh-CN/dark"],
  "states": ["default", "hover", "active", "focus-visible", "loading", "disabled", "empty", "error", "success"],
  "viewports": ["360x800", "768x1024", "1280x800", "1920x1039"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/play-v2-unification/play-visual-system-v2.png",
    "docs/visual-reviews/assets/play-v2-unification/blindbox-prototype-v2.png",
    "docs/visual-reviews/assets/play-v2-unification/quiz-prototype-v2.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/play-shell-navigation/baseline-checkin-app-shell-1920.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/play-v2-unification/play-visual-system-v2.png",
    "docs/visual-reviews/assets/play-v2-unification/blindbox-prototype-v2.png",
    "docs/visual-reviews/assets/play-v2-unification/quiz-prototype-v2.png"
  ],
  "commands": [
    "pnpm exec vitest run src/views/public/__tests__/ArenaView.competitive.spec.ts src/views/public/__tests__/AgentTeamView.competitive.spec.ts src/views/public/__tests__/BlindboxView.spec.ts src/views/public/__tests__/QuizQuestView.spec.ts src/components/layout/__tests__/AuthenticatedPlayShell.spec.ts src/views/public/__tests__/authenticatedPlayShellIntegration.spec.ts",
    "pnpm typecheck",
    "pnpm design:verify && pnpm build"
  ],
  "checks": {
    "keyboard": { "status": "passed", "reason": "Quiz question navigation uses native buttons and scrolls to a real question; radio options remain native inputs with visible focus through their labels." },
    "reduced_motion": { "status": "passed", "reason": "The new progress fills use a short width transition only; existing reduced-motion handling remains intact." }
  },
  "residual_risks": [
    "Artifacts are approved static boards, not authenticated browser captures with live reward data.",
    "Final acceptance must inspect all seven routes at the listed viewports with live user data, dark mode and English before deployment."
  ]
}
-->

## Scope

- Consolidate all play surfaces around one blue interaction token, reserving green for completion and orange for rewards or pending settlement.
- Refresh the authenticated Token Farm, Team, Blindbox and Quiz surfaces without changing reward, settlement, quiz submission or team behavior.
- Add a keyboard-accessible question navigator and clear selected-option feedback to the existing quiz flow.

## Baseline

- Existing authenticated play routes use the established console shell from the approved navigation review.
- The earlier prototype direction used a distinct accent for each play page, which made the set read as unrelated products.
- Blindbox and quiz already expose live pool and question data; no synthetic balance, reward or settlement data is introduced.

## Prototype

- User approved the V2 visual system, Blindbox and Quiz static prototypes on 2026-07-28.
- `play-visual-system-v2.png` defines semantic token roles; it is the source of the unified blue interaction treatment.
- `blindbox-prototype-v2.png` and `quiz-prototype-v2.png` define the information order for the two previously undesigned play pages.

## Reuse Decision

- Add the missing shared `tokens.css` file declared by the governance configuration and consume its semantic variables from existing play styles.
- Reuse `AppLayout`, `AuthenticatedPlayShell`, existing route data, native radios, buttons, reward cards and support behavior.
- Do not create an additional navigation shell, duplicate a reward calculator, or alter coupon, balance, quiz or settlement APIs.

## State Coverage

- Default: all play cards, buttons and progress use the blue interaction token.
- Hover and active: primary and secondary controls vary color only; no layout changes are added.
- Focus-visible: existing native controls remain available; quiz selection uses a native radio input inside its label.
- Loading, disabled, empty and error: existing API-driven messages and disabled button behavior remain unchanged.
- Success: completion remains green and rare/pending reward information remains orange, with text labels retained.

## Viewport Coverage

- 360px: existing stacked grids and native options preserve full-width touch targets.
- 768px: quiz option layout remains single-column and the side content follows normal document order.
- 1280px and 1920px: the existing AppLayout workspace frame retains sidebar-right width and avoids page-level max-width.
- Static boards are 1920x1039; final browser acceptance must cover the runtime viewports above.

## Evidence

- Baseline: `baseline-checkin-app-shell-1920.png` from the approved shell review.
- Approved V2 system board: `play-visual-system-v2.png`.
- Approved page boards: `blindbox-prototype-v2.png` and `quiz-prototype-v2.png`.
- Automated coverage: existing shell, arena, team, blindbox and quiz suites.

## Residual Risk

- Static boards cannot prove the runtime density of production pool data or long localized question text.
- Browser acceptance is still required before deployment; this task does not deploy or change backend reward logic.
