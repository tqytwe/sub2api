# Visual Review: Home Header Contact Overlap

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/HomeView.vue",
    "frontend/src/styles/home-view.css"
  ],
  "routes_or_surfaces": ["/home", "/en"],
  "languages_and_themes": ["zh-CN/light", "en/light"],
  "states": ["guest header", "support contact enabled", "desktop nowrap", "tablet compact"],
  "viewports": ["768x900", "900x900", "1024x900", "1100x900", "1180x900", "1200x900", "1240x900", "1280x900", "1366x900", "1440x900", "1600x900", "1920x900"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/home-header-contact-overlap/prototype-home-header-contact-overlap.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/home-header-contact-overlap/baseline-home-header-contact-overlap-board.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/home-header-contact-overlap/updated-home-zh-1024.png",
    "docs/visual-reviews/assets/home-header-contact-overlap/updated-home-zh-1600.png",
    "docs/visual-reviews/assets/home-header-contact-overlap/updated-home-en-1600.png"
  ],
  "commands": [
    "Playwright route mock for /api/v1/settings/public with support_contact enabled",
    "Playwright overlap matrix for /home 768-1920",
    "file docs/visual-reviews/assets/home-header-contact-overlap/*.png"
  ],
  "checks": {
    "keyboard": { "status": "not-applicable", "reason": "No new focusable component or interaction was introduced." },
    "reduced_motion": { "status": "not-applicable", "reason": "No animation behavior changed." },
    "locale": { "status": "passed", "details": "/home kept lang=zh-CN; /en screenshot kept English route coverage." },
    "overlap": { "status": "passed", "details": "Contact link no longer intersects the right page-nav area in the tested support-contact-enabled matrix." }
  },
  "residual_risks": [
    "Server-side Playwright evidence is deployment proof only; final acceptance remains the user's browser after production rollout."
  ]
}
-->

## Scope

- Routes: `/home` and `/en`.
- Roles: guest public homepage header.
- Languages and themes: Chinese light and English light.
- Change boundary: only the existing Home header layout rules and a stable data attribute for existing nav items.

## Baseline

- Current behavior: when public settings enable support contact, the Home primary navigation grows from 6 links to 7 links. The previous fixed 1200px header row plus nowrap rules let `联系我们` overlap the right toolbar at several widths.
- Baseline screenshot or recording: `docs/visual-reviews/assets/home-header-contact-overlap/baseline-home-header-contact-overlap-board.png`.
- Reproduced before the fix with support contact enabled:
  - `1024x900`: gap to toolbar `-78.6px`, overlap true.
  - `1100x900`: gap to toolbar `-6.6px`, overlap true.
  - `1600x900`: gap to toolbar `-0.6px`, overlap true.
  - `1920x900`: gap to toolbar `-31.4px`, overlap true.

## Prototype

- Prototype design image: `docs/visual-reviews/assets/home-header-contact-overlap/prototype-home-header-contact-overlap.png`.
- Approval status: user requested direct correction after screenshot showed `联系我们` covered in the Chinese homepage header.
- Scope boundary: preserve `联系我们` as the customer-support entry, do not change locale routing, and do not introduce a new menu or visual pattern.

## Reuse Decision

- Shared layouts and components reused: existing `HomeView.vue`, `PublicPageToolbar`, public route catalog, and `home-view.css` header rules.
- New shared pattern, if any: none.
- Design-system exception, if any: none.

## State Coverage

- Default: guest homepage header with support contact enabled was checked.
- Hover and active: existing link and button hover rules are unchanged.
- Focus-visible and keyboard: no new focusable behavior was added; existing router links and toolbar controls remain focusable.
- Loading, disabled, empty, error and success: not applicable to this static header layout fix.

## Viewport Coverage

- Mobile: below 768px remains on the existing mobile header behavior.
- Tablet: `768x900` and `900x900` hide the primary nav so the right toolbar has room.
- Desktop: `1024x900`, `1100x900`, `1180x900`, `1200x900`, `1240x900`, `1280x900`, `1366x900`, and `1440x900` were checked.
- Wide or short screen: `1600x900` and `1920x900` were checked after widening only the Home header row.
- 200% zoom and reduced motion: no motion behavior changed; responsive pressure was represented through narrow desktop and tablet viewport checks.

## Evidence

- Updated screenshot or recording:
  - `docs/visual-reviews/assets/home-header-contact-overlap/updated-home-zh-1024.png`.
  - `docs/visual-reviews/assets/home-header-contact-overlap/updated-home-zh-1600.png`.
  - `docs/visual-reviews/assets/home-header-contact-overlap/updated-home-en-1600.png`.
- Automated visual or overlap checks:
  - `/home` with support contact enabled kept `lang=zh-CN`.
  - Viewports `768`, `900`, `1024`, `1100`, `1180`, `1200`, `1240`, `1280`, `1366`, `1440`, `1600`, and `1920` all reported `overlap=false` and `overflowX=0`.
- Commands run:
  - `Playwright route mock for /api/v1/settings/public with support_contact enabled`.
  - `file docs/visual-reviews/assets/home-header-contact-overlap/*.png`.

## Residual Risk

- Known limitations: local Playwright validates implementation and deployment readiness; the final browser acceptance still belongs to the user's own machine after production deploy.
- Follow-up owner: frontend delivery.
