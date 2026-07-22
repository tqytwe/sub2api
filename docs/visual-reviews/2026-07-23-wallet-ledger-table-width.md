# Wallet Ledger Table Width Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/user/WalletView.vue"
  ],
  "routes_or_surfaces": [
    "/wallet authenticated balance ledger section",
    "wallet transaction source filter and pagination"
  ],
  "languages_and_themes": [
    "zh-CN light",
    "zh-CN dark class review"
  ],
  "states": [
    "transactions loaded",
    "source filter default all",
    "transaction empty and loading row",
    "pagination controls disabled and enabled"
  ],
  "viewports": [
    "390x844",
    "1366x768",
    "1920x1039"
  ],
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
    "pnpm vitest run src/views/user/__tests__/WalletView.spec.ts",
    "pnpm design:check",
    "git diff --check"
  ],
  "checks": {
    "keyboard": {
      "status": "not-applicable",
      "reason": "The change only alters table width classes and does not add or remove focusable controls."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "The change does not add or alter animation."
    }
  },
  "residual_risks": [
    "This shell does not have the user's authenticated browser session, so final browser screenshot acceptance on the real wallet page is still required.",
    "The updated artifact is a static review board; production data density should still be checked after deployment."
  ]
}
-->

## Scope

This review covers the authenticated wallet page balance ledger table. The visible fix is limited to the transaction table sizing contract inside `WalletView.vue`: the table must fill its card on desktop and wide screens while keeping a minimum width that allows horizontal scrolling on narrow mobile screens.

## Baseline

The reported screenshot shows the ledger table compressed against the left side of a full-width card. The source filter and pagination use the full card width, but the table itself only used a minimum content width, leaving a large blank area to the right on a 1920px viewport.

Baseline screenshot: `docs/visual-reviews/assets/wallet-ledger-table-width/baseline-wallet-ledger-left-table.png`.

## Prototype

The target contract is that the ledger table is `w-full` plus a stable minimum width. On desktop, the table expands to the card width. On mobile, the same table keeps its minimum readable width and scrolls inside the existing `overflow-x-auto` wrapper.

Prototype artifact: `docs/visual-reviews/assets/wallet-ledger-table-width/prototype-wallet-ledger-width-contract.png`.

## Reuse Decision

The implementation keeps the existing `AppLayout`, `PageFrame`, card, filter, and pagination structure. No new component abstraction was added. The change reuses the existing responsive scroll wrapper and gives the table an explicit full-width and fixed-column contract.

## State Coverage

Loaded transaction rows keep the same labels, amount coloring, and secondary delta text. The loading and empty row still spans all five columns. The source filter remains in the section header. Pagination stays below the table and continues to align with the full card width.

## Viewport Coverage

Mobile at 390x844 keeps horizontal scrolling through `overflow-x-auto` because the table minimum is wider than the viewport. Desktop at 1366x768 and wide desktop at 1920x1039 now fill the available card width instead of stopping at the left-side content width. Dark mode uses the same class structure and only inherits existing color tokens.

## Evidence

Updated artifact: `docs/visual-reviews/assets/wallet-ledger-table-width/updated-wallet-ledger-full-width-table.png`.

Automated evidence adds a `WalletView.spec.ts` assertion that the ledger table keeps `w-full`, `min-w-[820px]`, and `table-fixed`, rejects the old `min-w-[720px]` only contract, and preserves five columns.

Commands run:

```bash
pnpm vitest run src/views/user/__tests__/WalletView.spec.ts
pnpm design:check
git diff --check
```

## Residual Risk

The local environment does not include the user's logged-in browser session, so this review does not replace final browser acceptance on the real wallet page after deployment. The remaining check is a live screenshot or direct browser review with production-sized wallet data.
