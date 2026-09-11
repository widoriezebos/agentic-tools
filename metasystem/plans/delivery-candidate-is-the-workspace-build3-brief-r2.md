Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal delivery-candidate-is-the-workspace-not-the-ledger)
Date: 2026-09-11

# Round 2 of this chain: the seventh fix, then the whole proof list

Round 1 applied the reviewed candidate and made the six fixes (the
focused tests, the no-`--tree` publication, the worker door and the
isolated DCW-23 subtest all passed), then stopped on a gap the two reads
and the coordinator's out-of-sandbox bed runs all missed because none of
them ran the contract check: `metasystem test check` refuses the
candidate's `testing.json` with "testing group
section/fixture-bed-scenarios-fixtures references unknown section
fixture-bed-scenarios-fixtures". The coordinator reproduced it on the
round-1 tree. Stopping was right; this round makes the seventh fix and
runs everything.

# The seventh fix

Declare the section in metasystem/scripts/agents/validate-section-selector.sh
the way `land-fixtures` is declared: one tab-separated row in the
`sections` table (line 22 is `land-fixtures	landing chain fixtures`; add
`fixture-bed-scenarios-fixtures	fixture bed harness fixtures` beside it),
and, if the harness bed must also run in an adopted checkout, leave the
adopted-context skip list at lines 78-81 alone; if it must not (it builds
engines from the source tree, like `land-fixtures`), add it to that skip
list beside `land-fixtures`. Say which you chose and why in one line.
Then `metasystem test check --root .` passes, and `metasystem test plan
--root . --mode auto --purpose delivery --goal delivery-candidate-is-the-workspace-not-the-ledger`
reaches group selection (it may still stop at the sandbox's object-store
denial; report where it stopped and why).

Keep every other byte of round 1.

# Proof before you return

- `metasystem test check --root .` green.
- `go build ./... && go vet ./...`; `go test ./internal/landing/... ./internal/proofrun/... ./internal/gittree/... ./cmd/metasystem/...`
  (name the one sandbox-bound test if it is red).
- `bash scripts/agents/go-gate.sh --fast` green.
- `bash scripts/agents/go-build.sh` (default stamp), then
  `bash scripts/agents/land-fixtures.sh` green with 13 legs and
  `bash scripts/agents/fixture-bed-scenarios-fixtures.sh` green.
- `bash scripts/agents/validate-section-selector.sh` lists the new
  section (whatever its listing verb is; read the script's usage).

# Return

`diffBoundary` and `files` as before, now thirty-one paths. List under
`evidence` every command above with its observed result, and under
`deviations` every departure with the line. Wall-clock expectation: 60
minutes.
