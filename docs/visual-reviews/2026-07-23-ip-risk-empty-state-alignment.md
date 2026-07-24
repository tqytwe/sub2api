# IP Risk Empty State Alignment Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/features/ip-risk/IPRiskCaseDetail.vue",
    "frontend/src/features/ip-risk/__tests__/IPRiskCaseDetail.spec.ts",
    "docs/visual-reviews/assets/ip-risk-empty-state-alignment/prototype-ip-risk-empty-state-alignment.html"
  ],
  "routes_or_surfaces": [
    "/admin/proxies/risk",
    "IP risk case detail empty state"
  ],
  "languages_and_themes": [
    "zh-CN light static review board",
    "zh-CN and en-US product strings",
    "light and dark product token paths"
  ],
  "states": [
    "no selected risk case",
    "wide desktop split detail pane",
    "narrow desktop and mobile detail dialog"
  ],
  "viewports": [
    "360x800",
    "768x900",
    "1280x800",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/ip-risk-empty-state-alignment/prototype-ip-risk-empty-state-centered-1920.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/ip-risk-empty-state-alignment/baseline-ip-risk-empty-state-left-1920.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/ip-risk-empty-state-alignment/updated-ip-risk-empty-state-centered-1920.png"
  ],
  "commands": [
    "xvfb-run -a firefox --no-remote --profile <temporary-profile> --window-size 1920,1080 --screenshot <baseline-artifact> file://<prototype>?mode=before",
    "xvfb-run -a firefox --no-remote --profile <temporary-profile> --window-size 1920,1080 --screenshot <updated-artifact> file://<prototype>?mode=after",
    "pnpm --dir frontend exec vitest run src/features/ip-risk/__tests__/IPRiskCaseDetail.spec.ts src/__tests__/ipRiskWorkbench.spec.ts",
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend lint:check",
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend test:run",
    "pnpm --dir frontend build",
    "git diff --check"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "The empty prompt is non-interactive and the change does not alter focus order or keyboard controls."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "The change adds no animation or motion-dependent behavior."
    }
  },
  "residual_risks": [
    "The comparison artifacts are browser-rendered static review boards rather than authenticated captures of the production route.",
    "Final acceptance still requires refreshing /admin/proxies/risk in the user's local administrator browser session."
  ]
}
-->

## Scope

This review fixes only the unselected-case prompt in the wide IP risk workbench. The prompt now consumes the available detail-pane width and centers its copy horizontally and vertically. Risk detection, case selection, account evidence, action preview, policies, API calls and automation behavior are unchanged.

## Baseline

The detail pane is a horizontal flex container. Before this fix, the empty-state component had centering utilities but no grow or width utility, so it collapsed to the width of its text. Horizontal centering therefore had no available space and the prompt appeared against the left edge of the detail pane.

Baseline artifact: `docs/visual-reviews/assets/ip-risk-empty-state-alignment/baseline-ip-risk-empty-state-left-1920.png`.

## Prototype

The approved correction keeps the existing typography and semantic colors. It adds `flex-1` so the empty state owns the full detail pane, `text-center` for wrapped Chinese or English copy, and horizontal padding so long translations remain readable at constrained widths.

Prototype artifact: `docs/visual-reviews/assets/ip-risk-empty-state-alignment/prototype-ip-risk-empty-state-centered-1920.png`.

## Reuse Decision

The existing `IPRiskWorkbench` split layout and `IPRiskCaseDetail` component remain unchanged structurally. This is a local utility-class correction and does not introduce a new empty-state component or parallel design pattern.

## State Coverage

The regression test mounts `IPRiskCaseDetail` without a selected case and asserts that the empty state owns available flex space and retains horizontal and vertical centering. Selected-case content and its tabs are unaffected.

## Viewport Coverage

At `1920px`, the prompt is centered within the right-hand split pane. Below the `2xl` breakpoint, the workbench uses the existing shared dialog path; `flex-1` also makes the prompt fill that dialog's column container. At `360px`, `768px` and `1280px`, `px-6` and `text-center` protect wrapped Chinese and English copy without introducing a fixed width. The change uses no viewport-dependent font size or page-owned width.

Light and dark themes continue to use the existing gray text tokens. Chinese and English use the same layout contract. At 200 percent zoom, wrapping remains centered within the available pane.

## Evidence

Updated artifact: `docs/visual-reviews/assets/ip-risk-empty-state-alignment/updated-ip-risk-empty-state-centered-1920.png`.

The static comparison board reproduces the relevant flex behavior without production data. Automated evidence includes the focused component regression, the existing workbench tests, frontend governance and build gates, repository tests, and whitespace validation.

## Residual Risk

The static board proves the layout mechanism and visual intent, but it is not an authenticated production screenshot. Final browser acceptance remains the user's local administrator session after deployment, where `/admin/proxies/risk` should be refreshed and checked with no case selected.
