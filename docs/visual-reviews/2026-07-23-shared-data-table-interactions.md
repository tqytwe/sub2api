# Shared DataTable Interactions Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/common/DataTable.vue",
    "frontend/src/components/common/__tests__/DataTable.spec.ts"
  ],
  "routes_or_surfaces": [
    "shared DataTable desktop header sorting",
    "shared DataTable clickable desktop rows",
    "shared DataTable clickable mobile cards"
  ],
  "languages_and_themes": [
    "zh-CN light static board",
    "zh-CN dark static board"
  ],
  "states": [
    "sortable header default",
    "sortable header hover",
    "sortable header focus-visible",
    "active sort direction",
    "clickable row hover",
    "selected row",
    "keyboard row activation"
  ],
  "viewports": [
    "390x844",
    "1280x860",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/shared-data-table-interactions/prototype-shared-data-table-interactions.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/shared-data-table-interactions/baseline-shared-data-table-interactions.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/shared-data-table-interactions/updated-shared-data-table-interactions.png"
  ],
  "commands": [
    "python3 generated static shared DataTable review boards with PIL",
    "pnpm --dir frontend vitest run src/components/common/__tests__/DataTable.spec.ts",
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend lint:check",
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend build",
    "git diff --check"
  ],
  "checks": {
    "keyboard": {
      "status": "passed"
    },
    "reduced_motion": {
      "status": "passed"
    }
  },
  "residual_risks": [
    "The artifacts are static review boards rather than browser screenshots, so final browser screenshot acceptance is still required on representative tables.",
    "This change improves the shared component contract but cannot remove every local table-like implementation that does not use DataTable."
  ]
}
-->

## Scope

This review covers the shared `DataTable` sorting and clickable-row interaction contract. It updates the reusable table component and its focused unit tests. It does not change the columns, data fetching, pagination, filters, route layouts, or any page-specific table content.

## Baseline

The current desktop sort interaction is attached directly to `<th>`, which makes sorting mouse-first and inconsistent with the design-system requirement that sortable headers use buttons. Sort indicators are rendered by inline SVG in the table template. Desktop rows also receive a universal hover background even when the whole row is not clickable.

Baseline artifact: `docs/visual-reviews/assets/shared-data-table-interactions/baseline-shared-data-table-interactions.png`.

## Prototype

The prototype moves sortable header interaction into a real button with a visible hover state, active sort indicator, and 2px focus-visible ring. Row hover is reserved for rows that are explicitly clickable, and keyboard activation is supported for clickable desktop rows and mobile cards without changing table layout.

Prototype artifact: `docs/visual-reviews/assets/shared-data-table-interactions/prototype-shared-data-table-interactions.png`.

Approval status: this is a narrow follow-up batch under the user-approved frontend governance rule requiring prototype evidence before visible UI changes.

Scope boundary: shared DataTable sorting and row activation only; no page redesign and no new table family.

## Reuse Decision

The implementation reuses `Icon.vue` for sort arrows and keeps the existing `DataTable` slots, column definitions, row-key behavior, virtualization, sticky column logic, and selection model. Existing pages that already use `DataTable` inherit the same interaction contract.

## State Coverage

Sortable headers now expose default, hover, active sorted direction, and focus-visible states through a native button. Selected rows retain their current selected surface. Clickable rows gain pointer, hover, focus-visible, Enter, and Space activation. Non-clickable rows remain visually stable and do not pretend that the whole row is an action.

## Viewport Coverage

Desktop sorting is covered in the shared table layout. Mobile cards receive matching keyboard activation and focus treatment when `clickableRows` is enabled. The static board includes light and dark mode intent and uses fixed spacing, so the update does not depend on viewport-scaled type or route width.

## Evidence

Updated artifact: `docs/visual-reviews/assets/shared-data-table-interactions/updated-shared-data-table-interactions.png`.

Automated evidence includes focused `DataTable` tests plus design governance, lint, typecheck, production build, and whitespace checks.

Commands run:

```bash
python3 generated static shared DataTable review boards with PIL
pnpm --dir frontend vitest run src/components/common/__tests__/DataTable.spec.ts
pnpm --dir frontend design:check
pnpm --dir frontend lint:check
pnpm --dir frontend typecheck
pnpm --dir frontend build
git diff --check
```

## Residual Risk

Because this is a shared component-level change, browser acceptance should still spot-check representative DataTable consumers such as Accounts, Users, Groups, Announcements, API Keys, and payment/order tables. Local table-like views that do not use `DataTable` remain separate migration work.
