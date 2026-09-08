# Visual Review: v0.2.2 Upstream Merge Safety

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/TurnstileWidget.vue",
    "frontend/src/components/account/CreateAccountModal.vue",
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/components/account/accountExpiry.ts",
    "frontend/src/components/admin/group/CodexManifestAccountsField.vue",
    "frontend/src/components/admin/group/ReasoningEffortPolicyFields.vue",
    "frontend/src/components/channels/SupportedModelChip.vue",
    "frontend/src/components/common/GroupSelector.vue",
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/components/modelPlaza/PlazaModelPricingTable.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/admin/accounts.ts",
    "frontend/src/i18n/locales/en/admin/channels.ts",
    "frontend/src/i18n/locales/en/admin/ops.ts",
    "frontend/src/i18n/locales/en/admin/overview.ts",
    "frontend/src/i18n/locales/en/admin/settings.ts",
    "frontend/src/i18n/locales/en/common.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/admin/accounts.ts",
    "frontend/src/i18n/locales/zh/admin/channels.ts",
    "frontend/src/i18n/locales/zh/admin/ops.ts",
    "frontend/src/i18n/locales/zh/admin/overview.ts",
    "frontend/src/i18n/locales/zh/admin/settings.ts",
    "frontend/src/i18n/locales/zh/common.ts",
    "frontend/src/styles/onboarding.css",
    "frontend/src/views/admin/GroupsView.vue",
    "frontend/src/views/admin/SubscriptionsView.vue",
    "frontend/src/views/admin/groupModelAllowlist.ts",
    "frontend/src/views/admin/groupsReasoningEffort.ts",
    "frontend/src/views/admin/modelAllowlistCandidates.ts",
    "frontend/src/views/user/UsageView.vue"
  ],
  "routes_or_surfaces": ["/admin/accounts", "/admin/groups", "/admin/subscriptions", "/admin/settings", "/admin/orders", "/usage", "sidebar", "model plaza"],
  "languages_and_themes": ["zh-CN/light static review board", "en-US/light static review board", "dark-token static code review"],
  "states": ["default", "focus-visible", "loading", "disabled", "empty", "error", "success", "long model identifier"],
  "viewports": ["360x800", "768x1024", "1280x820"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png", "docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/v0200-upstream-safety/prototype-390.png", "docs/visual-reviews/assets/v0200-upstream-safety/prototype-1280.png"],
  "commands": ["pnpm --dir frontend vitest run src/i18n/__tests__/localeKeyCompleteness.spec.ts src/router/__tests__/guards.spec.ts", "pnpm --dir frontend typecheck", "pnpm --dir frontend design:check"],
  "checks": {
    "keyboard": {"status": "passed", "notes": "The new allowlist custom-model fields provide explicit focus-visible rings; other changed controls retain shared native, Toggle, Select, dialog and table focus behavior."},
    "reduced_motion": {"status": "passed", "notes": "Refresh and submission states use static icons, disabled controls and text. The subscription bar transitions only width rather than every property."}
  },
  "residual_risks": ["This is static review-board evidence only, not an authenticated browser capture. Final local-browser acceptance must inspect 360, 768 and 1280 layouts in Chinese and English, light and dark themes, keyboard focus, long model names and all protected administrator operations before release."]
}
-->

## Scope

- Surfaces: the visible upstream v0.2.2 model allowlist, account capability,
  subscription, usage, navigation, Turnstile and bilingual locale changes named
  in the manifest.
- Roles: administrator, ordinary user and member UI contracts stay within their
  existing route ownership and component families.
- Boundary: this record covers visual semantic preservation during the upstream
  merge. It does not authorize a layout redesign, production deployment or a
  replacement of fork-specific Play, wallet, order or localization behavior.

## Baseline

- The retained upstream-safety board provides the closest reviewed operational
  composition for the account, group, settings, model and usage surfaces.
- The merge adds capability data and configuration detail into existing dense
  administration forms and tables. It preserves the shared page frame,
  existing dialog structure, semantic colors and responsive stacking rules.

## Prototype

- The decoded 390px and 1280px static boards record the mobile and desktop
  density baseline reused by the merge.
- No new route shell, floating section card, decorative imagery or custom icon
  system is introduced. The reviewed prototype boundary is existing controls
  with expanded upstream model metadata and allowlist behavior.

## Reuse Decision

- Reused components include `Toggle`, `Select`, `Icon`, account dialogs,
  `GroupSelector`, the shared sidebar, existing table layouts and semantic
  light/dark tokens.
- Model labels remain ordinary text and chips. Long identifiers use the
  existing wrapping/truncation contracts instead of changing fixed dimensions.
- Allowlist custom input fields now show a concrete focus-visible ring, while
  loading uses disabled actions and static refresh iconography.

## State Coverage

- Default and success: Astra, Fast, reasoning-effort and model allowlist
  metadata render in their existing configuration locations.
- Empty and error: no available models, failed account checks and unavailable
  selection paths retain the existing localized empty and error states.
- Focus-visible: the two newly added custom allowlist inputs retain visible
  primary rings after the normal browser focus outline is removed.
- Loading and disabled: account checks, route refresh and subscription usage
  preserve disabled controls; no new continuous animation is used.

## Viewport Coverage

- Mobile: the 360px review boundary retains the existing stacked dialog and
  narrow table behavior.
- Tablet: 768px remains a required final browser verification point for forms,
  tables and side navigation.
- Desktop: the 1280px board retains the operational table/form density.
- Long localized text, 200 percent zoom and dark rendering use existing token
  constraints but remain final browser acceptance checks.

## Evidence

- The manifest artifacts are valid decoded static review boards, not browser
  screenshots. They are non-production development evidence.
- Locale completeness and router guard tests cover Chinese/English key parity
  and protected lazy-route behavior. Typecheck validates the merged component
  bindings; design governance checks the source-level interaction rules.

## Residual Risk

- This review does not replace authenticated local-browser or production
  acceptance. Before an authorized release, verify real administrator and user
  flows at 360/768/1280px in Chinese and English, light and dark themes,
  including focus, loading, error, success and overlong model content.
