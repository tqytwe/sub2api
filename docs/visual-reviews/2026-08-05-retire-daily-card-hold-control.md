# Retire Daily Card Hold Control Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/subscriptions.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/views/admin/SubscriptionsView.vue"
  ],
  "routes_or_surfaces": ["/admin/subscriptions"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["daily-card active row", "daily-card exhausted row", "quota compensation confirmation"],
  "viewports": ["390x844", "1280x820"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/daily-card-lifecycle/prototype-admin-subscriptions-effective-status.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/daily-card-lifecycle/baseline-admin-subscriptions.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/daily-card-lifecycle/prototype-admin-subscriptions-effective-status.png"],
  "commands": ["pnpm --dir frontend design:verify", "pnpm --dir frontend typecheck"],
  "checks": {
    "keyboard": {"status": "passed", "notes": "Removing the hold action removes its focus target; remaining buttons retain their existing keyboard behavior."},
    "reduced_motion": {"status": "passed", "notes": "No motion behavior changes."}
  },
  "residual_risks": ["Static evidence does not replace an authenticated administrator browser check after deployment."]
}
-->

## Scope

The administrator subscription table no longer presents a daily-card hold-release action. Daily-card compensation remains available and no monthly-card control changes.

## Baseline

The existing daily-card subscriptions review board supplies the dense table layout, action spacing, and responsive baseline.

## Prototype

The updated state removes only the obsolete hold action and its confirmation flow. Adjacent action controls retain their existing icons, labels, dimensions, and order.

## Reuse Decision

The review reuses the existing subscriptions table, shared button styles, localization system, and quota compensation dialog. No component or layout pattern is introduced.

## State Coverage

Active and exhausted daily-card rows no longer expose a hold action. The remaining compensation action does not describe or operate on holds. Monthly-card reset controls are unchanged.

## Viewport Coverage

On mobile and desktop, removal reduces the action column content without changing table tracks or introducing overflow. Existing focus and reduced-motion behavior is unchanged.

## Evidence

The referenced static review artifacts show the existing subscriptions surface at both target viewport classes. `design:verify` validates the manifest and artifact signatures; type checking validates the removed client API references.

## Residual Risk

Production acceptance still requires an authenticated administrator to confirm that the obsolete action is absent after deployment.
