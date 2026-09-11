Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal delivery-candidate-is-the-workspace-not-the-ledger)
Date: 2026-09-11

# A fresh chain: the reviewed candidate plus the closing read's six fixes

Chain dcwl-build2-20260910 built the whole change (its round 2 is the
reviewed candidate, whole-project tree 32c57bfd, installation subtree
c27379772a), closed on a live proof (dcwl-verify1-20260911) and was read
twice. The second read (`metasystem/artifacts/agents/dcwl-context/code-read-r2.md`,
with the coordinator's dispositions at its end, binding) found one
material regression and five low items. That chain is closed and takes no
further round, so this chain starts from `main` at 609071da2, applies the
reviewed candidate verbatim, and folds the six fixes. Its one round is
what the next read and live proof cover.

# Step 0: apply the reviewed candidate, verbatim

`metasystem/artifacts/agents/dcwl-context/rounds-1-2-and-legs.patch`
(sha256 d841f9dba3ce2e963b4a0eba869a1e1e1de6ad308d0cc857e2d1324434aa02fe)
is chain dcwl-build2-20260910's round-2 staged diff, taken with
`git diff --cached` at the repository toplevel. Apply it from the
repository toplevel with `git apply --index`; the coordinator has checked
it applies cleanly on 609071da2. Do not rewrite what it contains beyond
the six fixes below; if a fix forces more, change the least you can and
say so under `deviations`.

# The six fixes (the second read's dispositions, verbatim in intent)

1. **DCW-19, material.** `landing test-receipt --mode auto` without
   `--tree` must publish its receipt again. `PublishCommittedReceipt`
   takes the accepted index tree; the three callers in
   metasystem/cmd/metasystem/landing_verbs.go pass the raw `--tree` flag,
   which is empty when omitted, and metasystem/internal/landing/receipt.go
   refuses an empty tree. The accepted tree is the tree the run judged:
   the result's `CandidateTree` (site 1 resolves an empty `--tree` to the
   staged index, metasystem/cmd/metasystem/test.go:227-229), so pass
   `result.CandidateTree` (and, at the other two call sites, the attempt's
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
   variable is `1` (the worker's request check in
   metasystem/cmd/metasystem/test.go, after the patch).
3. **DCW-21.** The register row for `proof-input-moved-after-receipt`
   (metasystem/internal/refusal/register.go, after the patch) names the
   line that prints the refusal.
4. **DCW-22.** The `ledger-move-lands` leg prints the design's failure
   line, `site 8 refused a ledger-only move`, when the landing fails on
   the base engine (the leg's fail branch in
   metasystem/scripts/agents/land-fixtures.sh, after the patch), and keeps
   its land.out dump.
5. **DCW-23.** The step-0 "other tree" subtest in
   metasystem/internal/landing/testing_test.go computes its own second
   tree instead of reading `keyWhole` from an earlier subtest; it must
   pass under `-run` in isolation.
6. **DCW-24.** The candidate engine's build environment pins `GOAMD64`,
   `GOARM64` and `GOARM` to the Go defaults (`v1`, `v8.0`, `7`) in
   `candidateEngineBuildEnvironment` and the build-context string in
   `candidateEngineBuildIdentity` names them (both in
   metasystem/cmd/metasystem/test.go after the patch), with a one-line
   comment saying why.

# Proof before you return

- `go build ./... && go vet ./...`; `go test ./internal/landing/... ./internal/proofrun/... ./internal/gittree/... ./cmd/metasystem/...`
  (name the one sandbox-bound test if it is red), and the DCW-23 subtest
  under `-run` alone.
- `bash scripts/agents/go-gate.sh --fast` green.
- `bash scripts/agents/land-fixtures.sh` green with 13 legs;
  `bash scripts/agents/fixture-bed-scenarios-fixtures.sh` green.
- The engine for the beds: `bash scripts/agents/go-build.sh` in your
  worktree first (default stamp).

# Return

`diffBoundary` and `files` are repository-root paths (`metasystem/...`)
and include the thirty files of the applied patch. List under `evidence`
every command above with its observed result, and under `deviations`
every departure from the six fixes with the line. Wall-clock expectation:
60 minutes.
