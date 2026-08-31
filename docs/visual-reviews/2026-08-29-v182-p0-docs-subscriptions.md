# Visual Review: v182-p0-docs-subscriptions

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/router/customMenuTarget.ts",
    "frontend/src/router/navigationGeneration.ts",
    "frontend/src/router/index.ts",
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/components/common/SubscriptionProgressMini.vue",
    "frontend/src/views/user/SubscriptionsView.vue",
    "frontend/src/api/subscriptions.ts",
    "frontend/src/i18n/index.ts",
    "frontend/src/i18n/locales/en/core.ts",
    "frontend/src/i18n/locales/zh/core.ts",
    "frontend/src/i18n/locales/en/misc.ts",
    "frontend/src/i18n/locales/zh/misc.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/utils/platformColors.ts"
    ,"frontend/src/features/channel-monitor-v2/MetricCell.vue"
    ,"frontend/src/features/channel-monitor-v2/MonitorSettingsPanel.vue"
    ,"frontend/src/features/channel-monitor-v2/RelayPulseMatrix.vue"
    ,"frontend/src/features/channel-monitor-v2/monitorFormat.ts"
    ,"frontend/src/i18n/locales/en/channelMonitorV2.ts"
    ,"frontend/src/i18n/locales/en/dashboard.ts"
    ,"frontend/src/i18n/locales/zh/channelMonitorV2.ts"
    ,"frontend/src/i18n/locales/zh/dashboard.ts"
    ,"frontend/src/views/user/ChannelStatusV2View.vue"
    ,"frontend/src/views/user/KeysView.vue"
    ,"frontend/src/views/user/UsageView.vue"
    ,"frontend/src/components/layout/AppHeader.vue"
    ,"frontend/src/components/layout/PublicContentLayout.vue"
    ,"frontend/src/styles/public-pages.css"
    ,"frontend/src/views/user/CustomPageView.vue"
  ],
  "routes_or_surfaces": ["authenticated custom-menu docs entry", "/docs", "/en/docs", "custom-page iframe fallback privacy boundary", "locale-race navigation", "/subscriptions", "subscription progress header tooltip", "channel-monitor-v2", "API key provider labels"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "hover", "active", "focus-visible", "loading", "empty", "error", "success", "out-of-order locale navigation", "first-party iframe fallback"],
  "viewports": ["360x800", "768x900", "1280x800", "1920x1080"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/english-seo-layer/prototype-english-seo-layer.png",
    "docs/visual-reviews/assets/subscription-workspace-prototype/prototype-subscription-storefront-v3-390.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/english-seo-layer/baseline-english-seo-layer.png",
    "docs/visual-reviews/assets/subscription-workspace-prototype/user-previous-subscription-style-1920.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/english-seo-layer/updated-english-seo-layer.png",
    "docs/visual-reviews/assets/subscription-workspace-prototype/prototype-subscription-storefront-v3-1920.png"
  ],
  "commands": ["pnpm exec vitest run src/router/__tests__/customMenuTarget.spec.ts src/router/__tests__/navigationGeneration.spec.ts src/router/__tests__/routeLocaleGuardRace.spec.ts src/api/__tests__/subscriptions.spec.ts src/i18n/__tests__/routeLocaleRuntime.spec.ts src/utils/__tests__/platformColors.spec.ts src/components/layout/__tests__/AppSidebar.spec.ts src/components/common/__tests__/SubscriptionProgressMini.spec.ts", "pnpm design:check"],
  "checks": {
    "keyboard": {"status": "passed", "notes": "No new control was added; existing router links and locale controls retain keyboard focus behavior."},
    "reduced_motion": {"status": "not-applicable", "reason": "The P0 changes add no animation or motion."}
  },
  "residual_risks": ["Artifacts are prior approved static review images and not a live production capture. User local-browser acceptance must verify the authenticated menu entry, /docs redirect, and subscription copy at all required widths and themes."]
}
-->

## Scope

- Fixed-canonical-origin documentation menu resolution, the exact historic `/custom/095790f89fc04920` compatibility redirect, locale transition ordering, subscription API copy normalization, and subscription page duration/fallback labels.
- `CustomPageView` remains the third-party iframe fallback only. A same-site docs alias can never receive panel context in an iframe URL, while the strict canonical target is redirected to the native docs route before an iframe is created. The navigation generation guard gives only the latest asynchronous route guard authority to update locale, document language, and title.
- Channel monitor and API-key provider labels use the paired locale resources and
  formatting helpers. This is a copy-only change: the existing monitor table,
  settings controls, API-key layout, density, theme tokens, and interaction
  states are retained.
- External custom-page iframe targets remain unchanged and continue to use the existing “open in new tab” fallback.

## Baseline

- The production documentation menu opened `/custom/:id`, which embedded the site’s own `/docs` URL and was rejected by the target CSP because the configured parent origin was stale and protocol-mismatched. The published historic docs bookmark `095790f89fc04920` is now an exact native-route redirect and is not dependent on the current menu configuration.
- Subscription progress had a current backend envelope but an older frontend type, and the subscription page rendered `Group #id` plus `d/h/m` literals.
- Baseline references: the approved English public-layer board and previous subscription workspace board listed in the manifest.

## Prototype

- Prototype references use the existing public docs and subscription workspace visual language. No new page frame, card family, button system, or icon system was introduced.
- The native docs route intentionally reuses `DocsView` and drops all iframe query parameters; the subscription view keeps its existing layout and only replaces wire/display formatting at the boundaries.

## Reuse Decision

- Reused `DocsView`, `AppSidebar`, existing router locale guards, `platformColors`, `SubscriptionsView`, and API client conventions.
- CSP keeps the existing `frame-src` allowlist and `X-Frame-Options: SAMEORIGIN`; `frontend_url` is no longer an embedding policy input.

## State Coverage

- Default, hover, active, and focus-visible states remain owned by the existing sidebar, docs, and subscription components.
- Loading, empty, error, and success behavior is unchanged. The progress API
  boundary now normalizes current and retired wire fields for any progress
  endpoint consumer; the main subscription page intentionally continues to use
  the canonical `/subscriptions` DTO so it can retain group metadata and the
  full subscription history.
- An out-of-order locale chunk cannot replace the latest route language or document language.
- A same-site docs alias that reaches the fallback cannot receive `token`, `user_id`, `src_url`, theme, or locale panel parameters; canonical docs targets never instantiate the fallback iframe.
- Channel-monitor and provider-copy paths retain their existing loading, empty,
  error, hover, focus-visible, and keyboard behavior; only hard-coded English
  labels and abbreviations are replaced by localized equivalents.

## Viewport Coverage

- Static references cover mobile and desktop subscription layouts and bilingual public docs. Required live checks remain 360, 768, 1280, and 1920px in both themes, with 200% zoom and reduced motion.

## Evidence

- Focused automated evidence: 88 Vitest tests passed for canonical docs targets, alias-host rejection, legacy-bookmark redirect, exact-ID menu resolution, admin-settings cold-start routing, subscription normalization, locale ordering, platform labels, sidebar wiring, and the compact subscription fallback. The added store test also verifies concurrent callers share one settings request.
- The channel-monitor formatter unit tests cover English and Chinese labels,
  durations, and aggregate metrics. Because the change does not alter monitor
  geometry or controls, the existing static review artifacts are used only as
  layout references; live bilingual monitor and API-key review remain release
  acceptance requirements.
- `artifact_mode` is deliberately `static-review-board`; these PNGs are valid existing review artifacts, not browser or production screenshots.

## Residual Risk

- Production `frontend_url` must still be corrected through the audited administrator settings workflow and verified via password-reset and notification links.
- Final acceptance remains user-owned in a local browser after deployment; no production settings, database rows, or Zeabur deployment were changed in this task.
