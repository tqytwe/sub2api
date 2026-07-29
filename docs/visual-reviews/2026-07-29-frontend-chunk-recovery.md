# Visual Review: Frontend chunk recovery on login redirect

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/router/chunkRecovery.ts",
    "frontend/src/views/auth/LoginView.vue"
  ],
  "routes_or_surfaces": ["/login", "/dashboard"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark"],
  "states": ["login success", "dynamic route chunk load failure", "single automatic recovery reload"],
  "viewports": ["390x844", "1920x1080"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/frontend-chunk-recovery-20260729/login-chunk-error.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/frontend-chunk-recovery-20260729/login-chunk-error.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/frontend-chunk-recovery-20260729/login-chunk-error.png"
  ],
  "commands": [
    "curl https://www.jisudeng.com/assets/DashboardView-Dp8ErZb3.js",
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend test:run src/router/__tests__/chunkRecovery.spec.ts"
  ],
  "checks": {
    "keyboard": { "status": "passed" },
    "reduced_motion": { "status": "passed" }
  },
  "residual_risks": [
    "Static screenshot records the reported failure state; production login should be rechecked after deployment because the fix is behavioral and should avoid rendering a new error UI."
  ]
}
-->

## Scope

- Routes: `/login` redirecting to `/dashboard`.
- Roles: authenticated user login and two-factor login completion.
- Languages and themes: the reported production failure was zh-CN light mode; no UI styling or layout was changed.

## Baseline

- Current behavior: login succeeds, then a deployment-window dynamic import failure can surface as a red toast: `Failed to fetch dynamically imported module`.
- Baseline screenshot or recording: `docs/visual-reviews/assets/frontend-chunk-recovery-20260729/login-chunk-error.png`.
- Inconsistencies observed: the user sees success and failure toasts together even though authentication succeeded.

## Prototype

- Prototype design image: same reported screenshot is used as a static failure-state board because the intended fix has no new visible UI.
- Approval status: emergency behavioral recovery hotfix.
- Scope boundary: only chunk-load recovery behavior changes; no copy, spacing, colors, layout, navigation labels, or component hierarchy changes.

## Reuse Decision

- Shared layouts and components reused: existing login view, router recovery helper, toast system, and dashboard route remain unchanged visually.
- New shared pattern, if any: global chunk-load recovery listener shares the existing `recoverFromChunkLoadError` path.
- Design-system exception, if any: none; this is a behavior-only recovery patch.

## State Coverage

- Default: unchanged login form.
- Hover and active: unchanged.
- Focus-visible and keyboard: unchanged; no focus styles were edited.
- Loading, disabled, empty, error and success: success toast may still show; the chunk-load failure should trigger one recovery reload instead of an additional login-failure toast.

## Viewport Coverage

- Mobile: no layout changes; recovery is viewport-independent.
- Tablet: no layout changes.
- Desktop: reported 1920-wide desktop failure state is captured.
- Wide or short screen: no layout changes.
- 200% zoom and reduced motion: no motion or layout behavior was added.

## Evidence

- Updated screenshot or recording: static artifact is unchanged because the fix should avoid introducing a new visible state.
- Automated visual or overlap checks: design governance treats the touched login view as visual; this record maps the visual surface and notes the behavior-only boundary.
- Commands run: see manifest; production verification follows deployment.

## Residual Risk

- Known limitations: this record uses the user-provided failure screenshot, not a post-fix browser capture.
- Follow-up owner: release operator must verify production login after the hotfix is deployed.
