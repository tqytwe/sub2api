# v2.17.6 Channel Operations Database Slice

Status: migration candidate only. This slice does not implement or enable the channel operations API/UI, and it has not been deployed or checked against production PostgreSQL.

## Fixed comparison

- Fork base: `526f09d36fa4d0a69af823fec2fc5a9ed495193f` (`origin/play/main`).
- Upstream release: Ranxi `v2.17.6`, commit `a7ba38d873ff97faf3a127a97ab808d8860bb8b8`.
- Upstream `309_channel_operations.sql` SHA-256: `5f38c64c9a6d5d051921cb7942c9939f1a9fd0fb0d7271691e599bbb401708b6`.
- Upstream `309_credential_operation_refresh_failure.sql` SHA-256: `e16d3411143028b09d1f3fd5863e1cb3cbadbd518f15d7b334418afd1c519b72`.
- Fork migration files end at `281_gateway_identifier_diagnostics.sql`; the source's two `309_*.sql` filenames are not present in this tree.
- Candidate checksum (SHA-256 of trimmed content): `282_channel_operations.sql` = `aa8ca6209d84eece240a0b5dd8d6d923f500b818efe1befe98c9390598a9a45d`; `283_account_oauth_refresh_failures.sql` = `8d5e6cc30f6c32e8238feeb4816f047e91ffb0071d0776016e3ab515a55eb559`.

## Release feature database map

| Release feature | Database effect in v2.17.6 | Fork decision |
| --- | --- | --- |
| #572 priority scheduling | No new SQL migration in this release slice. It reads existing usage/quality evidence and stores settings through the existing settings repository. | No database work in this slice. Backend/API parity still needs a separate port and must preserve authoritative account eligibility, billing snapshots and unknown evidence. |
| #573 channel operations | Adds `309_channel_operations.sql` only: channel revision plus current upstream/account classification tables. | Adapted candidate is `282_channel_operations.sql`, described below. API/UI are not included here. |
| #574 credential failure diagnostics | Adds `309_credential_operation_refresh_failure.sql`, which alters `account_token_guard_v2_accounts`. | Adapted candidate is `283_account_oauth_refresh_failures.sql`, an independent Fork-native latest-observation table. It does not restore token guard/re-auth. Backend writer/admin reader must land in the same reviewed feature slice before the data is enabled. |
| #575 optional Jev quality judgment | No new SQL migration in this release slice. It extends the existing scheduled quality-rule configuration/result JSON contract. | No database work in this slice. Backend/API and permissions still require a separate port; keep the feature opt-in and unknown-safe. |

The feature-level map above is intentionally narrower than the release's complete historical migration tree. Upstream's pre-existing `282_regional_pricing.sql` is unrelated to #573 and is not imported by this slice. The adapted `282_channel_operations.sql` has a distinct full filename; the Fork runner orders and checksums full filenames. Confirm the exact production `schema_migrations` rows and object definitions before deployment because production was not queried here.

## Candidate migration

The adapted migration is `backend/migrations/282_channel_operations.sql`. It adds only the channel revision column and two operational-classification tables, their keys/index and a table comment. It adds no row backfill, does not alter `channel_groups` or `account_groups`, and does not write usage or billing records. Foreign keys cascade classification rows when a channel, account, or upstream classification is physically deleted. Soft-deleted accounts are not automatically deleted by this migration.

The migration keeps the source's full constraints and relationships but uses a 2-second lock timeout and 30-second statement timeout, matching the Fork's recent forward-only migration budget. `ALTER TABLE channels ADD COLUMN ... DEFAULT 0` is metadata-only on supported PostgreSQL versions, but still needs a brief table lock. The short lock timeout makes contention fail the migration attempt rather than wait unboundedly. The runner executes this ordinary migration and its `schema_migrations` filename/checksum insert in one transaction; failure rolls back both. It sorts and records the full filename and SHA-256, not the numeric prefix alone.

The new `282_` name avoids overwriting or aliasing either upstream `309_` file and sorts after the Fork's current 281 series. If the source `309_channel_operations.sql` was applied by a different binary already, the adapted DDL's `IF NOT EXISTS` clauses are replay-safe for the expected schema, and this runner records the new Fork filename separately. This does not validate arbitrary pre-existing objects with the same names but incompatible definitions; production preflight must inspect those objects and their constraints before deployment.

## Credential migration adaptation and dependency boundary

Do not include upstream `309_credential_operation_refresh_failure.sql`. It alters `account_token_guard_v2_accounts`, whose prerequisite is introduced by upstream `255_account_token_guard_v2.sql`; that table and its repository, scheduler, worker, handler and admin routes are absent from the Fork base. Applying the credential 309 migration alone fails at the `ALTER TABLE`.

The minimum schema chain for the source's complete credential-operations contract is larger than the one altered table:

- `254_openai_oauth_reauth.sql` creates the durable re-login configuration and task tables.
- `255_account_token_guard_v2.sql` extends those tables and creates `account_token_guard_v2_accounts`.
- `256_openai_oauth_reauth_proxy_override.sql`, `257_openai_oauth_reauth_proxy_source.sql` and `262_openai_oauth_reauth_engine.sql` add fields consumed by the current re-login configuration and worker paths.
- `300_account_relogin_budget.sql` adds persistent budget fields consumed by the current token-guard implementation.
- `309_credential_operation_refresh_failure.sql` adds the latest refresh failure code/time consumed by its repository and admin projection.

The Fork-native replacement is `backend/migrations/283_account_oauth_refresh_failures.sql`. It stores one latest observation per account: account ID, a constrained canonical failure code and timestamp. Its primary key is the account lookup index; the FK cascades on physical account deletion. The table does not drive account status, scheduling, retries, refresh policy, billing or re-login. It deliberately does not backfill old failures from current account state and cannot retain multiple events. The matching writer/admin reader is a companion backend task; until that lands and passes review, this migration alone is storage groundwork, not an enabled feature.

The replacement's code constraint mirrors the source canonical safe-code enum (`phone_verification_required`, `account_deactivated`, `account_deleted`, `account_retired`, `requires_relogin`, `password_rejected`, `mfa_required`, `security_check`, `authorization_denied`, `rate_limited`, `timeout`, `transport_error`, `service_error`, `unknown`). The backend writer must use these exact values, with unknown provider codes collapsed to `unknown`; raw response bodies and free-text exception messages are not valid stored codes.

The old `250_account_token_guard.sql` V1 event/state tables are a separate compatibility surface, not a prerequisite for the single ALTER in upstream 309; importing them would be necessary only if the broader V1 product endpoints were selected. The V2 chain above is itself a feature restoration, not a harmless schema prerequisite. In particular, migrations 254/255 introduce saved re-login configuration including optional encrypted password/TOTP fields and durable re-login tasks. Importing those migrations without the matching encryption, authorization, worker ownership, refresh policy, admin routes and user-data controls would create a partial credential-management system. The Fork-native latest-observation table avoids that product dependency while keeping the diagnostics contract bounded.

No passwords, TOTP secrets, OAuth credentials, or reauthorization behavior are introduced here. No billing/settlement migration is part of upstream v2.17.6; Fork migrations 275-281 and admin-only request-ledger access remain independent and untouched.

## Verification and deployment boundary

The migration unit tests passed with `go test -tags unit ./migrations -count=1`; they check the SQL contract, filename uniqueness, timeout preamble, candidate checksums and absence of routing/scheduler/ledger writes. The PostgreSQL integration test was not completed because the `postgres:16-alpine` image was not cached and the testcontainers dependency download did not finish. It is intended to apply and replay both migrations against a disposable PostgreSQL instance, check foreign-key behavior, and verify existing scheduler membership remains unchanged.

No production database was accessed. Current production `schema_migrations`, whether either source 309 file was applied, live lock contention, and exact production constraint state remain external preflight requirements. This candidate does not claim production compatibility until those facts are verified read-only and the deployment is separately approved.
