# 管理用量按请求时平台筛选

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/dashboard.ts",
    "frontend/src/api/admin/usage.ts",
    "frontend/src/components/admin/usage/UsageFilters.vue",
    "frontend/src/components/admin/usage/__tests__/UsageFilters.spec.ts",
    "frontend/src/components/charts/EndpointDistributionChart.vue",
    "frontend/src/components/charts/GroupDistributionChart.vue",
    "frontend/src/components/charts/ModelDistributionChart.vue",
    "frontend/src/components/common/NewFeatureBadge.vue",
    "frontend/src/components/common/__tests__/NewFeatureBadge.spec.ts",
    "frontend/src/constants/newFeatureNotices.ts",
    "frontend/src/i18n/locales/en/admin/resources.ts",
    "frontend/src/i18n/locales/en/common.ts",
    "frontend/src/i18n/locales/zh/admin/resources.ts",
    "frontend/src/i18n/locales/zh/common.ts",
    "frontend/src/stores/newFeatureNotices.ts",
    "frontend/src/views/admin/UsageView.vue",
    "frontend/src/views/admin/__tests__/UsageView.platform.spec.ts"
  ],
  "routes_or_surfaces": ["/admin/usage — summary charts and usage-detail filters"],
  "languages_and_themes": ["zh/light", "en/light", "zh/dark", "en/dark"],
  "states": [
    "all platforms (default)",
    "each supported platform",
    "unknown historical snapshot",
    "active platform disables cleanup",
    "switching to unsupported detail tabs clears platform",
    "platform query applied to list, stats and dashboard charts",
    "export pins the starting platform and other filters",
    "platform filter disables chart user drilldown and clears stale breakdowns",
    "keyboard selection through shared Select",
    "per-administrator feature announcement dismissal"
  ],
  "viewports": ["360x900", "768x900", "1280x720", "1920x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-26-usage-platform-filter/prototype-360.png",
    "docs/visual-reviews/assets/2026-11-26-usage-platform-filter/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-26-usage-platform-filter/baseline-360.png",
    "docs/visual-reviews/assets/2026-11-26-usage-platform-filter/baseline-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-26-usage-platform-filter/platform-active-360.png",
    "docs/visual-reviews/assets/2026-11-26-usage-platform-filter/platform-active-768.png",
    "docs/visual-reviews/assets/2026-11-26-usage-platform-filter/platform-active-1280.png",
    "docs/visual-reviews/assets/2026-11-26-usage-platform-filter/platform-active-1920.png"
  ],
  "commands": [
    "node /tmp/usage-platform-prototype.cjs (Playwright Chromium static review board; not the application)",
    "pnpm --dir frontend exec vitest run src/views/admin/__tests__/UsageView.platform.spec.ts src/components/admin/usage/__tests__/UsageFilters.spec.ts src/components/common/__tests__/NewFeatureBadge.spec.ts",
    "pnpm --dir frontend run typecheck"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "UsageView.platform.spec.ts opens the shared Select with ArrowDown, searches OpenAI, and selects with ArrowDown/Enter; list and stats receive the selected platform."},
    "reduced_motion": {"status": "not-applicable", "reason": "No animated interaction was introduced; the notice badge's color transition explicitly disables under motion-reduce, and Select motion remains the existing shared pattern."}
  },
  "residual_risks": [
    "The baseline, prototype, and active-platform PNGs are a static review board, not screenshots of the running application. Capture and inspect the actual admin page after deployment, then complete the required local-browser guest/user/admin acceptance."
  ]
}
-->

## Scope

- Route: `/admin/usage`, the administrator usage page.
- Languages/themes: Chinese and English; light and dark mode.
- Platform means the immutable request-time provider snapshot. Current account or group metadata is not used to backfill historical rows.

## Baseline

- Before this change the filters had no request-time platform selector; all other filters and actions used the shared `UsageFilters` layout.
- Static baseline boards: `assets/2026-11-26-usage-platform-filter/baseline-360.png` and `baseline-1280.png`.

## Prototype

- Pre-implementation proposal: `assets/2026-11-26-usage-platform-filter/prototype-360.png` and `prototype-1280.png`.
- Boundary: add one usage-only platform selector; keep the existing shared filter and chart components; explain legacy unknown rows; prevent unsupported cleanup and chart breakdown operations while a platform predicate is active.

## Reuse Decision

- Reused the existing `Select`, `UsageFilters`, dashboard chart components, shared spacing and semantic text colors.
- Reused the existing concrete platform catalog; added only the `unknown` sentinel for historical rows.
- Added the minimal per-administrator new-feature notice store/badge because this fork did not contain the upstream notice infrastructure.
- No design-system exception.

## State Coverage

- Default “All platforms” leaves the list's approximate-total path unchanged.
- Concrete and unknown selections are sent to list, statistics, model/group/trend dashboard queries; filtered lists request exact totals.
- Cleanup is disabled and guarded at the handler boundary while any platform is selected because the cleanup API cannot express a platform predicate.
- Switching to errors, ranking, connections or latency clears the usage-only platform.
- Model/group/endpoint user-breakdown rows are disabled while platform filtering is active; in-flight breakdown results are discarded after scope changes.
- Export captures all filters before its first asynchronous operation and reuses them on each page.
- The feature badge can be dismissed for the active administrator and persists in that browser.
- Default, error, empty, loading, success and disabled states otherwise reuse existing components and request handling. Keyboard behavior is covered by the integration-style component test; no custom hover-only action was added.

## Viewport Coverage

- Static proposal and active-state boards were rendered at 360, 768, 1280 and 1920 CSS pixels. At 360px fields stack and actions wrap; tablet uses three filter columns; desktop/wide layouts use four columns.
- Dark theme, 200% browser zoom and short-height scroll behavior remain for actual browser acceptance; these PNGs intentionally do not claim those as application captures.

## Evidence

- Static-board assets are generated from an isolated HTML layout with Playwright; they show the intended layout only.
- Real component behavior is verified by `UsageView.platform.spec.ts`, `UsageFilters.spec.ts`, and `NewFeatureBadge.spec.ts`; type-check passed.
- Production application screenshot and cross-theme visual inspection are pending deployment and local browser acceptance.

## Residual Risk

- These assets must not be represented as screenshots of the real application. The final visual/runtime check must use the deployed `/admin/usage` route with a test administrator and must avoid production secrets or user data.
