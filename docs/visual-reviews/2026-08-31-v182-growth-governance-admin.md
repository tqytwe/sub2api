# Visual Review: v0.1.182 growth-governance workbench

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/play.ts",
    "frontend/src/api/__tests__/admin.play.growthGovernance.spec.ts",
    "frontend/src/components/admin/play/GrowthGovernanceOperations.vue",
    "frontend/src/components/admin/play/__tests__/GrowthGovernanceOperations.spec.ts",
    "frontend/src/i18n/locales/en/admin/playOps.ts",
    "frontend/src/i18n/locales/zh/admin/playOps.ts",
    "frontend/src/views/admin/PlayOpsView.vue",
    "frontend/src/views/admin/__tests__/PlayOpsView.spec.ts"
  ],
  "routes_or_surfaces": ["/admin/play-ops?tab=growth-governance"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "loading", "empty", "error", "incomplete cohort", "approved", "inactive approval", "revoked", "disabled", "step-up", "success", "focus-visible"],
  "viewports": ["360x1120", "768x1024", "1280x960", "1440x1200", "1600x1000"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/v182-growth-governance-admin/prototype-360.png",
    "docs/visual-reviews/assets/v182-growth-governance-admin/prototype-1440.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-07-31-play-ops-growth-prototype.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/v182-growth-governance-admin/updated-360.png",
    "docs/visual-reviews/assets/v182-growth-governance-admin/updated-1440.png"
  ],
  "commands": [
    "firefox --headless --window-size 1440,1200 --screenshot /tmp/v182-growth-governance-updated-1440.png file:///home/codex/worktrees/sub2api-v182-governance-20260829/docs/visual-reviews/assets/v182-growth-governance-admin/static-review-board.html",
    "firefox --headless --window-size 360,1120 --screenshot /tmp/v182-growth-governance-updated-360.png file:///home/codex/worktrees/sub2api-v182-governance-20260829/docs/visual-reviews/assets/v182-growth-governance-admin/static-review-board.html",
    "pnpm --dir frontend exec vitest run src/api/__tests__/admin.play.growthGovernance.spec.ts src/components/admin/play/__tests__/GrowthGovernanceOperations.spec.ts src/views/admin/__tests__/PlayOpsView.spec.ts",
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend lint:check",
    "pnpm --dir frontend typecheck"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "notes": "Refresh, approval, revoke and retry controls are native buttons. Approval and revoke use labelled native inputs, and BaseDialog/TotpStepUpDialog retain existing focus management."
    },
    "reduced_motion": {
      "status": "passed",
      "notes": "The workbench adds no continuous animation, shimmer, layout shift, or transition-all utility."
    },
    "copy_locale": {
      "status": "passed",
      "notes": "All new visible text comes from paired zh/en admin Play Ops locale resources. Unavailable metric names and backend failure codes have explicit localized fallbacks."
    }
  },
  "residual_risks": [
    "The PNGs are static review boards, not an authenticated browser capture of the candidate component. Final merged-PR and production acceptance must verify a real normal administrator TOTP flow, dark mode, 200% zoom, and long localized operator reasons.",
    "Current production reward switches remain off. This UI does not approve or enable a production rollout and must not be treated as evidence that live rewards are issuing."
  ]
}
-->

## Scope

This change adds one dense operating surface inside the existing Play Ops route:
`/admin/play-ops?tab=growth-governance`. It shows the read-only cohort and
latest append-only governance decision, then lets a session-authenticated
administrator submit a reasoned approve or revoke action through the existing
TOTP step-up flow. It does not add a top-level navigation item, a new public
route, browser-owned eligibility, or a direct reward switch.

## Baseline

The existing Play Ops workbench has operations, membership, campaigns, invite
growth, teams, app analytics, feedback, and game configuration modules, but no
surface for the new growth-qualification cohort or the 268 governance decision.
The older workbench board at
`assets/2026-07-31-play-ops-growth-prototype.png` was reviewed as the shell and
density baseline.

## Prototype

`assets/v182-growth-governance-admin/prototype-1440.png` and
`prototype-360.png` are real PNG prototypes generated from the repository-local
static review board. They establish a compact operations sequence:

1. Server observation window and cohort completeness.
2. Cohort numerator/denominator evidence and explicit unavailable metrics.
3. Current decision, budget amount/spend/remaining amount, rollout and reason.
4. A reasoned protected action, with approval blocked before the server has a
   complete mature cohort.

The prototype intentionally calls out that the default server cohort ends at
least 30 days in the past. A fresh two-week range cannot be presented as
approval-ready just because it contains two weeks of participation.

## Reuse Decision

- The new surface stays inside `PlayOpsView` and reuses the existing full-width
  administrator AppLayout, tab strip, `card`, `btn`, `input`, `BaseDialog`,
  `TotpStepUpDialog`, `Icon`, toast store, semantic dark-mode classes, and
  `useStepUp` retry behavior.
- The component introduces no nested cards, custom page width, gradient,
  colored shadow, transition-all utility, hand-drawn SVG, or autonomous motion.
- Summary and evidence cells are bordered data regions inside a section, not
  independent clickable cards. Status always combines an icon and text rather
  than relying on color alone.

## State Coverage

| State | Intended behavior |
| --- | --- |
| Loading | A stable labelled loading region is shown until both read-only API calls settle. |
| Error | A visible alert retains any prior data and provides a retry control. |
| Empty | Cohort/evidence/governance regions state which server result is absent. |
| Incomplete cohort | Null or unavailable metrics are rendered as unavailable, listed as blockers, and disable approval. |
| Approved | Shows server budget amount, reservation spend, remaining amount, rollout, rule version, decision timestamp, reason, and enabled revoke action. |
| Inactive approval | Keeps the latest `approved` decision visible with an amber text status and still enables revoke. It never offers a second approval that would obscure the append-only audit sequence. |
| Revoked or none | Shows a closed text status and em dashes instead of inventing a zero budget. |
| Form validation | Budget must be finite and positive, rollout is an integer from 10 through 20, and reasons remain 10-500 Unicode characters. |
| Step-up | The protected endpoint is attempted through `useStepUp`; on `STEP_UP_REQUIRED` the existing TOTP dialog obtains the short-lived grant and retries once. |
| Success | The component announces a toast, closes the dialog, retains the returned state, and reloads both server resources. |

## Viewport Coverage

- `360x1120`: sidebar is not part of the mobile shell; the workbench becomes a
  single vertical stream, buttons remain full-width where necessary, and metric
  cells do not squeeze numbers into one line.
- `768x1024`: cohort summary uses two stable columns; actions wrap without
  hiding either protected operation.
- `1280x960`, `1440x1200`, and `1600x1000`: the two data panels sit side by
  side in the existing workspace width with no new centered max-width.
- The prototype includes a dark-theme contrast block. Implementation is also
  wired to paired locale resources. Real browser checks at 200% zoom and with
  long English reasons remain release gates rather than claims made by this
  static board.

## Evidence

- Updated static board: `assets/v182-growth-governance-admin/updated-1440.png`
  and `updated-360.png`.
- Component/API/router regression tests cover the API paths, server cohort
  payload, unavailable metrics, blocked approval, step-up wrapper, approval
  input, retryable read error, and Play Ops tab routing.
- `pnpm design:check`, lint and typecheck are recorded in the command list and
  must be rerun with the candidate as a whole before any PR is created.

## Residual Risk

Static review evidence cannot prove live service data, CSS layout in the real
authenticated shell, TOTP completion, final dark/200% browser behavior, or
production policy effects. Production remains unchanged until an authorized
merge, Zeabur deployment verification, read-only cohort review, and local
administrator acceptance have all occurred.
