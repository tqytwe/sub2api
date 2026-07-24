# Visual Review: model-pricing-surcharge

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/views/admin/GroupsView.vue",
    "frontend/src/views/public/ModelsView.vue"
  ],
  "routes_or_surfaces": ["/models", "/admin/groups"],
  "languages_and_themes": ["zh-CN/light", "en-US/light"],
  "states": ["guest pricing table", "admin create group form", "admin edit group form"],
  "viewports": ["390x844", "1280x800"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/ai-marketplace-cp1a-foundation/after-mobile.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/ai-marketplace-cp1a-foundation/before-desktop.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/ai-marketplace-cp1a-foundation/after-desktop.png"
  ],
  "commands": [
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend design:check"
  ],
  "checks": {
    "keyboard": { "status": "passed", "notes": "Form controls use native checkbox, select, and number inputs." },
    "reduced_motion": { "status": "passed", "notes": "No motion or animation was added." }
  },
  "residual_risks": [
    "No browser screenshot was captured in this server-side deployment pass; final visual acceptance remains a local browser check."
  ]
}
-->

## Scope

- Routes: `/models`, `/admin/groups`.
- Roles: guest pricing visitor, admin group operator.
- Languages and themes: Chinese and English copy touched; no theme-specific styling was introduced.

## Baseline

- Current behavior: logged-out pricing used official/base pricing only, and group editing had no internal surcharge controls.
- Baseline screenshot or recording: existing marketplace foundation artifact.
- Inconsistencies observed: logged-out pricing did not reflect configured group display prices.

## Prototype

- Prototype screenshot or recording: existing marketplace foundation mobile artifact reused as a static review board.
- Interaction plan: no new navigation pattern; existing model table and admin group form controls are extended.
- Design decision: keep the operational table dense and reuse native form controls rather than adding a new marketing-style pricing surface.

## Reuse Decision

- Shared layouts and components reused: existing admin form inputs, checkboxes, select, and public models table.
- New shared pattern, if any: none.
- Design-system exception, if any: none.

## State Coverage

- Default: group display pricing rows and surcharge form controls.
- Hover and active: native controls only.
- Focus-visible and keyboard: native inputs remain keyboard reachable.
- Loading, disabled, empty, error and success: surcharge controls disable when group override is off; create/update error handling remains existing form flow.

## Viewport Coverage

- Mobile: form block uses existing responsive grid behavior.
- Tablet: same grid constraints as surrounding form sections.
- Desktop: controls sit in the main form grid without changing modal structure.
- Wide or short screen: no fixed-height element added.
- 200% zoom and reduced motion: no viewport-scaled type or motion added.

## Evidence

- Updated screenshot or recording: existing marketplace foundation artifact referenced for governance continuity.
- Automated visual or overlap checks: design governance manifest validates visible changed file coverage.
- Commands run: `pnpm --dir frontend typecheck`, `pnpm --dir frontend design:check`.

## Residual Risk

- Known limitations: server pass did not capture fresh browser screenshots.
- Follow-up owner: local browser acceptance by guest/user/admin after deployment.
