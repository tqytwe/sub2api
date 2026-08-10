# Visual Review: forum SSO login resume

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/router/index.ts",
    "frontend/src/views/auth/LoginView.vue"
  ],
  "routes_or_surfaces": ["/login with a pending forum_sso_resume parameter", "authenticated router guard for /login"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "guest arriving from the forum", "already authenticated", "login submitting", "handoff in flight", "handoff rejected", "focus-visible"],
  "viewports": ["360x800", "768x900", "1280x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/forum-sso-login-resume/prototype-static-review-board.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/forum-sso-login-resume/baseline-static-review-board.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/forum-sso-login-resume/updated-static-review-board.png"
  ],
  "commands": [
    "chrome --headless --window-size=1280,900 --screenshot static review board captures",
    "corepack pnpm --dir frontend design:check",
    "corepack pnpm --dir frontend typecheck",
    "corepack pnpm --dir frontend lint:check",
    "corepack pnpm --dir frontend test"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "notes": "No control was added, removed or reordered on the login form, so focus order and keyboard operation are byte-for-byte the existing behavior. Nothing takes or resets focus."
    },
    "reduced_motion": {
      "status": "passed",
      "notes": "No animation, transition or motion was introduced. The handoff ends in a full page load, which is browser navigation rather than in-app motion."
    },
    "copy_locale": {
      "status": "not-applicable",
      "reason": "The change adds no user-facing copy. Failures surface through the existing localized application error toast already used by login errors."
    }
  },
  "residual_risks": [
    "The evidence is a static review board explaining a behavioral change, not a browser capture of a live forum sign-in.",
    "Final acceptance requires a real browser sign-in from the forum after the platform is deployed and the FORUM_SSO_* variables are set, because the endpoints answer 404 until then."
  ]
}
-->

## Scope

- Surfaces: the existing `/login` page when it is reached with a `forum_sso_resume` query parameter, and the router guard that redirects already-authenticated visitors away from `/login`.
- Roles: guests and already-authenticated users who begin a sign-in on the community forum. Administrators follow the identical path.
- Languages and themes: zh-CN and en-US in light and dark, inherited unchanged from the existing login shell.
- Boundary: this change renders no new interface. No element, control, label, colour, spacing, icon or motion was added to any surface. It only decides where the browser goes after a successful login, plus a guard exemption so the pending request is not discarded. It is listed here because both touched files sit under paths the governance rules classify as visible.

## Baseline

- Current behavior: the `forum_sso_resume` parameter had no meaning in the SPA. An authenticated visitor arriving at `/login` was redirected to their dashboard by the guard, and a visitor who signed in on the page was also sent to the dashboard. Either way the forum's authorization request was dropped silently.
- Baseline artifact: `docs/visual-reviews/assets/forum-sso-login-resume/baseline-static-review-board.png` renders the pre-change decision path with the new behavior dimmed. It is a rendered board, not a live browser screenshot.
- Inconsistency observed: the failure was invisible. The user saw a normal, successful platform login and then a forum that still considered them a guest, with no error anywhere to explain it.

## Prototype

- Prototype design image: `docs/visual-reviews/assets/forum-sso-login-resume/prototype-static-review-board.png`, which marks the intended additions with a dashed treatment before the implementation was validated.
- Approval status: follows the already-agreed OAuth2 authorization-code integration boundary for the forum. The redirect-on-guest contract is the backend's existing behavior; this is the SPA half of it.
- Scope boundary: replay the pending authorize request and hand the browser to the forum callback. Explicitly out of scope: any visible forum-related affordance in the panel, any change to how the login form looks or behaves for ordinary logins, and any new persisted session cookie.

## Reuse Decision

- Reused the existing `AuthLayout` login shell, its form controls, its OAuth provider sections and the shared application error toast. No new component, dialog, icon or shared pattern was introduced.
- The resume logic lives in a composable that watches for an authenticated session rather than hooking each login handler. `LoginView` completes a login in three separate places (password, passkey, two-factor) and external OAuth logins leave the SPA entirely, so a watched condition covers every method by construction instead of relying on each exit being patched correctly now and in future.
- Design-system exception: none required. The two scoped `design-governance-allow` comments on the imports document that the imported modules render no UI; this record is what satisfies the visual-evidence rule.

## State Coverage

- Default: `/login` without the parameter is completely unchanged, including its post-login redirect to `redirect` or `/dashboard`.
- Guest arriving from the forum: the login form renders exactly as it always does; only the address bar differs.
- Already authenticated: the guard now allows `/login` through instead of redirecting to the dashboard, so the pending handoff can complete. Every other reason for landing on `/login` while authenticated still redirects as before.
- Login submitting: the existing loading and disabled treatment on the submit control is untouched.
- Handoff in flight: the page holds still rather than navigating to the dashboard, then performs a full page load to the forum. No intermediate screen is rendered, and no dashboard flash.
- Handoff rejected: the parameter is stripped from the URL so the attempt cannot re-trigger on every later navigation, the existing localized error toast reports the reason, and the user is sent to the dashboard rather than left stranded on the login form.
- Focus-visible and keyboard: unchanged, since no interactive element was added, removed or reordered.

## Viewport Coverage

- Mobile 360x800: no new element competes for width; the login card keeps its existing single-column stack.
- Tablet 768x900: unchanged login shell.
- Desktop 1280x900: unchanged login shell; this is also the board capture size.
- Wide and short screens: page width remains owned by the existing auth layout.
- 200% zoom and reduced motion: no added copy to wrap and no added motion, so both inherit the existing login behavior.

## Evidence

- Baseline board: `docs/visual-reviews/assets/forum-sso-login-resume/baseline-static-review-board.png`.
- Prototype board: `docs/visual-reviews/assets/forum-sso-login-resume/prototype-static-review-board.png`.
- Updated board: `docs/visual-reviews/assets/forum-sso-login-resume/updated-static-review-board.png`.
- Board source: `docs/visual-reviews/assets/forum-sso-login-resume/static-review-board.html`, captured headless at 1280x900. All three PNGs decode as 1280x900 8-bit RGB.
- Automated checks: frontend design governance, TypeScript, ESLint and the Vitest suite, including 13 new tests covering parameter parsing, the rejection of an attacker-supplied absolute URL's host and path, waiting for authentication before replaying, single-shot resumption and the error path that strips the parameter.
- Security reasoning behind the evidence: the parameter arrives in a URL and is therefore attacker-supplied, so only its query is read and its path discarded. The single navigation target is the `redirect_uri` the backend returns, and the backend only returns one that exact-matches its configured allowlist, which is what keeps this from becoming an open redirector.

## Residual Risk

- Known limitations: a static board cannot prove a real end-to-end forum sign-in. It documents a decision path, not rendered pixels of a live service.
- The platform's SSO endpoints answer 404 in production until a release is tagged and the `FORUM_SSO_*` variables are configured, so this path cannot be exercised against production yet.
- Follow-up owner: the release owner must complete a browser sign-in starting from the forum, for a guest and for an already-authenticated user, after deployment and configuration, and confirm that a deliberately rejected `redirect_uri` produces the error toast and dashboard fallback rather than a stuck page.
