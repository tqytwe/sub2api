# Visual Review: State Kit Bilingual Runtime

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/PluginsView.vue",
    "frontend/src/i18n/locales/zh/admin/plugins.ts",
    "frontend/src/i18n/locales/en/admin/plugins.ts"
  ],
  "routes_or_surfaces": ["/admin/plugins", "/en/admin/plugins", "State Kit plugin configuration iframe"],
  "languages_and_themes": ["zh-CN/light", "en-US/light", "zh-CN/dark", "en-US/dark"],
  "states": ["loading", "compatible", "untested", "incompatible", "runtime healthy", "runtime error", "test success", "test failure", "configuration open"],
  "viewports": ["360x800", "768x1024", "1280x820"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png", "docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png", "docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"],
  "commands": ["pnpm design:check", "pnpm lint:check", "pnpm typecheck", "pnpm exec vitest run src/views/admin/__tests__/PluginsView.spec.ts"],
  "checks": {
    "keyboard": {"status": "not-applicable", "reason": "No control, focus order, or dialog structure changed."},
    "reduced_motion": {"status": "passed", "notes": "No motion was added."}
  },
  "residual_risks": ["Static review boards are non-production evidence.", "Authenticated browser acceptance remains required on the user's local computer after deployment."]
}
-->

## Scope

- The existing plugin-management layout and controls are unchanged.
- The host now appends a normalized `locale=zh|en` value to the sandboxed plugin UI fragment while preserving the one-time Bridge token.
- Compatibility and runtime summaries are rendered from frontend locale keys instead of backend Chinese prose.
- Plugin test success and failure toasts use host locale keys instead of rendering plugin-supplied prose.

## Baseline

- Before this change, the host opened the same iframe URL for Chinese and English routes, so the plugin could not select the active language.
- Compatibility and runtime prose came directly from backend Chinese strings.
- Baseline artifact: `docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png`.

## Prototype

- Prototype artifacts: `docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png` and `docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png`.
- The approved boundary is no layout or control change; only localized content selection changes.

## Reuse Decision

- This is a behavior-only localization change. The existing 390px and 1280px plugin-management review boards are reused as the approved no-layout-change prototype boundary.
- Existing `AppLayout`, `BaseDialog`, `Icon`, toast, step-up, cards, buttons, iframe sandbox and responsive grid are unchanged.
- No new colors, dimensions, motion, page shell, nested cards or custom controls were introduced.

## State Coverage

- Chinese default route passes `locale=zh`; the explicit English route passes `locale=en`; unsupported locales fall back to Chinese.
- The iframe fragment retains `bridge_token` and does not put locale into plugin configuration or server logs.
- Compatible, untested, incompatible, healthy, unavailable and error summaries are selected from symmetric Chinese and English locale resources.
- Plugin-list tests and configuration-bridge test failures use localized host summaries; detailed plugin output remains in server diagnostics rather than crossing locale boundaries.
- Automated tests cover both routes and reject direct rendering of backend Chinese compatibility/runtime messages on the English route.

## Viewport Coverage

- Because markup and styling did not change, the prior 360/768/1280 responsive and light/dark review boundary remains applicable.
- Long localized copy remains inside the existing wrapping text containers; no fixed-width text control was added.

## Evidence

- Updated static artifacts reuse the no-layout-change prototype images listed in the manifest.
- Automated evidence consists of the plugin manager Vitest, locale parity, design check, lint, typecheck and production build commands recorded in the manifest.
- Browser screenshots are intentionally deferred to authenticated local-computer acceptance after deployment; the static board is not presented as production evidence.

## Residual Risk

- This record is static non-production evidence and does not claim authenticated browser execution.
- After deployment, the user must open the installed plugin from both `/admin/plugins` and `/en/admin/plugins` in a local browser and verify light/dark themes and 360/768/1280 widths.
