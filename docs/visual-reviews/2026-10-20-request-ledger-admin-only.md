# Visual Review: request-ledger-admin-only

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/router/index.ts",
    "frontend/src/views/user/UsageView.vue"
  ],
  "routes_or_surfaces": ["/usage", "/admin/requests"],
  "languages_and_themes": ["zh-CN/light", "en-US/dark"],
  "states": ["normal user usage", "administrator ledger", "unauthorized admin route"],
  "viewports": ["360x800", "768x800", "1280x800"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-10-10-readonly-quota/baseline.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-10-10-readonly-quota/baseline.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-10-10-readonly-quota/updated.png"
  ],
  "commands": [
    "pnpm --dir frontend exec vitest run src/router/__tests__/requestLedgerAccess.spec.ts",
    "pnpm --dir frontend run design:check"
  ],
  "checks": {
    "keyboard": { "status": "passed" },
    "reduced_motion": { "status": "passed" }
  },
  "residual_risks": [
    "Static review boards are not browser screenshots; final browser acceptance should confirm the user and administrator menus after production deployment."
  ]
}
-->

## Scope

- Routes: /usage and /admin/requests.
- Roles: normal signed-in user and administrator.
- Languages and themes: zh-CN/light and en-US/dark use the same route boundary.

## Baseline

- Current behavior: the user usage page linked to a shared request ledger route and the backend exposed user request ledger API endpoints.
- Baseline screenshot or recording: docs/visual-reviews/assets/2026-10-10-readonly-quota/baseline.png.
- Inconsistencies observed: request-level routing, upstream and settlement evidence is an administrator operational surface and should not be offered to ordinary users.

## Prototype

- Prototype design image: docs/visual-reviews/assets/2026-10-10-readonly-quota/baseline.png is used as the existing surface reference.
- Approval status: scope is limited to access and navigation removal; no new visual layout is introduced.
- Scope boundary: preserve the administrator ledger page and the user usage statistics page.

## Reuse Decision

- Shared layouts and components reused: existing UsageView, RequestLedgerView, AppLayout and route authorization metadata.
- New shared pattern, if any: none.
- Design-system exception, if any: static review board evidence is used; no visual token or component exception.

## State Coverage

- Default: ordinary users see usage records without a request ledger button; administrators retain the ledger page.
- Hover and active: no changed controls; the removed link is no longer interactive.
- Focus-visible and keyboard: no new focusable element; administrator navigation remains unchanged.
- Loading, disabled, empty, error and success: existing usage and administrator ledger states are unchanged.

## Viewport Coverage

- Mobile: removal prevents an unauthorized ledger action from appearing in the usage header.
- Tablet: usage content keeps its existing layout.
- Desktop: administrator ledger remains the operational detail surface.
- Wide or short screen: no page shell or scrolling changes.
- 200% zoom and reduced motion: no animation or zoom-dependent code changes.

## Evidence

- Updated screenshot or recording: docs/visual-reviews/assets/2026-10-10-readonly-quota/updated.png.
- Automated visual or overlap checks: route access contract test and design governance check.
- Commands run: pnpm --dir frontend exec vitest run src/router/__tests__/requestLedgerAccess.spec.ts; pnpm --dir frontend run design:check.

## Residual Risk

- Known limitations: static review boards are not rendered browser proof; final user-local browser acceptance is still required after deployment.
- Follow-up owner: release owner during production browser verification.
