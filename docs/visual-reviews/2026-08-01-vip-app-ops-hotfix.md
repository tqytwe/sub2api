# Visual Review: VIP Publish And APP Analytics Recovery

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/play/AdminAppAnalyticsOperations.vue",
    "frontend/src/components/admin/play/AdminMembershipOperations.vue",
    "frontend/src/i18n/locales/en/admin/playOps.ts",
    "frontend/src/i18n/locales/zh/admin/playOps.ts"
  ],
  "routes_or_surfaces": ["/admin/play-ops?tab=app-analytics", "/admin/play-ops?tab=membership"],
  "languages_and_themes": ["zh-CN/light", "en-US/dark"],
  "states": ["default", "loading", "empty", "error", "disabled", "success", "step-up"],
  "viewports": ["360x800", "1280x800"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/vip-app-ops-hotfix/prototype-play-ops-recovery.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/vip-app-ops-hotfix/baseline-app-analytics-empty.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/vip-app-ops-hotfix/updated-play-ops-recovery-board.png"
  ],
  "commands": [
    "pnpm exec vitest run src/components/admin/play/__tests__/AdminAppAnalyticsOperations.spec.ts",
    "pnpm exec vue-tsc --noEmit",
    "pnpm design:check"
  ],
  "checks": {
    "keyboard": {
      "status": "not-applicable",
      "reason": "No browser runtime is available; final keyboard validation remains a browser acceptance item."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "This recovery changes no animation; final browser motion verification remains required."
    }
  },
  "residual_risks": [
    "Static review board only: a real administrator browser screenshot and final browser acceptance remain required after deployment."
  ]
}
-->

## Scope

- Routes: `玩法运营 → APP 数据` and `玩法运营 → 会员运营`.
- Roles: authenticated administrator with TOTP enabled for VIP publication.
- Languages and themes: Chinese light is the reported production state; English dark remains covered by shared i18n and existing layout tokens.

## Baseline

- Current behavior: when the API returned `versions: null`, the APP data component read `.length` and threw during render, leaving the selected tab blank.
- Baseline screenshot: `docs/visual-reviews/assets/vip-app-ops-hotfix/baseline-app-analytics-empty.png`; it is a cropped user-provided production capture with account-identifying header content removed.
- Inconsistencies observed: no empty state, no error action, and no way for the operator to distinguish an empty dataset from a broken page.

## Prototype

- Prototype design image: `docs/visual-reviews/assets/vip-app-ops-hotfix/prototype-play-ops-recovery.png`.
- Approval status: reuses the approved Play Ops workbench layout, metric cards, filters, compact tables, and tab system from the prior growth operations review.
- Scope boundary: no new page shell, card system, colors, icons, or routing. The change restores the existing APP data content and opens the existing TOTP step-up dialog before a sensitive VIP publish retry.

## Reuse Decision

- Shared layouts and components reused: existing Play Ops tabs, `Icon`, button/input classes, card/table classes, and `TotpStepUpDialog`.
- New shared pattern: none.
- Design-system exception: none.

## State Coverage

- Default: APP metrics, funnel, and version table retain their existing full-width operations layout.
- Hover and active: existing buttons and tabs remain unchanged.
- Focus-visible and keyboard: existing native controls and the shared TOTP dialog retain ownership; browser verification remains outstanding.
- Loading, disabled, empty, error and success: `versions: null` now resolves to the explicit empty state; request errors retain the existing retry panel; VIP publish disables the command while pending, requests TOTP when required, and reports localized validation/conflict messages.

## Viewport Coverage

- Mobile: the existing flex and grid breakpoints keep filters and metric cards stacked.
- Tablet: the existing two-column analytics grid is retained.
- Desktop: the static board validates the existing dense two-panel analytics layout.
- Wide or short screen: no new page-level width, height, or scrolling ownership was introduced.
- 200% zoom and reduced motion: no new fixed geometry or animation was introduced; final browser checks remain required.

## Evidence

- Updated static review board: `docs/visual-reviews/assets/vip-app-ops-hotfix/updated-play-ops-recovery-board.png`.
- Automated visual or overlap checks: component test exercises the legacy `versions: null` response without a render crash; TypeScript checks the TOTP controller and nullable array handling.
- Commands run: `pnpm exec vitest run src/components/admin/play/__tests__/AdminAppAnalyticsOperations.spec.ts`, `pnpm exec vue-tsc --noEmit`, and `pnpm design:check`.

## Residual Risk

- Known limitations: the evidence is a static review board because this environment has no browser runtime. It must not be treated as a browser screenshot.
- Follow-up owner: operations administrator completes Chinese light and English dark browser acceptance after deployment, including an empty APP dataset, populated versions, TOTP prompt, version conflict, and validation error states.
