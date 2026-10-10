# Responses cache rejection compatibility and diagnostics

## Scope and evidence

- Baseline: `play/main` at `2fbe13d90a2381a3bc8e7495a0c8b00abe2cfe3a`.
- Isolated branch: `codex/responses-rejected-path-20261010`.
- Fixed source reference: ranxi `v2.10.3`, commit `fd1b5ee4eeb20961fbb783fa6f136a1704271e90`. Its rejected-field helper matches the baseline; no upstream version change is included.
- Source-confirmed gaps: content-level cache paths are unsupported; message parsing can mistake an unsupported nested path's suffix for a root field; a stream cache rejection without an error type can request general failover.
- Production evidence supplied by the coordinating task describes 12 failed requests: three actual HTTP 400 responses and nine stream failures. Seven stream failures contain partial text. Missing usage records do not establish zero consumption. The stream transport status/content type, request breakpoint paths, and actual historical send counts are unknown.
- Nested paths are a confirmed compatibility gap, **not a demonstrated cause of those historical requests**. No production requests, settings, historical charges, or deployments are changed.

## Behavior

- Actual HTTP 400 rejection handling adds exact `input[i].content[j].prompt_cache_breakpoint` support. Only the named field is removed; sibling hints, cache options/keys, tools, and message content remain intact. Invalid indexes/shapes and conflicting or incompletely parsed paths fail closed.
- The existing six-transform inbound HTTP budget and per-attempt body hash guard remain unchanged. A separate HTTP wrapper keeps the new content-path capability out of native WS ingress, whose existing behavior is not expanded.
- A rejection body containing response/work evidence, including an empty usage object, cannot authorize a rejected-field transform. This does not replace every existing gateway retry/settlement policy.
- Explicit cache model failures (`invalid_parameter` plus the model-rejection message) do not request SSE failover. No HTTP 200 SSE or generic 502 field-replay path is added.
- Existing observed token/image usage remains available to settlement. Partial text without usage does not invent a usage record. Error attempts remain auditable separately from billing. A successful terminal following a bare error does not acquire a synthetic failure record.

## Diagnostics and privacy

New optional `responses_cache_diagnostic` metadata in existing Ops attempt JSON records transport HTTP status separately from semantic status, allowlisted content/event types, at most 32 breakpoint paths, HTTP attempt count, used/maximum field-transform budget, downstream-write state, output observation, and usage presence.

Paths describe the **current attempt's actual request body**, not all historical transformations. `downstream_written` means the response writer has committed data/headers; it is not billing evidence. `usage_present=false` does not mean free. The metadata contains no prompt/output text, hint values, cache keys, images, or raw upstream parameter strings. Existing queue bounds and sanitization still apply; no DB migration or frontend change is required.

## Validation and review

RED tests reproduced missing content-path support, suffix misidentification, conflicting paths, response-evidence replay, stream failover, event-header-only diagnostics, usage-presence omission, WS scope expansion, and false failure logging after a successful terminal.

Validation status is recorded in the PR and final delivery report after the reviewed tree passes:

- Targeted service and handler regressions, including token/image usage preservation and no fabricated charges.
- `make test` and `make test-backend-unit`.
- `make build`.
- `./scripts/check-fork-integrity.sh`.
- Sequential independent specification and code-quality review.
- Draft PR CI at the final branch SHA.

The baseline's `AGENTS.md` references `docs/PROJECT_HYGIENE.md`, but that file is absent. The available AGENTS instructions and `docs/DELIVERY_WORKFLOW.md` are followed.

## Merge coordination and remaining limits

Likely overlap with the parallel compact/protocol/SSE work is confined to `openai_gateway_forward.go`, `openai_gateway_passthrough.go`, and `openai_gateway_response_handling.go`. Other touched shared files are the rejected-field helper, `openai_ws_http_bridge.go`, and `ops_upstream_context.go`; regression and diagnostic files are separate. The coordinating task owns ordinary merge validation, production deployment, and user browser acceptance.

This patch does not establish a complete native WS replay/usage audit or settlement of unusual HTTP error bodies carrying usage. Native WS's existing shared-budget and pre-output drain behavior needs separate investigation before any WS compatibility expansion. Configured generic HTTP failover remains distinct from the rejected-field transform budget. Historical stream transport/path/send details cannot be recovered from the supplied logs, and no historical usage is estimated or charged.
