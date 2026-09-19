# Visual Review: Group Surcharge UI Recovery

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/GroupsView.vue",
    "frontend/src/views/admin/__tests__/GroupsView.surcharge.spec.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en/legacy/admin-resources.ts",
    "frontend/src/i18n/locales/zh/legacy/admin-resources.ts"
  ],
  "routes_or_surfaces": ["/admin/groups create dialog", "/admin/groups edit dialog"],
  "languages_and_themes": ["zh-CN/light static review board", "en-US automated locale coverage"],
  "states": ["default", "focus-visible", "disabled", "error", "success"],
  "viewports": ["390x1100", "768x900", "1280x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/group-surcharge-ui-recovery/prototype-1280.png",
    "docs/visual-reviews/assets/group-surcharge-ui-recovery/prototype-390.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/group-surcharge-ui-recovery/prototype-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/group-surcharge-ui-recovery/prototype-1280.png",
    "docs/visual-reviews/assets/group-surcharge-ui-recovery/prototype-390.png"
  ],
  "commands": [
    "firefox --headless --window-size 1280,900 --screenshot docs/visual-reviews/assets/group-surcharge-ui-recovery/prototype-1280.png file:///.../prototype.html",
    "firefox --headless --window-size 390,1100 --screenshot docs/visual-reviews/assets/group-surcharge-ui-recovery/prototype-390.png file:///.../prototype.html"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "The prototype preserves native checkbox, select and input focus treatment; implementation uses the existing input classes and native disabled state."},
    "reduced_motion": {"status": "passed", "notes": "The restored form adds no animation or continuous motion."}
  },
  "residual_risks": ["Static review boards are development evidence only. Authenticated Chinese and English production browser acceptance remains required after any authorized deployment."]
}
-->

## Scope

- Routes: `/admin/groups`, create and edit dialogs only.
- Roles: administrator.
- Languages and themes: the static review board covers Chinese light mode; automated locale and cold-route tests cover Chinese/English parity, while the implementation reuses the existing light/dark form tokens.

## Baseline

- Current behavior: the production form jumps directly from the group rate multiplier to RPM, so administrators cannot inspect or update persisted group surcharge values.
- Baseline evidence: the left side of the prototype review board records the missing production state reported by the administrator and confirmed in `387bfce52`.
- Inconsistency: the API, database fields, translations and billing path still exist, but the form state and controls are absent.

## Prototype

- Prototype design images: `prototype-1280.png` and `prototype-390.png`.
- Approval status: the user explicitly requested restoration of the previous group surcharge configuration entry.
- Scope boundary: restore the existing four-field contract between rate multiplier and RPM; do not alter database values, global settings, billing precedence, Astra, Fast or reasoning controls.

## Reuse Decision

- Reuse the current group dialog grid, native checkboxes, `.input`, `.input-label`, `.input-hint`, border and dark-mode tokens.
- No new component family, page shell, icon, card style or route is introduced.
- No design-system exception is required.

## State Coverage

- Default: an enabled override shows the persisted enabled flag, mode and value.
- Hover and active: native controls and existing input styles remain unchanged.
- Focus-visible and keyboard: native tab order is override, enabled, mode and value; existing focus styles remain visible.
- Disabled: enabled, mode and value are disabled when group override is off, while persisted values remain in form state.
- Error and success: existing group API toast and submit state are reused; invalid surcharge values continue to be rejected by the backend contract.

## Viewport Coverage

- Mobile: 390px stacks all four controls without horizontal scrolling.
- Tablet: the existing responsive grid transitions at the current `md` breakpoint.
- Desktop: 1280px retains the prior compact four-column arrangement.
- 200 percent zoom and reduced motion: the controls wrap through the responsive grid and add no motion.

## Evidence

- Updated artifact: the Chinese static review board shows the exact restored position and responsive control order.
- Automated checks: component payload tests, locale parity, cold-route coverage, design governance, lint, typecheck and Fork integrity passed before handoff.
- Browser boundary: no local service or container is used; this board does not replace authenticated production acceptance.

## Residual Risk

- The final rendered dialog still requires the user's local browser acceptance in Chinese and English after an explicitly authorized merge and deployment.
- No production settings or group rows are changed by this code repair.
