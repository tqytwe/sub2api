# Visual Review: admin-balance-surcharge-details

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/user/UserBalanceHistoryModal.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts"
  ],
  "routes_or_surfaces": ["/admin/users user balance history modal", "/admin/usage user balance history modal"],
  "languages_and_themes": ["zh-CN/light", "en-US/light"],
  "states": ["default ledger table", "row with billing surcharge badge", "expanded row details"],
  "viewports": ["390x844", "1440x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/wallet-ledger-table-width/prototype-wallet-ledger-width-contract.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/wallet-ledger-table-width/baseline-wallet-ledger-left-table.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/wallet-ledger-table-width/updated-wallet-ledger-full-width-table.png"
  ],
  "commands": [
    "pnpm --dir frontend exec vitest run src/components/admin/user/__tests__/UserBalanceHistoryModal.spec.ts",
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend design:check"
  ],
  "checks": {
    "keyboard": { "status": "passed", "notes": "The details toggle is a button moved into the first data column and remains keyboard focusable." },
    "reduced_motion": { "status": "passed", "notes": "No motion or animation behavior was added." }
  },
  "residual_risks": [
    "Static wallet ledger assets were reused; final browser acceptance should confirm the admin can see and open surcharge details in the live modal."
  ]
}
-->

## Scope

- Routes: admin user balance history modal from user and usage management surfaces.
- Roles: administrator only.
- Languages and themes: Chinese and English copy for surcharge labels.

## Baseline

- Current behavior: the details toggle sits in the far-right notes column and can disappear in a narrow modal/table layout.
- Baseline screenshot or recording: existing wallet ledger table-width baseline artifact.
- Inconsistencies observed: administrators cannot reliably discover surcharge metadata because the detail toggle is visually clipped.

## Prototype

- Prototype screenshot or recording: existing wallet ledger width contract board.
- Interaction plan: move the details toggle into the type column, keep the row expansion pattern, and add a surcharge badge only when metadata contains a positive surcharge.
- Design decision: use an extra-wide modal and horizontal table overflow rather than hiding operational columns.

## Reuse Decision

- Shared layouts and components reused: `BaseDialog`, existing table layout, native button, existing `Icon` component.
- New shared pattern, if any: none.
- Design-system exception, if any: none.

## State Coverage

- Default: rows without surcharge keep the existing title, description, notes, and metadata details.
- Hover and active: details toggle keeps existing button hover behavior.
- Focus-visible and keyboard: the details toggle remains a button and is earlier in tab order.
- Loading, disabled, empty, error and success: no state flow changed.

## Viewport Coverage

- Mobile: horizontal overflow prevents right-side table controls from being clipped.
- Tablet: extra-wide dialog constraints still cap at viewport width.
- Desktop: the modal uses the established extra-wide table pattern.
- Wide or short screen: table body keeps existing vertical scrolling.
- 200% zoom and reduced motion: no viewport-scaled type or motion added.

## Evidence

- Updated screenshot or recording: existing wallet ledger updated artifact referenced for the table-width contract.
- Automated visual or overlap checks: design governance plus component test for visible surcharge badge and details panel.
- Commands run: listed in the manifest.

## Residual Risk

- Known limitations: this pass did not capture a fresh browser screenshot with live surcharge data.
- Follow-up owner: admin browser acceptance after deployment.
