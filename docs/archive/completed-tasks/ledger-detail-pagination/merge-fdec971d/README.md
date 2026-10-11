# #361 frozen-binding and detail-pagination combination

Previous draft head: `48248352158fabaf691f582f4f31efba4ddf74d1`.
Ordinary mainline merge parent: `fdec971d2a01c25d93874b7688dffcff78f79088` (#361).
No rebase/force, new business edit, migration edit or production operation.

55 incoming files are byte-identical to mainline; the two shared documentation files preserve both sides.
The only incoming production change is `requestledger/billing.go`. All frontend source/dependencies,
pagination/session source, migrations including 278 and the restricted billing repository are unchanged.
See [source-proof.json](./source-proof.json).

Sequential independent specification review and code review both PASS. The frozen-target NULL-safe
checks do not alter source ordering/cursor keys. Rejected or missing binding is never reconstructed by
pagination; wallet/batch proof queries and existing committed-settlement visibility remain unchanged.
Reviewers were read-only and did not run gates.

Validation scope: rerun the complete backend default suite/lint, the two binding PG/race suites with
independent fresh databases, the 13 pagination/batch HTTP race cases, complete build, embed build/test,
and Fork Integrity. The prior full frontend suite (3680 passing / 8 existing opt-in skipped), 26 HTTP
session cases and both-role lifecycle browser results are reused only because their exact source and
dependencies are unchanged. The final rebuilt resource bytes are checked against the previously tested
build and exported for release verification. Short actual-build HTTP/PG pagination browser validation
is repeated for this backend combination. This is not production or user-local acceptance.

Current gate results and artifact hashes are recorded at completion. The old 56-case full protocol and
full tagged-unit records remain historical results; this combination does not claim to rerun them.
No historical repair, guessed binding, estimated charge or generation is performed. Future revocation /
frozen in-flight behavior is outside this merge, as specified by #361.

## Completed local results

All eight local gate commands exited 0: complete backend tests/lint, binding reproduction 4/4 race,
binding compatibility 36/36 race, paging/batch HTTP 13/13 race, full build, embed build/test and Fork.
The short actual-build HTTP/PG browser rerun passed for both roles at four widths, with 6 USD / 3 images
and zero page errors. All 356 resource bytes match the previous reviewed session build.

[Validation and compressed raw logs](./validation.json), [all resource SHA-256 values](./assets.sha256),
and [entry/ledger-chunk hashes](./entry-assets.json) support final-head review and release comparison.
The pre-browser waiter's scheduling deadline expired while Fork was running; the browser was then run
directly after Fork passed. This was not a test failure, and no completed gate was rerun.

Technical merge: `c25d74eb1f0383c95c839ea2e31d7b92d1430008`, with parents48248352 and fdec971d. The following metadata commit
records this SHA and changes no tested source or resource. Final pushed head and CI are reported on draft PR #360.
