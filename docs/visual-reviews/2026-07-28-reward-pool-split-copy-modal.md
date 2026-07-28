# Visual Review: reward pool split copy modal

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/play.ts",
    "frontend/src/components/admin/AdminCouponOperations.vue",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/views/public/BlindboxView.vue"
  ],
  "routes_or_surfaces": ["/play/blindbox", "/play/quiz", "/play", "/admin/promo-codes"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["blindbox reward split copy", "blindbox reward celebration", "quiz reward copy", "admin reward split editor", "pool list split label"],
  "viewports": ["360x800", "768x900", "1280x800", "1600x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/coupon-system/prototype-coupon-operations.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/coupon-system/baseline-coupon-operations.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/coupon-system/updated-coupon-operations.png"
  ],
  "commands": [
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend test:run -- AdminCouponOperations.spec.ts BlindboxView.spec.ts QuizQuestView.spec.ts PlayHubView.spec.ts"
  ],
  "checks": {
    "keyboard": { "status": "passed", "notes": "The split controls are native number inputs and preset buttons in the existing modal footer flow." },
    "reduced_motion": { "status": "passed", "notes": "Removing the success toast does not add motion; the existing reward celebration overlay remains user-dismissed." }
  },
  "residual_risks": [
    "Static review board evidence is reused from the coupon operations review; browser final acceptance should verify the published split values returned by the live API."
  ]
}
-->

## Scope

- Routes: public blindbox page, quiz page, play hub, and administrator coupon reward pool modal.
- Roles: public player and administrator.
- Change type: copy correction, split configuration controls, and reward celebration behavior.

## Baseline

- Old public copy described fixed blindbox and quiz reward ratios even after reward pools became configurable.
- The blindbox page calculated balance-tier odds with a fixed 40% branch weight.
- The admin reward-pool save flow could preserve entry weights while still submitting a fixed outer split.

## Prototype

- Public copy now says rewards follow the currently published backend pool instead of naming fixed ratios.
- Blindbox balance-tier odds and expected cash return use the API-provided balance split.
- The admin pool editor exposes coupon and balance split inputs, presets, and a 100% total check in the existing pool modal.

## Reuse Decision

- Reused the existing coupon operations page, modal shell, buttons, inputs, `DataTable`, and static coupon-system review board artifacts.
- No new navigation entry or standalone operations surface was added.

## State Coverage

- Blindbox: guest public pool, authenticated status, reward overlay after open, and last reward card below the action area.
- Quiz and play hub: pending reward copy no longer exposes fixed 80/20 or default balance formulas.
- Admin: draft copy, published-pool copy-to-draft, split presets, split total validation, and split label display.

## Viewport Coverage

- Mobile: split preset buttons wrap in the modal; number fields stack through the existing responsive grid.
- Desktop: split fields sit beside each other and preserve existing modal actions.
- Themes and languages: strings are provided in zh and en locale files; Chinese locale displays Chinese operational labels.

## Evidence

- Updated screenshot or recording: `docs/visual-reviews/assets/coupon-system/updated-coupon-operations.png`.
- Automated visual or overlap checks: design governance validates manifest coverage and changed-file tracking.
- Commands run: `pnpm --dir frontend design:check`, `pnpm --dir frontend typecheck`, targeted frontend tests, and focused backend tests.

## Residual Risk

- Production cache and already-open browser sessions may briefly show old bundles until the new deployment is loaded.
- Live acceptance should verify that changing a published pool split changes both public odds display and actual coupon/balance branch selection.
