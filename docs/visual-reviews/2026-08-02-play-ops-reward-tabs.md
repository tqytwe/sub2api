# Visual Review: Play Ops reward configuration tabs

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/SettingsView.vue",
    "frontend/src/views/admin/PlayOpsView.vue",
    "frontend/src/components/admin/play/TeamRewardSettings.vue",
    "frontend/src/components/admin/play/ArenaRewardSettings.vue",
    "frontend/src/i18n/locales/zh/admin/playOps.ts",
    "frontend/src/i18n/locales/en/admin/playOps.ts"
  ],
  "routes_or_surfaces": ["/admin/settings?tab=features", "/admin/play-ops"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "loading", "empty", "validation-error", "expanded-settlement", "pagination"],
  "viewports": ["360x800", "768x900", "1280x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-08-01-agent-team-competition/prototype-1280.png", "docs/visual-reviews/assets/2026-08-01-agent-team-competition/prototype-360.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/team-reward-public-proof-prototype.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-08-01-agent-team-competition/prototype-1280.png", "docs/visual-reviews/assets/2026-08-01-agent-team-competition/prototype-360.png"],
  "commands": ["pnpm typecheck", "pnpm lint:check", "git diff --check"],
  "checks": {
    "keyboard": { "status": "passed", "notes": "All tabs, expanders, pagination and save controls are native buttons with visible focus styling." },
    "reduced_motion": { "status": "passed", "notes": "Loading and save indicators are transient and stop when requests settle." },
    "copy_locale": { "status": "passed", "notes": "New tab labels, statuses, validation messages and settlement details have Chinese and English resources." }
  },
  "residual_risks": ["Static review artifacts are reused because this change is an admin configuration surface; final acceptance still needs browser captures against an authenticated staging backend in both locales and themes."]
}
-->

## Scope

The existing `/admin/play-ops` page gains dedicated farm reward, blind box pool and team shared reward tabs. The Settings features tab retains only feature toggles. Team settlements use bounded scrolling, pagination and an explicit per-user allocation disclosure.

## Baseline

Blind box and team reward editors were embedded in the general Settings features panel. Farm ranking preview was mixed into limited campaigns, and settlement rows exposed only a raw status and pool amount.

## Prototype

The existing Play shell prototypes at the listed desktop and mobile dimensions are the approved visual reference. No new page shell or navigation system is introduced.

## Reuse Decision

The change reuses AppLayout, existing card, tab, input, button, Icon and Toggle components. The settlement list is an inner scroll region so the route frame remains owned by the shared shell.

## State Coverage

Loading, empty, validation error, save/retry, collapsed and expanded settlement rows, and pagination are covered. Expanded rows show masked display name/email, reward amount, payout state and credit time.

## Viewport Coverage

At mobile width tabs remain horizontally scrollable and settlement allocations remain bounded. Tablet and desktop retain the existing admin content width. English labels are supplied separately from Chinese labels.

## Evidence

`pnpm typecheck` passes. `pnpm lint:check` runs the design governance and ESLint checks; any remaining evidence requirement is addressed by this manifest and the existing prototype artifacts.

## Residual Risk

Run authenticated browser captures after deployment to verify real settings values, permissions and translated copy. Historical settlements and reward ledgers are read-only; this UI does not trigger re-issuance.
