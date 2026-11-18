# Visual Review: NewAPI authorization

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/accounts.ts",
    "frontend/src/components/account/UpstreamBillingRateCell.vue",
    "frontend/src/components/account/NewAPIUpstreamConfigDialog.vue",
    "frontend/src/i18n/locales/en/admin/accounts.ts",
    "frontend/src/i18n/locales/zh/admin/accounts.ts",
    "frontend/src/types/index.ts",
    "frontend/src/views/admin/AccountsView.vue",
    "docs/visual-reviews/assets/newapi-authorization/prototype-1280.html"
  ],
  "routes_or_surfaces": [
    "/admin/accounts New API authorization dialog; admin only"
  ],
  "languages_and_themes": [
    "en/light",
    "en/dark",
    "zh/light",
    "zh/dark"
  ],
  "states": [
    "default",
    "focus-visible",
    "loading",
    "disabled",
    "empty candidates",
    "error",
    "matched preview",
    "save success",
    "unlink confirmation"
  ],
  "viewports": [
    "360x900",
    "768x900",
    "1280x900",
    "1920x900"
  ],
  "artifact_mode": "browser-capture",
  "baseline_artifacts": [
    "docs/visual-reviews/assets/api-key-multi-group/updated-billing-1280.png"
  ],
  "prototype_artifacts": [
    "docs/visual-reviews/assets/newapi-authorization/prototype-1280.png",
    "docs/visual-reviews/assets/newapi-authorization/prototype-360.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/newapi-authorization/component-en-360-light.png",
    "docs/visual-reviews/assets/newapi-authorization/component-en-360-dark.png",
    "docs/visual-reviews/assets/newapi-authorization/component-en-768-light.png",
    "docs/visual-reviews/assets/newapi-authorization/component-en-768-dark.png",
    "docs/visual-reviews/assets/newapi-authorization/component-en-1280-light.png",
    "docs/visual-reviews/assets/newapi-authorization/component-en-1280-dark.png",
    "docs/visual-reviews/assets/newapi-authorization/component-en-1920-light.png",
    "docs/visual-reviews/assets/newapi-authorization/component-en-1920-dark.png",
    "docs/visual-reviews/assets/newapi-authorization/component-zh-360-light.png",
    "docs/visual-reviews/assets/newapi-authorization/component-zh-360-dark.png",
    "docs/visual-reviews/assets/newapi-authorization/component-zh-768-light.png",
    "docs/visual-reviews/assets/newapi-authorization/component-zh-768-dark.png",
    "docs/visual-reviews/assets/newapi-authorization/component-zh-1280-light.png",
    "docs/visual-reviews/assets/newapi-authorization/component-zh-1280-dark.png",
    "docs/visual-reviews/assets/newapi-authorization/component-zh-1920-light.png",
    "docs/visual-reviews/assets/newapi-authorization/component-zh-1920-dark.png",
    "docs/visual-reviews/assets/newapi-authorization/component-error-360.png",
    "docs/visual-reviews/assets/newapi-authorization/component-disabled-360.png",
    "docs/visual-reviews/assets/newapi-authorization/component-unlink-360.png",
    "docs/visual-reviews/assets/newapi-authorization/component-200-percent.png"
  ],
  "commands": [
    "node docs/visual-reviews/assets/newapi-authorization/capture.mjs (Playwright screenshot capture)",
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend exec vitest run src/components/account/__tests__/NewAPIUpstreamConfigDialog.spec.ts src/components/account/__tests__/UpstreamBillingRateCell.spec.ts src/views/admin/__tests__/AccountsView.newAPIConfig.spec.ts src/api/__tests__/admin.accounts.newAPIConfig.spec.ts"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "Actual component: Tab/Shift+Tab wrap, Escape closes after transition, reopened token is blank."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "All 16 component scenarios render with reduced-motion enabled."
    }
  },
  "residual_risks": [
    "Component captures compile the real Vue dialog with synthetic API fixtures and do not represent an authenticated full AccountsView or production acceptance.",
    "Production administrator, regular-user and guest browser acceptance remains required."
  ]
}
-->

## Scope

Add an administrator-only action in the existing account billing cell to preview,
save and unlink a New API site authorization. The dialog displays masked token
inventory, never the system access token, and remains inside the established
account-table workflow.

## Baseline

The #432 account table had upstream rate probing and no New API site-authorization
configuration action. The baseline artifact is an existing capture of the same
account billing column. The existing screenshot predates this configuration flow; it is used only as the
billing-column baseline.

## Prototype

`prototype-1280.png` and `prototype-360.png` are browser-rendered static HTML
prototype captures. They record the intended dialog hierarchy and narrow layout;
they are explicitly not represented as screenshots of the application.

## Reuse Decision

Reuse `UpstreamBillingRateCell`, the existing `Icon`, `BaseDialog`, shared input
and button classes, account API module, and account table. No new layout system,
page shell, or icon drawing is introduced.

## State Coverage

Default, loading, disabled, candidate selection, error, preview, success, token
clearing, and unlink confirmation have component tests. The configure action is
rendered only for administrators. Hover and keyboard focus use existing button
behavior and focus-visible styling.

## Viewport Coverage

The actual compiled dialog was captured at 360, 768, 1280 and 1920px, in both
languages and themes, with reduced motion. All 16 combinations had no horizontal
overflow or browser runtime errors. A 200% zoom capture also verified that the
preview action remained reachable. At 360px the shared modal body scrolls while
the footer stays reachable.

## Evidence

- Prototype capture: Chromium headless rendered the static HTML at both documented viewports; the source HTML and decoded PNG files are checked in together.
- Actual component evidence: `capture.mjs` builds the real Vue dialog in memory and intercepts every request at the synthetic `newapi-review.invalid` origin. No server is started and no upstream call is made. The complete matrix and keyboard checks are recorded in `capture-results.json`.
- Component behavior: 24 NewAPI/API/AccountsView/rate-cell tests passed; the access token is an input-only secret and candidate keys are masked by the API response. Error, missing encryption and unlink-confirmation states have separate actual component captures.
- Keyboard: Tab and Shift+Tab wrap inside the shared dialog, Escape closes after the leave transition, and reopening clears the access token. The initial immediate DOM assertion was corrected to wait for the existing transition to detach the element.
- Baseline: `api-key-multi-group/updated-billing-1280.png` is an existing account billing-column screenshot. It does not show this new flow.

## Residual Risk

- The updated captures show the actual compiled Vue dialog with synthetic data. They do not show authenticated AccountsView integration or production data; those remain part of final browser acceptance.
- Browser acceptance with administrator, regular-user, and guest identities remains part of the release workflow.
