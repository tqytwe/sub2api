# Visual Review: public reward showcase

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/play.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/views/public/AgentTeamView.vue",
    "frontend/src/views/public/ArenaView.vue",
    "frontend/src/views/public/__tests__/AgentTeamView.competitive.spec.ts",
    "frontend/src/views/public/__tests__/ArenaView.competitive.spec.ts"
  ],
  "routes_or_surfaces": ["/agent-team", "/arena monthly tab"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["loading", "paid results", "empty results", "guest", "no team", "mobile"],
  "viewports": ["360x800", "768x900", "1280x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/team-reward-public-proof-prototype.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/team-reward-public-proof-prototype.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/team-reward-public-proof-prototype.png"],
  "commands": ["pnpm design:check", "pnpm typecheck", "pnpm vitest run src/views/public/__tests__/AgentTeamView.competitive.spec.ts src/views/public/__tests__/ArenaView.competitive.spec.ts"],
  "checks": {
    "keyboard": { "status": "passed", "notes": "The payout lists contain no new interactive controls and retain the existing page keyboard flow." },
    "reduced_motion": { "status": "passed", "notes": "No animation was introduced." },
    "copy_locale": { "status": "passed", "notes": "All new user-facing labels have Chinese and English locale entries." }
  },
  "residual_risks": ["The static prototype must be supplemented by a browser capture with production-like payout data before release acceptance."]
}
-->

## Scope

Expose verified, already-paid team and monthly Farm rewards to every visitor, including people who have not joined a team. The lists show masked public names, a team or settled month, rank where relevant, and actual credited amounts.

## Baseline

Team payout history was only loaded after a user joined a team. The Farm page showed daily payouts but did not show settled monthly payouts.

## Prototype

The static review board uses the existing compact payout-list treatment: avatar, masked public name, context, and an amount aligned at the end of each row.

## Reuse Decision

Reuse `PlayUserAvatar`, the existing arena summary panels, team settlement list spacing, responsive grid rules, and semantic success tokens. No new page shell, navigation, card nesting, or decorative surface was introduced.

## State Coverage

- Paid results: rows show public display name, actual credit, payout time, and team/month context.
- No team: the Team page still loads the public reward showcase.
- Empty: each page retains its existing empty-result wording when no settled payout is available.
- Guest: the public endpoint and the Team page showcase remain readable before joining a team.

## Viewport Coverage

At 360px the team payout context drops under the recipient and amount without overlap. At tablet and desktop widths the recipient, context, and amount remain in one scan row. The Farm monthly summary reuses the responsive daily-summary grid.

## Evidence

- Prototype: `docs/visual-reviews/assets/team-reward-public-proof-prototype.png`.
- Component tests cover no-team public display and settled monthly payout amounts.
- The backend handlers only serialize masked display names and omit user IDs, email addresses, contribution amounts, and allocation ratios.

## Residual Risk

The current artifact is a static review board. Browser and production acceptance should verify genuine settled data, empty states, and the available reward-balance source before release.
