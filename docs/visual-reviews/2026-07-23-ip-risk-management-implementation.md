# IP Risk Management Workbench Implementation Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/index.ts",
    "frontend/src/api/admin/ipRisk.ts",
    "frontend/src/features/ip-risk/types.ts",
    "frontend/src/features/ip-risk/IPRiskWorkbench.vue",
    "frontend/src/features/ip-risk/IPRiskCaseDetail.vue",
    "frontend/src/features/ip-risk/IPRiskActionDialog.vue",
    "frontend/src/features/ip-risk/IPRiskPolicyDialog.vue",
    "frontend/src/features/ip-risk/IPRiskActionsView.vue",
    "frontend/src/views/admin/ProxiesView.vue",
    "frontend/src/router/index.ts",
    "frontend/src/i18n/locales/zh/admin/resources.ts",
    "frontend/src/i18n/locales/en/admin/resources.ts",
    "docs/visual-reviews/assets/ip-risk-management/implementation-ip-risk-workbench.html"
  ],
  "routes_or_surfaces": [
    "/admin/proxies",
    "/admin/proxies/risk",
    "/admin/proxies/actions",
    "IP risk case detail dialog",
    "IP risk action preview and TOTP flow",
    "IP risk policy and automation dialog",
    "IP risk rollback dialog"
  ],
  "languages_and_themes": [
    "zh-CN light static implementation review board",
    "zh-CN and en-US product strings",
    "light and dark product token paths"
  ],
  "states": [
    "healthy shadow runtime",
    "degraded runtime",
    "loading",
    "empty and filtered empty",
    "selected critical case",
    "exact, inferred, and mixed evidence",
    "trusted and protected accounts excluded",
    "scan progress and failure",
    "preview ready and stale",
    "TOTP step-up",
    "completed, partial, failed, and rollback conflict",
    "allowlist, shared network, observe, temporary block, and permanent block policies"
  ],
  "viewports": [
    "390x844",
    "1280x800",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-workbench-desktop.png",
    "docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-case-detail-desktop.png",
    "docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-action-preview-desktop.png",
    "docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-policy-desktop.png",
    "docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-mobile.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/ip-risk-management/baseline-current-ip-management.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/ip-risk-management/implementation-ip-risk-workbench-1920.png",
    "docs/visual-reviews/assets/ip-risk-management/implementation-ip-risk-dialog-1280.png",
    "docs/visual-reviews/assets/ip-risk-management/implementation-ip-risk-mobile-390.png"
  ],
  "commands": [
    "xvfb-run -a firefox --no-remote --profile <temporary-profile> --window-size 1920,1080 --screenshot <artifact> file://<implementation-board>",
    "xvfb-run -a firefox --no-remote --profile <temporary-profile> --window-size 1280,800 --screenshot <artifact> file://<implementation-board>",
    "xvfb-run -a firefox --no-remote --profile <temporary-profile> --window-size 390,844 --screenshot <artifact> file://<implementation-board>",
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend lint:check",
    "pnpm --dir frontend typecheck"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "Interactive controls use native buttons, inputs and shared Select/BaseDialog focus management; final authenticated browser traversal remains required."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "The new feature does not add continuous spin, pulse, transition-all, layout motion, or animation-dependent status."
    }
  },
  "residual_risks": [
    "These artifacts are static code-grounded review boards, not authenticated captures of the implemented production routes.",
    "Final local-browser acceptance must still verify actual API data overflow, focus order, 200 percent zoom, English expansion, dark theme, and mobile safe-area behavior.",
    "Production automatic registration blocking remains disabled until the required shadow calibration checkpoint."
  ]
}
-->

## Scope

The existing `/admin/proxies` page now owns one IP management workspace with three route-compatible tabs: IP resources, risk detection, and action history. The risk implementation includes runtime status, overview metrics, filters, asynchronous scans, list/detail behavior, evidence and related-user review, impact preview, TOTP step-up, policy management, partial results, and safe rollback.

## Baseline

The baseline artifact records the existing `/admin/proxies` resource-management page before the risk workspace was added. The existing proxy table, filters, resource checks, account associations, dialogs, and route behavior remain intact and continue to be the default `/admin/proxies` surface.

## Prototype

The implementation follows the user-approved prototype artifacts under `docs/visual-reviews/assets/ip-risk-management/`. The desktop workbench retains the wide split view at 1536px and above. Narrow desktop moves details into the shared `BaseDialog`; mobile uses the same detail content as a full-width flow with the important action footer kept reachable.

## Reuse Decision

`ProxiesView.vue` remains the only `AppLayout` owner. The implementation reuses `TablePageLayout`, `BaseDialog`, `Pagination`, `Select`, `Icon`, existing button/badge/input classes, `useStepUp`, `TotpStepUpDialog`, app toasts, semantic status colors, and the route workspace frame. It adds no parallel page shell, icon set, modal system, or raw color contract.

## State Coverage

The product code explicitly handles loading, empty, filtered empty, request error, degraded runtime, shadow runtime, running/completed/failed scans, resolved and ignored cases, exact/inferred/mixed evidence, protected administrators, trusted funded accounts, preview expiry/staleness, step-up cancellation, partial action results, policy expiry, rollback eligibility, and rollback conflicts.

Automatic blocking is configurable but defaults off in migration `215`. When later enabled, it remains exact-IP and registration-only; the UI explains that login and existing API access are unaffected.

## Viewport Coverage

The static implementation board records:

- `1920x1080`: route workspace with list/detail split.
- `1280x800`: list route with shared-dialog detail fallback.
- `390x844`: mobile full-width detail and fixed primary action flow.

The product CSS itself uses the shared responsive page frame and a 1536px split threshold. No feature root owns page width, global scroll, or a private centered container.

## Evidence

The updated artifacts are rendered from a documentation-safe implementation review board and use reserved IP ranges and simulated counts only. They are intended for code and visual review, not as production acceptance evidence.

## Residual Risk

Authenticated route screenshots require a real administrator session and live APIs. Per the repository delivery policy, server-side screenshots cannot replace the user’s final local browser acceptance. After deployment, the user must verify guest/user/admin boundaries plus admin risk workflows at 360/768/1280/1920 widths, light/dark themes, Chinese/English, keyboard navigation, and 200 percent zoom.
