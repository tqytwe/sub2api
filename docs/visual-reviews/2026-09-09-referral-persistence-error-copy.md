# Visual Review: Referral Persistence Error Copy

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/i18n/locales/en/admin/playOps.ts",
    "frontend/src/i18n/locales/zh/admin/playOps.ts",
    "frontend/src/utils/apiError.ts",
    "frontend/src/components/admin/play/__tests__/AdminInviteGrowthOperations.spec.ts"
  ],
  "routes_or_surfaces": ["/admin/play/operations invite growth early-close error toast"],
  "languages_and_themes": ["zh-CN light static board", "en-US copy parity review", "dark token code review"],
  "states": ["persistence-error", "TOTP-cancelled", "business-conflict", "success"],
  "viewports": ["360x800", "1280x800"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/referral-campaign-early-close/prototype-360.png",
    "docs/visual-reviews/assets/referral-campaign-early-close/prototype-1280.png"
  ],
  "baseline_artifacts": ["docs/visual-reviews/assets/referral-campaign-early-close/updated-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/referral-campaign-early-close/updated-1280.png"],
  "commands": [
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend lint:check",
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend exec vitest run src/components/admin/play/__tests__/AdminInviteGrowthOperations.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "No focusable element or keyboard interaction changes; the existing BaseDialog and TOTP dialog remain the control owners."},
    "reduced_motion": {"status": "passed", "notes": "Only localized error copy changes; no animation or transition is added."}
  },
  "residual_risks": ["This uses the existing early-close static design boards because no browser or image-generation tool is available in this development task. It is not a new browser capture. Final local-browser acceptance must verify the exact Chinese and English toast after an authorized deployment."]
}
-->

## Scope

- Surface: the existing Invite Growth early-close error Toast.
- Roles: a JWT administrator who has passed the existing TOTP step-up flow.
- Boundary: the localized copy and the shared error-envelope reader change. The route, dialog, form, action, TOTP retry, toast primitive, layout, colors and all other campaign outcomes remain unchanged.

## Baseline

- The earlier review at `2026-09-08-referral-campaign-early-close.md` documents the existing `BaseDialog`, TOTP and Toast workflow at 360px and 1280px.
- A PostgreSQL persistence failure previously fell through to the generic `internal error` response when a client exposed the existing API envelope under `response.data.reason`, which gave an operator no safe indication of whether a retry was appropriate.

## Prototype

- Reused prototype images: `assets/referral-campaign-early-close/prototype-360.png` and `assets/referral-campaign-early-close/prototype-1280.png`.
- The changed toast copy is intentionally concise: Chinese states that rewards and budget were not changed; English states the same recovery action. It introduces no new visual pattern.

## Reuse Decision

- Reuses the current `extractI18nErrorMessage` path, app Toast store, `BaseDialog`, `TotpStepUpDialog`, native fields and existing Play Ops layout.
- No icon, component, route, color token, spacing, radius, card, dialog primitive or animation is added.

## State Coverage

- Persistence error: the stable `REFERRAL_CAMPAIGN_EARLY_CLOSE_FAILED` reason resolves from both flattened and nested API envelopes to paired Chinese and English operator guidance rather than a raw database message or generic `internal error`.
- TOTP cancelled: unchanged; no write is sent and the existing form remains intact.
- Business conflict and success: unchanged; their existing stable reasons and success reload continue to use the current Toast path.

## Viewport Coverage

- Mobile 360x800: reuses the existing Toast and modal layout from the referenced prototype; the new copy is allowed to wrap without changing controls.
- Desktop 1280x800: the same existing operations workspace and modal remain in place.
- No responsive CSS changed. Light/dark and 200% zoom remain final browser acceptance checks.

## Evidence

- Static boards are reused only as a layout baseline, not represented as a new browser capture.
- The backend handler and service tests assert the stable reason; locale parity and the existing Play Ops component test are required CI checks.

## Residual Risk

- No local browser, service or production state is used for this review. The exact Toast wrapping, dark contrast, real TOTP interaction and localized post-deployment behavior require your local browser acceptance.
