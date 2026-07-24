# IP Risk Step-Up Layering Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/auth/TotpStepUpDialog.vue"
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
    "nested dialog keyboard ownership",
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
      "reason": "The repair changes DOM ownership and focus management only; the existing visual transitions and loading indicator are unchanged."
    }
  },
  "residual_risks": [
    "The artifacts are browser-rendered static review boards rather than an authenticated production screenshot.",
    "Final acceptance still requires executing a protected risk action in the user's local administrator browser and confirming the TOTP fields and cancel action are clickable."
  ]
}
-->

## Scope

This repair covers the shared administrator TOTP step-up layer when it opens above an existing `BaseDialog`, with `/admin/proxies/risk` as the reported path. It does not change risk scoring, selected accounts, preview contents, execution permissions, TOTP verification rules or action results.

## Baseline

`BaseDialog` correctly marks the application root as inert while its teleported modal is open. The TOTP component previously remained inside that inert application root. When a protected action requested step-up, both the background page and the nested verification UI could therefore become non-interactive.

The baseline board recreates the blocked state with simulated IP and account information only.

## Prototype

The existing user-approved IP risk action prototype already requires administrator TOTP before destructive execution:

`docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-action-preview-desktop.png`.

This fix preserves the approved visual language and changes only the layer ownership needed to make the verification flow operable.

## Reuse Decision

The TOTP component now teleports to `body` and participates in the existing `useDialogAccessibility` stack. No parallel modal system, new button style, new icon family or new risk action flow is introduced.

## State Coverage

- Default: the risk action dialog remains unchanged.
- Step-up required: TOTP renders above the inert application root at the existing z-index.
- Keyboard: the TOTP layer owns focus, Tab trapping and Escape while it is topmost.
- Cancel: closing TOTP restores focus to the underlying action dialog without unlocking the page prematurely.
- Loading: verification continues to disable cancellation and input using the existing behavior.
- Error: localized TOTP errors and input reset behavior are unchanged.

## Viewport Coverage

The static board covers desktop and 390px mobile presentation. The component retains its existing `max-w-md` width, viewport padding and responsive sizing. Light and dark product token paths are unchanged. Chinese and English use the same DOM and focus contract.

## Evidence

The updated boards show the existing TOTP appearance above the simulated risk workbench. Automated regression coverage mounts a real `BaseDialog` and the real TOTP component together, confirms the TOTP is outside the inert `#app`, and proves Escape cancels only the topmost verification layer.

## Residual Risk

The static boards explain and render the layering correction but do not replace an authenticated production execution. Final administrator acceptance must preview a destructive risk action, click execute, interact with all six TOTP fields, cancel once, then reopen and complete verification.
