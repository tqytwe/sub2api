# Visual Review: Agent Team competition states

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/play.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/views/public/AgentTeamView.vue",
    "frontend/src/views/public/__tests__/AgentTeamView.competitive.spec.ts"
  ],
  "routes_or_surfaces": ["/agent-team"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["guest", "no-team", "member", "captain", "loading", "empty", "error", "application-pending", "history-deferred"],
  "viewports": ["360x800", "768x900", "1280x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-08-01-agent-team-competition/prototype-1280.png",
    "docs/visual-reviews/assets/2026-08-01-agent-team-competition/prototype-360.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/team-reward-public-proof-prototype.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-08-01-agent-team-competition/prototype-1280.png",
    "docs/visual-reviews/assets/2026-08-01-agent-team-competition/prototype-360.png"
  ],
  "commands": [
    "pnpm exec vitest run src/views/public/__tests__/AgentTeamView.competitive.spec.ts",
    "pnpm typecheck",
    "pnpm design:check"
  ],
  "checks": {
    "keyboard": { "status": "passed", "notes": "All new command, form and link controls retain visible focus outlines; no hover-only operation was added." },
    "reduced_motion": { "status": "passed", "notes": "The page adds no animation or movement." },
    "copy_locale": { "status": "passed", "notes": "All new labels, states and errors are supplied in Chinese and English locale resources." },
    "privacy": { "status": "passed", "notes": "Public rows render team-level data only. Reward proof renders a masked email or the localized anonymous label; user IDs, personal spend and personal payout amounts are not rendered." }
  },
  "residual_risks": [
    "These are static review boards, not browser captures of a production-like backend. Final acceptance still needs desktop and mobile browser captures against the merged team competition API, in light and dark themes."
  ]
}
-->

## Scope

- Route: `/agent-team`, preserving `AuthenticatedPlayShell`, `PublicPageToolbar`, `PublicPlayBackLink`, the existing Play page layout, and the existing route.
- Roles: guest, authenticated user without a team, team member, and captain.
- Boundary: no new top-level route, no new navigation system, no member consumption board, and no personal payout amount in public or captain-facing content.

## Baseline

The prior page centered the active member experience around per-member contribution cards and detailed allocation records. Guests and users without a team had no usable directory-first competition path, and historical reward proof was not connected to the live team ranking surface.

The earlier static reward-proof board is retained as the baseline evidence. It uses the same existing Play shell and payout-list density but does not cover the new complete state model.

## Prototype

The prototype is a real decodable PNG generated from the local static review board before the Vue view was changed:

- Desktop: `docs/visual-reviews/assets/2026-08-01-agent-team-competition/prototype-1280.png`
- Mobile: `docs/visual-reviews/assets/2026-08-01-agent-team-competition/prototype-360.png`

It establishes one page flow: my-team status when available, public live ranking, team directory/application entry, historical top ten and paid proof. The role strip records the allowed extra controls for guest, no-team, member, and captain states. Scope is approved by the task specification; no standalone activity site or parallel design system is introduced.

## Reuse Decision

- Reused `AuthenticatedPlayShell`, public toolbar/back link, `PlayUserAvatar`, `Icon`, `play-btn`, Play page panels, semantic tokens, existing input and button dimensions.
- The ranking, directory, application queue and settlement rows are plain bordered list rows inside existing panels. No nested cards or new decorative surface was introduced.
- The API adapter follows the upcoming public directory, public leaderboard, season history, application lifecycle, invite rotation, and recruiting routes. It does not derive or invent member-level financial data.

## State Coverage

- Guest: sees public rank, directory, and a registration path; no authenticated request is made.
- No team: sees directory, connected application form/status, create-team and direct-invite fallback.
- Member: own team is highlighted in the ranking; rank, prior-team gap, estimated pool and private settlement status appear without member spend or payout amounts.
- Captain: sees pending application actions, recruiting toggle, and invite rotation/copy controls only.
- Loading, public-load error, empty board/directory/history, disabled, pending application, closed/team-full, success and history-load error states are present.

## Viewport Coverage

- `360x800`: the prototype collapses to a one-column reading order; ranking details move below the team name and controls remain reachable.
- `768x900`: the page keeps two-column history/captain content where space permits.
- `1280x900`: live ranking and history proof remain scan-friendly within the existing wide Play page frame.
- Source styles include explicit 900px and 640px reductions without page-level width, height or scrolling ownership.
- No movement is introduced, so reduced-motion is unaffected. Actual 200% zoom, English expansion and dark theme require the merged backend browser pass.

## Evidence

- Focused Vitest coverage exercises guest, no-team application, member highlighting/lazy history, and captain approval/invite rotation.
- The static review board was inspected at desktop and mobile dimensions. It is explicitly not represented as a rendered application screenshot.
- The test and type-check commands listed in the manifest are rerun after the visual record is added.

## Residual Risk

The competing backend worktree is still in progress. Before release, verify the final merged route DTOs, including `is_recruiting` on the authenticated team summary. The UI intentionally disables the recruitment toggle until that authoritative state exists. Also verify actual production-like data, status error mappings, Safari/Android browser layout, dark theme, English text expansion and real invite/application permissions in a browser.
