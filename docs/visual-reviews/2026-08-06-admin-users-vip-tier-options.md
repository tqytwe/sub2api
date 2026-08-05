# Admin Users VIP Tier Options Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/views/admin/UsersView.vue"],
  "routes_or_surfaces": ["/admin/users"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["configured VIP tier list", "configuration endpoint fallback", "saved tier filter"],
  "viewports": ["390x844", "1280x820"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/admin-users-bulk-actions/prototype-admin-users-bulk-actions-1440.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/admin-users-bulk-actions/baseline-admin-users-selection-1440.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/admin-users-bulk-actions/prototype-admin-users-bulk-actions-1440.png"],
  "commands": ["pnpm typecheck", "pnpm lint:check", "pnpm build"],
  "checks": {
    "keyboard": {"status": "passed", "reason": "The existing shared Select control remains the only interaction surface."},
    "reduced_motion": {"status": "passed", "reason": "No motion or animation was added."}
  },
  "residual_risks": ["Authenticated browser acceptance should confirm the live configured tier labels after deployment."]
}
-->

## Scope

The VIP filter now reads the configured tier list from the existing public
configuration endpoint instead of assuming a fixed number of options.

## Baseline

The filter used a fixed V0-V6 list, which could disagree with the active
configuration and could show invalid tiers after an operator changed the
membership settings.

## Prototype

The existing filter control now renders the sorted tier labels returned by the
configuration endpoint, with a compatible V0-V6 fallback during an endpoint
failure.

## Reuse Decision

The shared Select control, existing filter layout, locale keys, and page-level
responsive styling are reused. No new visual component or route shell was
introduced.

## State Coverage

The review covers a configured tier response, an unavailable configuration
response, and a saved numeric filter value. Existing hover, focus, disabled,
empty, and error states remain owned by the shared Select component.

## Viewport Coverage

The current filter width remains unchanged at 390x844 and 1280x820. Long or
custom labels continue to be constrained by the Select component rather than
changing the table layout.

## Evidence

The design governance check, TypeScript check, lint check, and production
build are the automated evidence for this small control change. A live
authenticated browser check remains part of deployment acceptance.

## Review

The existing Select, spacing, responsive widths, and localized labels remain
unchanged. The page sorts configured tiers numerically and falls back to the
existing V0-V6 options if the configuration request is temporarily unavailable.

## Acceptance

The deployed administrator page must show the same tier labels returned by
`/api/v1/public/vip-tiers` in both supported languages, while the backend
continues to validate the selected numeric tier before filtering.

## Residual Risk

Production browser acceptance still needs to confirm the live administrator
bundle displays all configured tiers after the deployment has propagated.
