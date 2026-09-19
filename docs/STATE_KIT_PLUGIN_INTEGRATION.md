# State Kit Plugin Integration

## Scope

The v0.2.7 merge supplies the official Sub2API plugin host contract required by
State Kit 0.3.x: host services, Redis-backed plugin KV, account discovery,
outbound identity resolution, and the single OpenAI OAuth outbound transport
slot. The host keeps account credentials, billing, usage, and OAuth token
lifecycle in the main application.

State Kit is loaded through the existing administrator plugin manager. It is
not compiled into the gateway and does not use the v0.2.6 source overlay. A
production installation must upload a signed `.s2plugin`, add the publisher
key to the existing trusted-publisher configuration, and enable only one
OpenAI OAuth transport plugin.

## Safety Boundary

- This repository does not contain a plugin package, publisher private key,
  OAuth credential, proxy credential, or STATE value.
- Plugin KV stores opaque ticket state in the host Redis namespace; it does
  not expose access tokens to the plugin UI.
- The plugin cannot change account billing, balance settlement, payment orders,
  group surcharge, Play/VIP qualification, or production database migrations.
- The plugin is single-instance unless the host deployment provides an
  external coordination mechanism; multi-replica locking is not assumed.

## Verification

Run the host checks in this worktree before a PR:

```text
cd backend && go test -tags=unit ./internal/service/... ./internal/server/...
./scripts/check-fork-integrity.sh
```

The separately released plugin must be verified with its own `go test -race
./...` and `node --test ui-tests/*.test.cjs`. Its configuration page must
provide symmetric `zh-CN` and `en-US` resources before production enablement;
the upstream package currently requires that follow-up and is therefore not a
production acceptance artifact from this worktree.
