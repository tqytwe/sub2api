# Visual Review: V3 Channel Monitor

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/features/channel-monitor-v3/StatusPage.vue",
    "frontend/src/features/channel-monitor-v3/StatusBar.vue",
    "frontend/src/features/channel-monitor-v3/StatusDot.vue",
    "frontend/src/features/channel-monitor-v3/V3SettingsPanel.vue",
    "frontend/src/features/channel-monitor-v3/IncidentsDialog.vue",
    "frontend/src/features/channel-monitor-v3/IdleProbeSettings.vue",
    "frontend/src/features/channel-monitor-v3/ComponentEditorDialog.vue",
    "frontend/src/features/channel-monitor-v3/monitorV3.ts",
    "frontend/src/views/user/ChannelStatusV3View.vue",
    "frontend/src/views/user/ChannelStatusView.vue",
    "frontend/src/views/admin/ChannelMonitorView.vue",
    "frontend/src/views/admin/SettingsView.vue",
    "frontend/src/i18n/index.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en/index.ts",
    "frontend/src/i18n/locales/zh/index.ts",
    "frontend/src/i18n/locales/en/admin/settings.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts",
    "frontend/src/i18n/locales/zh/channelMonitorV3.ts",
    "frontend/src/i18n/locales/en/channelMonitorV3.ts"
  ],
  "routes_or_surfaces": ["/monitor", "/admin/channel-monitor"],
  "languages_and_themes": ["zh-CN/light"],
  "states": ["default", "status colors", "incident badge", "responsive category layout"],
  "viewports": ["390x844", "1440x1000"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/channel-monitor-v3/upstream-user-reference.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-08-28-channel-monitor-availability/baseline-1440.png"],
  "updated_artifacts": [
    "docs/visual-reviews/assets/channel-monitor-v3/mobile-390.png",
    "docs/visual-reviews/assets/channel-monitor-v3/desktop-1440.png"
  ],
  "commands": ["Playwright Chromium capture of the actual StatusPage.vue component using deterministic fixture data"],
  "checks": {
    "keyboard": { "status": "passed", "notes": "Tooltip interaction and tab keyboard behavior are covered by component tests." },
    "reduced_motion": { "status": "passed", "notes": "V3 status and loading surfaces use static states; no continuous animation is required." }
  },
  "residual_risks": ["Screenshots use deterministic fixture data and do not represent production traffic or authenticated shell navigation."]
}
-->

## Scope

- Routes: `/monitor` and admin channel monitor settings.
- Roles: authenticated users see public V3 status; admins configure V3 settings.
- Language and theme: Chinese light theme.

## Baseline

- Before this change, the user route selected only V1 and V2, and the V3 component page had no local production surface.
- Historical monitor baseline: [baseline-1440.png](assets/2026-08-28-channel-monitor-availability/baseline-1440.png). It documents the existing V2 monitor surface and is used only as a prior-state reference.
- The captures show the actual V3 `StatusPage.vue` component; the fixture is deterministic test data, not production traffic.
- V3 mode now selects the V3 status view while V2 continues to use its existing route component.

## Prototype

- Upstream reference: [channel-monitor-v3-user-upstream.png](prototypes/channel-monitor-v3-user-upstream.png). This is reference evidence for upstream behavior, not an implementation mockup.
- Scope is the V3 component status page and administrator configuration; no V3-only traffic system was added.

## Reuse Decision

- Reuses `AppLayout`, existing V2 passive aggregation, shared form controls, and existing V1 account-test scheduling path.
- Status colors use the existing Tailwind semantic palette. Loading states remain static and corner radii use the project 8px limit.
- No design-system exception was required.

## State Coverage

- Default and status states: operational, degraded, outage, insufficient traffic, availability, and incident badge are visible in the fixtures.
- Interactive states: category collapse, history paging, tooltip pointer/focus behavior and incident dialog are covered by component tests.
- Loading, empty and failure states are covered by `StatusPage`, `V3SettingsPanel`, and `IncidentsDialog` tests.
- Reduced motion: no continuous animation is used by the V3 surfaces.

## Viewport Coverage

- Mobile: 390x844; controls wrap and cards stay within the viewport.
- Tablet: no dedicated capture; layout uses the same one-column grid up to the desktop breakpoint.
- Desktop: 1440x1000; categories render in two columns and the final category spans the row.
- Wide/short screen and 200% zoom: not separately captured; component tests cover keyboard and focus behavior.

## Review

- The user page presents the featured status, aligned history bars, grouped components, availability, incident count, and explanatory footer.
- At 390px, controls wrap and each category remains within the viewport without horizontal overflow. At 1440px, categories use two columns with the final category spanning the row.
- Status colors use the shared emerald, amber, and red palette. Loading surfaces remain static and card corners stay within the 8px project limit.
- V3 locale strings are loaded by the route-scoped `channel-monitor` locale loader for both English and Chinese.

## Evidence

- Mobile: [mobile-390.png](assets/channel-monitor-v3/mobile-390.png)
- Desktop: [desktop-1440.png](assets/channel-monitor-v3/desktop-1440.png)
- The captures render the actual `StatusPage.vue` component and project CSS with deterministic fixture status data. They do not claim production or authenticated-shell acceptance.
- Automated component interaction tests cover keyboard/focus tooltip behavior, paging, empty and live-window states.

## Residual Risk

- Authenticated shell navigation and live production data still require integration acceptance after merge and deployment.
- Follow-up owner: V3 integration and release owner.
