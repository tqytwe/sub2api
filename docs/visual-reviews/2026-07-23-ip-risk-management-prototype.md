# IP Risk Management Workbench Prototype Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-management.html"
  ],
  "routes_or_surfaces": [
    "/admin/proxies",
    "/admin/proxies/risk",
    "/admin/proxies/actions",
    "IP risk case detail dialog",
    "IP risk action preview dialog",
    "IP risk detection policy"
  ],
  "languages_and_themes": [
    "zh-CN light static prototype",
    "en-US and dark theme implementation contract"
  ],
  "states": [
    "existing IP resources baseline",
    "risk workbench selected case",
    "exact and inferred evidence",
    "temporary registration block active",
    "trusted and protected accounts excluded",
    "action impact preview",
    "step-up TOTP required",
    "shadow calibration active",
    "runtime healthy",
    "shared network and allowlist policy"
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
    "docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-workbench-desktop.png",
    "docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-case-detail-desktop.png",
    "docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-action-preview-desktop.png",
    "docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-policy-desktop.png",
    "docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-mobile.png"
  ],
  "commands": [
    "xvfb-run -a firefox --no-remote --profile <temporary-profile> --window-size 1920,1080 --screenshot <artifact> file://<prototype>?view=<desktop-view>",
    "xvfb-run -a firefox --no-remote --profile <temporary-profile> --window-size 390,844 --screenshot <artifact> file://<prototype>?view=mobile",
    "pnpm --dir frontend design:check",
    "git diff --check"
  ],
  "checks": {
    "keyboard": {
      "status": "not-applicable",
      "reason": "CP0 is a non-interactive static prototype; keyboard behavior remains an implementation acceptance item."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "The CP0 prototype contains no animation or transition behavior."
    }
  },
  "residual_risks": [
    "These are browser-rendered static prototype boards, not screenshots of implemented product routes; real authenticated browser screenshots and final acceptance are required after implementation.",
    "Dark theme, English copy, 1280px dialog fallback, 768px tablet layout, keyboard flow and 200 percent zoom remain implementation acceptance items.",
    "Counts, IP addresses, users, timestamps and policy entries are simulated and do not validate production data quality or scoring accuracy."
  ]
}
-->

## Scope

CP0 defines the product and interaction direction for upgrading the existing IP management area into one workbench with three tabs: IP resources, risk detection and action history. It covers proactive batch-registration discovery, explainable evidence, related-account review, action preview, step-up confirmation, rollback boundaries, runtime status and policy controls.

This checkpoint changes documentation and prototype artifacts only. It does not modify `frontend/src`, backend services, routes, database migrations, APIs, user state, API Key state or production configuration.

## Baseline

The current `/admin/proxies` surface is represented as an IP resource table focused on proxy inventory, connection quality and upstream account association. It has no visible risk-event list, evidence scoring, related-account review, action preview or registration-block policy management.

Baseline artifact: `docs/visual-reviews/assets/ip-risk-management/baseline-current-ip-management.png`.

The baseline is a repository-grounded reconstruction for design comparison, not a production browser acceptance screenshot.

## Prototype

The workbench keeps `/admin/proxies` compatible as the default IP resources entry and adds risk detection and action history beside it. Wide desktop uses a list-detail split so an administrator can compare cases without losing filter context. Narrow desktop moves the detail into the existing dialog pattern. Mobile converts cases and accounts into scan-friendly cards with a fixed action footer.

The five requested prototype artifacts are:

- `docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-workbench-desktop.png`
- `docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-case-detail-desktop.png`
- `docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-action-preview-desktop.png`
- `docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-policy-desktop.png`
- `docs/visual-reviews/assets/ip-risk-management/prototype-ip-risk-mobile.png`

Approval status: approved by the user on 2026-07-23; CP1 implementation may proceed.

Scope boundary: no product implementation begins until the user confirms the prototype direction.

## Reuse Decision

Implementation should reuse the existing `AppLayout`, `TablePageLayout`, `DataTable`, `BaseDialog`, button, badge, input, checkbox, pagination, toast and semantic color contracts. The prototype intentionally follows the current operational density, 8px card radius and restrained status colors instead of introducing a parallel visual system.

The risk score ring, evidence contribution rows, runtime status list and protected-account treatment are proposed compositions. During implementation they should be built from shared primitives where possible, with any genuinely reusable risk pattern promoted deliberately rather than copied between pages.

## State Coverage

The prototype shows a selected severe case, exact evidence, inferred-history labeling, an active 30-minute registration-only block, shared-network suppression, protected trusted accounts, default account selection, action impact preview, TOTP step-up, preview expiry, rollback eligibility, shadow calibration and healthy runtime status.

Implementation must additionally cover loading, empty, request error, degraded scanner, stale preview `409`, partial action failure, disabled/resolved cases, expired policies, scan progress and rollback conflict. Destructive actions must remain unavailable until preview, reason and required TOTP checks pass.

## Viewport Coverage

At `1920x1080`, the risk list and selected case use a split workbench. At widths below 1536px, the implementation contract is a full-width list with `BaseDialog` detail instead of squeezing both columns. At `390x844`, the prototype uses cards, compact evidence badges and a fixed two-action footer.

The implementation acceptance matrix remains `360px`, `768px`, `1280px` and `1920px`, plus 200 percent zoom. The current CP0 PNGs directly prove only `390x844` and `1920x1080`; `1280x800` is recorded as the dialog-fallback contract for CP1 implementation.

## Evidence

All six PNGs were rendered from the static HTML prototype with Firefox 136 under Xvfb. The artifacts use documentation-safe IP ranges and `example.test` accounts; no production user or network data is present.

Commands run:

```bash
xvfb-run -a firefox --no-remote --profile <temporary-profile> --window-size 1920,1080 --screenshot <artifact> file://<prototype>?view=<desktop-view>
xvfb-run -a firefox --no-remote --profile <temporary-profile> --window-size 390,844 --screenshot <artifact> file://<prototype>?view=mobile
pnpm --dir frontend design:check
git diff --check
```

Visual inspection confirmed that the desktop list-detail layout, case evidence table, action preview dialog and policy side panel fit inside the target canvas without clipping. The mobile fixed action footer preserves the primary preview action and keeps trusted accounts visibly unselected.

## Residual Risk

The artifacts are static review boards rendered in a real browser, not captures of implemented authenticated routes. After CP1 and CP2, real product screenshots must verify shared-component fidelity, live data overflow, scroll ownership, dialogs at 1280px, mobile safe areas, keyboard focus order, dark mode, English expansion and 200 percent zoom.

The scoring and account selections shown here are illustrative. Detection precision, historical inference quality, protection rules, preview staleness and rollback safety require backend tests and at least 24 hours of shadow-mode calibration before any automatic registration block is enabled.
