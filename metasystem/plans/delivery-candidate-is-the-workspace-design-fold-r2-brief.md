Working Mode: design
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, coordinator under goal delivery-candidate-is-the-workspace-not-the-ledger)
Date: 2026-09-10

# Fold brief: revision 3 of the workspace-candidate design

## Your authority to author this revision

You are dispatched to REVISE a design. The composed prompt's Working Modes
table forbids design authoring to every delegate; that row is scoped to the
mode, not the role, and goal `design-prohibition-is-role-scoped` exists to
fix it. Until it lands this paragraph is the override, and it is standing
authority: R-89-m1d and R-25 put design authoring on this lane. Author the
revision; do not stop to report an authority conflict.

## What you are revising

`metasystem/plans/delivery-candidate-is-the-workspace-design.md`, revision
2, in your worktree at commit bc164d658 (sha256
dca76908f7538f987844abdec61d699c58fb13d552381395ce2d0d27c75601ae). Revise
it IN PLACE to revision 3: revision line updated, a revision record entry
naming each finding and what moved. The read is
`metasystem/artifacts/agents/dcwl-context/critique-r2.md`: six findings,
all material, each verified against the tree by the coordinator before
disposition; the dispositions at its end are binding. This is the last
design round: what you write is what gets built. Everything revision 2
settled stays settled; the six items below are the whole fold. Read the
code at 2fbd77535 as before.

## DCW-08, high: the observer must not fail open (section 4.4)

`landing observe` runs the verify core when the receipt carries
`workspace`, but section 4.4 hands the raw result to `readTestReceipt`,
whose exact and W fast paths run before the only sufficiency check (the
key branch). Between commit.sh's `test verify` (`metasystem/scripts/agents/commit.sh:272`)
and the observer's run, retained attempts, the destination policy state,
physical inputs, and the engine or environment can differ. Revise 4.4 so
that, when the receipt carries the field, the observer refuses BEFORE any
fast path whenever the core returns an error, a result whose
`Delivery.Sufficient` is false, or a `CandidateTree` whose `TreeOf` is not
`--tree`. Name the refusal (`chain-test-receipt-refused` with a detail
naming which of the three, or a command error; say which and why) and add
to 8.4 a test in which commit.sh's run is sufficient and the observer's
run is not (a retained attempt removed between them is the simplest
lever). `metasystem/internal/landing/observe.go:161-177` is where the
observer's refusals are shaped today.

## DCW-09, high: site 5's lifecycle as the launcher makes it (section 3)

Section 3 says a non-ledger index move at the end of a battery "finalizes
as a success without a committed receipt" and that the next
`landing test-receipt` composes with zero execution. The tree says
otherwise: `PrepareTestingReceiptPayload` is called from the launcher's
success callback (`metasystem/cmd/metasystem/proof_run.go:160-172`), and
an error there sets the result to 1 and the attempt terminal to FAILED
(`metasystem/internal/proofrun/launcher.go:445-451`, `:457-465`); a
failed attempt's groups are not reusable (`validateTestingAttemptOwners`
requires terminal success, `metasystem/internal/landing/testing.go:113-118`;
`ExactReusableTestResult` requires success plus a committed receipt,
`metasystem/internal/proofrun/test_result.go:258-264`). Withdraw the
"success without receipt" lifecycle. State the residual as it is: a
non-ledger move of the INDEX while a battery runs is a terminal failure
at today's cost, a re-run, and nothing in this design changes that; the
W fast path is what this design adds at site 5, nothing more. Add
`metasystem/cmd/metasystem/proof_run.go` (`:160-172`, `:628-642`) to the
readers in section 2 and to the files list in section 11 if
`PrepareTestingReceiptPayload`'s signature or contract moves; say whether
it does.

## DCW-10, high: an honest cutover leg (section 8.2)

The `receipt-cutover` leg shims `bin/metasystem` so `landing workspace`
is unknown but forwards every other verb to the NEW engine, so a receipt
carrying `workspace` would be accepted by the new reader where a real
older engine refuses it (`DisallowUnknownFields`,
`metasystem/internal/landing/receipt.go:458-462`). Revise the leg: either
the shim reproduces the older engine's strict decoding for every verb
that reads a receipt (refuse any receipt carrying `workspace`, refuse
`landing workspace` as unknown), or the leg builds the base engine once
(`metasystem/scripts/agents/go-build.sh --out` from the seed's base
commit) and uses it as the old engine. Choose, say the cost, and make the
leg prove three things: an old receipt lands exactly when the tip did not
move; the same landing refuses at site 8's first sentence after the
peer's ledger move; a receipt carrying the field is refused by the old
engine.

## DCW-11, medium: the shared cap must initialise itself (section 8.3)

`harness_fixture_scaled_cap` refuses when `METASYSTEM_FIXTURE_CAP_SCALE_MILLI`
is unset (`metasystem/scripts/agents/fixture-budget.sh:431-438`). Of the
six beds that source `metasystem/scripts/agents/fixture-bed-scenarios.sh`
(brain, land, mission, return-schema, second-session, witness-gate), only
`metasystem/scripts/agents/mission-fixtures.sh:17` calls
`harness_fixture_budget_init`; the others would refuse before their first
scenario. Revise 8.3 so `run_fixture_bed_scenarios` initialises the
budget itself when the scale is unset, idempotently, and inherits it when
a parent bed already set it; name the fixture that proves both the
standalone and the inherited-scale run.

## DCW-12, high: reap the scenario's process group (section 8.3)

`fixture_bed_parent_cleanup` (`fixture-bed-scenarios.sh:20-29`) and the
calibration loop the page reuses (`fixture-budget.sh:359-369`) signal the
direct child pid only; a timed-out scenario's descendants survive, keep
locks and state, and contaminate the next scenario. This repository has
learned this the hard way (fixtures that leak compound into self-worsening
flakes). Revise 8.3: every scenario runs in its own process group (say
how in bash: `setsid` where present, else a subshell with job control, or
the engine's own bounded-exec verb if one fits); on timeout or on a parent
signal the group gets TERM, a grace, then KILL, and the direct child is
waited for; and one scenario spawns a descendant that ignores TERM and
proves no survivor remains after the ceiling. Name the fixture.

## DCW-13, high: the engine identity, narrowed (section 5, constraint 1)

The identity you wrote hashes `go version` and `go env` values
(`metasystem/scripts/agents/go-gate.sh:186-190`,
`metasystem/internal/proofrun/execution_context.go:91-110`), not the bytes
of the compiler selected; two toolchains can report the same and emit
different engine bytes. Revise constraint 1 to bind the toolchain closure
(the selected `go` executable's digest and its `GOROOT/VERSION`) into the
engine identity, or drop the byte-identity claim in favour of "equal
identity, equal bytes, proven once per toolchain on this host". Choose
and say why. The same-report/different-output case is a unit test of the
identity function with a substituted toolchain path, not a two-compiler
bed; say so.

## What must not change in this fold

Every decision of revision 2 outside the six items above. The key is the
selected component identities; W is the floor; site 9 is today's call;
recertification stays exact; the exclusion set of 1.1.

## Deliverable

The revised page in place, revision 3, with the revision record. Cite
lines at 2fbd77535 for everything you add. Wall-clock budget: 35 minutes.
Design only; you implement nothing and run no bed.
