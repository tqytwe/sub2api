# Visual review: administrator model billing settings

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/SettingsView.vue",
    "frontend/src/components/admin/ModelBillingSettings.vue",
    "frontend/src/i18n/locales/zh/admin/settings.ts",
    "frontend/src/i18n/locales/en/admin/settings.ts"
  ],
  "routes_or_surfaces": ["/admin/settings modelBilling tab"],
  "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
  "states": ["disabled-empty", "configured", "selected-groups", "validation-error", "save-conflict", "saved", "loading", "save-error"],
  "viewports": ["360x950", "768x950", "1280x950", "1920x950"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/model-billing-ui/zh-baseline-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/model-billing-ui/zh-baseline-1280.png"],
  "updated_artifacts": [
    "docs/visual-reviews/assets/model-billing-ui/zh-configured-360.png",
    "docs/visual-reviews/assets/model-billing-ui/zh-configured-dark-768.png",
    "docs/visual-reviews/assets/model-billing-ui/en-configured-1280.png",
    "docs/visual-reviews/assets/model-billing-ui/en-conflict-dark-1280.png"
  ],
  "commands": [
    "pnpm exec vitest run src/components/admin/__tests__/ModelBillingSettings.spec.ts src/api/__tests__/admin.modelBilling.spec.ts",
    "pnpm exec vitest run src/views/admin/__tests__/SettingsView.spec.ts -t 'mounts model billing only' --maxWorkers=1 --minWorkers=1",
    "Temporary Vitest mounts exported actual component DOM; Tailwind CLI compiled the existing src/style.css; Chromium CDP rendered the resulting static HTML and checked control bounds. Temporary capture specs and generated HTML were removed from the worktree."
  ],
  "checks": {
    "keyboard": {"status": "passed", "reason": "Static actual component DOM: Tab moves from scope select to a group checkbox and Space toggles it. Live Vue save/group behavior is covered by component tests; full deployed keyboard acceptance remains pending."},
    "reduced_motion": {"status": "passed", "reason": "All captured states used prefers-reduced-motion: reduce; the shared stylesheet supplies the existing reduced-motion override. No new animation was added."}
  },
  "residual_risks": [
    "These are static renders of actual mounted source components with synthetic API fixtures and the existing stylesheet, not production or hydrated browser captures.",
    "The baseline is the existing gateway overload-cooldown settings card, used only to demonstrate shared card, Toggle, input and button reuse. The prototype slot references that reuse baseline because the user explicitly rejected invented prototypes.",
    "Thirteen bounded state/viewport cases had no horizontal document overflow and no controls outside the viewport; five representative images are retained. Settings tab navigation, full application shell, 200% zoom and final administrator save/refresh must still be accepted on the deployed page.",
    "UI and contract implementation alone do not establish runtime billing acceptance; the release must include the corresponding billing runtime and numeric reconciliation tests."
  ]
}
-->

## Scope

One administrator settings tab with its own revisioned form outside the general
settings form. No route, sidebar item or ordinary-user entry is introduced. Drafts
survive tab switches; a 409 retains edits and requires explicit discard/reload.
Inactive saved groups stay visible and cannot silently become global. They block
enabling the strategy but remain preserved when saving it disabled, so administrators
can turn off the strategy without first changing its group scope. The save notice
states the runtime cache's maximum 15-second delay.

## Baseline

The actual existing gateway overload-cooldown settings card was mounted with synthetic
API fixtures and the shared Toggle. Its captured static DOM demonstrates the existing
card, input and button styles. It is not production evidence.

## Prototype

Per the user's explicit request, no invented prototype was created. The prototype
manifest slot references that same existing-component reuse baseline.

## Reuse Decision

The settings page, adjacent gateway card, group-rate editor, shared Toggle and existing
input/button styles were inspected. The dedicated component reuses those patterns;
its bounded group-choice list does not alter the page shell or scroll ownership.

## State Coverage

Live Vue tests cover disabled defaults, explicit scopes, validation, loading, saving,
load failure, ordinary save failure, conflict retention and successful revision reuse.
Static component renders cover configured, empty, validation, conflict and success.
Native controls/shared focus styles are reused; hover hides no action. Tab and Space
were checked on the static native group controls. Deployed keyboard checks remain due.

## Viewport Coverage

Thirteen bounded cases across 360, 768, 1280 and 1920 pixels, Chinese/English and
light/dark had no horizontal document overflow or controls outside the viewport.
The full shell, tab bar and 200% zoom remain production acceptance items.

## Evidence

Five representative PNGs are retained. Actual component DOM was mounted with Vitest,
serialized with live input values/selections and rendered using the existing Tailwind
stylesheet by Chromium CDP. It is static source rendering, not a hydrated or production
capture. Temporary capture specs and generated HTML were removed from the worktree.
No standalone prototype or development server was added.

## Residual Risk

Final production acceptance remains the administrator's real save/refresh flow, full
shell/keyboard checks and the release's billing reconciliation. The UI cannot establish
runtime billing behavior without the corresponding backend runtime integration.
