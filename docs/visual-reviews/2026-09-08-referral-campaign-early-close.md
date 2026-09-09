# Visual Review: Referral Campaign Early Close

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/play.ts",
    "frontend/src/components/admin/play/AdminInviteGrowthOperations.vue",
    "frontend/src/components/admin/play/__tests__/AdminInviteGrowthOperations.spec.ts",
    "frontend/src/i18n/locales/en/admin/playOps.ts",
    "frontend/src/i18n/locales/zh/admin/playOps.ts",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/zh/dashboard.ts"
  ],
  "routes_or_surfaces": ["/admin/play/operations invite growth", "/affiliate referral campaign"],
  "languages_and_themes": ["zh-CN light static board", "en-US copy parity review", "dark token code review"],
  "states": ["claim window", "preview loading", "reason required", "confirmation required", "TOTP required", "TOTP cancelled", "business conflict", "success", "expired reward"],
  "viewports": ["360x800", "1280x800"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/referral-campaign-early-close/prototype-360.png",
    "docs/visual-reviews/assets/referral-campaign-early-close/prototype-1280.png"
  ],
  "baseline_artifacts": ["docs/visual-reviews/assets/referral-campaign-early-close/baseline-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/referral-campaign-early-close/updated-1280.png"],
  "commands": [
    "pnpm --dir frontend exec vitest run src/components/admin/play/__tests__/AdminInviteGrowthOperations.spec.ts src/views/user/__tests__/AffiliateView.spec.ts",
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend lint:check",
    "pnpm --dir frontend typecheck"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "The implementation reuses BaseDialog, native textarea and checkbox, and text-labelled controls. The confirmation action remains disabled until both required form conditions are met."},
    "reduced_motion": {"status": "passed", "notes": "The change adds no animation. Preview and TOTP retry retain existing loading and dialog behavior."}
  },
  "residual_risks": ["These are static review boards, not browser screenshots. Final browser acceptance must inspect the authenticated administrator workflow at 360, 768 and 1280px in Chinese and English, light and dark themes, including focus order and the real TOTP dialog."]
}
-->

## Scope

- Surface: the existing administrator Invite Growth operations section and the existing user referral campaign progress surface.
- Roles: JWT administrator for early close; ordinary user for normal claim-window reward availability.
- Boundary: an early close expires only still-claimable rewards. It does not introduce a new route, component family, reward type, balance mutation path or direct user credit operation.

## Baseline

- During `settling`, the existing generic status operation implied that the campaign could be normally closed, even though the server correctly rejects that transition before `claim_deadline`.
- The baseline board records this ambiguous generic close action. The original implementation changes were already present as uncommitted work when this corrective review was started; this record does not misrepresent the board as a pre-change browser capture.
- The reviewed correction replaces that action with a read-only impact preview and a dangerous, reasoned confirmation workflow.

## Prototype

- Prototype design images: `assets/referral-campaign-early-close/prototype-360.png` and `assets/referral-campaign-early-close/prototype-1280.png`.
- The board keeps the dense administration composition: a status hint above a narrow `BaseDialog`, a summary of expiring and preserved rewards, a required reason, an explicit checkbox and textual Cancel/Early close actions.
- Scope is limited to the existing claim window. Normal `claimable` reward buttons remain available to users until their deadline; after early close the server returns the existing expired state and no claim button remains.

## Reuse Decision

- Reused `BaseDialog`, shared `btn`, `input`, textarea and checkbox patterns, `TotpStepUpDialog`, shared Toast behavior, `Icon`, existing status labels and semantic light/dark classes.
- No new page frame, route, icon, raw color token, decorative gradient, nested card, custom dialog primitive or independent TOTP implementation was added.
- The warning panel uses the existing danger semantic classes and textual explanation, so the operation is not communicated by color alone.

## State Coverage

- Claim window: displays `领奖中` in Chinese and `Claim window` in English with its claim deadline.
- Preview loading and ineligible state: the action is disabled while the authoritative preview loads; the server decides whether early close remains eligible.
- Validation: the primary action stays disabled until a nonblank reason and explicit confirmation are supplied.
- Sensitive action: the existing step-up composable opens TOTP only when the protected endpoint asks for it, then retries exactly one operation.
- Cancellation and errors: cancelling TOTP preserves the dialog and form without a false save error. Stable campaign error codes map to localised operator messages rather than raw backend text.
- Success and user outcome: success reloads campaign data; normal settling claims still work, while server-expired rewards are rendered as a status rather than a claim action.

## Viewport Coverage

- Mobile 360x800: the dialog uses a single readable column and wraps the amount/count summaries and confirmation text without relying on hover.
- Tablet 768px: the existing `BaseDialog` width contract remains authoritative; no fixed page width or private scroll container was added.
- Desktop 1280x800: the compact modal preserves the current operations workspace density and leaves campaign context visible outside the modal.
- 200% zoom and reduced motion: text-labelled controls can wrap within the existing dialog; this change adds no transition or persistent motion.

## Evidence

- Baseline board: `assets/referral-campaign-early-close/baseline-1280.png`.
- Prototype boards: `assets/referral-campaign-early-close/prototype-360.png` and `assets/referral-campaign-early-close/prototype-1280.png`.
- Updated board: `assets/referral-campaign-early-close/updated-1280.png`.
- Automated evidence covers preview loading, required inputs, TOTP wrapping, cancellation and the absence of the invalid normal close action. Locale parity and cold-route checks remain release gates.

## Residual Risk

- Static boards and component tests do not validate a real administrator session, browser focus trapping, native mobile keyboard behavior, actual TOTP entry or dark-theme contrast against live data.
- Final acceptance remains a user-run browser check after authorised deployment. It must not be replaced by this development-machine evidence.
