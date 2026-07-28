# Visual Review: coupon operations atmosphere tracking

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/coupon.ts",
    "frontend/src/api/play.ts",
    "frontend/src/components/admin/AdminCouponOperations.vue",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/types/coupon.ts",
    "frontend/src/views/public/BlindboxView.vue"
  ],
  "routes_or_surfaces": ["/play/blindbox", "/admin/promo-codes"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["recent balance win", "recent coupon win", "coupon prize preview", "admin wallet list", "source filter", "status filter", "issued time filter", "used order conversion"],
  "viewports": ["360x800", "768x900", "1280x800", "1600x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/coupon-system/prototype-coupon-operations.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/coupon-system/baseline-coupon-operations.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/coupon-system/updated-coupon-operations.png"
  ],
  "commands": [
    "file docs/visual-reviews/assets/coupon-system/*.png",
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend lint:check",
    "pnpm --dir frontend typecheck"
  ],
  "checks": {
    "keyboard": { "status": "passed", "notes": "New admin filters use native inputs/selects and existing DataTable actions; blindbox recent wins remain static text rows." },
    "reduced_motion": { "status": "passed", "notes": "No new continuous animation was added for coupon prize rows or admin tracking controls." }
  },
  "residual_risks": [
    "Static review board evidence is reused from the coupon-system review; browser final acceptance should verify real user coupon rows and live blindbox recent-win data after deployment."
  ]
}
-->

## Scope

- Routes: public blindbox page and the existing administrator promo-code coupon operations page.
- Roles: guest/signed-in user for blindbox atmosphere, administrator for coupon wallet tracking and replay.
- Languages and themes: Chinese and English labels, light and dark theme semantic colors.

## Baseline

- Current behavior: blindbox recent wins previously emphasized balance wins, and the administrator coupon wallet did not give enough context to identify who received a coupon, where it came from, or whether it converted into a paid order.
- Baseline screenshot or recording: `docs/visual-reviews/assets/coupon-system/baseline-coupon-operations.png`.
- Inconsistencies observed: raw enum values such as reward sources and coupon statuses could leak into Chinese UI, and operations could not filter by issue source or issue window for campaign replay.

## Prototype

- Prototype design image: `docs/visual-reviews/assets/coupon-system/prototype-coupon-operations.png`.
- Approval status: this increment follows the approved coupon operations flow and the user's corrected scope: blindbox pool odds, RTP, expected reward, versions, and VIP prediction remain visible; quiz public copy avoids exposing fixed 80/20 and balance formula language.
- Scope boundary: no new administrator navigation entry, no change to configured admin reward-pool ratios, and no extra payment-coupon entry outside the existing promo-code operations surface.

## Reuse Decision

- Reused public play panels, `DataTable`, `Pagination`, `BaseDialog`, `ConfirmDialog`, existing badge styles, native form controls, and the coupon-system review board artifacts.
- Added only narrow display mapping and filters: source label, status label, order status label, source reference, issue-time filters, and conversion summary.
- No design-system exception is required; blindbox coupon highlights now use semantic emerald utility classes instead of hard-coded raw colors.

## State Coverage

- Default: blindbox shows coupon prize entries and recent coupon wins alongside balance wins.
- Admin tracking: the wallet table shows user identity, issued source, source reference, expiry, localized status, used order, and conversion value.
- Filtering: administrators can filter by user, template, source, status, issued-from, and issued-to. Empty time fields mean no time filtering.
- Empty and error: existing empty table, loading, and failure states remain in place.
- Hover and active: no new custom hover behavior; existing buttons/selects keep their prior interaction model.
- Focus-visible and keyboard: all new controls are native inputs/selects/buttons.
- Loading, disabled, empty, error and success: existing table loading, disabled save/publish actions, empty lists, and success toasts remain reused.

## Viewport Coverage

- Mobile: added filters wrap into the existing responsive grid; blindbox recent rows keep simple text flow.
- Tablet: prize and recent-win panels remain in the existing play layout.
- Desktop: admin table gains operational columns without creating a separate page or navigation branch.
- Wide or short screen: no fixed viewport height, absolute overlay, or custom scroll container was added.
- 200% zoom and reduced motion: labels and filters wrap; no motion was introduced.

## Evidence

- Updated screenshot or recording: `docs/visual-reviews/assets/coupon-system/updated-coupon-operations.png`.
- Automated visual or overlap checks: design governance validates the manifest, required sections, referenced artifacts, changed-file coverage, and raw-color rules.
- Commands run: `pnpm --dir frontend design:check`, `pnpm --dir frontend lint:check`, and `pnpm --dir frontend typecheck`.

## Residual Risk

- Known limitations: this review uses the existing static coupon-system board, not authenticated production data.
- Follow-up owner: release owner verifies real blindbox recent coupon wins, admin source/status labels, filters, and conversion columns after production deployment.
