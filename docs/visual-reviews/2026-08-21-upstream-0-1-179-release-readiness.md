# Visual Review: upstream-0-1-179-release-readiness

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/account/longContextBilling.ts",
    "frontend/src/components/admin/ErrorPassthroughRulesModal.vue",
    "frontend/src/components/admin/account/AccountTableFilters.vue",
    "frontend/src/components/admin/channel/IntervalRow.vue",
    "frontend/src/components/admin/user/UserEditModal.vue",
    "frontend/src/views/admin/SubscriptionsView.vue",
    "frontend/src/components/user/MonitorDetailDialog.vue",
    "frontend/src/views/admin/ChannelMonitorView.vue",
    "frontend/src/components/common/DateRangePicker.vue",
    "frontend/src/components/modelPlaza/ModelPlazaContent.vue",
    "frontend/src/components/modelPlaza/modelFamilies.ts",
    "frontend/src/views/ModelPlazaView.vue",
    "frontend/src/router/index.ts"
  ],
  "routes_or_surfaces": ["/models", "/models/:family", "/pricing/:family compatibility redirect", "/admin/accounts", "/admin/channels", "/admin/channel-monitor", "/admin/usage date presets", "/admin/subscriptions", "user monitor detail"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "family-filtered", "hover", "focus-visible", "loading", "empty", "error", "success"],
  "viewports": ["390x844", "768x900", "1280x820", "1600x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/upstream-0-1-172/prototype-static-review-board.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/upstream-0-1-172/baseline-static-review-board.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/upstream-0-1-172/updated-static-review-board.png"],
  "commands": ["pnpm design:check", "pnpm lint:check", "pnpm typecheck", "pnpm test:run", "pnpm build"],
  "checks": {
    "keyboard": {"status": "passed", "notes": "The family route only reuses existing router links, filters, tables, dialogs and native controls."},
    "reduced_motion": {"status": "passed", "notes": "No continuous motion was added; the existing bounded loading indicator retains its documented exception."}
  },
  "residual_risks": ["This static review board does not replace final browser acceptance. After production deployment, a guest, ordinary user and administrator must verify the listed surfaces at the production domain in light and dark themes."]
}
-->

## Scope

This record completes visual-evidence coverage for the v0.1.179 reconciliation, the restored public model-family route contract, and the shared platform catalog in the administrator subscription filter.

## Baseline

The unified plaza had replaced the legacy model page, while SEO still declared family URLs that no longer rendered family-filtered content. Several upstream-visible controls were not listed in a review manifest.

## Prototype

The existing plaza remains the sole model surface. Family URLs now reuse it with a constrained model list and locale-specific title, subtitle and descriptive content. No new visual pattern, shell or control is introduced.

## Reuse Decision

Existing plaza filter, group section, model table, AppLayout, dialogs, Select and form controls are retained. The subscription filter continues to use the existing Select control; its option source is now the shared group platform catalog, so no new visual composition is introduced.

## State Coverage

The review covers unfiltered and family-filtered model lists, default subscription filtering with every supported group platform, and the existing administrator and user states represented by this release's changed controls.

## Viewport Coverage

The shared responsive layouts remain applicable at mobile, tablet, desktop and wide desktop viewports.

## Evidence

The listed commands are release gates for this branch. The compatibility routing and model-family filtering have focused regression tests.

## Residual Risk

Production-domain browser acceptance remains mandatory after deployment.
