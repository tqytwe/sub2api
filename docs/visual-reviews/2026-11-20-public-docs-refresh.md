# Visual Review: public documentation refresh

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/public/DocsView.vue",
    "frontend/src/router/index.ts",
    "frontend/src/styles/public-docs.css",
    "frontend/src/content/public-docs-reader.ts",
    "frontend/src/content/public-docs-integrations.ts",
    "frontend/src/content/public-docs-tool-guides.ts",
    "frontend/src/content/public-docs-data.zh.ts",
    "frontend/src/content/public-docs-data.en.ts",
    "frontend/src/content/public-docs-articles.en.ts",
    "frontend/src/content/public-docs-tree.ts",
    "frontend/src/content/__tests__/publicDocsMobileLayout.spec.ts",
    "frontend/src/content/__tests__/publicDocsReader.spec.ts",
    "frontend/public/doc-assets/integrations/codex-quick-config-zh.png",
    "frontend/public/doc-assets/integrations/codex-quick-config-en.png",
    "frontend/public/doc-assets/integrations/create-api-key-zh.png",
    "frontend/public/doc-assets/integrations/create-api-key-en.png"
  ],
  "routes_or_surfaces": [
    "/docs",
    "/docs?cat=tools&page=codex",
    "/docs?cat=tools&page=cline",
    "/docs?cat=deploy&page=codex",
    "/en/docs",
    "/en/docs?cat=tools&page=codex",
    "/docs?cat=tutorial",
    "/docs?cat=recharge-vip&page=vip-levels",
    "/en/docs?cat=recharge-vip&page=check-in",
    "/en/docs?cat=tools",
    "/login?lang=en",
    "/keys"
  ],
  "languages_and_themes": ["zh-CN/light", "en-US/dark"],
  "states": ["default", "search", "mobile navigation", "on-page navigation", "focus-visible", "code copied", "loading", "error", "empty search"],
  "viewports": ["360x800", "768x860", "1280x900", "1440x960", "1920x1000"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/public-docs-refresh/prototype-index-1440.png",
    "docs/visual-reviews/assets/public-docs-refresh/prototype-article-1440.png",
    "docs/visual-reviews/assets/public-docs-refresh/prototype-codex-1440.png",
    "docs/visual-reviews/assets/public-docs-refresh/prototype-codex-360.png",
    "docs/visual-reviews/assets/public-docs-refresh/prototype-apps-1440.png",
    "docs/visual-reviews/assets/public-docs-refresh/prototype-quick-config-zh.png",
    "docs/visual-reviews/assets/public-docs-refresh/prototype-quick-config-en.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/public-docs-refresh/baseline-index-1280.png",
    "docs/visual-reviews/assets/public-docs-refresh/baseline-codex-360.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/public-docs-refresh/updated-apps-1440.png",
    "docs/visual-reviews/assets/public-docs-refresh/updated-codex-1440.png",
    "docs/visual-reviews/assets/public-docs-refresh/updated-codex-360.png",
    "docs/visual-reviews/assets/public-docs-refresh/updated-codex-768.png",
    "docs/visual-reviews/assets/public-docs-refresh/updated-codex-en-1920-dark.png",
    "docs/visual-reviews/assets/public-docs-refresh/updated-apps-en-360-dark.png",
    "docs/visual-reviews/assets/public-docs-refresh/updated-search-1440.png",
    "docs/visual-reviews/assets/public-docs-refresh/updated-codex-figure-1440.png",
    "docs/visual-reviews/assets/public-docs-refresh/updated-quick-config-zh.png",
    "docs/visual-reviews/assets/public-docs-refresh/updated-quick-config-en.png"
  ],
  "commands": [
    "DOCS_PREVIEW_ORIGIN=http://127.0.0.1:4182 node /data/projects/.workspace/delivery-evidence/public-docs-browser-check.cjs",
    "DOCS_PREVIEW_ORIGIN=http://127.0.0.1:4182 node /data/projects/.workspace/delivery-evidence/public-docs-full-audit.cjs",
    "DOCS_PREVIEW_ORIGIN=http://127.0.0.1:4182 DOCS_GUIDE_CAPTURE_DIR=/data/projects/.workspace/delivery-evidence/public-docs-current-captures node /data/projects/.workspace/delivery-evidence/public-docs-browser-check.cjs --capture-guide",
    "pnpm design:check"
  ],
  "checks": {
    "keyboard": { "status": "passed", "notes": "Search uses Control+K; sidebar, mobile table of contents, article anchors, and code-copy controls are keyboard reachable." },
    "reduced_motion": { "status": "passed", "notes": "The documentation surface does not add continuous motion; browser capture disables screenshot animations." }
  },
  "residual_risks": [
    "Final production verification still depends on the user's local browser, real API settings, and authenticated key configuration.",
    "Tool-specific upstream CLIs may change their configuration flags; each guide links to the upstream documentation and keeps provider values explicit."
  ]
}
-->

## Scope

- Routes include both documentation indexes, all category and article pages, legacy article redirects, and the authenticated key page used for Codex quick-config evidence.
- Roles: public visitor for documentation; authenticated demo fixture for the redacted quick-config capture.
- Languages and themes: `zh-CN/light` and `en-US/dark`.
- Goal: make the docs usable as a reference workflow, with a persistent category tree, article table of contents, searchable tool index, copyable code, setup screenshots, and responsive mobile navigation. The sidebar is the single entry point for tool tutorials; the index does not duplicate the tool list.

## Baseline

- Current behavior: the previous docs page presented older category content with limited hierarchy and did not provide a focused tool-integration index or article-level navigation. During review, a duplicate homepage tool table was identified and removed so the sidebar remains the single tool entry point.
- Baseline evidence: [baseline-index-1280.png](assets/public-docs-refresh/baseline-index-1280.png) and [baseline-codex-360.png](assets/public-docs-refresh/baseline-codex-360.png).
- Inconsistencies observed: tool setup instructions, protocol/base URL distinctions, mobile navigation, and screenshots were difficult to scan or absent from the old surface.

## Prototype

- Prototype design images: [prototype-index-1440.png](assets/public-docs-refresh/prototype-index-1440.png), [prototype-article-1440.png](assets/public-docs-refresh/prototype-article-1440.png), [prototype-codex-1440.png](assets/public-docs-refresh/prototype-codex-1440.png), and [prototype-codex-360.png](assets/public-docs-refresh/prototype-codex-360.png). The final index intentionally omits the prototype's repeated integration table.
- Supporting configuration prototypes: [prototype-quick-config-zh.png](assets/public-docs-refresh/prototype-quick-config-zh.png) and [prototype-quick-config-en.png](assets/public-docs-refresh/prototype-quick-config-en.png).
- Approval status: implementation follows the provided MoleAPI information architecture reference: left navigation, article body, right-side section navigation, responsive mobile collapse, and tool-specific setup details.
- Scope boundary: content and presentation are limited to public docs and the existing key quick-config evidence; no deployment or backend API contract changes were made.

## Reuse Decision

- Shared layouts and components reused: existing public page toolbar, app/router localization, shared icon component, public settings/API base URL, key/model/usage routes, and existing design tokens.
- New shared pattern: `DocsView.vue` now owns the documentation reader shell while `public-docs-reader.ts` owns sanitized article parsing, generated heading navigation, and code-copy metadata.
- Design-system exception: none. Documentation-specific styling is isolated in `frontend/src/styles/public-docs.css` and uses existing surface, text, border, action, and focus tokens.

## State Coverage

- Default: index and article pages captured in light and dark themes.
- Hover and active: navigation links, category expansion, table links, search results, pagination links, and copy controls are covered by the shared CSS states.
- Focus-visible and keyboard: `Control+K` opens search; keyboard focus styling is explicit; article anchors, mobile menus, and copy buttons are reachable without a pointer.
- Loading, disabled, empty, error and success: loading/error article states remain rendered by the view; empty search is asserted; code-copy success is asserted; no asynchronous disabled state is introduced by this change.

## Viewport Coverage

- Mobile: `360x800`, including collapsed navigation, mobile table of contents, article anchors, and long-code overflow.
- Tablet: `768x860`, with the two-column reader layout and mobile table of contents breakpoint behavior.
- Desktop: `1280x900` and `1440x960`, including the index without a duplicate tool table, article rail, search dialog, and setup figure.
- Wide or short screen: `1920x1000` English dark-mode article.
- 200% zoom and reduced motion: no continuous motion is introduced; browser evidence uses stable dimensions and disables screenshot animations. A manual 200% browser pass remains part of final local acceptance.

## Evidence

- Updated captures: [updated-apps-1440.png](assets/public-docs-refresh/updated-apps-1440.png), [updated-codex-1440.png](assets/public-docs-refresh/updated-codex-1440.png), [updated-codex-360.png](assets/public-docs-refresh/updated-codex-360.png), [updated-codex-768.png](assets/public-docs-refresh/updated-codex-768.png), [updated-search-1440.png](assets/public-docs-refresh/updated-search-1440.png), and [updated-codex-figure-1440.png](assets/public-docs-refresh/updated-codex-figure-1440.png).
- Dark and English captures: generated by the same browser check as [updated-codex-en-1920-dark.png](assets/public-docs-refresh/updated-codex-en-1920-dark.png) and [updated-apps-en-360-dark.png](assets/public-docs-refresh/updated-apps-en-360-dark.png).
- Configuration evidence: [updated-quick-config-zh.png](assets/public-docs-refresh/updated-quick-config-zh.png) and [updated-quick-config-en.png](assets/public-docs-refresh/updated-quick-config-en.png) are redacted browser captures; the same images are served by the docs article from `frontend/public/doc-assets/integrations/`.
- Automated checks: viewport overflow, article position, theme class, tool count, legacy route normalization, search, anchor navigation, image decode, code copy, and browser console errors are asserted by the browser script.
- The bilingual browser audit opens all 55 articles and eight category pages per language at 360, 768, and 1440 px. The index, categories, and articles share the same left edge and content widths of 328, 488, and 800 px respectively. The audit includes a populated VIP tier table and checks internal links, image decoding, visible icons, section anchors, category and home breadcrumbs, browser Back, and sign-in redirects from keys and check-in.
- The authenticated browser fixture creates and copies a demo key through the form opened from the docs, downloads both Codex configuration scripts, and returns to the article. The public test verifies the login redirect and language query before returning with browser Back.
- Key creation screenshots from the current form are served from `frontend/public/doc-assets/integrations/` and use sample data.
- Platform content was checked against `99db0e5f7`. Recharge guides point to the live quote, orders, and wallet. Check-in describes participation, energy, and reward eligibility. The public creation guide follows the redirect to the separate canvas application and uses the image workbench labels observed on that site.

## Residual Risk

- The browser script uses loopback API fixtures and cannot prove production settings, permissions, or every tool's current upstream release behavior.
- Final acceptance requires a production browser pass with real account permissions. The demo flow did not execute a downloaded script, make a paid API request, or complete real payments or rewards.
- Follow-up owner: documentation maintainer should refresh tool-specific screenshots and command details when upstream clients change their configuration format.
