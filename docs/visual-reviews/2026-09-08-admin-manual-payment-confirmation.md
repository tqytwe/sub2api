# Visual Review: Admin Manual Payment Confirmation

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/orders/AdminOrdersView.vue",
    "frontend/src/api/admin/payment.ts",
    "frontend/src/types/payment.ts",
    "frontend/src/i18n/locales/en/misc.ts",
    "frontend/src/i18n/locales/zh/misc.ts"
  ],
  "routes_or_surfaces": ["/admin/orders"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["recoverable unpaid order", "ineligible paid/refund order", "empty reference", "TOTP challenge", "TOTP cancellation", "TOTP unavailable", "submitting", "fulfillment pending", "success"],
  "viewports": ["360x800", "768x1024", "1280x820"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/admin-manual-payment-confirmation/prototype-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/admin-manual-payment-confirmation/prototype-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/admin-manual-payment-confirmation/prototype-1280.png"],
  "commands": ["pnpm design:check", "pnpm lint:check", "pnpm typecheck", "vitest order-management tests"],
  "checks": {
    "keyboard": {"status": "passed", "notes": "The existing BaseDialog focus trap, native input and explicit cancel/confirm buttons keep the only new operator path fully keyboard reachable in the static review."},
    "reduced_motion": {"status": "passed", "notes": "Submitting uses a disabled button and localized processing label with a static icon; this change adds no continuous motion."}
  },
  "residual_risks": ["The bitmap is a static review board, not browser evidence. Authenticated browser validation remains required after deployment for Chinese and English, light and dark themes, and the TOTP retry." ]
}
-->

## Scope

The orders table gains a narrowly scoped action for orders verified as paid
outside the platform when callbacks failed. The action is unavailable for paid,
completed, processing, and refund-related orders.

## Baseline

The current order-management screen provides cancellation, retry fulfillment,
and refund paths. It has no operator workflow for a genuinely paid but still
unpaid order, forcing an unsafe manual database intervention.

## Prototype

`prototype-1280.png` is a static review board. It shows the existing full-width
order table and a normal-width confirmation dialog. The dialog uses the existing
modal hierarchy: fixed title, compact read-only order facts, one transaction
reference input, secondary cancel, and a danger confirmation command.

## Reuse Decision

The implementation reuses `BaseDialog`, the current orders table action style,
`Icon.vue`, existing form classes, application toast handling, and
`TotpStepUpDialog`. No route shell, page card, color system, or bespoke modal
component is introduced.

## State Coverage

- Default: only `PENDING`, `FAILED`, `EXPIRED`, and `CANCELLED` orders without
  an existing paid timestamp can open the dialog.
- Validation: the transaction reference is required, trimmed, bounded by the
  backend, and retains user input after an API error.
- TOTP: the first protected request prompts for a code and retries once;
  cancelling preserves the dialog and produces no failure toast.
- Blocked: missing TOTP or an administrator API key displays a localized,
  actionable error and never retries.
- Submit: controls are disabled while the request is in flight.
- Success: the table and selected detail reload; a fulfillment-pending result
  states that the existing retry-fulfillment action remains available.

## Viewport Coverage

At 360px the dialog stays within the shared modal gutter and the read-only
facts use a single column. At 768px and 1280px they form a compact two-column
definition list. Long transaction references wrap in audit detail and never
alter the dialog width.

## Evidence

- The listed prototype, baseline and updated artifacts are decoded static review
  boards. They document the existing table composition and the bounded dialog;
  they are not browser captures or production evidence.
- The focused Vitest coverage validates recoverable-status visibility, refund
  exclusion, TOTP challenge/retry and cancellation preservation. Type checking,
  lint and the design-governance check validate the template contract.

## Residual Risk

No local server or browser is started for this change. This static artifact is
development evidence only. Final authenticated browser acceptance must verify
the real order table, focus return, TOTP cancellation/retry, two locales, both
themes, and success/failure refresh states after deployment.
