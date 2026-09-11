Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal delivery-candidate-is-the-workspace-not-the-ledger)
Date: 2026-09-10

# A fresh chain that carries two built rounds and adds the five land legs

Chain dcwl-build1c-20260910 built rounds 1 and 2 of this goal (the
engine and shell code with unit tests; the harness ceiling, budget and
process-group reap; the new harness bed), all green, and its round 4 was
cancelled by the coordinator's own `stop --repo` while re-arming the
seat, not by any finding. That chain can take no follow-up. This chain
starts from `main` at 6c28684cd, applies those two rounds verbatim, and
builds the five land legs on the decisions of the round-3 and round-4
briefs. The closing read then reviews all of it as one round.

# Step 0: apply the two built rounds, verbatim

`metasystem/artifacts/agents/dcwl-context/rounds-1-2.patch` (sha256
93069e31f473b03ce484c3ccc39487015ac1cb98055eda1144a421628b4c4347) is
the staged diff of chain dcwl-build1c-20260910 after round 2, taken with
`git diff --cached` at the repository toplevel. Apply it from the
repository toplevel with `git apply --index rounds-1-2.patch`; the
coordinator has checked it applies cleanly on 6c28684cd. Do not rewrite
what it contains unless a leg proves it wrong; if one does, change the
least you can and say so under `deviations`. Read its 29 files once
before you build, alongside the design page.

# The specification

metasystem/plans/delivery-candidate-is-the-workspace-design.md, revision 3
(landed at 6c28684cd with two coordinator notes at the end of its
revision record), read twice on the Sol lane
(metasystem/records/misc/delivery-candidate-is-the-workspace-critique-r1.md,
metasystem/records/misc/delivery-candidate-is-the-workspace-critique-r2.md).
The build briefs of the first chain are landed and stand:
metasystem/plans/delivery-candidate-is-the-workspace-build-brief.md (round 1),
metasystem/plans/delivery-candidate-is-the-workspace-build-brief-r2.md (round 2),
metasystem/plans/delivery-candidate-is-the-workspace-build-brief-r3.md
(decisions D1 to D4 and D6 for the land legs), and
metasystem/plans/delivery-candidate-is-the-workspace-build-brief-r4.md
(D5 corrected: the cutover control is the old engine's own receipt at the
same path, no field stripped by hand).

# What you build: the five land legs of section 8.2, on D1 to D6

In metasystem/scripts/agents/land-fixtures.sh: the contract-bearing seed
(D1: `metasystem.conf` with `testing.contract=testing.json`,
`metasystem.runtimes=fake`, `dispatch.cap-min=1`, `dispatch.cap-max=120`;
a `testing.json` in the shape of
metasystem/cmd/metasystem/landing_verbs_test.go:1358-1375 with one group
declaring `inputs: ["payload.txt", "scripts/**"]`; the fx goal through
the bed's existing verbs), the engine stamped with the seed's first
commit (D2: `METASYSTEM_BUILD_STAMP=$H0 bash "$root/scripts/agents/go-build.sh" --out ...`,
committed as H1), clone one enrolled with `steward arm --repo` in fixture
mode and stopped with `stop --repo` before the leg returns (D3, with the
fake process-identity table and the receipt canary's environment), the
enrolled-engine receipt (D4), the old engine from `git -C "$root" archive
6bc19ba1c metasystem/` built the same way (D5), and the four scenarios
`ledger-move-lands` (with the drift checks), `records-move-lands`,
`input-move-refuses`, `receipt-cutover`, the success line at 13 legs
(D6). The three proofs of the cutover leg are as the round-4 brief
states them. If a decision meets a contradiction in the tree, build the
legs that do not depend on it, record it under `deviations` with the
line, and keep going.

# Facts

- Two landings sit between the first chain's base and yours:
  92b31f774 (the enrollment binds the engine by the skew rule) and
  79b0af799 (section beds find the candidate engine by path). Their
  hunks in metasystem/internal/proofrun/test_build.go and
  metasystem/testing.json do not overlap the patch; read them anyway
  before you rely on `test_build.go` behaviour.
- The sandbox denies process enumeration (`sysctl`). The coordinator ran
  the dispatch and mission beds and the one process-group Go test outside
  it on the two-round tree: all green. Do not work around the sandbox;
  report what it blocks.
- The engine the bed copies is `$root/bin/metasystem`; build it in your
  worktree with `bash scripts/agents/go-build.sh` (default stamp) before
  running the bed.

# Proof before you return

- `go build ./... && go vet ./...`; `go test ./internal/landing/... ./internal/proofrun/... ./internal/gittree/... ./cmd/metasystem/...`
  (report the one sandbox-bound test by name if it is red).
- `bash scripts/agents/go-gate.sh --fast` green.
- `bash scripts/agents/land-fixtures.sh` green with 13 legs.
- `bash scripts/agents/fixture-bed-scenarios-fixtures.sh` green; `bash scripts/agents/return-schema-fixtures.sh` green.
- After every leg, no process started by a leg survives; say how you
  checked.

# Return

`diffBoundary` and `files` are repository-root paths (`metasystem/...`)
and include the 29 files of the applied patch. List under `evidence`
every command above with its observed result, and under `deviations`
every departure from D1 to D6 with the line. Wall-clock expectation: 100
minutes; at the cap return what is green and say what is red and why.
