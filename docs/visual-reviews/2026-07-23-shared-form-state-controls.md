# Shared Form State Controls Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/common/Input.vue",
    "frontend/src/components/common/SearchInput.vue",
    "frontend/src/components/common/Select.vue",
    "frontend/src/components/common/Skeleton.vue",
    "frontend/src/components/common/Toast.vue",
    "frontend/src/components/common/Toggle.vue",
    "frontend/src/components/common/__tests__/FormStateControls.spec.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/common.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/common.ts",
    "frontend/src/style.css"
  ],
  "routes_or_surfaces": [
    "shared Input fields across auth, profile, admin dialogs, and settings",
    "shared SearchInput filters across user and admin tables",
    "shared Select controls across filters, dialogs, payment/admin settings",
    "shared Toggle switches across admin and setup flows",
    "shared Toast and Skeleton feedback states"
  ],
  "languages_and_themes": [
    "zh-CN light static board",
    "zh-CN dark compatibility through shared token classes"
  ],
  "states": [
    "default",
    "hover",
    "focus-visible",
    "disabled",
    "error",
    "clearable search",
    "open select",
    "toast success/info/warning/error announcement",
    "reduced-motion skeleton"
  ],
  "viewports": [
    "390x844",
    "768x1024",
    "1280x860",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/shared-form-state-controls/prototype-shared-form-state-controls.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/shared-form-state-controls/baseline-shared-form-state-controls.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/shared-form-state-controls/updated-shared-form-state-controls.png"
  ],
  "commands": [
    "python3 generated static shared form state review boards with PIL",
    "pnpm --dir frontend exec vitest run src/components/common/__tests__/FormStateControls.spec.ts",
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend lint:check",
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend build",
    "git diff --check"
  ],
  "checks": {
    "keyboard": {
      "status": "passed"
    },
    "reduced_motion": {
      "status": "passed"
    }
  },
  "residual_risks": [
    "The artifacts are static review boards rather than browser screenshots, so representative browser acceptance is still required after deployment.",
    "This batch strengthens shared controls but does not migrate every page-specific hand-rolled input, switch, or table-like filter."
  ]
}
-->

## Scope

This review covers the shared form and feedback components used across user, admin, authentication, and setup surfaces: `Input`, `SearchInput`, `Select`, `Toggle`, `Toast`, and `Skeleton`. It does not redesign any route layout, subscription page, payment shelf, dashboard data layout, or business flow.

## Baseline

The existing shared controls had partial state coverage: `Toggle` exposed only an on/off button with no disabled or support text contract, `SearchInput` lacked clear and disabled behavior, `Select` still used `transition-all` on the trigger and 12px trigger corners, `Input` displayed hints/errors without native field association, `Skeleton` was not hidden from assistive technology, and error toasts used the same polite announcement behavior as non-blocking messages.

Baseline artifact: `docs/visual-reviews/assets/shared-form-state-controls/baseline-shared-form-state-controls.png`.

## Prototype

The prototype keeps the existing quiet console style: 8px control corners, no layout movement, no page-level spacing changes, no new color family, and no decorative motion. It adds clear default, focus-visible, disabled, error, open, and clearable states while keeping old component APIs compatible.

Prototype artifact: `docs/visual-reviews/assets/shared-form-state-controls/prototype-shared-form-state-controls.png`.

Approval status: this is a narrow continuation batch under the approved frontend governance rule requiring prototype evidence before visible UI changes.

Scope boundary: shared control states only; no route-width changes, no subscription page changes, no new component family.

## Reuse Decision

The implementation reuses existing shared components and `Icon.vue`. It updates component contracts in place so current user/admin pages inherit the same state language without adding parallel search, switch, toast, or field implementations. The `Select` dropdown remains a 12px overlay, while the trigger itself follows the 8px control rule.

## State Coverage

`Input` now links labels, hints, and errors to the native field. `SearchInput` supports clear, disabled, accessible labels, and stable focus. `Toggle` supports disabled, labels, descriptions, field errors, and native switch semantics. `Select` exposes trigger/listbox relationships and removes broad transition behavior. `Toast` differentiates assertive error announcements from polite status messages. `Skeleton` is decorative to assistive technology and respects reduced-motion through the shared `.skeleton` utility.

## Viewport Coverage

The change does not add page-level width, centering, full-height shells, or route scrolling. The static review board covers mobile, tablet, desktop, and wide-screen intent because the modified controls use fixed token spacing and container-relative widths rather than viewport-scaled typography.

## Evidence

Updated artifact: `docs/visual-reviews/assets/shared-form-state-controls/updated-shared-form-state-controls.png`.

Commands run:

```bash
python3 generated static shared form state review boards with PIL
pnpm --dir frontend exec vitest run src/components/common/__tests__/FormStateControls.spec.ts
pnpm --dir frontend design:check
pnpm --dir frontend lint:check
pnpm --dir frontend typecheck
pnpm --dir frontend build
git diff --check
```

## Residual Risk

The artifacts are static review boards, not browser screenshots of every route. After deployment, representative pages such as Dashboard, API Keys, Usage filters, admin settings dialogs, account tables, and login/profile forms should still be spot-checked in the browser for local utility overrides or page-specific wrapping.
