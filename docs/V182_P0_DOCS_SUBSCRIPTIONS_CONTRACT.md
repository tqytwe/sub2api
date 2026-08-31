# v0.1.182 P0 Docs And Subscription Contract

Status: implementation candidate. This document records source behavior and
release acceptance requirements. It does not claim that a production setting,
database, or deployment has changed.

## Route Contract

| Input | Required behavior |
| --- | --- |
| A current custom menu URL whose parsed origin equals the fixed canonical origin `https://www.jisudeng.com` and whose exact path is `/docs` or `/en/docs` | Route to `/docs` for Chinese and `/en/docs` for English. Discard query and fragment values before navigation. |
| `/custom/095790f89fc04920` | Redirect to the native docs route for the active locale without loading public menu configuration. |
| Any other `/custom/:id` or any external/mismatched docs URL | Preserve the existing custom-page iframe behavior. First-party `jisudeng.com` docs aliases receive no panel-derived query context, including `token`, `user_id`, theme, locale, or source URL. |

The resolver never uses menu label text or a substring test. In particular,
`docs.example.com`, a different scheme/host, a path below `/docs`, userinfo, and
an arbitrary custom page must not become a first-party route.

## Security Header Contract

- `Content-Security-Policy` always resolves `frame-ancestors` to `'self'`.
- `X-Frame-Options: SAMEORIGIN` remains present.
- `frontend_url` is only the canonical base URL for email and external redirect
  generation. It does not widen `frame-ancestors`.
- Explicit external iframe sources continue to be evaluated by the existing
  `frame-src` allowlist. This change does not permit wildcard ancestor origins,
  `ALLOWALL`, or a removed X-Frame-Options header.
- Notification email links prefer the configured `frontend_url` over
  `api_base_url`, so an API gateway hostname cannot silently replace the public
  site origin.

## Subscription Boundary Contract

`GET /api/v1/subscriptions/progress`,
`GET /api/v1/subscriptions/:id/progress`, and
`GET /api/v1/admin/subscriptions/:id/progress` retain their backend-owned wire
format. The frontend API boundary maps both current and retired field names to
one internal model:

```text
SubscriptionProgress = {
  id, groupName, expiresAt, expiresInDays,
  daily|weekly|monthly: {
    limitUsd, usedUsd, remainingUsd, percentage,
    windowStart, resetsAt, resetsInSeconds
  } | null
}
```

Accepted current names include `id`, `group_name`, `expires_in_days`,
`limit_usd`, `used_usd`, `remaining_usd`, and `resets_at`. Accepted retired
names include `subscription_id`, `days_remaining`, `limit`, `used`, and
`reset_in_seconds`. Components must consume only the normalized model.

`GET /api/v1/subscriptions` remains the source of the complete subscription
management list, including active, expired, revoked, and suspended records.
`GET /api/v1/subscriptions/progress` remains active-only. The management view
merges the active progress entries by subscription ID into the complete list;
records absent from the active-only response receive an empty normalized
progress model with their stored expiration, never retired raw usage fields.
The header mini component reads active normalized progress from the shared
subscription store. Login preload, the existing five-minute poller, and a
forced store refresh all update that same reactive state. Subscription purchase
completion and subscription redemption use that forced combined refresh, so the
header does not wait for a scheduled poll to reflect a new entitlement.

`GET /api/v1/subscriptions/:id/progress` is an owner-scoped user route. The
handler reads the JWT subject, then the service verifies that the loaded
subscription's `user_id` equals that subject before it calculates or returns
progress. A missing subscription and a subscription owned by someone else both
return the same `404 SUBSCRIPTION_NOT_FOUND` response; the route must never
re-use the administrator handler or disclose cross-user progress. The separate
administrator route remains protected by the administrator middleware.

## Production `frontend_url` Change Record

Owner: a human administrator with the required production-account authority.
Do not use direct SQL, a deploy-time environment override, an administrator API
key, or a copied browser session to apply this change.

The administrator Settings form accepts an empty value or a trimmed absolute
HTTP(S) URL without a query, fragment, or userinfo. It must preserve an invalid
typed value in the field, associate a localized error with that field, and make
no update request until the operator corrects it. The backend repeats the same
validation and remains authoritative for requests that bypass the UI. This
validation protects the audited mail/redirect origin only; it does not alter
the `frame-ancestors 'self'` CSP contract.

The input is intentionally visible even when `email_verify_enabled` or
`password_reset_enabled` is disabled. `frontend_url` also determines
notification and external redirect origins, and a historic noncanonical value
must remain repairable through the audited Settings workflow instead of
requiring a direct database change.

1. In the production administrator Settings UI, read the current `frontend_url`
   value and record the operator, timestamp, and intended replacement in the
   change record without placing credentials or reset links in this repository.
2. Set the value to exactly `https://www.jisudeng.com` through the existing
   `PUT /api/v1/admin/settings` save flow. Leave unrelated settings unchanged.
3. Confirm the administrator-side `settings updated` audit event identifies
   `frontend_url` as a changed field and the expected administrator as actor.
4. Request a password reset for a controlled test account and verify the email
   link begins with `https://www.jisudeng.com/`. Verify notification and
   NextChat-origin links use the same HTTPS `www` canonical origin. Notification
   links use `frontend_url` ahead of `api_base_url`, so an API gateway hostname
   cannot silently replace the audited public origin.
5. Check a normal HTML response has `frame-ancestors 'self'` and
   `X-Frame-Options: SAMEORIGIN`; do not treat a corrected `frontend_url` as a
   reason to add it to the CSP ancestor list.

Rollback is the same audited Settings UI flow to the last verified canonical
value. A rollback does not change the CSP contract.

## Required Release Evidence

- Focused frontend tests for docs target parsing, historic-bookmark routing,
  locale latest-wins behavior, subscription normalization, platform labels, and
  sidebar wiring.
- Backend security-header, owner-scoped subscription, notification-link, and
  frontend URL validation tests. The setting-service validation test is in the
  repository's `unit` test group, so it must be run with `go test -tags=unit`;
  a plain `go test` command is not evidence for that behavior.
- `pnpm design:check`, `pnpm lint:check`, `pnpm typecheck`, production build,
  and `./scripts/check-fork-integrity.sh` once all parallel changes are
  integrated.
- This P0 contains no database migration and makes no database schema or data
  change. The production `frontend_url` update remains an authorized manual
  administrator operation after deployment.
- Merge commit: `ceb8d5aed015b7a5db3b2fdecd0d0381faf9bf37` on
  `origin/play/main` (PR #296), with all required GitHub checks passing.
- Production probes after the merge returned `/health` 200 `{"status":"ok"}`;
  `/docs` and `/en/docs` returned 200 with localized document titles and the
  new `index-BkSlP1tg.js` asset; unauthenticated subscription and progress
  routes returned 401 as required.
- Production HTML responses now send `frame-ancestors 'self'` and
  `X-Frame-Options: SAMEORIGIN`. A Playwright production capture confirmed the
  native Chinese docs directory renders populated content without an iframe
  refusal page.
- Zeabur deployment ID/SHA could not be read because the local CLI session
  returned `ERROR_INVALID_TOKEN`; public asset/header/health evidence confirms
  the new build is serving, but the deployment identifier must be recorded after
  Zeabur re-authentication.
- The administrator must still set `frontend_url` to exactly
  `https://www.jisudeng.com` through Settings and complete local-browser guest,
  ordinary-user, and administrator acceptance. Until those steps are recorded,
  the release is not production-complete.
