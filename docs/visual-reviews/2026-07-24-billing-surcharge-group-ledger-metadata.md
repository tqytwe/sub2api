# Visual Review: billing-surcharge-group-ledger-metadata

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts"
  ],
  "routes_or_surfaces": ["/admin/users balance history dialog"],
  "languages_and_themes": ["zh-CN/light", "en-US/light"],
  "states": ["expanded API usage ledger details"],
  "viewports": ["390x844", "1280x800"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/ai-marketplace-cp1a-foundation/after-mobile.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/ai-marketplace-cp1a-foundation/before-desktop.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/ai-marketplace-cp1a-foundation/after-desktop.png"
  ],
  "commands": [
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend lint:check",
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend test:run -- UserBalanceHistoryModal.spec.ts"
  ],
  "checks": {
    "keyboard": { "status": "passed", "notes": "No controls or interaction behavior changed." },
    "reduced_motion": { "status": "passed", "notes": "No motion was added." }
  },
  "residual_risks": [
    "Static evidence records the no-layout-change boundary; final production content verification remains a local admin browser check."
  ]
}
-->

## Scope

- Surface: expanded API usage details in the admin user balance history dialog.
- Roles: administrator.
- Languages and themes: Chinese and English labels only; no theme styling changed.

## Baseline

- Current behavior: unknown metadata keys fall back to raw English identifiers.
- Baseline evidence: existing administration review board reused for the unchanged dialog structure.
- Inconsistency: new API Key group metadata would appear as `api_key_group_id` and `api_key_group_name` in Chinese.

## Prototype

- Prototype design image: existing compact operational UI review board.
- Approval status: the user requested Chinese administrator-facing details while preserving the existing ledger layout.
- Scope boundary: add localized labels only; do not change dialog dimensions, rows, controls, or user-facing usage history.

## Reuse Decision

- Reused the existing generic metadata renderer and locale dictionaries.
- No new component or visual pattern was introduced.
- No design-system exception is required.

## State Coverage

- Default: unchanged.
- Expanded details: group ID and group name use localized labels.
- Hover, active, focus, loading, disabled, empty, error, and success: unchanged because no interaction code changed.

## Viewport Coverage

- Mobile, tablet, desktop, and wide layouts retain the existing metadata wrapping behavior.
- No viewport-sized typography, motion, or fixed geometry was added.

## Evidence

- Static review board records the unchanged visual boundary.
- Component tests cover the additional metadata entries.
- Design governance, lint, typecheck, focused test, and build are required before merge.

## Residual Risk

- No fresh browser screenshot is claimed from the server.
- Final verification is an administrator opening a newly generated API usage ledger entry after production deployment.
