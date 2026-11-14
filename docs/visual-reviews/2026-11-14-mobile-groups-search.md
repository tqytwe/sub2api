# Visual Review: mobile groups search and table height

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/views/admin/GroupsView.vue", "frontend/src/components/layout/TablePageLayout.vue", "frontend/src/views/admin/__tests__/GroupsView.mobileSearch.spec.ts"],
  "routes_or_surfaces": ["/admin/groups mobile search and table layout"],
  "languages_and_themes": ["zh/light"],
  "states": ["default", "focused search", "long mobile list", "wide table"],
  "viewports": ["390x844", "1280x844"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/mobile-groups-prototype.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/mobile-groups-prototype.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/mobile-groups-updated.png", "docs/visual-reviews/assets/mobile-groups-updated-wide.png"],
  "commands": ["Playwright Chromium static capture at 390x844 and 1280x844", "pnpm exec vitest run src/views/admin/__tests__/GroupsView.mobileSearch.spec.ts src/components/layout/__tests__/TablePageLayout.spec.ts", "pnpm exec eslint src/views/admin/GroupsView.vue src/components/layout/TablePageLayout.vue src/views/admin/__tests__/GroupsView.mobileSearch.spec.ts"],
  "checks": {"keyboard": {"status": "passed"}, "reduced_motion": {"status": "passed"}},
  "residual_risks": ["Screenshots use a minimal static shell; authenticated production acceptance remains a release gate."]
}
-->

## Scope

Port upstream #831's two mobile fixes without changing group permissions, filters, or data behavior.

## Baseline

Before the port, a restored mobile search focus could remain behind the sticky application header, and the mobile table container kept a viewport-based height that could end the sticky region before the final rows.

## Prototype

The prototype capture records the existing group search and table density at 390px before applying the upstream offset and height rules.

## Prototype and reuse

The existing group search input and `TablePageLayout` remain the owners. The prototype records the existing search/table density; the implementation only adds the shared sticky offset and mobile parent height rule.

## Reuse Decision

The implementation reuses the existing search input, sticky `AppHeader`, `TablePageLayout`, and table scroll container. No new route, control, icon, color, or page shell was introduced.

## State Coverage

The search input keeps a `scroll-mt-20` offset and scrolls into view when focus is restored beneath the fixed header. Mobile table pages use document height so the sticky header remains active through the full list. Focus and long-list states are covered by the source contract test and browser captures.

## Viewport Coverage

Browser captures cover 390x844 mobile and 1280x844 wide layouts. The source tests cover the focus handler and mobile height rule; reduced-motion and keyboard requirements remain satisfied by the native input and existing layout behavior.

## Evidence

The targeted Vitest suite has 3 passing tests and ESLint passes. Captures cover 390px mobile and 1280px desktop widths. The production route and authenticated data remain part of the release acceptance gate.

## Residual Risk

Screenshots use a minimal static shell; authenticated production acceptance remains a release gate.
