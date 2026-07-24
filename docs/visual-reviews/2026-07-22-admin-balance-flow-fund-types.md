# Visual Review: admin-balance-flow-fund-types

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/user/UserBalanceHistoryModal.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/admin/overview.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/admin/overview.ts"
  ],
  "routes_or_surfaces": [
    "/admin/users balance history modal",
    "/admin/usage balance history modal entry"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "zh-CN/dark",
    "en-US/light",
    "en-US/dark"
  ],
  "states": [
    "default rows",
    "fund-management source filter options",
    "expanded ledger details",
    "empty state unchanged",
    "loading state unchanged"
  ],
  "viewports": [
    "360x800",
    "768x900",
    "1280x820"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/admin-balance-flow-fund-types/prototype-admin-balance-flow-fund-types.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/admin-balance-flow-fund-types/before-admin-balance-flow-fund-types.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/admin-balance-flow-fund-types/after-admin-balance-flow-fund-types.png"
  ],
  "commands": [
    "python3 static PNG artifact generation for admin balance flow fund source prototype",
    "python3 static PNG artifact generation for admin balance flow fund source review",
    "pnpm vitest run src/components/admin/user/__tests__/UserBalanceHistoryModal.spec.ts"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "The existing Select and row detail buttons remain the same focusable controls; this change only changes option coverage and row text."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "No animation, transition, or motion behavior was added or changed."
    }
  },
  "residual_risks": [
    "Static review boards do not replace the final administrator browser screenshot or final acceptance after deployment.",
    "A real production admin account should confirm Chinese and English modal rows with live fund-management data."
  ]
}
-->

## Scope

- Routes: admin user list and admin usage entries that open `UserBalanceHistoryModal`.
- Roles: administrator.
- Languages and themes: Chinese and English strings were updated; existing light and dark modal classes were reused.

## Baseline

- Current behavior: fund-management rows such as `ops_gift`, `offline_recharge`, and `fund_refund_reject` surfaced raw source keys and English descriptions in the row title/description.
- Baseline screenshot or recording: `before-admin-balance-flow-fund-types.png`.
- Inconsistencies observed: row notes ignored `metadata.reason`, and the before/after balance arrow could wrap across lines in the fixed table column.

## Prototype

- Prototype design image: `prototype-admin-balance-flow-fund-types.png`.
- Approval status: implementation target derived from the reported admin gift row defect plus the same modal's fund-management, refund, and withdrawal source types.
- Scope boundary: label and description readability, reason visibility, source localization, and nowrap balance ranges; no dialog layout, pagination, or action workflow changes.

## Reuse Decision

- Shared layouts and components reused: existing `BaseDialog`, `Select`, `Icon`, table layout, row expansion, and amount color classes.
- New shared pattern, if any: none.
- Design-system exception, if any: none.

## State Coverage

- Default: admin balance rows now map fund-management, withdrawal, refund, gift, and alias source types to readable labels.
- Hover and active: unchanged; the row detail chevron and buttons keep existing hover behavior.
- Focus-visible and keyboard: unchanged; the Select and detail buttons remain keyboard reachable through existing components.
- Loading, disabled, empty, error and success: unchanged; loading spinner, disabled reconciliation button, empty state, and console error behavior were not modified.

## Viewport Coverage

- Mobile: static review confirms row text is shorter and balance range uses nowrap tabular numbers; the existing table remains horizontally constrained by the dialog.
- Tablet: additional filter options use the existing Select control without changing its width.
- Desktop: static review confirms fund rows read as Admin Gift, Offline Recharge, and Recharge Return states instead of raw keys.
- Wide or short screen: no page frame, dialog width, or scroll container behavior changed.
- 200% zoom and reduced motion: no viewport-scaled text or motion was added.

## Evidence

- Updated screenshot or recording: `after-admin-balance-flow-fund-types.png`.
- Automated visual or overlap checks: static review board plus targeted component test verifying labels, descriptions, notes, and nowrap class.
- Commands run: static PNG generation with Pillow and targeted Vitest listed in the manifest.

## Residual Risk

- Known limitations: this is a static review board rather than a live authenticated browser capture, so production data density still needs a real admin browser check.
- Follow-up owner: release owner should verify the deployed modal on `/admin/users` with live gift, offline recharge, refund, and withdrawal rows.
