# Visual Review: internal API key speed test

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/keys/UseKeyModal.vue",
    "frontend/src/views/user/KeysView.vue",
    "frontend/src/views/user/SpeedTestView.vue",
    "frontend/src/router/index.ts",
    "frontend/src/i18n/locales/zh/dashboard.ts",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts"
  ],
  "routes_or_surfaces": ["/keys", "/keys/speed-test", "Use API Key dialog"],
  "languages_and_themes": ["zh-CN/light", "en-US/light", "zh-CN/dark"],
  "states": ["missing-payload", "loading-models", "ready", "running", "cancelled", "error", "success", "empty-results"],
  "viewports": ["360x800", "768x800", "1280x858"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/internal-speed-test/prototype-internal-speed-test.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/internal-speed-test/baseline-internal-speed-test.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/internal-speed-test/updated-internal-speed-test.png"
  ],
  "commands": [
    "node generated docs/visual-reviews/assets/internal-speed-test/*.png",
    "pnpm --dir frontend exec vitest run src/utils/__tests__/internalSpeedTest.spec.ts src/views/user/__tests__/SpeedTestView.spec.ts src/components/keys/__tests__/UseKeyModal.spec.ts"
  ],
  "checks": {
    "keyboard": { "status": "passed" },
    "reduced_motion": { "status": "passed" }
  },
  "residual_risks": [
    "Static review board only; browser screenshots and final acceptance on the user's local machine are still required after deployment."
  ]
}
-->

## Scope

- Routes: authenticated `/keys` entry and new authenticated `/keys/speed-test` workspace page.
- Roles: normal authenticated user with an active API key.
- Languages and themes: Chinese and English copy added; light and dark classes follow existing console page patterns.

## Baseline

- Current behavior: the Use API Key dialog sends the selected key into an LMSpeed URL on click; LMSpeed frontend parsing bugs can show JSON parse errors.
- Baseline screenshot or recording: `docs/visual-reviews/assets/internal-speed-test/baseline-internal-speed-test.png`.
- Inconsistencies observed: the key-specific speed-test workflow depends on a third-party page and cannot control stream buffering behavior.

## Prototype

- Prototype design image: `docs/visual-reviews/assets/internal-speed-test/prototype-internal-speed-test.png`.
- Approval status: user approved replacing the third-party LMSpeed handoff with a built-in speed-test page.
- Scope boundary: keep endpoint-only HTTP speed links unchanged; replace only the key-specific model speed-test action.

## Reuse Decision

- Shared layouts and components reused: `AppLayout`, global `btn` classes, native `select`, and shared `Icon.vue` icons.
- New shared pattern, if any: a focused utility for one-time speed-test payload storage and OpenAI SSE parsing.
- Design-system exception, if any: none.

## State Coverage

- Default and ready: model selector, run count, base URL, and start action are visible after loading `/v1/models`.
- Loading: model reload button spins and controls are disabled during model fetch.
- Running and cancel: sequential runs show status pills and the cancel button aborts pending requests.
- Error and empty: direct route visits show the missing-payload state; model-load and run errors remain inline.
- Success: metrics show average first token, average rate, total output tokens, and expandable output text.

## Viewport Coverage

- Mobile: controls stack vertically and buttons keep touch-friendly heights.
- Tablet: controls use a three-column grid when width allows.
- Desktop: workspace frame spans the AppLayout content area without private page width.
- Wide or short screen: no page-owned full-height or page scroll behavior was added.
- 200% zoom and reduced motion: no new continuous motion, transforms, or viewport-scaled type added.

## Evidence

- Updated screenshot or recording: `docs/visual-reviews/assets/internal-speed-test/updated-internal-speed-test.png`.
- Automated visual or overlap checks: targeted tests cover no third-party URL handoff, one-time payload consumption, direct-route missing state, model loading, and split SSE parsing.
- Commands run: `pnpm --dir frontend exec vitest run src/utils/__tests__/internalSpeedTest.spec.ts src/views/user/__tests__/SpeedTestView.spec.ts src/components/keys/__tests__/UseKeyModal.spec.ts`.

## Residual Risk

- Known limitations: static review board only; authenticated browser screenshots, deployed route probes, and final local browser acceptance are still required.
- Follow-up owner: deployment and user-side browser acceptance remain separate gates.
