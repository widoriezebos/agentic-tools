Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal delivery-candidate-is-the-workspace-not-the-ledger)
Date: 2026-09-11

# Round 3 of this chain: fold the closing read

Round 2 closed on its live proof and was read twice (dcwl-cc1-20260911,
dcwl-cc2-20260911). The second read
(`metasystem/artifacts/agents/dcwl-context/code-read-r2.md`, with the
coordinator's dispositions at its end, binding) found one material
regression and five low items. This round folds all six and nothing else;
keep every other byte of rounds 1 and 2.

# The six fixes

1. **DCW-19, material.** `landing test-receipt --mode auto` without
   `--tree` must publish its receipt again. `PublishCommittedReceipt`
   takes the accepted index tree; the three callers in
   metasystem/cmd/metasystem/landing_verbs.go pass the raw `--tree` flag,
   which is empty when omitted, and metasystem/internal/landing/receipt.go
   refuses an empty tree. The accepted tree is the tree the run judged:
   the result's `CandidateTree` (site 1 resolves an empty `--tree` to the
   staged index, metasystem/cmd/metasystem/test.go:227-229), so pass
   `result.CandidateTree` (and, on the other two call sites, the attempt's
   `TestResult.CandidateTree`), never the flag. Add one test to
   metasystem/cmd/metasystem/landing_verbs_test.go that runs the mode form
   WITHOUT `--tree` in the contract-bearing fixture repository and
   asserts the receipt is published at the index tree's path. The shipped
   instructions that use the no-tree form
   (metasystem/docs/project-adaptation.md:14, metasystem/scripts/adopt.sh:466)
   stay as they are.
2. **DCW-20.** A worker-level test pins the compatibility door: the test
   worker refuses a request without `CandidateEngineBuildIdentity` when
   `METASYSTEM_POLICY_PROBE_WORKER` is unset and accepts it when the
   variable is `1` (metasystem/cmd/metasystem/test.go:953-958).
3. **DCW-21.** The register row for `proof-input-moved-after-receipt`
   (metasystem/internal/refusal/register.go:113) names the line that
   prints the refusal.
4. **DCW-22.** The `ledger-move-lands` leg prints the design's failure
   line, `site 8 refused a ledger-only move`, when the landing fails on
   the base engine (the leg's fail branch near
   metasystem/scripts/agents/land-fixtures.sh:1231), and keeps its
   land.out dump.
5. **DCW-23.** The step-0 "other tree" subtest in
   metasystem/internal/landing/testing_test.go computes its own second
   tree instead of reading `keyWhole` from an earlier subtest; it must
   pass under `-run` in isolation.
6. **DCW-24.** The candidate engine's build environment pins
   `GOAMD64`, `GOARM64` and `GOARM` to fixed values
   (`candidateEngineBuildEnvironment`, metasystem/cmd/metasystem/test.go
   near line 512) and the build-context string in
   `candidateEngineBuildIdentity` names them, so the build identity
   covers the microarchitecture. Pick the Go defaults (`v1`, `v8.0`, `7`)
   and say so in a comment of one line.

# Proof before you return

- `go build ./... && go vet ./...`; `go test ./internal/landing/... ./internal/proofrun/... ./cmd/metasystem/...`
  (name the one sandbox-bound test if it is red), and the DCW-23 subtest
  under `-run` alone.
- `bash scripts/agents/go-gate.sh --fast` green.
- `bash scripts/agents/land-fixtures.sh` green with 13 legs (the
  `steward disarm` step now works in the sandbox; if any step is denied,
  say which).

# Return

`diffBoundary` and `files` as before. List under `evidence` every command
above with its observed result, and under `deviations` every departure
from the six fixes with the line. Wall-clock expectation: 45 minutes.
