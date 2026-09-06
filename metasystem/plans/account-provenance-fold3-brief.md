Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal account-provenance)
Date: 2026-09-07

# Round 4: fold the second closing critique (chain account-provenance-build1)

The fresh critic on round 3 (chain account-provenance-crit4) found one
material item, APC-03, and one note, APC-04. APC-03: two success-path
capture fixtures in internal/account/account_test.go (a round-1 file,
cited without the tree prefix), TestCaptureStampsCapturedAt and
TestCaptureNeverStoresAdapterStderr, run the adapter under a one-second
bound and expect it to finish; under parallel package test load the
bound expires and both fail with the cause token `timeout`, so the go
gate can go red on a healthy tree. The design says the capture ceiling
is a hang bound, not a speed assertion; these two fixtures make a speed
assertion. APC-04, noted and not actioned: the fake adapter's `hang`
knob is exercised by no fixture; the design names it, so it stays.

# Decisions (the orchestrator's; decided, not open)

D20. In internal/account/account_test.go, every success-path fixture
that expects the adapter to finish runs under the production bound
(twenty seconds) or more; the one-second bound stays only in
TestCaptureTimeoutKillsAdapterGroup, whose allowance already covers it.
No product code changes.

D21. The diff boundary is the round-3 boundary; nothing else changes.

# Verification

Required, run from the worktree and reported at evidence level ran:
`scripts/agents/go-gate.sh --fast`, then `go test ./internal/account/
-count=1` three times in a row beside `go test ./internal/adapter/
./internal/lease/ ./internal/landing/ ./internal/up/ ./internal/census/
-count=1` running in parallel, all green.

# Constraints

Wall-clock budget: 20 minutes. Return per the implementer schema with
the cumulative diff boundary listed. Gap rule: stop and report a gap;
never fill it silently.
