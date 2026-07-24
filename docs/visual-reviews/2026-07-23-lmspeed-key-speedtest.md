# Visual Review: LMSpeed API key speed test

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/keys/UseKeyModal.vue",
    "frontend/src/views/user/KeysView.vue",
    "frontend/src/i18n/locales/zh/dashboard.ts",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts"
  ],
  "routes_or_surfaces": ["/keys", "Use API Key dialog"],
  "languages_and_themes": ["zh-CN/light", "en-US/light", "zh-CN/dark"],
  "states": ["default", "hover", "active", "focus-visible", "disabled-hidden-for-inactive-key"],
  "viewports": ["360x800", "768x800", "1280x858"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/lmspeed-key-speedtest/prototype-use-key-modal-lmspeed.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/lmspeed-key-speedtest/baseline-api-key-page-user.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/lmspeed-key-speedtest/updated-static-use-key-modal-lmspeed.png"
  ],
  "commands": [
    "python3 generated docs/visual-reviews/assets/lmspeed-key-speedtest/prototype-use-key-modal-lmspeed.png",
    "pnpm --dir frontend exec vitest run src/utils/__tests__/lmspeed.spec.ts src/components/keys/__tests__/UseKeyModal.spec.ts"
  ],
  "checks": {
    "keyboard": { "status": "passed" },
    "reduced_motion": { "status": "passed" }
  },
  "residual_risks": [
    "Static review board only; final browser screenshot and final acceptance on the user's local machine are still required."
  ]
}
-->

## Scope

- Routes: authenticated API Key page, specifically the Use API Key dialog opened from a key row.
- Roles: authenticated user with at least one active API key.
- Languages and themes: Chinese and English copy added; light and dark classes follow the existing dialog status strip pattern.

## Baseline

- Current behavior: the API Key page has a tiny endpoint-level HTTP speed icon near the endpoint text, but the Use API Key dialog only shows client configuration snippets.
- Baseline screenshot or recording: `docs/visual-reviews/assets/lmspeed-key-speedtest/baseline-api-key-page-user.png`.
- Inconsistencies observed: no visible LMSpeed LLM speed-test entry exists in the key-specific workflow, and the endpoint-level speed icon cannot include the current API key.

## Prototype

- Prototype design image: `docs/visual-reviews/assets/lmspeed-key-speedtest/prototype-use-key-modal-lmspeed.png`.
- Approval status: user approved the additive, non-breaking approach in chat before implementation.
- Scope boundary: keep the endpoint HTTP speed icon unchanged; add one LMSpeed action strip inside the key-specific dialog only.

## Reuse Decision

- Shared layouts and components reused: existing `BaseDialog`, global `btn` classes, and `Icon.vue` icons `bolt` and `externalLink`.
- New shared pattern, if any: none.
- Design-system exception, if any: none.

## State Coverage

- Default: active keys with a base URL and API key show the LMSpeed action strip above client tabs.
- Hover and active: global `btn btn-primary` handles the external action button states.
- Focus-visible and keyboard: native button remains keyboard focusable; unit test triggers the action through the button.
- Loading, disabled, empty, error and success: no loading state is needed because the action only opens a new tab; inactive keys hide the LMSpeed action.

## Viewport Coverage

- Mobile: action strip stacks vertically with the button below the description.
- Tablet: spacing remains within the existing wide dialog gutter.
- Desktop: action strip uses the dialog width and keeps the client tabs below it.
- Wide or short screen: no page-level width, scroll, or shell behavior changed.
- 200% zoom and reduced motion: no new animation, transform, pulse, or viewport-scaled type added.

## Evidence

- Updated screenshot or recording: `docs/visual-reviews/assets/lmspeed-key-speedtest/updated-static-use-key-modal-lmspeed.png`.
- Automated visual or overlap checks: targeted unit tests cover link construction, no LMSpeed href containing the API key, click behavior, and inactive-key hidden state.
- Commands run: `pnpm --dir frontend exec vitest run src/utils/__tests__/lmspeed.spec.ts src/components/keys/__tests__/UseKeyModal.spec.ts`.

## Residual Risk

- Known limitations: the updated artifact is a static review board, not a live authenticated browser screenshot.
- Follow-up owner: final browser screenshot and final acceptance remain with the user on the local machine after deployment.
