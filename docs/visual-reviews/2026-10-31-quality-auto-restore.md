# Visual Review: explicit quality automatic restoration

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/AccountQualityView.vue",
    "frontend/src/i18n/locales/en/admin/resources.ts",
    "frontend/src/i18n/locales/zh/admin/resources.ts",
    "frontend/src/types/index.ts"
  ],
  "routes_or_surfaces": ["/admin/account-quality rule form", "/admin/account-quality template form"],
  "languages_and_themes": ["zh/light", "en/light", "zh/dark", "en/dark"],
  "states": ["default-off", "explicit-on", "edit-readback", "preset-switch", "loading", "disabled", "error", "reset"],
  "viewports": ["360x800", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/quality-auto-restore-20261031/incremental-control-design.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/quality-auto-restore-20261031/production-form-baseline.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/quality-auto-restore-20261031/incremental-control-design.png"],
  "commands": [
    "pnpm exec vitest run src/views/admin/__tests__/AccountQualityView.spec.ts",
    "pnpm exec vue-tsc --noEmit",
    "pnpm design:check"
  ],
  "checks": {
    "keyboard": {"status": "passed", "reason": "Reuses the existing native-button Toggle with role=switch, accessible label, described help, checked state and disabled behavior; no custom keyboard handler introduced. Browser keyboard acceptance remains pending."},
    "reduced_motion": {"status": "not-applicable", "reason": "No new animation is introduced; the shared Toggle's existing transition is unchanged."}
  },
  "residual_risks": [
    "The baseline is a crop of the user's actual production screenshot; account details, balances and credentials are excluded.",
    "The incremental image is an annotation of that baseline, not an updated runtime or browser screenshot. The same bounded design annotation is referenced rather than inventing a second screen.",
    "The viewport, theme and language list is the required browser acceptance matrix, not evidence that those screenshots have been captured. Final acceptance on the deployed page remains pending.",
    "No production rule, account scheduling state or paid probe was modified during implementation."
  ]
}
-->

## Scope

Administrator-only quality rule and template forms on the existing account-quality route.

## Baseline

The user-provided production image shows the real quality form with the failure action,
enabled checkbox and preset/save controls. The retained crop contains only that form area.
The change adds an explicit automatic-restoration choice to the existing rule and template
forms; it does not redesign the page, create a new navigation entry or configure production rules.

## Prototype

The scope follows the user's request to complete the actual deployed product rather than
invent a parallel prototype. The annotation was prepared before changing the template.
It explains one incremental control on the production baseline and is not browser evidence.

## Reuse Decision

The neighbouring ScheduledTestsPanel already uses the shared `Toggle` for recovery settings.
AccountQualityView uses that same labelled control, existing form grid and save/error notices.

## State Coverage

New rules and templates start with restoration off. Editing and applying a preset read the
saved boolean; a preset without the field resets it to off. Saving and saving as a preset
send the explicit value. An existing rule's value is not silently enabled or discarded.

The control is disabled during its save request. Restoring by an administrator through the
existing ownership preview remains a separate, explicit operation and is unchanged.
The backend still requires the exact owned state and version before an automatic restoration.

## Viewport Coverage

The existing responsive form grid and full-span toggle row are retained. The listed mobile,
tablet, desktop and wide viewports remain the required browser matrix; no new viewport capture
is claimed by this review board.

## Evidence

Component regression tests cover create, edit, preset reuse and template round trips.
The visual PNG is only the bounded design annotation described above. No updated production
screenshot is claimed. Desktop/mobile, 200% zoom, language/theme, keyboard focus and the
live save/refresh flow remain required on the deployed page; server tests cannot replace
the user's local browser acceptance.

## Residual Risk

The design annotation does not prove rendered layout, themes or browser interactions. Final
administrator save/refresh, keyboard and responsive checks must use the deployed page.
