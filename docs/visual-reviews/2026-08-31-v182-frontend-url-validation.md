# Visual Review: v182 frontend URL validation

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/SettingsView.vue",
    "frontend/src/views/admin/__tests__/SettingsView.spec.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts",
    "frontend/src/i18n/locales/en/admin/settings.ts"
  ],
  "routes_or_surfaces": ["/admin/settings security tab", "frontend_url password-reset and notification-link setting"],
  "languages_and_themes": ["zh-CN/light static review", "en-US/light static review"],
  "states": ["default", "invalid submit", "corrected retry", "success", "focus-visible", "loading", "disabled", "error"],
  "viewports": ["1280x760", "1600x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/v182-frontend-url-validation/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/v182-frontend-url-validation/baseline-1280.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/v182-frontend-url-validation/updated-1280.png"
    ,"docs/visual-reviews/assets/v182-frontend-url-validation/updated-1600.png"
  ],
  "commands": [
    "cd frontend && pnpm exec playwright screenshot --viewport-size=1280,760 file:///home/codex/worktrees/sub2api-v182-governance-20260829/docs/visual-reviews/assets/v182-frontend-url-validation/review-board.html?mode=baseline ../docs/visual-reviews/assets/v182-frontend-url-validation/baseline-1280.png",
    "cd frontend && pnpm exec playwright screenshot --viewport-size=1280,760 file:///home/codex/worktrees/sub2api-v182-governance-20260829/docs/visual-reviews/assets/v182-frontend-url-validation/review-board.html?mode=prototype ../docs/visual-reviews/assets/v182-frontend-url-validation/prototype-1280.png",
    "cd frontend && pnpm exec playwright screenshot --viewport-size=1280,760 file:///home/codex/worktrees/sub2api-v182-governance-20260829/docs/visual-reviews/assets/v182-frontend-url-validation/review-board.html?mode=updated ../docs/visual-reviews/assets/v182-frontend-url-validation/updated-1280.png",
    "cd frontend && pnpm exec playwright screenshot --viewport-size=1600,900 file:///home/codex/worktrees/sub2api-v182-governance-20260829/docs/visual-reviews/assets/v182-frontend-url-validation/review-board.html?mode=updated ../docs/visual-reviews/assets/v182-frontend-url-validation/updated-1600.png",
    "cd frontend && pnpm exec vitest run src/views/admin/__tests__/SettingsView.spec.ts"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "The existing shared Input keeps native tab and focus-visible behavior; no new focus model was introduced."},
    "reduced_motion": {"status": "not-applicable", "reason": "This validation change adds no motion."}
  },
  "residual_risks": [
    "Artifacts are static review boards, not authenticated application screenshots.",
    "After deployment, an administrator must verify Chinese and English, light and dark themes, 360/768/1280/1600+ widths, 200% zoom, keyboard focus, invalid URL correction, and the actual audited save response in a local browser."
  ]
}
-->

## Scope

- Route: administrator Settings, Security tab, visible only while email verification and password reset are enabled.
- The setting is the canonical base URL for password-reset and notification links. It is not a CSP embedding allowlist.
- The change prevents a bad value from being silently converted to an empty value before the audited settings update request.

## Baseline

- The former raw input used the general optional-URL rule. A malformed value could be cleared before save, allowing an administrator to accidentally remove the configured canonical site URL.
- Baseline board: `assets/v182-frontend-url-validation/baseline-1280.png`.

## Prototype

- The prototype retains the existing Settings card, spacing, submit action, and shared form-control vocabulary. It adds only the inline validation state that preserves the submitted input for correction.
- Prototype board: `assets/v182-frontend-url-validation/prototype-1280.png`.
- Scope is limited to `frontend_url`; `doc_url` intentionally keeps its historic optional clear-on-invalid behavior.

## Reuse Decision

- Replaced the page-local raw input with the existing shared `Input` component, which already owns label association, hint/error switching, `aria-invalid`, `aria-describedby`, focus appearance, and error color treatment.
- Reused the existing Settings save action and `appStore.showError` feedback. No card, button, dialog, icon, width, animation, or page-shell pattern was added.
- The backend `normalizeFrontendURLSetting` remains the authority that rejects any request bypassing the browser.

## State Coverage

- Default: the existing label, hint, and input layout remain unchanged.
- Error: query, fragment, userinfo, non-HTTP(S), relative, or malformed input remains visible, sets a field-associated error, raises the existing error toast, and does not call `updateSettings`.
- Corrected retry: typing a valid absolute HTTP(S) value clears the field error; trim normalization occurs before the normal audited update request.
- Success and loading: retain existing Settings save semantics and button state.
- Focus-visible and keyboard: owned by the shared native input; no new keyboard interaction is introduced.
- Disabled/empty: empty remains an accepted explicit setting value; the field still follows its existing conditional visibility.

## Viewport Coverage

- The static board was rendered at 1280x760 and 1600x900 and inspected for text wrapping, visible error association, error contrast, and unchanged action geometry.
- Real responsive admin screenshots at 360, 768, 1280, and 1600+ are intentionally not claimed by this static review and remain release acceptance work.

## Evidence

- Updated board: `assets/v182-frontend-url-validation/updated-1280.png`.
- All three referenced PNG files are browser-rendered, valid 1280x760 RGB PNGs; the board is explicitly a static review artifact, not a live route capture.
- `pnpm exec vitest run src/views/admin/__tests__/SettingsView.spec.ts` passed 39 tests, including unsafe-value preservation, blocked submission, field ARIA binding, correction/retry, and canonical whitespace trimming.

## Residual Risk

- This environment has no authenticated production administrator session and no deployment was performed.
- Production acceptance must verify the actual `/admin/settings` save response, email-link use of `https://www.jisudeng.com`, and the language/theme/responsive matrix listed in the manifest after an authorized rollout.
