# API Key Onboarding Phase 1 Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/keys/APIOnboardingPanel.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/en/misc.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/dashboard.ts",
    "frontend/src/i18n/locales/zh/misc.ts",
    "frontend/src/views/admin/orders/APIOnboardingConfigPanel.vue",
    "frontend/src/views/admin/orders/AdminPaymentPlansView.vue",
    "frontend/src/views/user/KeysView.vue"
  ],
  "routes_or_surfaces": [
    "/keys empty state for authenticated users without API keys",
    "/admin/payment/plans embedded API onboarding settings panel",
    "API key create dialog group preselection",
    "payment purchase route entry for recharge and subscription CTAs"
  ],
  "languages_and_themes": [
    "zh-CN light static board",
    "zh-CN dark class review",
    "en-US light copy review",
    "en-US dark class review"
  ],
  "states": [
    "public config disabled",
    "new user empty state enabled",
    "recommended group available",
    "recommended group unavailable",
    "recommended plan available",
    "recommended plan unavailable",
    "recharge CTA",
    "buy plan CTA",
    "open docs CTA",
    "admin loading",
    "admin add card",
    "admin validation error",
    "admin save success"
  ],
  "viewports": [
    "390x844",
    "768x1024",
    "1440x900",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/api-key-onboarding-no-nav-v3/prototype-api-onboarding-settings-embedded.png",
    "docs/visual-reviews/assets/api-key-commercial-onboarding-v2/prototype-api-key-onboarding-v2-user.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/api-key-new-user-onboarding-prototype/prototype-api-key-new-user-page.png",
    "docs/visual-reviews/assets/api-key-new-user-onboarding-prototype/prototype-api-key-create-modal.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/api-key-onboarding-no-nav-v3/prototype-api-onboarding-settings-embedded.png",
    "docs/visual-reviews/assets/api-key-commercial-onboarding-v2/prototype-api-key-onboarding-v2-user.png"
  ],
  "commands": [
    "pnpm vitest run src/components/keys/__tests__/APIOnboardingPanel.spec.ts src/views/admin/orders/__tests__/APIOnboardingConfigPanel.spec.ts",
    "pnpm vitest run src/views/user/__tests__/KeysView.spec.ts",
    "pnpm typecheck",
    "pnpm design:check",
    "go test -tags=unit ./internal/service -run TestAPIOnboarding"
  ],
  "checks": {
    "keyboard": {
      "status": "not-applicable",
      "reason": "No browser runner was used in this shell; controls remain native buttons, inputs and selects and are covered by component tests."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "The implementation only uses color transitions already present in the local button and card patterns."
    }
  },
  "residual_risks": [
    "The artifacts are static review boards and accepted prototype images, so final browser screenshot acceptance is still required before release sign-off.",
    "The rollout is intentionally limited to the API Keys empty state; existing users with keys keep the current table-first page and may need a later non-empty guidance entry."
  ]
}
-->

## Scope

This review covers phase 1 of the API Key onboarding and commercial recommendation flow. The user-facing change appears only in the `/keys` empty state for authenticated users with no API keys. The admin-facing change is embedded inside the existing payment plans admin page, so it does not add a new backend/admin sidebar navigation item.

The implementation adds a centrally stored public `api_onboarding` config, an admin recommendation-card editor, and a shared `APIOnboardingPanel` consumed by `KeysView`. It also updates Chinese and English copy for the affected Keys and payment admin surfaces.

## Baseline

The previous `/keys` new-user path required the user to click “Create API Key” before understanding groups, recharge, subscription plans or docs. The create dialog could list real groups, but the page did not explain why one group or plan should be chosen. The earlier prototype artifacts show that first-step concept and modal flow, but they did not address the later no-new-nav admin boundary.

Baseline artifacts:

- `docs/visual-reviews/assets/api-key-new-user-onboarding-prototype/prototype-api-key-new-user-page.png`
- `docs/visual-reviews/assets/api-key-new-user-onboarding-prototype/prototype-api-key-create-modal.png`

## Prototype

The accepted boundary is to manage API onboarding recommendations inside the existing payment plans admin surface. The prototype shows an embedded settings panel with a left card list and right editor, using 8px bordered admin cards and ordinary inputs, selects, checkboxes and toolbar buttons. The user-facing prototype shows recommendation cards above the normal empty state, with CTAs for creating a key, topping up, buying a plan or opening docs.

Prototype artifacts:

- `docs/visual-reviews/assets/api-key-onboarding-no-nav-v3/prototype-api-onboarding-settings-embedded.png`
- `docs/visual-reviews/assets/api-key-commercial-onboarding-v2/prototype-api-key-onboarding-v2-user.png`

Approval status: user approved implementation after confirming no new admin nav item and preserving the current project visual style.

Scope boundary: no subscription page redesign, no page width change, no new visual system, no new admin navigation item, and no rewrite of the existing API key table.

## Reuse Decision

The admin panel follows the existing `PlanStorefrontConfigPanel` pattern: bordered white cards, compact toolbar buttons, left list, right editor and existing toast/store error handling. The user panel uses the existing `Icon.vue`, `GroupBadge`, `btn` classes, public settings fetch path and current `/purchase` route contract. The create-key dialog remains owned by `KeysView`; onboarding only preselects a valid group when the user can actually access it.

No design-system exception was added. The change does not introduce a page shell, arbitrary max width, custom icon family or new sidebar route.

## State Coverage

Default: when `api_onboarding.enabled` is false or missing, the `/keys` page falls back to the existing empty state. When enabled, usable recommendation cards render above the existing empty state.

Filtering: create-key cards with unavailable groups are hidden. Buy-plan cards with unavailable plans are hidden. Docs cards are hidden when `doc_url` is empty.

Actions: create-key cards open the existing dialog and pass the recommended group id only if it is present in `/groups/available`. Recharge goes to `/purchase`. Plan CTAs go to `/purchase?tab=subscription&group=<group_id>`. Docs open `doc_url` using the existing internal/external routing split.

Admin states: loading shows a bordered compact loading state; add/delete/sort/edit use existing controls; validation blocks missing titles, duplicate enabled titles, missing groups, missing plans and buy-plan cards without a plan; save success uses the app toast.

## Viewport Coverage

Mobile at 390px keeps the recommendation cards stacked through the existing grid behavior. Tablet at 768px keeps form fields readable and does not add page-level scrolling. Desktop at 1440px and wide desktop at 1920px keep the admin editor embedded inside the payment plans page rather than creating a centered standalone settings page.

The implementation does not change `AppLayout`, `TablePageLayout`, `PageFrame`, route max width or authenticated console gutter behavior.

## Evidence

Updated static review artifacts are the accepted admin embedded settings board and the user onboarding board:

- `docs/visual-reviews/assets/api-key-onboarding-no-nav-v3/prototype-api-onboarding-settings-embedded.png`
- `docs/visual-reviews/assets/api-key-commercial-onboarding-v2/prototype-api-key-onboarding-v2-user.png`

Automated coverage added:

- `APIOnboardingPanel.spec.ts` verifies usable-card filtering, balance hint rendering and emitted CTA actions.
- `APIOnboardingConfigPanel.spec.ts` verifies admin add/save payload and buy-plan validation.
- `KeysView.spec.ts` verifies the empty-state public config, recommended group preselection, recharge routing and plan routing.

Commands run during this pass:

```bash
pnpm vitest run src/components/keys/__tests__/APIOnboardingPanel.spec.ts src/views/admin/orders/__tests__/APIOnboardingConfigPanel.spec.ts
pnpm vitest run src/views/user/__tests__/KeysView.spec.ts
pnpm typecheck
go test -tags=unit ./internal/service -run TestAPIOnboarding
```

## Residual Risk

This record uses static review boards and prototype PNGs, not live browser screenshots. Before release sign-off, the owner should capture the real `/keys` empty state and the admin payment plans page at mobile and desktop widths after seeding one enabled onboarding config.

This phase only improves the no-key new-user path. A later phase can add a smaller non-empty “接入建议/消费路径” entry for existing users without disturbing the API key table.
