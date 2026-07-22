# Visual Review: support-contact-announcement-governance

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/common/AnnouncementBell.vue",
    "frontend/src/components/common/AnnouncementPopup.vue",
    "frontend/src/components/common/AnnouncementContent.vue",
    "frontend/src/components/common/SupportFloatingCard.vue",
    "frontend/src/components/layout/AppHeader.vue",
    "frontend/src/components/layout/PageFrame.vue",
    "frontend/src/i18n/locales/en/admin/resources.ts",
    "frontend/src/i18n/locales/jisudeng-home.en.ts",
    "frontend/src/i18n/locales/jisudeng-home.zh.ts",
    "frontend/src/i18n/locales/zh/admin/resources.ts",
    "frontend/src/router/index.ts",
    "frontend/src/router/publicNavigation.ts",
    "frontend/src/router/title.ts",
    "frontend/src/views/HomeView.vue",
    "frontend/src/views/admin/AnnouncementsView.vue",
    "frontend/src/views/public/ContactView.vue",
    "frontend/src/views/user/ProfileView.vue",
    "frontend/src/views/user/RedeemView.vue"
  ],
  "routes_or_surfaces": [
    "/home",
    "/contact",
    "/login and /register support aside",
    "/admin/announcements editor",
    "/profile",
    "/redeem",
    "global support floating card",
    "authenticated user menu"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "zh-CN/dark",
    "en-US/light",
    "en-US/dark"
  ],
  "states": [
    "default support panel",
    "two primary QR codes",
    "secondary contact list",
    "copy and open actions",
    "announcement edit preview",
    "announcement upload loading",
    "empty QR fallback",
    "public settings refresh"
  ],
  "viewports": [
    "360x800",
    "768x1024",
    "1280x800",
    "1600x900"
  ],
  "artifact_mode": "static-review-board",
  "baseline_artifacts": [
    "docs/visual-reviews/assets/support-contact-announcement-governance/before-contact-popover.png",
    "docs/visual-reviews/assets/support-contact-announcement-governance/before-admin-contact-text.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/support-contact-announcement-governance/after-support-contact-panel.png",
    "docs/visual-reviews/assets/support-contact-announcement-governance/after-announcement-editor.png",
    "docs/visual-reviews/assets/support-contact-announcement-governance/after-home-nav-page-frame.png"
  ],
  "commands": [
    "go test ./internal/service -run 'Announcement|SupportContact'",
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend exec vitest run src/utils/__tests__/announcementMarkdown.spec.ts src/router/__tests__/title.spec.ts src/components/layout/__tests__/docUrlSanitization.spec.ts src/views/user/__tests__/ProfileView.spec.ts",
    "node static PNG artifact generation for review boards"
  ],
  "checks": {
    "keyboard": {
      "status": "not-applicable",
      "reason": "No browser runner is installed in this shell; focusable controls were reviewed in component templates."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "No browser runner is installed in this shell; motion-sensitive surfaces remain governed by CSS policy."
    }
  },
  "residual_risks": [
    "Rendered browser screenshots still need to be captured in an environment with Playwright or a manual browser session.",
    "The generated updated artifacts are static review boards and do not replace final browser acceptance."
  ]
}
-->

## Scope

- Routes: `/home`, `/contact`, `/admin/announcements`, `/profile`, `/redeem`, login/register support aside, user menu, and the global support floating card.
- Roles: guest, authenticated user, and admin.
- Languages and themes: Chinese and English copy were updated for light and dark surfaces.

## Baseline

- Current behavior: support contact information appeared as plain text in a small popover and as one unstructured settings input, so QQ, WeChat and copy behavior were mixed together.
- Baseline screenshot or recording: `before-contact-popover.png` and `before-admin-contact-text.png`.
- Inconsistencies observed: public, authenticated and admin surfaces could diverge because contact rendering and announcement Markdown rendering were not owned by shared components.

## Reuse Decision

- Shared layouts and components reused: `SupportContactPanel`, `AnnouncementContent`, `PageFrame`, `Icon.vue`, and existing app store public settings refresh.
- New shared pattern, if any: `router/publicNavigation.ts` centralizes public route-name navigation used by Home and the authenticated user menu.
- Design-system exception, if any: Home remains an allowed art page, but route links, title behavior, customer-service rendering and announcement content stay under shared contracts.

## State Coverage

- Default: support panel reads one public `support_contact` config and PageFrame removes ad hoc page width choices on profile/redeem.
- Hover and active: contact copy/open actions remain button controls; the support floating trigger now uses the shared icon system.
- Focus-visible and keyboard: static template review confirms controls are real buttons or router links; browser tab order still needs live verification.
- Loading, disabled, empty, error and success: announcement image upload has a loading label, upload failures use existing toast, empty QR contacts degrade to icon/value rows, and public settings refresh keeps contact surfaces near-real-time.

## Viewport Coverage

- Mobile: primary QR cards are expected to stack and secondary contacts remain list rows.
- Tablet: PageFrame keeps form pages within a consistent readable measure.
- Desktop: two primary QR cards sit side by side and homepage nav uses the shared route catalog.
- Wide or short screen: workspace-style pages keep max width through PageFrame instead of independent route-specific centering.
- 200% zoom and reduced motion: not executed in this shell; CSS governance remains the local guard until browser evidence is added.

## Evidence

- Updated screenshot or recording: static review boards under `docs/visual-reviews/assets/support-contact-announcement-governance/`.
- Automated visual or overlap checks: governance manifest validates file coverage and media signatures; no browser overlap runner is installed in this environment.
- Commands run: backend service tests, frontend typecheck, targeted Vitest, and static artifact generation listed in the manifest.

## Residual Risk

- Known limitations: browser screenshots and real hover/focus captures are still needed before release acceptance.
- Follow-up owner: release owner running the local browser or CI Playwright job should capture rendered artifacts and update this record if visual output differs.
