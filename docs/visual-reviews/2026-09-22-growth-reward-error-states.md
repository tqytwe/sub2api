# Growth Reward Error States

artifact_mode: static-review-board

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/play/GrowthGovernanceOperations.vue",
    "frontend/src/i18n/locales/en/admin/playOps.ts",
    "frontend/src/i18n/locales/en/legacy/user-misc.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/i18n/locales/zh/admin/playOps.ts",
    "frontend/src/i18n/locales/zh/legacy/user-misc.ts",
    "frontend/src/views/public/BlindboxView.vue",
    "frontend/src/views/user/CheckInView.vue"
  ],
  "routes_or_surfaces": ["/check-in", "/blindbox", "/admin/play"],
  "languages_and_themes": ["zh-CN/light", "en-US/light", "zh-CN/dark", "en-US/dark"],
  "states": ["ready", "blocked", "qualification", "governance", "pool unavailable", "unknown error"],
  "viewports": ["360x800", "768x1024", "1280x800"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/v182-growth-qualification/prototype-growth-qualification.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/v182-growth-qualification/prototype-growth-qualification.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/v182-growth-qualification/prototype-growth-qualification.png"],
  "commands": ["pnpm typecheck", "pnpm lint:check", "pnpm design:check"],
  "checks": {"keyboard": {"status": "passed", "reason": "Existing buttons and toast/status patterns are reused."}, "reduced_motion": {"status": "passed", "reason": "No motion was added."}},
  "residual_risks": ["Authenticated production browser acceptance remains required."]
}
-->

## Scope

- Routes: `/check-in`, `/blindbox`.
- Changed boundary: existing toast/status handling only; no new layout, colors, icons, or spacing.
- Goal: map server-owned qualification, governance, budget, and pool errors to actionable messages.

## Baseline

- Existing pages showed a generic failure message for most server-owned reward gates.

## Prototype

- Baseline: existing CheckInView and BlindboxView error handlers, which collapsed most server errors into a generic failure message.
- Prototype artifact: `docs/visual-reviews/assets/v182-growth-qualification/prototype-growth-qualification.png`.

## Reuse Decision

- Reused patterns: existing `showInfo`/`showError` toasts, existing growth eligibility status block, existing loading and retry behavior.

## State Coverage

| State | Check-in | Blindbox | Expected behavior |
| --- | --- | --- | --- |
| Feature disabled | error | error | Explain that the activity is unavailable. |
| Qualification not met | info | info | Explain account progress/eligibility, without exposing internal data. |
| Governance not approved | info | info | Explain that redeemable rewards are paused. |
| Rollout excluded | info | info | Explain that the account is outside the current rollout. |
| Budget exhausted | info | info | Explain that the current budget is exhausted. |
| Pool unavailable/invalid | info/error | info/error | Explain configuration or maintenance state and refresh status. |
| Unknown/server failure | error | error | Preserve a retry path and generic fallback. |

## Viewport Coverage

- 360px, 768px, 1280px, and wide desktop are unchanged structurally; only copy and toast branches were added.

## Evidence

- `frontend/src/views/user/CheckInView.vue`
- `frontend/src/views/public/BlindboxView.vue`
- `frontend/src/components/admin/play/GrowthGovernanceOperations.vue`
- `docs/visual-reviews/assets/v182-growth-qualification/prototype-growth-qualification.png`

## Residual Risk

- Desktop and mobile layout are unchanged because this change only adds localized copy and error branches.
- Keyboard, focus, loading, disabled, empty, and success behavior remain owned by the existing components.
- Remaining risk: authenticated browser acceptance is required to verify each server error code against a real account.

## changed_files

- `frontend/src/components/admin/play/GrowthGovernanceOperations.vue`
- `frontend/src/i18n/locales/en/admin/playOps.ts`
- `frontend/src/i18n/locales/en/legacy/user-misc.ts`
- `frontend/src/i18n/locales/jisudeng-pages.en.ts`
- `frontend/src/i18n/locales/jisudeng-pages.zh.ts`
- `frontend/src/i18n/locales/zh/admin/playOps.ts`
- `frontend/src/i18n/locales/zh/legacy/user-misc.ts`
- `frontend/src/views/public/BlindboxView.vue`
- `frontend/src/views/user/CheckInView.vue`
