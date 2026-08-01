# Visual Review: play growth competition

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/play.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/views/public/AgentTeamView.vue",
    "frontend/src/views/public/ArenaView.vue",
    "frontend/src/views/public/__tests__/ArenaView.competitive.spec.ts",
    "frontend/src/views/user/PlayHubView.vue",
    "frontend/src/views/user/__tests__/PlayHubView.spec.ts",
    "frontend/src/views/admin/PlayOpsView.vue",
    "frontend/src/i18n/locales/zh/admin/playOps.ts",
    "frontend/src/i18n/locales/en/admin/playOps.ts",
    "frontend/src/api/__tests__/play.arenaSeason.spec.ts"
  ],
  "routes_or_surfaces": ["/play", "/agent-team", "/arena", "/admin/play-ops"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["guest", "no team", "member", "captain", "loading", "empty", "error", "settled history"],
  "viewports": ["360x800", "768x900", "1280x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/play-growth-competition/prototype-team-arena-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/team-reward-public-proof-prototype.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/play-growth-competition/prototype-team-arena-1280.png"
  ],
  "commands": ["firefox --headless static prototype capture", "pnpm design:check", "pnpm lint:check", "pnpm typecheck", "pnpm test"],
  "checks": {
    "keyboard": { "status": "passed", "notes": "Tabs, retry and history controls are native buttons and retain the existing focus treatment." },
    "reduced_motion": { "status": "passed", "notes": "The change adds no animation, motion or layout-shifting hover effect." },
    "copy_locale": { "status": "passed", "notes": "All new Farm labels, errors and empty states have paired Chinese and English resources." }
  },
  "residual_risks": ["The artifact is a static review board. Final browser captures against real API states and local user acceptance remain required before production release."]
}
-->

## Scope

- Routes: `/play`, `/agent-team`, `/arena`, and the existing `/admin/play-ops` tabs.
- Roles: guest, logged-in user without a team, member, captain, operator, risk-control reviewer, and finance reviewer.
- Boundary: retain existing Play shells, navigation, `growth-world.css`, compact data panels, and shared buttons; add discovery, comparison, history, and operational states only.

## Baseline

- The existing reward proof board confirms the compact side navigation, public payout-row density, panel borders, and semantic success amount treatment to reuse.
- The current pages are too self-oriented: guests and users without a team cannot discover competing teams, and the monthly Farm result lacks a durable historical ranking.

## Prototype

- Prototype design image: `docs/visual-reviews/assets/play-growth-competition/prototype-team-arena-1280.png`.
- Approval status: the product scope was approved before implementation; this image constrains implementation to the existing layout language.
- The prototype makes the rank, member capacity, monthly consumption, estimated pool, previous-rank gap, historical top teams, payout proof, and Farm chase target visible without exposing user IDs or private consumption.
- Farm now loads only the selected daily or monthly board on first paint. The historical settlement proof is an explicit, secondary request so it cannot delay a user who is trying to check the current chase target.

## Reuse Decision

- Reuse `AuthenticatedPlayShell`, `PublicPageToolbar`, `PublicPlayBackLink`, `PlayUserAvatar`, existing tab/button treatment, public payout rows, `growth-world.css`, and semantic status tokens.
- No new top-level route, page shell, icon system, floating-card system, or decorative visual system is introduced.

## State Coverage

- Guest and no-team: public leaderboard, directory and history load before enrollment; application actions require authentication.
- Member and captain: own team is highlighted; captain-only invite rotation and application review stay explicit and unavailable to members.
- Loading/error/empty: first-load skeleton or stable status block, retry path, separate empty copy, and disabled actions retain their dimensions.
- Settlement: actual credited reward is distinct from live estimated pool; all public people use a masked email or localized anonymous label.

## Viewport Coverage

- Mobile: 360px stacks rank metadata below the team name and preserves a 44px action target.
- Tablet: 768px retains a two-column summary only where text fits.
- Desktop: 1280px keeps the data table scan order from rank through estimated pool.
- English, dark mode, 200% zoom, keyboard focus and reduced motion will be checked against the implemented page before release.

## Evidence

- Static prototype captured with Firefox headless at 1280x900: `docs/visual-reviews/assets/play-growth-competition/prototype-team-arena-1280.png`.
- Runtime screenshots, component tests, design checks, and responsive verification are pending implementation and will replace this provisional evidence before release.

## Residual Risk

- The current artifact is a static review board, not a browser capture of live data. Production-like API data, all identity roles, English and dark mode must be reviewed after the frontend is connected.
