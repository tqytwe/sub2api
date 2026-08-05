# Homepage Navigation and Pricing Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/HomeView.vue",
    "frontend/src/styles/home-view.css",
    "frontend/src/router/publicNavigation.ts",
    "frontend/src/router/index.ts",
    "frontend/src/components/home/ChannelTV.vue",
    "frontend/src/components/home/LmspeedBadge.vue",
    "frontend/src/components/home/LmspeedProviderProof.vue",
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/components/layout/AuthLayout.vue",
    "frontend/src/components/layout/PublicContentLayout.vue",
    "frontend/src/content/featured-models.ts",
    "frontend/src/content/play-features.ts",
    "frontend/src/content/public-docs-data.zh.ts",
    "frontend/src/i18n/index.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/common.ts",
    "frontend/src/i18n/locales/jisudeng-home.en.ts",
    "frontend/src/i18n/locales/jisudeng-home.zh.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/common.ts",
    "frontend/src/utils/publicLocaleRoute.ts",
    "frontend/src/utils/routeSeo.ts",
    "frontend/src/views/NotFoundView.vue",
    "frontend/src/views/public/AboutView.vue",
    "frontend/src/views/public/ContactView.vue",
    "frontend/src/views/public/DocsView.vue",
    "frontend/src/views/public/ModelsView.vue",
    "frontend/src/views/public/PromptSquareView.vue"
  ],
  "routes_or_surfaces": [
    "Chinese homepage /",
    "/home",
    "/pricing",
    "/pricing/deepseek",
    "/en",
    "/en/models",
    "homepage mobile navigation",
    "homepage compact mode"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "zh-CN/dark",
    "en-US/light",
    "en-US/dark"
  ],
  "states": [
    "guest",
    "authenticated user",
    "administrator",
    "all play channels disabled",
    "selected play channels enabled",
    "empty external doc_url fallback"
  ],
  "viewports": [
    "360x800",
    "768x1024",
    "1280x820"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/home-mobile-navigation/prototype-360.jpg"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/home-mobile-navigation/baseline-360.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/home-mobile-navigation/updated-1280.png"
  ],
  "commands": [
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend vitest run src/router/__tests__/publicNavigation.spec.ts src/content/__tests__/play-features.spec.ts"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "Mobile menu has a real button, focus-visible states, Escape close, route-change close, and a click-outside backdrop."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "No new animation is introduced; existing homepage motion still needs live browser review."
    }
  },
  "residual_risks": [
    "The prototype is the user-provided 360px baseline screenshot showing the former /home navigation failure.",
    "The local Firefox captures show the shell while the intro animation waits for its WebGL reveal and the local API proxy is unavailable; a real browser run with 360, 768, and 1280px screenshots is still required for final visual acceptance.",
    "Production auth-state, feature-toggle, external LMSpeed image, and doc_url fallback behavior require live acceptance."
  ]
}
-->

## Scope

The homepage is reviewed as a public acquisition surface for Chinese `/` and English `/en`, including compact mode, guest sticky actions, authenticated console actions, administrator routing, public pricing, documentation fallbacks, and play-channel visibility.

## Prototype

`prototype-360.jpg` is the user-provided mobile baseline. It records the original `/home` URL and the collapsed header state that motivated this repair. The updated implementation adds an accessible menu at phone and tablet widths while keeping the existing desktop hierarchy.

## Baseline

The supplied mobile capture showed `/home`, no primary navigation entries, and a header that exposed only language, theme, download, and login controls. The pricing CTA also shared auth-start behavior instead of linking to a public pricing page.

## Reuse Decision

The existing homepage visual language, typography, animation, public toolbar, sticky guest actions, and external LMSpeed proof are retained. The change adds only route-contract data, a responsive menu surface, semantic fallback links, and feature-gated channel data.

## Updated Contract

- `/home` is a compatibility redirect to `/`; Chinese public pricing uses `/pricing` and `/pricing/:family`.
- Header primary links are generated from one route catalog and remain available in a mobile panel below 1024px.
- Public pricing uses a router link and never calls the auth entry point.
- Empty Chinese `doc_url` values fall back to `/docs`; English uses `/en/docs`.
- ChannelTV fails closed until public settings load and only exposes explicitly enabled destinations.
- Menu focus, Escape, outside click, route changes, scroll lock, and safe-area sticky actions are covered in the implementation.

## State Coverage

The implementation covers guest, authenticated user, administrator, compact-home, custom-home, missing `doc_url`, English route, Chinese route, and disabled/enabled channel settings. Authenticated console destinations are selected from the administrator flag; guests keep register/login and download actions.

## Viewport Coverage

The responsive contract targets 360px phones, 768px tablets, and 1280px desktop. At widths below 1024px the primary nav becomes a two-column menu with 44px targets; desktop keeps the existing inline nav. The 360px baseline artifact is included, while updated browser screenshots remain a release acceptance task.

## Evidence

`pnpm design:check`, `pnpm typecheck`, focused Vitest navigation/channel tests, HomeView compact/performance tests, and Go SEO tests were run during implementation. The manifest's browser matrix still needs live screenshots and click-through results before production sign-off.

## Acceptance

Run the browser matrix from the manifest before deployment. Record screenshots for each viewport and language/theme, then click every header, hero, image, channel, onboarding, pricing, footer, and sticky CTA and assert the final route or external URL. Production checks must also confirm `/models` remains the authenticated OpenAI-compatible API endpoint and does not become a frontend pricing route.

## Residual Risk

No production deployment, authenticated browser session, administrator session, or external LMSpeed outage simulation was performed in this worktree. Those checks remain required, as does a reduced-motion pass for the existing animated homepage.

## Visual Review Status

The implementation record is complete and design governance is satisfied. Updated product screenshots are intentionally not fabricated; the remaining browser evidence is explicitly recorded as residual risk above.
