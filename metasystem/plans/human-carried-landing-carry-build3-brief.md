Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal human-carried-landing-carry)
Date: 2026-09-11

# A fresh chain: the folded carried landing, rebased and proven

Chain hcl-build2-20260911 built the carried landing (round 1, read on
Opus: `metasystem/artifacts/agents/hcl-context/code-read-r1.md`, fifteen
findings) and folded the read in round 2 under
`metasystem/artifacts/agents/hcl-context/build2-fold-r1-brief.md`, whose
decisions are binding here. Round 2 finished its implementation, its
goal-cli and dispatch beds and the fast gate, then spent its last forty
minutes waiting on a sandbox-bound package test and hit the cap; a
timed-out round takes no follow-up. So this chain starts from `main` at
01124a60c, applies round 2's work, resolves the rebase, and proves it.
Its one round is what the second read and the live proof cover. The goal
is human-carried-landing-carry
(metasystem/plans/goals/human-carried-landing-carry.md); the design is
metasystem/plans/human-carried-landing-carry-design.md revision 6, with
the fold brief's three page amendments (the stop list's empty-staging rule
excludes the rerun states; single-machine mode out of scope with
`carry-remote-required`; `goal carry` speaks after its transaction).

# Step 0: apply round 2, resolve the rebase

`metasystem/artifacts/agents/hcl-context/build2-r2.patch` (sha256
9012a8467c7653de510248ca8239894b813bbfcdf48997251da4a1692ec5351c) is
round 2's staged diff against 6ac70b344, 55 files, taken with
`git diff --cached --binary` at the repository toplevel. Trunk moved
between 6ac70b344 and 01124a60c (per-landing verification standard with
deep runs at cadence; the land bed's cutover leg building each checkout's
candidate from its own engine; the review-round limit counting critic
chains per class; exhaustion decided on a fresh projection). Apply the
patch from the toplevel with `git apply --3way --index`; it conflicts in
at least metasystem/internal/goal/severity_tiered_rigor_coverage_test.go,
metasystem/internal/steward/health.go and
metasystem/internal/testpolicy/contract_test.go. Resolve every conflict
as a union: trunk's change and the patch's change both kept, and say each
resolution under `deviations`. Keep everything else in the patch as it is.

# The rule of this round

No fixture claims what it does not drive (the fold's rule). And: **no
waiting on the sandbox.** A package test or bed scenario that is
sandbox-bound (process enumeration, process-group operations, the linked
worktree's object store, a vanishing local bare repository) is named in
your return within five minutes of first seeing it and left to the
coordinator's host rerun; never poll it, never rerun it, never let it run
to a bound. Round 2 lost its cap to exactly that.

# What this round does, in order

1. Step 0; `go build ./... && go vet ./...`.
2. Verify the fold, decision by decision (the fold brief's A1 to A10, B11,
   B12, C13, C14): each one is present in the applied tree at file:line
   or you finish it now. Round 2 reported A and B complete, C13 reduced
   to two legs that drive land.sh (a fresh carried landing; a
   post-reservation mismatch that closes its own row), and a `--entry`
   selector defect fixed on the way.
3. Block C, as far as it fits, in this order: the two HCL-33 two-seat
   legs (interleave; trailer-without-row as abandonment and as expiry,
   seat B asking carry-debt-unpaid naming A's word); the HCL-38
   ledger-path leg (a ledger-class path staged beside a chainless payload
   refuses with `ledger-path-not-goal-verb`); the HCL-35 rerun leg (crash
   after the push, `land.sh --carried` completes the record without a
   staged set). Every leg drives land.sh's own reservation. A leg you do
   not add is named under `deviations`.
4. `bash scripts/agents/go-build.sh` then `metasystem test check --root .`
   green; `go test ./internal/... ./cmd/metasystem/...` with the
   sandbox-bound tests named, not waited for.
5. `bash scripts/agents/land-fixtures.sh`, `bash scripts/agents/goal-cli-fixtures.sh`,
   `bash scripts/agents/dispatch-fixtures.sh`: every leg and scenario that
   the sandbox can run green; the sandbox-bound ones named.
6. `bash scripts/agents/go-gate.sh --fast` green.
7. `git write-tree` of the staged candidate in the return.

# Return

`diffBoundary` and `files` are repository-root paths. Under `evidence`
every command with its observed result; under `deviations` every
resolution, every finding or leg left out, by id, with the reason. Return
BEFORE the 120-minute cap: at 90 minutes stop adding legs, run steps 4 to
7 and return. A return with two legs named as missing beats a timeout.
