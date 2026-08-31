# Visual Review: v182 P0 Docs And Subscriptions

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/App.vue",
    "frontend/src/components/common/SubscriptionProgressMini.vue",
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/i18n/locales/en/admin/settings.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/misc.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/misc.ts",
    "frontend/src/router/customMenuTarget.ts",
    "frontend/src/router/index.ts",
    "frontend/src/router/navigationGeneration.ts",
    "frontend/src/views/admin/SettingsView.vue",
    "frontend/src/views/user/CustomPageView.vue",
    "frontend/src/views/user/PaymentView.vue",
    "frontend/src/views/user/RedeemView.vue",
    "frontend/src/views/user/SubscriptionsView.vue"
  ],
  "routes_or_surfaces": ["authenticated custom-menu docs entry", "/custom/:id legacy bookmark redirect", "/docs", "/en/docs", "/subscriptions", "/admin/settings security tab"],
  "languages_and_themes": ["zh-CN/light static review", "en-US behavior covered by locale tests"],
  "states": ["default", "legacy iframe refusal baseline", "native route redirect", "invalid frontend URL", "corrected retry", "loading", "normalized subscription progress refresh", "out-of-order locale navigation"],
  "viewports": ["1280x760", "1600x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/v182-p0-docs-subscriptions/prototype-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/v182-p0-docs-subscriptions/baseline-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/v182-p0-docs-subscriptions/updated-1280.png", "docs/visual-reviews/assets/v182-p0-docs-subscriptions/updated-1600.png"],
  "commands": [
    "cd frontend && pnpm exec playwright screenshot --viewport-size=1280,760 file:///home/codex/worktrees/sub2api-v182-p0-20260831/docs/visual-reviews/assets/v182-p0-docs-subscriptions/review-board.html?mode=baseline ../docs/visual-reviews/assets/v182-p0-docs-subscriptions/baseline-1280.png",
    "cd frontend && pnpm exec playwright screenshot --viewport-size=1280,760 file:///home/codex/worktrees/sub2api-v182-p0-20260831/docs/visual-reviews/assets/v182-p0-docs-subscriptions/review-board.html?mode=prototype ../docs/visual-reviews/assets/v182-p0-docs-subscriptions/prototype-1280.png",
    "cd frontend && pnpm exec playwright screenshot --viewport-size=1280,760 file:///home/codex/worktrees/sub2api-v182-p0-20260831/docs/visual-reviews/assets/v182-p0-docs-subscriptions/review-board.html?mode=updated ../docs/visual-reviews/assets/v182-p0-docs-subscriptions/updated-1280.png",
    "cd frontend && pnpm exec playwright screenshot --viewport-size=1600,900 file:///home/codex/worktrees/sub2api-v182-p0-20260831/docs/visual-reviews/assets/v182-p0-docs-subscriptions/review-board.html?mode=updated ../docs/visual-reviews/assets/v182-p0-docs-subscriptions/updated-1600.png",
    "cd frontend && pnpm exec vitest run src/router/__tests__/customMenuTarget.spec.ts src/router/__tests__/navigationGeneration.spec.ts src/router/__tests__/routeLocaleGuardRace.spec.ts src/api/__tests__/subscriptions.spec.ts src/stores/__tests__/subscriptions.spec.ts src/components/common/__tests__/SubscriptionProgressMini.spec.ts src/views/user/PaymentView.spec.ts src/views/user/RedeemView.subscriptionRefresh.spec.ts src/views/user/__tests__/SubscriptionsView.progress.spec.ts src/views/admin/__tests__/SettingsView.spec.ts src/__tests__/App.startup.spec.ts"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "The visible controls reuse RouterLink and the shared Input component; the new Settings regression exercises the existing form submission path."},
    "reduced_motion": {"status": "not-applicable", "reason": "P0 changes navigation targets, copy, form validation, and locale ordering only; it adds no motion."}
  },
  "residual_risks": ["These PNGs are static review boards, not authenticated application captures. Before deployment, browser screenshots and final acceptance must cover 360, 768, 1280, and 1600+ widths; zh/en; light/dark; 200% zoom; keyboard focus; reduced motion; and guest, user, and administrator identities."]
}
-->

## Scope

- A custom menu URL reaches a native docs route only when its parsed origin is
  exactly `https://www.jisudeng.com` and its path is exactly `/docs` or
  `/en/docs`. Query and fragment data do not survive the navigation.
- The published legacy custom-page ID redirects to the locale-appropriate docs
  route before an iframe can be created. Third-party custom pages remain in the
  existing iframe path.
- First-party docs aliases that still reach the iframe fallback receive no
  panel-derived `token`, `user_id`, theme, locale, source, or embedded-mode
  query value.
- The administrator's canonical `frontend_url` input remains visible and
  repairable even while email verification and password reset are disabled.
- Subscription progress is normalized at the frontend API boundary; missing
  group and duration fallback text is localized. The authenticated-app preload,
  five-minute subscription poll, purchase completion, and subscription-code
  redemption update the same header progress state.

## Baseline

The navigation treated the first-party docs link as a custom iframe page. Its
target CSP did not authorize the current parent origin, so the browser showed
the deterministic refusal rather than a usable document. The previous
`frontend_url` placement also hid the required repair control whenever both
email switches were unavailable.

`assets/v182-p0-docs-subscriptions/baseline-1280.png` is a static board of
those two concrete failure states. It is not presented as an authenticated or
production screenshot.

## Prototype

The prototype retains the established sidebar density, the native docs reading
route, the Settings form layout, and the shared Input component. It introduces
no new card family, page frame, button style, icon set, or animation.

`assets/v182-p0-docs-subscriptions/prototype-1280.png` records the intended
native-route and always-repairable-settings behaviors before the final board.

## Reuse Decision

The implementation reuses `DocsView`, the existing router guard, `AppSidebar`,
shared `Input`, existing Settings save/audit behavior, existing subscription
components, and the established localized duration/message paths. CSP preserves
`frame-src` configuration while the ancestor policy is fixed to `'self'` and
continues to send `X-Frame-Options: SAMEORIGIN`.

## State Coverage

- Default: a canonical menu entry links to the locale-appropriate native docs
  route; unrelated custom pages retain their established behavior.
- Error: an invalid `frontend_url` stays in the field with a localized,
  field-associated error and does not issue an update request.
- Corrected retry: a corrected trimmed URL clears the error and follows the
  normal Settings save flow.
- Loading and empty: existing route, form, and subscription loading/empty
  ownership is retained; no new async visual shell is introduced.
- Progress refresh: the header mini component reacts to the shared normalized
  progress store after login preload, scheduled refresh, or a forced refresh;
  it does not retain a component-local quota snapshot. Subscription purchase
  completion and subscription redemption use the forced refresh path.
- Locale race: only the latest navigation can set locale, `html[lang]`, title,
  or SEO state after lazy language modules resolve.

## Viewport Coverage

The board was rendered and inspected at 1280x760 and 1600x900. The 1280 and
1600 captures show complete panels, wrapping long policy text inside its
containers, and no overlapping labels or actions.

This is not a substitute for live responsive capture. The 360/768,
dark-theme, 200%-zoom, reduced-motion, and authenticated browser matrix remains
explicit release acceptance work.

## Evidence

- The baseline, prototype, and updated PNGs were rendered by Playwright from
  `assets/v182-p0-docs-subscriptions/review-board.html`; they are valid,
  decodable browser-rendered PNGs and are explicitly labelled static boards.
- The docs target, navigation generation, locale race, subscription boundary,
  sidebar, iframe query stripping, and Settings regression tests are run as
  part of P0 verification. The Settings regression specifically proves that
  `frontend_url` remains editable when email verification and password reset
  are both disabled.
- The immutable implementation contract and post-deployment responsibilities
  are maintained in `docs/V182_P0_DOCS_SUBSCRIPTIONS_CONTRACT.md` in this same
  change set.

## Residual Risk

The code was merged to `origin/play/main` as
`ceb8d5aed015b7a5db3b2fdecd0d0381faf9bf37` and the public production shell now
serves the new asset with `frame-ancestors 'self'`; `/health` and both native
docs routes are healthy. No production setting or database row was changed by
this review. The Zeabur CLI session is currently invalid, so its deployment ID
and recorded SHA remain pending re-authentication. A human administrator must
still set `frontend_url` to `https://www.jisudeng.com` through the Settings UI,
verify audit output and reset/notification links, and complete the guest/user/
admin local-browser matrix before this can be called production-complete.
