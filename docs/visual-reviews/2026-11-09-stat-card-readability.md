# Visual review: restore readable dashboard statistics

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/components/common/StatCard.vue", "frontend/src/style.css"],
  "routes_or_surfaces": ["/home (also /)", "/admin/affiliates/overview", "shared StatCard"],
  "languages_and_themes": ["zh and en", "light and dark"],
  "states": ["default", "success", "empty", "loading", "error"],
  "viewports": ["360x640", "768x360", "1280x360", "1920x360"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-09-stat-card-prototype-1920-light.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-09-stat-card-production-before.png", "docs/visual-reviews/assets/2026-11-09-stat-card-baseline-1920-light.png"],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-09-stat-card-updated-360-light.png",
    "docs/visual-reviews/assets/2026-11-09-stat-card-updated-768-light.png",
    "docs/visual-reviews/assets/2026-11-09-stat-card-updated-1280-light.png",
    "docs/visual-reviews/assets/2026-11-09-stat-card-updated-1920-light.png",
    "docs/visual-reviews/assets/2026-11-09-stat-card-updated-1280-dark.png",
    "docs/visual-reviews/assets/2026-11-09-stat-card-updated-360-dark.png"
  ],
  "commands": ["pnpm --dir frontend design:check", "pnpm --dir frontend exec vitest run src/components/common/__tests__/StatCard.styleIsolation.spec.ts", "node docs/archive/completed-tasks/2026-11-09-stat-card-readability/component-browser-check.cjs updated"],
  "checks": {
    "keyboard": {"status": "passed", "notes": "No interactive elements, focus or keyboard handlers are changed."},
    "reduced_motion": {"status": "passed", "notes": "No motion changed; the homepage odometer styles and behavior are retained."}
  },
  "residual_risks": ["These are real Chromium captures of an isolated component template with repository CSS, not a logged-in production route. Final local browser acceptance remains pending. System fallback fonts are used on the server."]
}
-->

## Scope

Fix the user's production screenshot: overview values 0, 4, 66.7% and 70 have their top and bottom clipped. Inspect the actual overview, adjacent administrator dashboard, shared StatCard and homepage CSS before implementation. Preserve statistics and financial behavior.

## Baseline

The supplied production image is copied without identity details. An isolated Chromium fixture renders the real StatCard template with compiled global styles and the homepage stylesheet retained after navigation. At 1920px the home selector changes the shared value to a 58px serif font while the component keeps its 32px line box and overflow clipping. This reproduces the screenshot.

## Prototype

Before changing the shared component, generate the prototype showing isolated dashboard typography. The existing shared design becomes readable at 24px / 32px. This is the user-authorized boundary: restore readable values, with no page redesign or data changes.

## Reuse Decision

Reuse StatCard, its typography, semantic colors and responsive grid. Give the shared card number and label dedicated class names, and map them to the existing shared typography rules. Keep the legacy selectors for other existing consumers. The homepage artwork, stylesheet and odometer stay unchanged. No additional component, icon, control or styling exception is introduced.

## State Coverage

Default/success and the zero value use complete numeric glyphs. Loading/error/empty templates and their semantics remain unchanged. Cards are noninteractive: hover, active, focus-visible and disabled actions are not applicable. Existing homepage reduced-motion behavior is retained.

## Viewport Coverage

Chromium captures at 360, 768, 1280 and 1920px; dark at 360 and 1280px. All four values use 24px font size and 32px line-height, with no horizontal truncation. The home reference retains its display typography. Neither desktop nor mobile home selectors match the dedicated shared-card classes. The CSS regression checks every matching home rule, including rules under media queries, against actual mounted StatCard value and label elements.

## Evidence

The isolated fixture is archived with the task. It compiles repository Tailwind CSS, renders the real shared template, retains the home CSS and checks computed styles. It starts no development server. These captures are development evidence, not final production acceptance. Server fallback fonts are visible in Chinese labels.

## Residual Risk

After deployment, refresh the user's local browser and revisit home then the administrator overview. Confirm the four values in light/dark themes and browser zoom. Existing local-browser three-identity acceptance remains a separate release gate.
