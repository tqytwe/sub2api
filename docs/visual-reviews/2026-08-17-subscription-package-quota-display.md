# Visual Review: Subscription Package Quota Display

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/views/admin/SubscriptionsView.vue"
  ],
  "routes_or_surfaces": ["/admin/subscriptions"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["active package", "exhausted package", "expired package", "legacy subscription", "reset confirmation"],
  "viewports": ["360x800", "768x1024", "1280x860", "1920x1080"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/subscription-package-quota-display/prototype-admin-subscriptions.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/subscription-package-quota-display/baseline-admin-subscriptions.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/subscription-package-quota-display/prototype-admin-subscriptions.png"],
  "commands": ["pnpm --dir frontend vitest run src/utils/__tests__/packageQuota.spec.ts", "pnpm --dir frontend design:check", "pnpm --dir frontend lint:check", "pnpm --dir frontend typecheck", "pnpm --dir frontend build"],
  "checks": {
    "keyboard": {"status": "passed", "reason": "The existing DataTable and ConfirmDialog controls remain unchanged."},
    "reduced_motion": {"status": "passed", "reason": "Only existing width and color progress transitions are reused."}
  },
  "residual_risks": ["The artifacts are static review boards; authenticated browser capture and final local-browser acceptance are still required after deployment."]
}
-->

## Scope

- Route: `/admin/subscriptions`.
- Roles: administrator.
- Scope boundary: expose and render package counters in the existing usage cell; do not change filters, column preferences, assignment, extension, revocation, or legacy subscription display.

## Baseline

- `assets/subscription-package-quota-display/baseline-admin-subscriptions.png`
- The existing table presents legacy daily, weekly, and monthly USD windows only.

## Prototype

- prototype_artifacts: `assets/subscription-package-quota-display/prototype-admin-subscriptions.png`
- Package rows replace those legacy windows only when the API supplies a package entitlement. The rows show request, amount, and Token counters in the existing usage-cell density.

## Reuse Decision

- Reuse `DataTable`, `GroupBadge`, `Icon`, `ConfirmDialog`, existing usage progress bars, and the existing actions column.
- Do not change filters, column preferences, assignment, extension, revocation, or legacy subscription display.

## State Coverage

| State | Usage cell | Status | Reset action |
| --- | --- | --- | --- |
| Active package | Request, amount, Token counters | Existing active badge | Resets the selected unexpired package only |
| Exhausted package | Same counters with the exhausted dimension at its cap | Package exhausted badge | Restores the selected unexpired package after explicit confirmation |
| Expired package | Last package counters if available | Existing expired badge | Not available |
| Legacy subscription | Existing day/week/month rows | Existing status | Existing legacy reset behavior |

## Viewport Coverage

- Check 360px, 768px, 1280px and wide desktop after implementation. The table may use its established horizontal-scroll behavior on narrow screens.

## Evidence

- The baseline and prototype artifacts are listed in the manifest. The implementation reuses the existing DataTable usage-cell layout and progress indicators.
- Automated checks include the package quota unit test, frontend visual/lint/type checks, backend DTO/service tests, production build, and fork-integrity check.

## Residual Risk

- This is a static review board, not a browser capture. Final browser screenshots, locale review, theme review, and local-browser production acceptance remain required.
