# Lossy encrypted history recovery on HTTP 400

The administrator setting is under **System settings → Gateway forwarding** and
is disabled by default. It is independent from the existing HTTP 409 recovery
setting. The API field is
`openai_400_omit_encrypted_history_enabled`; omitted fields in older admin
clients keep the saved value.

When native OpenAI Responses returns an explicit encrypted-content HTTP 400,
the gateway first uses the existing precise recovery. If that path cannot
resolve the rejection, this opt-in fallback may schedule one additional retry
on the same account, credential, model, and route. It never applies to an
ordinary 400, a transport failure, a WebSocket request, a stream error after
HTTP 200, or a request whose delivery state is uncertain. The retry cannot be
later than the third native upstream send.

The fallback removes only historical encrypted content before the current
readable task. Supported readable summaries, text, tool-call relationships,
ordering, and numeric precision are retained. The omitted context cannot be
recovered, and continuation is not guaranteed. Current-task ciphertext,
compaction state, server-side conversation pointers, and unknown input shapes
stop the fallback rather than being silently rewritten.

The client request and local history are not modified. A retry can increase
latency and upstream usage. Keep the setting disabled unless the loss of
encrypted context is an acceptable operational tradeoff.

Diagnostics are recorded as a fixed decision value in the administrator
upstream-error detail. They do not expose ciphertext, request bodies,
credentials, or plaintext reasoning.

## Verification

The implementation is covered by service, handler, admin-settings, API
contract, frontend, and browser-preview regressions. The browser preview uses
synthetic settings state and does not represent production acceptance.
