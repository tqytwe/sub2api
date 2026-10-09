# Fingerprint-bound usage-log settlement recovery

## Scope

This change repairs the displayed charged amounts when the same ordinary billing
command is retried after a billing or log-write failure. It does not retry billing
autonomously, backcharge accounts, adopt old rows, change request identity,
recalculate prices, or replace the existing financial transaction and ledger.
Image Studio's separate reconciliation flow and simple-mode paths stay unchanged.
Simple-mode key-rate-limit-only failures, including request conflicts, retain the
prior billed-cost/surcharge audit snapshot and still write their failure log;
they carry no recovery fingerprint or settlement marker.

## Proof and durable identity

Capture the existing normalized command fingerprint before `Apply` and before
zeroing a failed attempt's displayed charge. Do not change fingerprint derivation
or monetary rounding. A financial result is verified only after its transaction
commits, or after exact matching against an already committed live/archive dedup
record. An `Applied` boolean alone, a nil result, or an amount greater than zero is
not proof. Verified zero-cost commands are valid settlements.

Two internal SQL columns bind each newly written usage row to its command:
`billing_request_fingerprint` (nullable) and `billing_settled` (default false).
Metadata carried on the Go usage-log object must be excluded from JSON. Historical
rows with a null fingerprint remain immutable; the migration does not backfill.

## Atomic promotion rule

Preserve the existing prepared-row and batched INSERT paths. A conflict may promote
an existing row only if all of these conditions hold:

1. Existing and incoming nonempty fingerprints match exactly.
2. Existing state is unsettled and incoming state has verified settlement proof.
3. One SQL snapshot of the union of live and archive dedup records contains a
   matching fingerprint and contains no conflicting fingerprint for that key.

Update only `actual_cost`, `billed_cost`, `billing_surcharge_cost`,
`billing_surcharge_mode`, `billing_surcharge_value`, and `billing_settled`.
Preserve the original tokens, owner, account, subscription, account-cost snapshot,
model, timestamps, and routing fields. A stale failure cannot downgrade settlement.
An unrelated fingerprint cannot overwrite the row. A rejected financial fingerprint
must not create a new failed row that blocks the legitimate request's missing log.
Financial Apply and log persistence remain separate transactions; a subsequent
same-command retry is what closes a failure between them.

The atomic upsert must preserve `Create`'s true-insert return contract: a repair is
not a new insertion. PostgreSQL integration coverage must verify this distinction,
including concurrent insertion/promotion, before delivery.
Metadata-bearing upserts use PostgreSQL's `xmax = 0` tuple metadata to distinguish
insert from update. This is PostgreSQL-specific, not a portable action API, and
requires supported-version integration tests. Generic unmarked writes retain their
existing SQL/return semantics; a metadata-bearing result never authorizes a debit.

## Queue, cache, and batch boundaries

Keep the 256-row/20-ms best-effort batching and the 64-row/3-ms Create batching,
queue capacities, and overflow behavior. Do not replace normal logs with per-row
transactions. Metadata-bearing records bypass the recent-key suppression cache so
a verified retry cannot be mistaken for an already-persisted failed row.

Within either batch, keep the first complete prepared row. A later row may promote
only the charge/state fields when its fingerprint is the same and settlement is
verified. Never replace the original noncharge snapshot. Different fingerprints,
legacy/unmarked rows, and later failed attempts cannot promote it. Single-write
fallbacks preserve the same metadata and conditional-upsert rules.

## Acceptance tests and limits

- Billing failure → success → duplicate: one financial effect and repaired charges.
- Debit success → log failure → retry: one financial effect and a complete log.
- Conflicting fingerprints, including identical client IDs and changed payloads.
- Nonzero surcharge and account-cost fields; only charge fields are promoted.
- Legacy/unverified/no-op paths cannot authorize promotion.
- Verified zero settlement, stale failure after success, and duplicate success.
- Both batch coalescers, recent-cache bypass, and single-write fallbacks.
- Financial rollback/commit proof, archive movement, and concurrent promotion.
- Existing actual-cost UPDATE rollup invalidation remains active.

Older generic hourly/daily aggregates may fall outside their configured refresh
windows. This change does not claim full historical dashboard reconciliation.
No production database access or mutation is part of these tests.

## Forwarding slice acceptance

The previously recorded passthrough result-plus-error gap is addressed by the
separate OpenAI forwarding slice. Confirmed aggregate input/output usage or
completed image work is retained with the original error and settled once;
compact/model/account retries stop after that observed expense. Detail-only
subsets or output deltas alone do not authorize estimated charges. Unmetered
compact retry remains covered with the real wire-level trigger.

The bounded native Responses, passthrough, Chat/Messages text compatibility and
WS bridge cases passed the frozen combined targeted run on2026-10-09. This
includes failed-terminal image retention, progressive aggregate preservation,
buffered complete-event read failures, staged pre-output WS errors, preemption,
mandatory delivery and compatible-provider scope controls.

This does not certify every billable endpoint, Grok error-settlement behavior,
or image generation through the existing Chat/Messages conversion bridges.
Those explicit source-parity boundaries remain in CORE_MIGRATION_RANXI.md.
Full aggregate gates and eventual combined-branch/production acceptance are
separate requirements. No production reconciliation or historical charge is
performed or authorized by these tests.
