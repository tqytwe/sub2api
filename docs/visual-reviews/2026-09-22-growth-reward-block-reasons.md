# Growth Reward Blocking Reasons

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/user/CheckInView.vue",
    "frontend/src/views/public/BlindboxView.vue",
    "frontend/src/api/play.ts",
    "backend/internal/handler/play_handler.go",
    "backend/internal/handler/play_handler_extended.go",
    "backend/internal/service/play_service.go",
    "backend/internal/service/play_extended.go"
  ],
  "routes_or_surfaces": ["/check-in", "/blindbox", "authenticated play status APIs"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["loading", "growth-energy", "governance-not-approved", "rollout-excluded", "budget-exhausted", "pool-unavailable", "ready", "success", "error"],
  "viewports": ["360x844", "768x1024", "1280x900", "1600x1000"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/growth-reward-block-reasons/prototype.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/growth-reward-block-reasons/prototype.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/growth-reward-block-reasons/prototype.png"],
  "commands": ["pnpm --dir frontend run design:check", "pnpm --dir frontend run lint:check", "pnpm --dir frontend run typecheck", "pnpm --dir frontend exec vitest run src/views/user/__tests__/CheckInView.spec.ts src/views/public/__tests__/BlindboxView.spec.ts"],
  "checks": {
    "keyboard": {"status": "passed", "notes": "Existing native buttons retain keyboard focus and disabled behavior."},
    "reduced_motion": {"status": "not-applicable", "reason": "No motion behavior changed."},
    "locale": {"status": "passed", "notes": "Existing Chinese and English governance messages are reused."}
  },
  "residual_risks": ["Static prototype evidence is not production browser acceptance; authenticated user and administrator flows remain required after deployment."]
}
-->

## Scope

This change covers the authenticated check-in and blind-box status/action surfaces.
It separates growth-governance blocking from coupon-pool availability so a configured
pool is not reported as missing.

## Prototype

- `artifact_mode: static-review-board`
- `prototype_artifacts:`
  - `docs/visual-reviews/assets/growth-reward-block-reasons/prototype.png`
- Prototype source:
  - `docs/visual-reviews/assets/growth-reward-block-reasons/prototype.html`

## Baseline

- Existing authenticated status responses exposed `coupon_pool_ready=false` when
  governance, qualification, or rollout blocked a redeemable reward.
- Check-in and blind-box views then rendered the coupon-pool-unavailable message.

## Changed Files

- `changed_files:`

- `backend/internal/handler/play_handler.go`
- `backend/internal/handler/play_handler_extended.go`
- `backend/internal/service/play_service.go`
- `backend/internal/service/play_extended.go`
- `frontend/src/api/play.ts`
- `frontend/src/views/user/CheckInView.vue`
- `frontend/src/views/public/BlindboxView.vue`
- Related backend and frontend tests.

## Reuse Decision

The existing `gw-quest-banner`, `play-note`, disabled button, localization scope,
and shared page shells are reused. No new visual component or token is introduced.

## State Coverage

| Surface | Governance not approved | Rollout excluded | Budget exhausted | Coupon pool unavailable | Ready |
| --- | --- | --- | --- | --- | --- |
| Check-in | governance message, disabled | rollout message, disabled | budget message, disabled | pool message, disabled | action enabled |
| Blind-box | governance message, disabled | rollout message, disabled | budget message, disabled | pool message, disabled | open enabled |

## Reuse and Visual Boundaries

- Reuses existing `gw-quest-banner`, `play-note`, and existing localized messages.
- No new icon, color, card, shell, or page-width pattern was introduced.
- Loading, disabled, empty, success, and error behavior remain owned by the existing
  views and shared layout.

## Viewport Coverage

- Prototype checked at 1440x1000 and 760px breakpoint behavior.
- Required final review targets: 360x844, 768x1024, 1280x900, and 1600x1000.
- Long Chinese and English governance messages must wrap without covering the action.

## Evidence

- `prototype_artifacts:`
  - `docs/visual-reviews/assets/growth-reward-block-reasons/prototype.png`
- Prototype source:
  - `docs/visual-reviews/assets/growth-reward-block-reasons/prototype.html`
- Automated evidence:
  - `frontend/src/views/user/__tests__/CheckInView.spec.ts`
  - `frontend/src/views/public/__tests__/BlindboxView.spec.ts`
  - `backend/internal/handler/play_handler_reward_status_test.go`

## Checks

- Related backend handler tests.
- Related CheckInView and BlindboxView tests.
- Full backend unit tests, frontend lint/typecheck/test/build, design check, and
  `scripts/check-fork-integrity.sh` are required before merge.

## Residual Risks

- This record uses static prototype evidence; production browser acceptance remains
  required for guest, user, and administrator sessions.
- Production governance approval and budget are operational data and are not mutated
  by this code change.
