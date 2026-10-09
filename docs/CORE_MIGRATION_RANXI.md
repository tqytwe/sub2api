# Ranxi core migration: scope and acceptance

Status: implementation in progress. This document is not a release approval.

## Fixed comparison points

- Initial deployed comparison point: `tqytwe/sub2api`, `play/main`, `7508cb0cd8a9c38a33805795922ec13662d7753e`.
- Integrated deployed PR338 base: `96d202ce266a3858e34df10987ae906942be5f37`; its TPS, tooltip, source-pin and fixture changes remain intact.
- Latest integrated production-branch base: `a7e1c00330170754683216e3018cbd4279d7f733` (PR339); its update-policy API/UI and release safeguards remain intact. Deployment health is recorded separately.
- Source: `ranxi2001/sub2api`, formal release `v2.10.3`, `fd1b5ee4eeb20961fbb783fa6f136a1704271e90`.
- Work branch: `codex/core-migration-continue-20261009`.
- Preserved draft: based on `7b58f1e676eb2cc67bdcf6ab7217da5ddb739234`; selectively reapplied over deployed PR337. Security toolchain/dependencies, private billing identity, full CI and hermetic regression fixes remain from PR337.

The goal is to adopt source core behavior for account management, routing,
usage recording, settlement, and statistics without replacing destination
business rules. This is a staged semantic integration, not a whole-tree merge.
Source CI success does not establish correctness of this integration.

## Destination contracts that must survive

- Atomic balance/frozen-balance/ledger mutations and ownership checks.
- Separate actual cost, surcharge, billed cost, and account-stat cost semantics.
- Request-time pricing and authenticated parent-group snapshots.
- Subscription/package quotas and existing withdrawal/entitlement boundaries.
- Qualification-based welfare, check-in/makeup/milestone idempotency, VIP and
  recharge rules; do not restore removed cohort-approval gates.
- Per-part Gemini image-alias counting and response/usage/charge consistency.
- StarFrame explicit protocol selection, frozen quote, permanent submission
  claim, credential/account binding, and zero-charge status/content operations.
- Existing branding, localization, public/mobile correlation contracts, and
  production branch/deployment configuration.
- Immutable historical SQL migration filenames and checksums. In particular,
  preserve destination `226_channel_monitor_quota_mode.sql` unchanged.

## Core integration areas

| Area | Required evidence before completion |
| --- | --- |
| Account statistics | Today and retained-history aggregates, distinct cost meanings, batch/fallback behavior, timezone and failure handling |
| Membership model policy | Optional schema/API support, validation, cloning/cache preservation, atomic binding plus policy changes, deletion/rebind concurrency, enforced routing intersection |
| Authoritative admission | Consistent primary-database snapshot; constructor enforcement; cancellation; stale account/group/route rejection without upstream health penalties |
| HTTP forwarding | Native Responses, API-key passthrough, Chat and Messages compatibility, actual send/retry boundaries, immutable public/outbound model distinction |
| WebSocket forwarding | Every logical turn and actual write/retry path; immutable billing context; response-ID and missing-response-ID behavior |
| Usage settlement | Observed partial usage, stopped-worker delivery, successful/failed/duplicate settlement, no inferred charges from unmetered failures |
| Usage-log recovery | Exact verified fingerprint, atomic metadata, safe pending-to-settled promotion, retained batching, conflict/legacy protection and real concurrent database tests |
| Request identity | Independent gateway billing identity; preserved client correlation; same-expense idempotency; forced media and durable async identities; detached context propagation |
| Other billable endpoints | Explicit coverage decision for embeddings, images, audio and realtime; do not infer coverage from text Responses tests |
| Presentation/API compatibility | Existing account/usage/statistics screens and DTOs remain compatible; new core fields must not silently alter prices or permissions |

Each implementation slice needs behavioral RED/GREEN evidence, specification
review, and a separate code-quality review. A passed slice does not establish
whole-tree or whole-platform completion. Known source limitations must be
documented rather than silently described as fixed.

## Billing incident evidence boundary

An operator reported approximately 400 upstream requests while a local OpenAI
Key account row showed 8. The displayed cost figures derive from usage logs;
they are not independent verification of upstream invoices or balance-ledger
transactions. Production data has not been reconciled in this migration.

The destination middleware accepts incoming `X-Client-Request-ID`; the pinned
source ordinarily generates a fresh internal UUID. That difference was
introduced by destination commit
`6be07f7e40a39cfdad54c96af0a700014c237a20`. Reused correlation IDs therefore
need explicit regression coverage. This source evidence does not establish the
cause of the reported production discrepancy, nor authorize historical charges.

## Final gates and release boundary

- Run targeted tests, complete repository tests/builds, lint, generated-code
  consistency checks and `scripts/check-fork-integrity.sh` on the final tree.
- Run real isolated database migration, rollback and concurrency tests. Record
  PostgreSQL/Redis versions and distinguish them from exact CI image versions.
- Test an existing destination schema as well as a fresh database; do not
  modify a production database to obtain test evidence.
- Preserve exact source/destination/implementation commit identities and review
  evidence. Do not force-push or rebase `play/main`.
- Production merge/deployment requires separate authorization for this core
  migration. Local tests and cloud screenshots do not replace repository-required
  user-local production acceptance.
- No backcharges, refunds, balance edits, credential changes, or paid replay
  requests are part of this work.

## First-batch boundary and remaining routing work

The first forwarding batch is OpenAI-only. Shared parsers and error-settlement
paths must opt in by account platform so Grok-compatible traffic does not acquire
new charging or replay behavior incidentally. Membership selection/discovery
also follows the source's non-OpenAI selected-snapshot policy; that does not make
non-OpenAI final sends authoritative.

| Remaining area | Source parity versus additional work |
| --- | --- |
| Generic Anthropic, Bedrock, Gemini and Antigravity final sends | Pinned source has no final authoritative membership reread in these provider-specific forwards. This batch retains that boundary; adding latest-state admission is additional hardening, not something source parity already delivers. |
| Grok and other OpenAI-compatible providers | The source applies membership to the selected snapshot, but skips the OpenAI primary-state reread. Full compatible-provider partial-error settlement and latest-state behavior remain separately scoped. |
| Other source policy consumers | Applicability still needs audit for `batch_image_public.go:647/956`, `profit_preview.go:133`, `quality_judge.go:87`, and `openai_excel_bps_models_manifest.go:79` at the pinned source commit. |
| Image-generation tools through Chat/Messages conversion bridges | Both source and deployed conversion readers lack completed-image collection. This bounded text-compatibility slice preserves that existing limitation; adding bridge image support requires its own acceptance. Native/passthrough Responses completed-image retention is separately included. |
| Images, embeddings, audio and realtime endpoints | Explicit endpoint-by-endpoint account/routing/settlement acceptance remains required. Text Responses/Chat/Messages and retained WebSocket tests do not implicitly certify these endpoints. |
| Production acceptance | This local continuation does not provide a deployment, production ledger reconciliation, or user-local guest/user/admin browser acceptance. |

Chat silent-refusal and raw pre-byte truncation were checked against the pinned
source. Their guards exclude already observed usage: any usage object releases
pending raw output, while silent refusal requires no usage. Those branches are
retained rather than changed speculatively. Failed-terminal detail-only frames
are a separate case: they must not erase previously observed aggregate totals.

The detailed membership comparison below records source-parity boundaries and
explicit destination hardening. Test status remains subordinate to the final
validation record; neither this matrix nor source review alone is release
approval.

### Membership policy source-parity matrix

Source: ranxi v2.10.3 commit fd1b5ee4eeb20961fbb783fa6f136a1704271e90.
Destination baseline: 7508cb0cd; isolated branch codex/core-migration-continue-20261009.
Scope: per-account/per-group AllowedModels selection, affinity, diagnosis, model-list discovery, and the source final-send membership predicate. No claim that the full routing migration is complete.

| Path | Source evidence | Destination behavior in this batch | Explicit remaining boundary |
|---|---|---|---|
| Generic Anthropic/Bedrock/Antigravity routing, load/routed/sticky/fallback | gateway_scheduling.go:303,360,545,671; legacy 1883,1940,2005,2054; mixed 2147,2206,2271,2321; shared predicate2606–2607 | All corresponding candidate and affinity gates intersect provider support with membership using the routed request model. Composite explicit routes use the routed target; account-level aliases still use the alias at selection. | Selection snapshots are not a new primary-DB final-send transaction. |
| Generic model-not-found diagnosis | gateway_model_availability.go:105 | Uses same provider-support plus membership predicate. Local existing panic-safe conservative fallback retained. | Diagnostic lookup failure remains conservative503. |
| Gemini normal and sticky selection | gemini_messages_compat_service.go:129,149,238,253–285,342–363 | Group ID carried through both paths; membership narrows provider support before selection. | Native/compat Gemini forwards do not reread latest account membership before every send. |
| Generic mapped model discovery | gateway_service.go:1463–1480; group_model_allowlist.go:35–61 | Mapping keys and concrete restricted models are filtered per member; OpenAI default supplementation is membership-aware. | Source's nil/default ambiguity requires the additional final handler catalog intersection below. |
| Handler generic/default/public/composite and native Gemini discovery | Source handler fallback remains separate from service membership filtering; source gemini_v1beta_handler.go has only public allowlist filtering | Destination intersects fully expanded handler catalogs, including direct static Antigravity, with current membership support, preserving unknown Gemini upstream metadata/envelope/headers and the public allowlist namespace. Explicit Composite routing is resolved before membership/capability filtering; public output IDs remain unchanged. This closes restricted-empty and default-union leaks demonstrated by handler RED. | This is catalog filtering, not request authorization or atomic account snapshots. |
| Non-OpenAI providers forwarded through OpenAIGatewayService | openai_turn_admission.go:254–257 returns the supplied non-OpenAI account without authoritative reread; then359–365 evaluates IsModelAllowedInGroup(outboundModel), before367–368 returns non-OpenAI. Call sites include openai_gateway_chat_completions.go:74, raw:77, forward:111/1234, passthrough:380 and responses_chat_fallback:149. | Narrow parity repair applied after verified RED: membership is checked on the selected snapshot before the destination's early non-OpenAI return. Keyless calls remain unscoped. No non-OpenAI DB reread added. | This does not promise latest-state admission for Grok or other compatible providers; source also has none. Each forwarding worker owns its final-send call sites. |
| OpenAI simple mode | Source's group-binding simple-mode exemption is local at openai_turn_admission.go:308–317; membership remains checked in caller363–366. | Membership is enforced for keyed simple-mode requests independently of the account/group-binding exemption (verified RED and frozen combined GREEN). | Do not conflate model restriction with the simple-mode scheduling scope. |
| Generic/Gemini/Antigravity final sends | Source gateway_forward.go:91; gateway_forward_as_chat_completions.go:29; gateway_forward_as_responses.go:31; gemini_messages_compat_service.go:629/1171; antigravity_gateway_claude.go:30; antigravity_gateway_gemini.go:44. Source membership grep finds no final AllowedModels guard in these methods. | No new final primary-DB admission added in these provider-specific implementations. | Full non-OpenAI latest-state hardening is not delivered by source parity or by this batch. |

#### Naming contract

Public Group.ModelAllowlist is evaluated against the immutable client/public model. AccountGroup.AllowedModels checks routed account aliases at selection and the actual mapped outbound model at final-send admission. A restricted account-level alias therefore needs both alias and target for end-to-end forwarding. An explicit Composite route is a distinct earlier rewrite: membership follows its routed target while catalog output and the public allowlist retain the original public name.

#### Wider source migration remains open

Other source consumers include batch_image_public.go:647/956, profit_preview.go:133, quality_judge.go:87, and openai_excel_bps_models_manifest.go:79. They are outside this worker's generic/OpenAI gateway selection/discovery batch and require their own applicability audit. Global migration, full tests/build, PR, production deployment, and local-browser acceptance are not established by this matrix.


## Phased-release safeguards to carry into coordination

- Migrations273/274 are additive and tested with an existing-schema fixture and
  a legacy writer. That compatibility does not certify production rollout.
- Both new migration files set transaction-local `lock_timeout = '2s'` and
  `statement_timeout = '30s'` before their DDL. A contended lock fails that
  startup attempt instead of waiting indefinitely; the unchanged runner rolls
  back that file's DDL and history record together. Previously committed
  migration files remain committed. The limits reset when the transaction ends.
  They bound each lock wait and statement, not the entire migration or all
  request latency: even a waiting exclusive lock can briefly queue traffic.
  Release coordination must check contention and retry a failed rollout after
  the blocker clears, without changing historical SQL or global DB settings.
- Once operators configure membership model restrictions, rolling back to an
  older binary without enforcement can reopen models. Any rollback must retain
  enforcement or explicitly resolve that policy change; do not silently clear
  restrictions as a technical workaround.
- Keep billing identity and frozen-price behavior together across any staged
  application rollout. Recovery metadata never authorizes historical charging.
- Rerun the required gates on the eventual combined branch after integrating
  independently reviewed TPS/upstream-maintenance work; this base-specific local
  test record cannot certify that later merged tree.

## Local validation checkpoint

The frozen targeted run completed successfully on 2026-10-09 at 20:04:55 UTC:
557 top-level tests (1281 including subtests), six backend packages. All
bounded specification and code-quality reviews passed. This is a scoped local
checkpoint, not overall migration completion or release approval. Full tests,
builds, lint, first-time Wire generation, broader integration coverage and the
private billing-identity acceptance suite are recorded separately as they run.

The earlier Ent regeneration check produced 398 byte-identical files. The real
isolated PostgreSQL 17.11/Redis 8.0.2 core suite passed 22 top-level tests/suites (13 live-database and nine SQL-mock), including
concurrent reconciliation and a pre-273 existing-schema upgrade/legacy writer.
These versions and scoped passes are distinct from hosted PostgreSQL 18.1/
Redis 8.4 checks. Current Ent output is byte-identical to that successful generated copy.
The earlier PG run had no source hash manifest; the final frozen-tree integration
gate establishes current-input evidence. Security manifests, release workflows and frontend source remain at
the deployed base.

The dedicated `Core Migration PostgreSQL and Redis` PR workflow reuses the
repository's isolated PostgreSQL 18.1 / Redis 8.4 harness for the core upgrade,
lock-failure rollback/retry, account/group policy, reconciliation, billing and
cache tests. It checks that required named tests actually passed, so an empty
selection cannot count as coverage. It does not access production or change
existing release workflows. A local run on different database versions is not
hosted compatibility evidence; record the actual PR run before release.

A published fixture-only correction from commit
68039d761232e3e1fd62e35d79b1650ee6411d33 initializes the inflight-estimate test's
shared rate cache/resolver like production. The 20,000-model loop and 8 MiB limit
remain intact; no TPS or UI change was imported with that isolated fixture.

The first aggregate attempt was interrupted before a terminal result. It exposed
two test fixtures with missing authoritative account-state readers; those fixtures
were corrected without weakening admission or their existing behavior assertions.
The old targeted checkpoint does not certify the later PR338/PR339-integrated tree.
