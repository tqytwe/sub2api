# Visual review: Excel OAuth binding in the account editor

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/components/account/ExcelOAuthBinding.vue",
    "frontend/src/i18n/locales/zh/admin/accounts.ts",
    "frontend/src/i18n/locales/en/admin/accounts.ts"
  ],
  "routes_or_surfaces": ["/admin/accounts — existing OpenAI OAuth account editor"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en/light"],
  "states": ["not-connected", "authorization-callback", "connected-capabilities", "status-error", "loading", "disabled"],
  "viewports": ["360x800", "1280x900"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/account-policy-ui/baseline-edit.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/account-policy-ui/baseline-edit.png"],
  "updated_artifacts": [
    "docs/visual-reviews/assets/bps-excel-oauth/callback-360.png",
    "docs/visual-reviews/assets/bps-excel-oauth/connected-1280.png",
    "docs/visual-reviews/assets/bps-excel-oauth/error-360.png"
  ],
  "commands": [
    "/data/cache/pnpm/bin/pnpm exec vitest run src/components/account/__tests__/ExcelOAuthBinding.capture.spec.ts",
    "/data/cache/pnpm/bin/pnpm exec tailwindcss -i ./src/style.css -o /data/tmp/bps-excel-oauth-visual/style.css --minify",
    "Chromium browser CDP screenshot capture of mounted ExcelOAuthBinding DOM at 360x800 and 1280x900"
  ],
  "checks": {
    "keyboard": {"status": "not-applicable", "reason": "The panel uses native buttons, link, and textarea with no custom keyboard interaction; component tests exercise callback and button actions."},
    "reduced_motion": {"status": "passed", "reason": "Captures used prefers-reduced-motion: reduce; this panel adds no animation."}
  },
  "residual_risks": [
    "Captures mount the actual Vue component with synthetic safe API fixtures and the production stylesheet; they are not authenticated production screenshots.",
    "The baseline is an existing account-editor screenshot reused to verify shared visual patterns; no new prototype was invented.",
    "After deployment, verify the panel in the real administrator account editor. This UI connects credentials and discovers models; BPS request routing remains a separate implementation task."
  ]
}
-->

## Scope

Add the account-scoped Excel OAuth controls to the existing OpenAI OAuth account editor. No navigation, page, account type, or user-facing entry is added. Only the safe authorization state and filtered model capabilities are displayed.

## Baseline

The reused `baseline-edit.png` shows the existing account-editor visual system. It is a local account-editor screenshot, not a current authenticated production capture. The new panel follows the existing card, form, button, and dark-theme patterns.

## Prototype

No new prototype was created. The existing account-editor baseline is reused because the change adds a real component within that surface and introduces no new design pattern.

## Reuse Decision

The panel is mounted inside `EditAccountModal.vue` for non-shadow, non-PAT OpenAI OAuth accounts. It uses the existing form controls and account API module, with no new route or shared design primitive.

## State Coverage

- Not connected: status and connect action are shown.
- Callback: authorization link, callback input, and disabled bind action are visible until a code is supplied.
- Connected: safe expiry/update metadata and discovered allowlisted models are shown; credentials are never rendered.
- Error: status failures show a local summary without echoing server response text.
- Loading and disabled states were checked in component tests. The existing modal and its controls keep native keyboard behavior.

## Viewport Coverage

Actual component DOM was captured at 360×800 and 1280×900 in Chinese light, English light, and Chinese dark mode. The checked controls remain within the viewport and no horizontal document overflow was observed.

## Evidence

The three updated PNGs show the callback-entry, connected-models, and status-error states. A temporary Vitest capture mounted the actual component; Tailwind compiled the existing stylesheet; headless Chromium rendered those DOM fragments with synthetic fixtures. The temporary capture test and rendering files are not part of the product bundle.

## Residual Risk

Final browser acceptance still requires the real administrator account editor after deployment. This binding and capability-discovery UI does not enable BPS request routing.
