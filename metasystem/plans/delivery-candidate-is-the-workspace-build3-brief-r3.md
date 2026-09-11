Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal delivery-candidate-is-the-workspace-not-the-ledger)
Date: 2026-09-11

# Round 3 of this chain: the accepted tree is the index the run judged, not the result's tree

Round 2 made the seventh fix (the contract check passes, the selector
lists the section) and then found, in the full Go test, that the
coordinator's DCW-19 instruction was wrong: passing `result.CandidateTree`
to `PublishCommittedReceipt` breaks exact reuse after a records-only
staged move, because `ExactReusableTestResult` returns the ORIGINAL
result and therefore the original tree, and publication then compares the
current index with the wrong accepted tree and refuses
(`TestLandingSharedTestingReceiptPublicRecoveryKeepsExactOuterAuthority/root`).
Round 2's diagnosis and its named repair are right; this round makes it.

# The eighth fix (decided)

In `runLandingTestReceipt` (metasystem/cmd/metasystem/landing_verbs.go,
the `--mode` branch around line 120): before `runTestRun`, resolve and
keep the accepted index tree: the `--tree` value when given, else the
staged whole-project index tree resolved exactly as `prepareTesting`
resolves an empty tree (metasystem/cmd/metasystem/test.go:227-229, the
`StagedTree` of the project workspace). Pass THAT tree to
`PublishCommittedReceipt`; the receipt's own `tree` and path stay the
result's, as today. The legacy `--command` branch already passes
`preparation.AcceptedIndexTree()` at its two sites and does not change.
Keep the no-`--tree` test of round 1 and make the round-2 failing test
pass again; add nothing else.

Keep every other byte of rounds 1 and 2.

# Proof before you return

- `go build ./... && go vet ./...`; `go test ./internal/landing/... ./internal/proofrun/... ./internal/gittree/... ./cmd/metasystem/...`
  (name each sandbox-bound test if red: the Git-object-store and
  process-visibility ones; the coordinator reruns them outside).
- `metasystem test check --root .` green.
- `bash scripts/agents/go-gate.sh --fast` green.
- `bash scripts/agents/go-build.sh` (default stamp), then
  `bash scripts/agents/land-fixtures.sh` green with 13 legs and
  `bash scripts/agents/fixture-bed-scenarios-fixtures.sh` green.

# Return

`diffBoundary` and `files` as before (thirty-one paths). List under
`evidence` every command above with its observed result, and under
`deviations` every departure with the line. Wall-clock expectation: 50
minutes.
