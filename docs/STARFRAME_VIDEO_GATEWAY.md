# StarFrame Video Gateway (First Version)

Protocol reference: https://docs.xzapi.vip/ (verified 2026-10-05).

Status: proposal, isolated worktree only; no production configuration or deployment.

## Administrator Activation

This is an opt-in OpenAI API-key account adapter, not a model-name heuristic.
Use the existing administrator account API to configure these credentials:

```json
{
  "video_protocol": "starframe",
  "base_url": "https://api.xzapi.vip",
  "openai_capabilities": ["starframe"]
}
```

Supply the upstream API key through the existing secret input; never commit it.
`base_url` must be an explicitly trusted HTTPS origin or API base, with no query,
fragment or userinfo. Existing gateway URL security policy still applies.

Use `PUT /api/v1/admin/accounts/{id}` with the full non-sensitive `credentials`
object, preserving existing model mappings and other non-sensitive settings.
The existing update API preserves stored `api_key` when omitted; do not send an
empty key as a placeholder. New keys still use the existing secret input.
Do not use the restricted batch-credentials endpoint for these fields.

Place the account in an OpenAI group available to the Canvas API key. Configure
the account's existing exact model mapping for the user's selected custom IDs,
with identity mappings to forward IDs unchanged. Do not strip channel/speed suffixes.
Avoid overlapping Agnes and StarFrame account model routes in the same group.
No production catalog entries, model capabilities or price defaults are installed.

Configure existing group video prices for every enabled model/resolution (USD/s).
StarFrame looks up the exact complete model ID, case-sensitively, and the selected
resolution. If that row/tier is absent, only a positive explicitly configured old
group-flat price for the same resolution may supplement it. Model-family or case
folding is confined to legacy queries and cannot create StarFrame configuration.
Missing, non-positive or non-finite prices fail before any upstream submission.
Existing media rate multipliers and usage recording remain authoritative.
Status and content requests are never billed.

The exact requested model/duration/resolution and positive USD/s price are frozen
before claiming or submitting. Channel-mapped models, token billing modes and later
group price edits cannot replace that snapshot. Only explicit StarFrame video
requests skip the text-profit-rate gate; quota, permissions and account health remain.

## Client Contract

Canvas uses `https://api.jisudeng.com/v1/videos`, not the upstream base or key.
POST JSON requires `model`, `prompt`, `mode` (`references` or `frames`) and
`client_task_id`. The client ID accepts letters, digits, `_`, `-`, `.` up to 128
characters. `frames` requires first/last-frame URLs; modes are mutually exclusive.
`references` must be an object. For each of image/video/audio, singular and plural
forms are mutually exclusive; plural URL arrays require at least two entries.
Materials require public HTTP(S) URLs; data/blob URLs, userinfo, fragments, localhost
and private/reserved literal IP addresses are rejected before claiming. The gateway
does not resolve or download reference URLs: the upstream owns DNS/redirect safety
when fetching them. Duplicate JSON object fields are rejected to prevent billing
and upstream parsers interpreting different model/duration/resolution values.
Unknown model-specific fields are passed through, not interpreted as capabilities.

The upstream client ID is a deterministic `sfn_` SHA-256 namespace of
group/user/key IDs and the original `client_task_id`, to avoid collisions between
users sharing one upstream account. Create/status replies echo the original ID.
The UTF-8 hash seed is `starframe:{groupID}:{userID}:{keyID}:{client_task_id}`.

For exact existing billing, this first version also requires numeric integer
`duration` between 1 and 15 and `resolution` exactly `480p`, `720p` or `1080p`.
This billing limitation is not a claim that all models support these settings.
Model-specific duration, aspect ratio and reference limits remain upstream-owned.

Creation returns an opaque local `sfv_` ID. Query `/v1/videos/{id}` and download
`/v1/videos/{id}/content` using the same user/key/group. `metadata.url` is rewritten
to the local relative content endpoint; `metadata.fail_reason` is preserved.
Downloads use only the fixed content path on the original configured account.
Arbitrary response URLs and redirects are never followed with upstream credentials.
Agnes historical IDs and Grok routes retain their existing behavior.

## Retention And Recovery Boundary

Local owner/account/base bindings are retained for 24 hours in existing Redis JSON
storage, independent of short scheduler sticky bindings. Missing, expired or lost
bindings fail closed with 404; no random account lookup is attempted. Changing the
original account protocol or API base invalidates access rather than moving a task.
An internal SHA-256 fingerprint also pins the original upstream API key. Replacing
that key invalidates status/download before any HTTP request; handle pending tasks
before rotation. Neither the upstream key nor its fingerprint is returned to clients.

Submission claims are retained for 48 hours, scoped to user/key/group/client ID.
Duplicate submissions return 409 without another upstream request or usage record.
Ambiguous submissions (timeout, upstream failure or binding write failure) keep
their claim and do not fail over. A successful upstream submission whose local
binding cannot be saved returns an explicit error, not a usable-looking task ID.
Operators must reconcile unknown submissions with the upstream before recovery;
do not generate a fresh client ID or change accounts to retry an unknown task.

Redis retention is not durable financial evidence. Expiry, eviction or data loss
also loses the submission claim. After that boundary, this version cannot promise
cross-account submission idempotency. Existing usage idempotency uses the fixed
upstream account and task ID; status/download never create usage records.

## Server Validation Record

The isolated `feat/starframe-video-20261005` candidate passed specification and
quality review on 2026-10-05. The final source passed these actual server commands:

- `make test-backend-unit`: complete tagged backend unit suite, exit 0.
- `make test`: ordinary Go suite, golangci-lint (zero issues), design governance,
  frontend lint/typecheck and 448 Vitest files / 3,063 tests, exit 0.
- `make build`: backend production binary and frontend production artifacts, exit 0.
- `./scripts/check-fork-integrity.sh`: exit 0, `Fork integrity passed`.

Toolchain: Go 1.27.1, golangci-lint 2.13.0, pnpm 10.34.4 and Node 24;
`NODE_OPTIONS=--max-old-space-size=4096` supplied sufficient typecheck heap.
Earlier timing-sensitive test and lint failures remain in development logs; they
are not counted as passing commands. Dependency install lifecycle scripts were
not approved or executed. Quick and already-acquired admission paths have actual
unit coverage; the WaitPlan path was checked statically, not as a queued test.

These results do not establish PR CI, production deployment/health, database or
Redis acceptance, paid supplier availability, or user-local browser acceptance.
Release commit and deployment SHAs must be recorded separately when available.
