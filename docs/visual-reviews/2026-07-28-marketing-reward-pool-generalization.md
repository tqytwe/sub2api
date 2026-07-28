# Visual Review: marketing reward pool generalization

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/play.ts",
    "frontend/src/api/redeem.ts",
    "frontend/src/components/admin/AdminCouponOperations.vue",
    "frontend/src/components/play/RewardCelebrationOverlay.vue",
    "frontend/src/content/public-docs-data.zh.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/types/coupon.ts",
    "frontend/src/views/admin/RedeemView.vue",
    "frontend/src/views/admin/PromoCodesView.vue",
    "frontend/src/views/public/BlindboxView.vue",
    "frontend/src/views/user/CheckInView.vue",
    "frontend/src/views/user/RedeemView.vue"
  ],
  "routes_or_surfaces": ["/admin/promo-codes", "/admin/redeem-codes", "/blindbox", "/checkin", "/redeem"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["reward pool activity tabs", "three-way split editor", "redeem code prize rows", "balance prize rows", "redeem code batch fields", "issued redeem code trace columns", "blindbox persistent reward overlay", "check-in random reward copy", "check-in eligibility warning", "check-in coupon/redeem success toast"],
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
    "pnpm --dir frontend test:run AdminCouponOperations CheckInView BlindboxView QuizQuestView"
  ],
  "checks": {
    "keyboard": { "status": "passed", "notes": "Tabs, inputs, checkboxes, add/remove buttons, and modal actions reuse native focusable controls." },
    "reduced_motion": { "status": "passed", "notes": "No new continuous animation was added; check-in still uses existing page motion classes." },
    "copy_locale": { "status": "passed", "notes": "Chinese locale maps reward source/status/activity labels to Chinese strings." }
  },
  "residual_risks": [
    "Static review board evidence is used because no local browser acceptance is performed in this server environment.",
    "Final product acceptance should verify the admin modal at 360px and 1280px after deployment."
  ]
}
-->

## Scope

This review covers the visible admin and user-facing changes for the generalized marketing reward pool:

- Admin coupon operations now includes a check-in pool tab.
- Pool editor now shows coupon, redeem-code, and balance split fields.
- Admin can add redeem-code prize rows and balance prize rows.
- Check-in page copy no longer promises a fixed balance amount; it says the result is randomly opened and issued immediately.

## Baseline

- Admin pool tabs only covered blind box and quiz reward pools.
- The split editor only showed coupon and balance branches.
- Check-in hero copy displayed a fixed daily balance reward.

## Prototype

- Reuse the existing coupon operations modal and table density.
- Keep the new controls as plain form rows with native inputs and existing button classes.
- Keep check-in copy short and operational: “完成签到后随机开奖，结果即时发放”.

## Reuse Decision

- Reused `BaseDialog`, `DataTable`, existing `btn` and `input` classes, `Icon`, and current tab navigation.
- Did not introduce a new top-level marketing page in this iteration; it remains in the existing promo/coupon operations entrance.

## State Coverage

- Admin: draft edit, copy published pool as draft, split total success/error, empty prize rows, enabled/disabled prize rows.
- Check-in: loading, disabled, already checked in, random reward success, coupon success, redeem-code success.

## Viewport Coverage

- Mobile: split fields and prize rows stack.
- Tablet/Desktop: split fields fit in three columns; prize rows use compact grids.
- Theme/language: zh and en locale files updated; Chinese UI displays Chinese source/status labels.

## Evidence

- Static prototype/review board: `docs/visual-reviews/assets/coupon-system/prototype-coupon-operations.png`.
- Automated design manifest check covers changed visible files.

## Residual Risk

- Needs final browser acceptance after deployment because this environment does not provide production browser screenshots.
