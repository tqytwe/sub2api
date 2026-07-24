# IP Risk Step-Up Layering Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/auth/TotpStepUpDialog.vue",
    "frontend/src/components/admin/user/BulkUserActionDialog.vue",
    "frontend/src/features/ip-risk/IPRiskActionDialog.vue",
    "frontend/src/features/ip-risk/IPRiskActionsView.vue",
    "frontend/src/features/ip-risk/IPRiskPolicyDialog.vue"
  ],
  "routes_or_surfaces": [
    "/admin/proxies/risk",
    "shared administrator TOTP step-up dialog"
  ],
  "languages_and_themes": [
    "zh-CN light static review board",
    "zh-CN and en-US existing localized strings",
    "light and dark existing token paths"
  ],
  "states": [
    "risk action dialog open",
    "step-up required",
    "admin table sticky overlay coverage",
    "nested dialog keyboard ownership",
    "pre-execution TOTP prompt before destructive API calls",
    "prompt cancellation returns to the original confirmation dialog",
    "cancelled verification",
    "verification loading"
  ],
  "viewports": [
    "390x844",
    "768x900",
    "1280x800",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-action-preview-desktop.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/ip-risk-step-up-layering/baseline-blocked-step-up-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/ip-risk-step-up-layering/updated-interactive-step-up-1280.png",
    "docs/visual-reviews/assets/ip-risk-step-up-layering/updated-interactive-step-up-390.png"
  ],
  "commands": [
    "xvfb-run -a firefox --no-remote --profile <temporary-profile> --window-size 1280,800 --screenshot <baseline-artifact> file://<review-board>?mode=before",
    "xvfb-run -a firefox --no-remote --profile <temporary-profile> --window-size 1280,800 --screenshot <updated-artifact> file://<review-board>?mode=after",
    "xvfb-run -a firefox --no-remote --profile <temporary-profile> --window-size 390,844 --screenshot <mobile-artifact> file://<review-board>?mode=after",
    "pnpm --dir frontend exec vitest run src/components/auth/__tests__/TotpStepUpDialog.spec.ts src/composables/__tests__/useDialogAccessibility.spec.ts src/__tests__/ipRiskActions.spec.ts",
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
      "reason": "The nested regression proves Escape is owned by the TOTP layer and does not close the underlying risk action dialog."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "The repair changes DOM ownership, focus management and overlay stacking only; the existing visual transitions and loading indicator are unchanged."
    }
  },
  "residual_risks": [
    "The artifacts are browser-rendered static review boards rather than an authenticated production screenshot.",
    "Final acceptance still requires executing a protected risk action in the user's local administrator browser and confirming the TOTP fields and cancel action are clickable."
  ]
}
-->

## Scope

This repair covers the shared administrator TOTP step-up layer when it opens above an existing `BaseDialog`, with `/admin/proxies/risk` and `/admin/users` as the reported paths. The follow-up adjustment moves proactive step-up prompts out of the parent dialog's executing/saving state: user batch actions, IP risk actions, risk action rollback, permanent blocking policy creation, and auto-block enablement now ask for TOTP first, then enter the irreversible API call. It does not change risk scoring, selected accounts, preview contents, execution permissions, TOTP verification rules or action results.

## Baseline

`BaseDialog` correctly marks the application root as inert while its teleported modal is open. The TOTP component previously remained inside that inert application root. When a protected action requested step-up, both the background page and the nested verification UI could therefore become non-interactive. Later user-management acceptance runs exposed two related stacking cases: the shared TOTP layer first used `z-[60]`, while the admin data table sticky header and sticky header columns reserve `z-index: 200` and `220`; then `z-[1000]` appeared in the lazy component chunk but did not produce a matching Tailwind CSS rule in the deployed stylesheet. Both cases could leave the existing verification panel visually covered even though the controller opened. A second production report showed the page still behaving as if it were waiting for step-up; the affected proactive flows were entering their parent dialog's executing or saving state before opening the TOTP controller, so cancellation or a hidden prompt could leave the original controls disabled until refresh.

The baseline board recreates the blocked state with simulated IP and account information only.

## Prototype

The existing user-approved IP risk action prototype already requires administrator TOTP before destructive execution:

`docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-action-preview-desktop.png`.

This fix preserves the approved visual language and changes only the layer ownership needed to make the verification flow operable.

## Reuse Decision

The TOTP component now teleports to `body`, participates in the existing `useDialogAccessibility` stack and uses an explicit inline overlay z-index above admin table sticky overlays and onboarding layers. Proactive step-up flows now call the existing page-owned controller before switching the parent dialog into executing/saving mode, then keep `stepUp.run()` for the backend `STEP_UP_REQUIRED` retry fallback. No parallel modal system, new button style, new icon family or new risk action flow is introduced.

## State Coverage

- Default: the risk action dialog remains unchanged.
- Step-up required: TOTP renders above the inert application root and above admin table sticky headers/columns before the destructive API request starts.
- Keyboard: the TOTP layer owns focus, Tab trapping and Escape while it is topmost.
- Cancel: closing TOTP restores focus to the underlying action dialog without leaving execute/save controls stuck.
- Loading: verification continues to disable cancellation and input using the existing behavior.
- Error: localized TOTP errors and input reset behavior are unchanged.

## Viewport Coverage

The static board covers desktop and 390px mobile presentation. The component retains its existing `max-w-md` width, viewport padding and responsive sizing. Light and dark product token paths are unchanged. Chinese and English use the same DOM and focus contract.

## Evidence

The updated boards show the existing TOTP appearance above the simulated risk workbench. Automated regression coverage mounts a real `BaseDialog` and the real TOTP component together, confirms the TOTP is outside the inert `#app`, confirms the explicit elevated overlay z-index, and proves Escape cancels only the topmost verification layer.

## Residual Risk

The static boards explain and render the layering correction but do not replace an authenticated production execution. Final administrator acceptance must preview a destructive risk action, click execute, interact with all six TOTP fields, cancel once, then reopen and complete verification.
