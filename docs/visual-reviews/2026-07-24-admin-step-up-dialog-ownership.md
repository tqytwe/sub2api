# Administrator Step-Up Dialog Ownership Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/user/BulkUserActionDialog.vue",
    "frontend/src/features/ip-risk/IPRiskActionDialog.vue",
    "frontend/src/features/ip-risk/IPRiskActionsView.vue",
    "frontend/src/features/ip-risk/IPRiskPolicyDialog.vue",
    "frontend/src/features/ip-risk/IPRiskWorkbench.vue",
    "frontend/src/views/admin/ProxiesView.vue",
    "frontend/src/views/admin/UsersView.vue"
  ],
  "routes_or_surfaces": [
    "/admin/users",
    "/admin/proxies/risk",
    "/admin/proxies/actions",
    "administrator TOTP step-up dialog"
  ],
  "languages_and_themes": [
    "zh-CN light static review board",
    "zh-CN and en-US existing localized strings",
    "light and dark existing token paths"
  ],
  "states": [
    "bulk user action preview ready",
    "IP risk action preview ready",
    "IP risk policy save and delete",
    "IP risk action rollback",
    "step-up required",
    "verification loading",
    "verification cancelled",
    "verified action retry"
  ],
  "viewports": [
    "390x844",
    "768x900",
    "1280x800",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/admin-users-bulk-actions/prototype-admin-users-bulk-actions-1440.png",
    "docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-action-preview-desktop.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/admin-users-bulk-actions/baseline-admin-users-selection-1440.png",
    "docs/visual-reviews/assets/ip-risk-step-up-layering/baseline-blocked-step-up-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/ip-risk-step-up-layering/updated-interactive-step-up-1280.png",
    "docs/visual-reviews/assets/ip-risk-step-up-layering/updated-interactive-step-up-390.png",
    "docs/visual-reviews/assets/admin-users-bulk-actions/prototype-admin-users-bulk-actions-390.png"
  ],
  "commands": [
    "pnpm --dir frontend vitest run src/__tests__/adminStepUpDialogs.integration.spec.ts src/components/admin/user/__tests__/BulkUserActionDialog.spec.ts src/views/admin/__tests__/UsersView.spec.ts src/__tests__/ipRiskActions.spec.ts src/__tests__/ipRiskWorkbench.spec.ts src/__tests__/ipRiskRouting.spec.ts",
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend lint:check",
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend test:run",
    "pnpm --dir frontend build",
    "git diff --check"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "Real-component integration tests verify that the topmost TOTP dialog remains outside the inert application root, receives focusable six-digit inputs and returns control to the protected operation after verification."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "The repair changes controller ownership only and introduces no new animation, transition or continuous motion."
    }
  },
  "residual_risks": [
    "The referenced artifacts are browser-rendered static review boards rather than authenticated production screenshots.",
    "Final acceptance still requires entering a current administrator TOTP in the user's local browser after deployment."
  ]
}
-->

## Scope

- Routes: `/admin/users`, `/admin/proxies/risk` and `/admin/proxies/actions`.
- Roles: administrator only.
- Languages and themes: existing Chinese and English strings with the current light and dark semantic tokens.
- Behavior: move TOTP controller ownership to the page root and share it with user batch actions, IP risk actions, IP policies and action rollback.

No scoring rule, selected-account rule, preview payload, authorization requirement, TOTP verification rule or destructive-operation behavior changes in this repair.

## Baseline

The user and IP risk workflows already used the approved dialogs, but each nested workflow created its own `useStepUp()` controller and rendered its own TOTP component. In the reported failure, the underlying page became inert while the expected six-digit verification panel did not become a reliable interactive top layer.

The baseline artifacts contain simulated IP addresses and `example.test` accounts only.

## Prototype

- User bulk actions continue to use the approved desktop and mobile prototypes under `docs/visual-reviews/assets/admin-users-bulk-actions/`.
- IP risk actions continue to use `docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-action-preview-desktop.png`.
- The expected top-layer result remains the approved desktop and mobile TOTP boards under `docs/visual-reviews/assets/ip-risk-step-up-layering/`.
- Approval status: the user approved the original prototypes and requested the complete TOTP repair on July 24, 2026.

This change intentionally preserves all visible layout, copy, spacing, color and responsive behavior.

## Reuse Decision

- Reuse `AppLayout`, `BaseDialog`, `TotpStepUpDialog`, `useStepUp`, existing buttons, inputs, badges and semantic colors.
- `UsersView` owns one controller for both batch disable and batch delete.
- `ProxiesView` owns one controller shared by risk case actions, policy management and rollback history.
- Child dialogs receive a typed `StepUpController` prop and no longer create parallel verification layers.
- No design-system exception or new modal implementation is introduced.

## State Coverage

- Default: user and IP management pages render exactly as before.
- Preview: existing user and risk impact previews remain open while verification is requested.
- Step-up required: the page-owned TOTP dialog teleports outside `#app`, above the inert application root.
- Input: all six numeric cells remain focusable and auto-submit after the sixth digit.
- Verified: the original operation retries once using the short-lived backend grant.
- Cancelled: the original operation stops without showing a generic failure toast.
- Loading and error: existing disabled controls, localized verification errors and input reset behavior remain unchanged.
- Policy and rollback: both use the same page-owned controller as risk case actions.

## Viewport Coverage

- Mobile: the existing 390px TOTP panel remains centered with reachable inputs and cancel action.
- Tablet: dialog sizing and wrapping continue through existing `BaseDialog` behavior.
- Desktop: the step-up panel remains above user and IP management dialogs at 1280px and 1920px.
- 200% zoom: no fixed-width page content or new clipping behavior is introduced.
- Reduced motion: no new motion exists; existing modal behavior is unchanged.

## Evidence

- Updated desktop and mobile artifacts show the approved interactive TOTP layer.
- The new integration suite mounts real `BaseDialog`, `TotpStepUpDialog` and sensitive action components without mocking `useStepUp`.
- It simulates a first request returning `403 STEP_UP_REQUIRED`, verifies `#app` is inert, verifies the TOTP dialog is outside `#app`, enters six digits, completes `totpAPI.stepUp` and confirms the original user/IP operation runs a second time.
- Existing unit coverage also verifies policy save and rollback route through the injected controller.

## Residual Risk

- Static review boards cannot prove a live administrator secret or production session.
- The remaining acceptance step is one authenticated local-browser execution using the administrator's current TOTP after the merged build is deployed.
- Follow-up owner: user for final secret-bearing browser acceptance; engineering for any reproduced post-deploy issue.
