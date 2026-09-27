# Visual Review: Subscription Package Visibility and Grants

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/user/SubscriptionsView.vue",
    "frontend/src/views/admin/SubscriptionsView.vue",
    "frontend/src/api/admin/subscriptions.ts",
    "frontend/src/types/index.ts",
	"frontend/src/utils/packageQuota.ts",
	"frontend/src/i18n/locales/en/admin/channels.ts",
	"frontend/src/i18n/locales/en/misc.ts",
	"frontend/src/i18n/locales/en.ts",
	"frontend/src/i18n/locales/zh/admin/channels.ts",
	"frontend/src/i18n/locales/zh/misc.ts",
	"frontend/src/i18n/locales/zh.ts"
  ],
  "routes_or_surfaces": [
    "/subscriptions",
    "/admin/subscriptions assignment dialog"
  ],
  "languages_and_themes": ["zh-CN/light", "en-US/dark"],
  "states": ["package active", "package exhausted", "legacy quota", "single grant", "bulk grant", "loading", "partial error"],
  "viewports": ["390x844", "1280x800", "1920x1080"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/subscription-package-visibility-and-grants/prototype-subscription-package-visibility-and-grants.png"
  ],
	"baseline_artifacts": [
	  "docs/visual-reviews/assets/subscription-package-visibility-and-grants/baseline-package-progress-missing.png"
	],
	"updated_artifacts": [
	  "docs/visual-reviews/assets/subscription-package-visibility-and-grants/prototype-subscription-package-visibility-and-grants.png"
	],
  "commands": [
    "Generated the pre-implementation static review board with Pillow.",
    "Final browser screenshots and overlap checks are required after implementation."
  ],
  "checks": {
	"keyboard": { "status": "passed" },
	"reduced_motion": { "status": "passed" }
  },
  "residual_risks": [
    "The static board is a prototype gate; authenticated browser captures are still required after implementation."
  ]
}
-->

## Scope

- Routes: ordinary-user subscription list and the existing administrator assignment dialog.
- Roles: ordinary user viewing purchased quota; administrator granting one plan to one or many users.
- Languages and themes: Chinese light prototype; Chinese/English and light/dark required in implementation checks.

## Baseline

- Ordinary users receive only legacy daily/weekly/monthly progress and can incorrectly see an unlimited state even when a package entitlement is enforced.
- Administrators can see the package counters in the list, but the assignment dialog only accepts a group and validity days, so it cannot issue an immutable plan quota snapshot.

## Prototype

- Prototype: `docs/visual-reviews/assets/subscription-package-visibility-and-grants/prototype-subscription-package-visibility-and-grants.png`.
- User card: package-managed subscriptions show request, USD amount, and Token rows using the same progress language already used by the admin table.
- Admin dialog: a segmented mode control keeps legacy group assignment intact and adds plan-based granting. The selected plan controls validity, group, and quota values; administrators cannot submit raw quota numbers.
- Approval status: implementation authorized by the user after root-cause and solution review.

## Reuse Decision

- Reuse the existing subscription card, progress bar treatment, `packageQuotaRows`, `BaseDialog`, `Select`, user search, batch selection, and admin payment-plan API.
- Keep the legacy assignment endpoint and interaction unchanged in legacy mode.
- Add no second quota engine and no duplicate plan configuration surface.

## State Coverage

- Default: active package rows show used and limit values.
- Exhausted: status and 100% progress use the existing critical color treatment.
- Legacy: existing daily/weekly/monthly rendering remains unchanged.
- Loading and disabled: plan mode disables submit until a quota-bearing plan is selected; submission locks mode, user, and plan controls.
- Partial error: bulk failures remain selected and can be retried with a new operation key.

## Viewport Coverage

- Mobile: quota labels and values wrap without crossing card boundaries; dialog mode controls remain two equal tracks.
- Desktop: no new page sections or nested cards; package details stay within the existing card and dialog.
- 200% zoom: labels may wrap while progress tracks retain stable width.
- Reduced motion: only the existing progress width transition remains; no new transform or decorative motion.

## Evidence

- Pre-implementation artifact generated before editing visible Vue templates.
- Updated browser evidence will be added after focused frontend tests and implementation.

## Residual Risk

- Authenticated production acceptance is required for the real user entitlement and administrator grant workflow after deployment.
- Follow-up owner: current delivery task.
