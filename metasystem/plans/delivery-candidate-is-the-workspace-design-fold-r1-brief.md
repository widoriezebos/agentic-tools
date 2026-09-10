Working Mode: design
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, coordinator under goal delivery-candidate-is-the-workspace-not-the-ledger)
Date: 2026-09-10

# Fold brief: revision 2 of the workspace-candidate design

## Your authority to author this revision

You are dispatched to REVISE a design. The composed prompt's Working Modes
table forbids design authoring to every delegate; that row is scoped to the
mode, not the role, and goal `design-prohibition-is-role-scoped` exists to
fix it. Until it lands this paragraph is the override, and it is standing
authority: R-89-m1d and R-25 put design authoring on this lane. Author the
revision; do not stop to report an authority conflict.

## What you are revising

`metasystem/plans/delivery-candidate-is-the-workspace-design.md`, revision
1, landed at 440081320. Revise it IN PLACE to revision 2: revision line
updated, a revision record at the end naming each finding and what moved.
The read is
`metasystem/artifacts/agents/dcwl-context/critique-r1.md` (the register, projected from the job return; it lands under `records/misc/` with the chain):
seven findings, six material. The coordinator's dispositions at the end of
that register are binding on this fold. Decisions 1.2 (one file, two
declarations), 1.3 (`FilterTreePrefixes`), 2.1 (the drift check inside one
checkout stays exact), 6 (recertification stays exact, follow-up named) and
the twelve-site inventory stand. Sections 0, 2, 3, 4, 5, 7, 8 and 9 are
what this fold revises. Read the code at 2fbd77535 and the two patches
under `metasystem/artifacts/agents/dcwl-context/` as before.

## DCW-01, high: the key is the selected component identities, W is the floor

Revision 1 made W the universal cross-tip equivalence and refused every
non-ledger difference. Measured on `origin/main` for the day before the
read: 375 goal-verb commits and 45 other landings, most of them records
only. Under revision 1 a sibling's critique register or design page still
voids a receipt whose every selected component identity is unchanged. That
fails the sentence the goal exists for. The contract's own design
(`metasystem/plans/application-testing-contract-design.md:231`) names the
key: "compares its component input identities against the current
candidate". `RevalidateRetainedGroupExecutionIdentities`
(`metasystem/internal/proofrun/test_build.go:482`) already recomputes
exactly that for a tree: declared and retained implicit inputs, environment,
executable bytes, and through `groupExecutionIdentity` (`:641`) the
contract, policy and section-engine digests.

Revise so that:

- The cross-tip acceptance at sites 7, 8 and 9 is: for the landing goal
  and accounting revision, every selected group's revalidated execution
  identity on the CURRENT tree equals the identity a retained successful
  attempt owns (the composition `reusedTestResult` already performs). Say
  where each site gets that answer: site 8 in the shell, site 7 inside
  `readTestReceipt`, site 9 after the rebase. Prefer one engine function
  that all three call, and say whether it is `test verify` itself.
- W stays: recorded in the receipt (`workspace`, section 2.3) as the
  auditable minimum projection, printed by `landing workspace`, compared
  as a cheap fast path where you want one, never as the sole refusal.
- Section 3 follows: exact reuse (`ExactReusableTestResult`) matches on
  the per-group identities it already checks plus the digests, with
  `CandidateTree`, `BaseCommit`, `PolicyBaseCommit` as recorded provenance;
  say what `PlanDigest` is taken over now that W is not the key (the plan
  is a function of the candidate's own changed paths and risk; say whether
  it needs to move at all).
- The refusal of section 4 is issued only when a selected group's identity
  changed after the rebase, and its message names the group and the paths
  whose digests moved. Keep the code `workspace-changed-after-proof` or
  rename it to what it now means; verbs and codes name intent
  (`metasystem/memory/rulings.md`, R-25 family of rulings on names).
- The fixtures follow: the negative leg changes a DECLARED input
  (`metasystem/scripts/agents/coverage-delta.sh` is declared by
  `coverage-policy` and by every `section/*` group through
  `metasystem/scripts/**`); a new positive leg lands after a records-only,
  non-input move (a file under `records/misc/`), in addition to the
  ledger-only leg.
- The residual case, a declared input really moved after the receipt, is a
  real re-proof. Revision 1's "re-receipt and land again" leaves the
  rebased commit on the local branch with no `land.sh` path that pushes an
  existing commit. State the recovery as it is today (reset the branch to
  the pushed tip, re-stage, re-receipt, land) in one paragraph, and keep
  the one-line follow-up goal `land-resumes-a-refused-push`; it is not this
  goal's DONE.

## DCW-02, high: the command-line cutover

`test verify` at 2fbd77535 (`metasystem/cmd/metasystem/test.go:139-155`)
has `root`, `goal`, `tree`, `cap-min`, `retry-decision`, `result`, and no
`--receipt`. `land.sh` runs the ENROLLED engine (`:12-15`), and commit.sh
uses the same engine for `test verify` when `testing.contract` is set
(`:271-278`); the proof engine built at `:313` is the legacy branch and
does not read this receipt. The landing that lands this change therefore
runs revision-1-shaped `land.sh` against an engine without the flag.

First try to need no new flag: after the rebase, `test verify --tree
HEAD^{tree}` already revalidates identities and composes; if DCW-01's
site-9 answer is exactly that call, site 9 does not change and the cutover
question is confined to sites 7 and 8. If a flag stays, gate it on the
receipt carrying the `workspace` field (a receipt without it takes today's
exact path and today's call), and name a cutover leg that runs the
older-engine plus new-`land.sh` pair with an old receipt and lands when the
tip did not move.

## DCW-03 and DCW-04, high: the engine identity, as constraints on the engine-bed chain

Revision 1 keyed retained candidate-engine evidence by W (section 5,
constraint 2). An engine digest belongs to the engine BUILD identity, not
to workspace bytes: a records-only landing changes W without changing the
engine and would lose every section group again. Rewrite constraint 2:
retained candidate-engine evidence is located by a complete engine build
identity, never by T or W.

Rewrite constraint 1 with the tree as it is: there is no `vendor/`
directory at 2fbd77535 (`git ls-files vendor` is empty); `go-gate.sh`
exports `GOFLAGS=-mod=readonly` (`metasystem/scripts/agents/go-gate.sh:66`)
and `go-build.sh` does not (`metasystem/scripts/agents/go-build.sh:73`
and `:82` pass only `-buildvcs=false`); the engine-bed patch clears
`GOFLAGS` and pins `GOTOOLCHAIN=local`. The constraint reads: the build
forces `-mod=readonly`; the engine identity binds the ENGINE projection
(`metasystem/internal/behaviorsurface/policy.v2.json:3-10`, which already
holds `go.mod` and `go.sum`), the toolchain identity and the platform;
byte-identity is claimed only for an identical build context; and one
fixture fails when an input outside that key changes the engine bytes.
Keep both as constraints on that chain, stated so its owner can apply them
without reading this page twice.

## DCW-05, high: proof proportionate to a landing-gate change

Revision 1's shell legs take the first-transition policy path and promise a
two-minute ceiling that nothing enforces:
`metasystem/scripts/agents/fixture-bed-scenarios.sh:43-53` launches and
waits for each child without a deadline, and
`metasystem/internal/proofrun/test_build.go:196-216` only sums `targetMs`.
Revise section 8 so that:

- one bounded leg exercises the production shell-to-engine path with a
  receipt that carries the `workspace` field (the bed copies the source
  engine into each fixture, `metasystem/scripts/agents/land-fixtures.sh:9-17`
  and `:50-61`, so the engine in the bed is the candidate's own);
- the cutover leg from DCW-02 exists;
- the per-leg ceiling names the mechanism that enforces it, or the promise
  is withdrawn; if enforcing it means touching
  `metasystem/scripts/agents/fixture-bed-scenarios.sh`, say so and add it
  to the files list;
- the first-transition legs stay for what they prove, with their limit
  stated as revision 1 states it.

## DCW-06, medium: two legacy ledger paths

`metasystem/internal/goal/goal.go:118-153` routes unconverted repositories
through the legacy store whose paths are `plans/goals.md` and
`plans/goals-accepted.json` (`:124-128`); `metasystem/internal/goal/migrate.go:283-288`
deletes both in the migration transaction. They are absent at 2fbd77535
and an absent filtered path is a no-op (section 1.3), so add both to
`WorkspaceExclusions()`, to the documented rule and to the projection
fixtures. Remove the sentence that omits them because they are absent.

## DCW-07, low: the counselor rationale

`metasystem/cmd/metasystem/goalsync_mutations.go:341-350` publishes the
accepted-risk decision first and appends the counselor register after;
`:1457-1461` does the same for a tier-raising edit; the goal transaction
(`metasystem/internal/goal/txn.go:231-292`) builds the published commit
from explicit changes only. So the register is not "in the same ledger
commit"; it is a local append after the publish. Correct the row in 1.1;
the exclusion stands on the other two reasons (goal verbs write it, no
group declares it, the counselor report is its only other reader:
`metasystem/internal/counselor/sources.go:78-89`).

## What must not change in this fold

Everything in section 9 as revised by DCW-01: T recorded; the drift check
inside one checkout exact; no group leaves `testing.json`; commit.sh's
postcondition; the contracts of `Snapshot`, `StagedTree`,
`SnapshotRelevant`, `FilterTree`; nothing from 1b12f534 weaker. The
`goal-records` surface stays.

## Deliverable

The revised page in place, revision 2, with the revision record. Cite
lines at 2fbd77535 for everything you add. Wall-clock budget: 45 minutes.
Design only; you implement nothing and run no bed.
