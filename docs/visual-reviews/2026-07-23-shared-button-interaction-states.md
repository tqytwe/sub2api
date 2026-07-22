# Shared Button Interaction States Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/style.css"
  ],
  "routes_or_surfaces": [
    "shared .btn controls across authenticated console pages",
    "shared .btn controls across admin settings and dialogs",
    "payment brand buttons and shared icon buttons"
  ],
  "languages_and_themes": [
    "zh-CN light static board",
    "zh-CN dark static board"
  ],
  "states": [
    "default",
    "hover",
    "active",
    "focus-visible",
    "disabled",
    "payment brand default and hover"
  ],
  "viewports": [
    "390x844",
    "1280x820",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/shared-button-interaction-states/prototype-shared-button-states.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/shared-button-interaction-states/baseline-shared-button-states.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/shared-button-interaction-states/updated-shared-button-states.png"
  ],
  "commands": [
    "python3 generated static shared button review boards with PIL",
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
    "The artifacts are static review boards rather than browser screenshots, so final browser screenshot acceptance is still required on key authenticated pages.",
    "This shared CSS affects many existing buttons; page-specific cramped text or local utility overrides may still need later route-level review."
  ]
}
-->

## Scope

This review covers the shared `.btn` family in `frontend/src/style.css`: base buttons, primary, secondary, ghost, danger, success, warning, payment brand buttons, sizes, and icon buttons. It does not change page layout, payment shelf composition, route data, or business behavior.

## Baseline

The current shared button styles use large rounded corners, decorative gradients, colored shadows, and active scale feedback. Because `.btn` is used across user pages, admin pages, payment dialogs, API key flows, empty states, and support panels, those effects make different surfaces feel visually inconsistent and more animated than the operational design system allows.

Baseline artifact: `docs/visual-reviews/assets/shared-button-interaction-states/baseline-shared-button-states.png`.

## Prototype

The prototype keeps buttons stable in size and changes only border, background, color, and subtle neutral elevation across default, hover, active, focus-visible, and disabled states. Primary and status buttons use solid colors instead of gradients. Payment buttons keep recognizable brand colors but drop colored glow shadows.

Prototype artifact: `docs/visual-reviews/assets/shared-button-interaction-states/prototype-shared-button-states.png`.

Approval status: this is a narrow implementation batch under the user-approved frontend governance rule that visible changes must carry prototype evidence first.

Scope boundary: shared button interaction tokens only; no route layout, no subscription page redesign, no new visual component family.

## Reuse Decision

The implementation reuses the existing global `.btn` classes instead of adding another button component. This makes existing user/admin/payment surfaces inherit one state language without touching individual templates. Standalone `.btn-icon` usage gains the same stable focus and disabled behavior while remaining compatible with local size utilities such as `h-8 w-8` or `grid`.

## State Coverage

Default buttons keep existing labels and semantic hierarchy. Hover is limited to color, border, and neutral shadow changes. Active states use darker or pressed surface colors without scale or layout movement. Focus-visible has a 2px ring with offset. Disabled states remove pointer behavior, mute text/background, and suppress hover-like shadows. Loading buttons continue to be represented by existing disabled/submitting states without changing width.

## Viewport Coverage

The static board includes mobile, desktop, and wide-screen intent because button sizing uses fixed spacing and does not depend on viewport width. The change does not introduce page-level width, centering, or scroll behavior. Dark mode uses the same state model with dark surface and border tokens.

## Evidence

Updated artifact: `docs/visual-reviews/assets/shared-button-interaction-states/updated-shared-button-states.png`.

Automated evidence is expected from design governance, lint, typecheck, production build, and diff whitespace checks.

Commands run:

```bash
python3 generated static shared button review boards with PIL
pnpm --dir frontend design:check
pnpm --dir frontend lint:check
pnpm --dir frontend typecheck
pnpm --dir frontend build
git diff --check
```

## Residual Risk

The local board verifies the shared interaction contract, but it is not a browser screenshot of every route. After deployment, representative pages such as Dashboard, Payment, Wallet, API Keys, Settings, support popover, and mobile login still need final browser acceptance to catch local utility overrides or page-specific wrapping issues.
