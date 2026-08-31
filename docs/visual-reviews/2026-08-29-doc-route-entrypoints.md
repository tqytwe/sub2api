# Visual Review: native docs entrypoints

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/layout/AppHeader.vue",
    "frontend/src/components/layout/PublicContentLayout.vue",
    "frontend/src/views/HomeView.vue",
    "frontend/src/views/user/UsageView.vue",
    "frontend/src/components/layout/__tests__/docUrlSanitization.spec.ts"
  ],
  "routes_or_surfaces": ["public home docs CTA and footer", "authenticated header docs action", "usage empty state docs action", "public layout docs action"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "hover", "active", "focus-visible", "loading", "empty", "error"],
  "viewports": ["360x800", "768x900", "1280x800", "1600x1000"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/v182-home-status/prototype-1440.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/v182-browser-qa/docs-360x800.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/v182-browser-qa/en-docs-360x800-overflow-fixed.png"],
  "commands": ["Playwright Chromium captures from docs/visual-reviews/2026-08-29-v182-browser-qa.md", "pnpm vitest run src/components/layout/__tests__/docUrlSanitization.spec.ts", "pnpm typecheck", "pnpm design:check"],
  "checks": {
    "keyboard": {"status": "passed", "notes": "RouterLink and external anchor variants retain native focus and keyboard activation."},
    "reduced_motion": {"status": "not-applicable", "reason": "The change only chooses navigation target and adds no motion."}
  },
  "residual_risks": ["Production browser acceptance must verify the configured doc_url and locale-specific route after deployment."]
}
-->

## Scope

When the configured documentation URL is the exact canonical Jisudeng docs origin,
the Header, usage empty state, and public layout now use an in-app RouterLink to
`/docs` or `/en/docs`. Arbitrary third-party documentation URLs remain external
anchors with `noopener` protection.

## Baseline

The configured first-party docs URL previously rendered as a normal anchor in
some entrypoints and as an iframe target in the custom-menu path. The browser
could therefore open a new document context or hit the frame-ancestor refusal.

## Prototype

The review reuses the existing public docs shell prototype at
`docs/visual-reviews/assets/v182-home-status/prototype-1440.png`; no new frame or
component family was designed.

## Reuse Decision

The change reuses `resolveCustomMenuRoute`, Vue Router links, existing button
classes, `PublicContentLayout`, and the existing book icon. Third-party URLs keep
the existing external-anchor behavior.

## State Coverage

Default, hover, active, focus-visible, loading, empty, and error states remain
owned by the existing link/button and page-shell components. The only new branch
is the canonical first-party versus third-party target selection.

## Viewport Coverage

The surrounding docs page was captured at 360, 768, 1280, and 1600px in Chinese
and English, light and dark themes. The 360px English follow-up has no horizontal
overflow. The new links do not introduce fixed widths or page-level scrolling.

## Evidence

`docUrlSanitization.spec.ts` covers all three entrypoints and the strict resolver
branch. `pnpm typecheck` and `pnpm design:check` are required gates; the existing
browser PNGs provide decoded visual evidence for the reused docs shell.

## Review Notes

The route selection is URL-parser based and shares the custom-menu resolver. The
existing public page frame, button classes, icon, focus behavior, and responsive
layout are reused. No iframe or embedded-document surface is introduced.

The referenced PNGs are real decoded visual artifacts from the v182 browser review;
they are evidence of the surrounding docs visual language, not a claim of live
production acceptance. Production CSP/XFO and authenticated browser acceptance
remain deployment gates.

## Residual Risk

The production administrator must still save `frontend_url` and `doc_url` through
the audited Settings UI. After deployment, verify both authenticated and public
English/Chinese clicks in the user's local browser and confirm CSP/XFO headers.
