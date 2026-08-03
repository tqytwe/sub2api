# Visual Review: New-user growth in the existing invite surface

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/user/AffiliateView.vue",
    "frontend/src/views/admin/PlayOpsView.vue",
    "frontend/src/api/play.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh/dashboard.ts",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/zh/admin/playOps.ts",
    "frontend/src/i18n/locales/en/admin/playOps.ts"
  ],
  "routes_or_surfaces": ["/affiliate", "/admin/play-ops?tab=campaigns"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "loading", "disabled", "empty", "validation-error", "claimable", "claimed", "revoked"],
  "viewports": ["360x800", "768x900", "1280x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-08-03-new-user-growth/prototype-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-08-03-new-user-growth/prototype-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-08-03-new-user-growth/updated-1280.png"],
  "commands": ["pnpm design:check", "pnpm typecheck", "pnpm lint:check", "git diff --check"],
  "checks": {
    "keyboard": {"status": "not-applicable", "reason": "Static review artifact; browser keyboard review remains a release gate."},
    "reduced_motion": {"status": "not-applicable", "reason": "Static review artifact; no animation is represented."},
    "copy_locale": {"status": "passed", "notes": "Chinese and English labels are supplied for activity type, metric, policy and reward states."}
  },
  "residual_risks": ["Authenticated browser acceptance and production three-role verification remain release gates."]
}
-->

## Scope

The existing `/affiliate` page gains a new-user growth section above the existing inviter campaign and standard 10% rebate sections. The admin limited-event editor gains linked invite campaign, metric, tier schedule and explicit rebate-policy controls. No new navigation or route is introduced.

## Baseline

The approved Play Ops shell prototype at `docs/visual-reviews/assets/2026-08-03-new-user-growth/prototype-1280.png` is reused for density, spacing and card treatment. The user page already owns the invite campaign surface; the new section keeps that hierarchy.

## Prototype

The existing Play Ops growth prototype is the visual baseline and approved shell reference. The scope boundary is limited to one inline growth section and additional fields in the existing campaign editor.

## Reuse Decision

The implementation reuses `AppLayout`, `Icon`, existing growth-world panels, shared input/select/button styles and the existing referral claim flow. No new icon, route, sidebar item, page width or floating card system is added.

## State Coverage

The user section covers no eligible campaign, progress below a tier, claimable tier, claimed/frozen tier and revoked/debt states returned by the shared reward ledger. The admin form covers benefit-only events, linked growth events, policy default/opt-in and invalid tier JSON.

## Viewport Coverage

The growth tier grid collapses from four columns to two and then one column. Long bilingual labels wrap within their controls. A browser capture at 360px, 768px and 1280px is still required after dependencies are installed.

## Evidence

Static prototype and updated artifact paths are present. `pnpm design:check`, `pnpm typecheck` and focused lint checks passed locally.

## Residual Risk

Authenticated browser review and production acceptance must verify the inviter, invited new user and administrator paths.
