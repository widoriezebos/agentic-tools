# lane-one-state U1: the full gate's red (2026-10-13 01:57) — a repeated verb must write nothing (R-129)

Working Mode: Implement, on branch build/los1 (worktree /Users/wido/LocalStorage/GitHub/agentic-tools-m1e-los1), as uncommitted changes on top of 9780b9597 (folded into that commit by hand). Never commit, push, rebase or reset. Tests are the spec: fix the production code, never weaken a test.

The full gate `go test -p 4 -count=1 ./cmd/metasystem` failed one test: `TestEveryStatefulActionRepeatsAsSuccess` for `landing resolve` (intent_landing_resolve_test.go:132: "landing resolve on a resolved tree (records) wrote <lane dir>") and `landing prove` (landing_plain_verbs_test.go:436: "a repeated landing prove (records) wrote …/lane"). Cause: the end-of-verb shadow refresh (U1) writes state.json and events.jsonl on every verb, including a repeat whose effect already stands — ruling R-129: a repeat is a success that writes nothing.

Fix: `RefreshShadow` writes only when something changed — when `Diff(previous, next)` is empty (after `Carry`) it rewrites nothing (no state.json, no version bump, no event; LastObservation handling stays as corrected: a recovery after a failed observation is a change); `state.Replace` with an empty diff returns without touching the files (mtime included). The keeper's tick keeps the same rule. Test: two consecutive refreshes of an unchanged lane → the second leaves every file under the lane dir byte-identical with the same mtimes (the idempotency audit's own check); the two named subtests pass: `go test -count=1 -run 'TestEveryStatefulActionRepeatsAsSuccess' ./cmd/metasystem`.

## Checks
`go vet ./internal/landing/state ./internal/landing/plain ./cmd/metasystem`; `go test -p 4 -count=1 -timeout 30m ./internal/landing/state ./internal/landing/plain`; `go test -count=1 -run 'TestEveryStatefulActionRepeatsAsSuccess|TestEveryPublicActionHasAnIdempotencyRow|TestLandingShadow|TestLandingHealth' ./cmd/metasystem`.
