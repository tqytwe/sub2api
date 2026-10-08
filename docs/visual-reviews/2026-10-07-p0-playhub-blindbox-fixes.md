# Visual Review: P0 PlayHub and Blindbox Fixes

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/user/PlayHubView.vue",
    "frontend/src/views/public/BlindboxView.vue",
    "frontend/src/i18n/locales/zh/legacy/user-misc.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts"
  ],
  "routes_or_surfaces": ["/play-hub", "/blindbox"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark"],
  "states": ["default", "hover"],
  "viewports": ["360x800", "768x800", "1280x800"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/p0-playhub-blindbox-fixes/prototype-p0-2-arena.png",
    "docs/visual-reviews/assets/p0-playhub-blindbox-fixes/prototype-p0-3-blindbox.png"
  ],
  "baseline_artifacts": ["docs/visual-reviews/assets/p0-playhub-blindbox-fixes/baseline-production.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/p0-playhub-blindbox-fixes/updated-production.png"],
  "commands": ["cd frontend && NODE_OPTIONS=\"--max-old-space-size=4096\" pnpm typecheck", "cd frontend && pnpm design:check"],
  "checks": {
    "keyboard": { "status": "not-applicable", "reason": "Text and visual display only, no new interactive controls" },
    "reduced_motion": { "status": "passed" }
  },
  "residual_risks": [
    "Final browser verification required: prototype is static HTML, actual rendering needs production environment check",
    "Weight distribution colors (purple/blue/green) need verification in both light and dark themes",
    "Mobile viewport stacking (< 768px) needs manual device testing"
  ]
}
-->

## Scope

- Routes: `/play-hub` (Arena card), `/blindbox` (reward weight visualization)
- Roles: Authenticated users with play module access
- Languages and themes: zh-CN in light and dark modes

## Baseline

- Current behavior:
  - **P0-2**: PlayHubView Arena card shows dynamic subtitle with rank/gap/tokens (e.g. "第 3 名 · 距上一名差 1500 tokens")
  - **P0-3**: BlindboxView shows text description of reward split but no visual weight distribution
- Baseline screenshot: Not captured (prototypes available, production deployment required for final verification)
- Inconsistencies observed:
  - Arena subtitle provides specific numerical data that may confuse users about how to participate
  - Blindbox reward split explanation is text-only, making it hard to understand the probability distribution at a glance

## Prototype

- Prototype design:
  - P0-2: `docs/visual-reviews/prototypes/p0-2-arena-reward-mechanism.html` - Arena card with fixed call-to-action text
  - P0-3: `docs/visual-reviews/prototypes/p0-3-blindbox-reward-branches.html` - Three-row weight visualization with progress bars
- Approval status: Design guidance provided in task documents
- Scope boundary: Text-only change for Arena; visual component addition for Blindbox without modifying existing pool/prize display logic

## Reuse Decision

- Shared layouts and components reused:
  - Uses existing `.gw-panel`, `.gw-balance-label`, `.gw-subtitle` classes from growth-world.css
  - Follows existing CSS custom property pattern for theming (`var(--gw-ink)`, `var(--gw-line)`, `var(--gw-panel-bg)`)
  - Reuses standard 8px border-radius for cards, 12px gap, 16px padding from design system
- New shared pattern: Weight distribution visualization could be generalized for other reward pool displays (quiz, checkin) but implemented inline first
- Design-system exception: Uses inline `style` attributes for dynamic progress bar widths and colors; added `design-governance-allow: page-shell-ownership` for component-local responsive grid behavior

## State Coverage

- Default: ✓ Fixed subtitle for Arena, three-row weight bars with percentages for Blindbox
- Hover and active: ✓ Existing hover states on Arena card preserved, weight bars are non-interactive display
- Focus-visible and keyboard: N/A - text display and visual indicator only, no new focusable controls
- Loading, disabled, empty: Covered by existing BlindboxView states (prizePool null check, disabled flag)
- Error and success: Not applicable to these display-only changes

## Viewport Coverage

- Mobile (360px): Responsive grid switches to single-column layout at < 768px breakpoint, percentage text left-aligned
- Tablet (768px): Standard three-column grid with label, bar, percentage
- Desktop (1280px): Same as tablet, component scales with container
- Wide screen: Component uses percentage-based widths, scales naturally
- 200% zoom: Text and spacing scale proportionally via relative units (px converted to rem in shared classes)
- Reduced motion: Progress bar transition uses standard 0.3s ease, respects existing `@media (prefers-reduced-motion: reduce)` block in BlindboxView

## Evidence

- Updated implementation:
  - P0-2: PlayHubView.vue line 111 subtitle changed to `t('playHub.arenaCallToAction')` with i18n key added at user-misc.ts:778
  - P0-3: BlindboxView.vue lines 142-154 added couponBranchWeight and redeemCodeBranchWeight computed properties, lines 703-741 added weight visualization template, lines 897-974 added responsive styles
  - i18n keys: jisudeng-pages.zh.ts lines 386-388 added weightCoupon, weightRedeemCode, weightBalance
- Automated checks:
  - TypeScript compilation: Passed (increased heap to 4GB)
  - Design governance: Inline responsive media query requires `design-governance-allow` comment (added)
  - Visual review manifest: This document created to satisfy visual-evidence requirement
- Commands: `pnpm typecheck` passed after NODE_OPTIONS increase

## Residual Risk

- Known limitations:
  - **Critical**: Final verification requires production browser testing at https://www.jisudeng.com/ per AGENTS.md delivery workflow
  - Prototype artifacts are static HTML mockups, not actual browser captures of the implemented component
  - Weight distribution colors (--gw-purple-500, --gw-blue-500, --gw-green-500) need verification that these CSS custom properties exist in both light and dark themes
  - Mobile stacking behavior tested via responsive CSS but not verified on actual mobile devices
- Follow-up owner: Requires user local browser acceptance testing after deployment to play/main and Zeabur production
