# Growth Reward Error States

artifact_mode: functional-state-copy

## Scope

- Routes: `/check-in`, `/blindbox`.
- Changed boundary: existing toast/status handling only; no new layout, colors, icons, or spacing.
- Goal: map server-owned qualification, governance, budget, and pool errors to actionable messages.

## Baseline and Prototype

- Baseline: existing CheckInView and BlindboxView error handlers, which collapsed most server errors into a generic failure message.
- Prototype artifact: `docs/visual-reviews/assets/v182-growth-qualification/prototype-growth-qualification.png`.
- Reused patterns: existing `showInfo`/`showError` toasts, existing growth eligibility status block, existing loading and retry behavior.

## State Matrix

| State | Check-in | Blindbox | Expected behavior |
| --- | --- | --- | --- |
| Feature disabled | error | error | Explain that the activity is unavailable. |
| Qualification not met | info | info | Explain account progress/eligibility, without exposing internal data. |
| Governance not approved | info | info | Explain that redeemable rewards are paused. |
| Rollout excluded | info | info | Explain that the account is outside the current rollout. |
| Budget exhausted | info | info | Explain that the current budget is exhausted. |
| Pool unavailable/invalid | info/error | info/error | Explain configuration or maintenance state and refresh status. |
| Unknown/server failure | error | error | Preserve a retry path and generic fallback. |

## Verification

- Desktop and mobile layout are unchanged because this change only adds localized copy and error branches.
- Keyboard, focus, loading, disabled, empty, and success behavior remain owned by the existing components.
- Remaining risk: authenticated browser acceptance is required to verify each server error code against a real account.
