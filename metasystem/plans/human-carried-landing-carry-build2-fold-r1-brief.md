Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal human-carried-landing-carry, follow-up round on chain hcl-build2-20260911)
Date: 2026-09-11

# Fold: the closing read's findings on the carried landing

Round 1 of this chain (reviewed tree b38cf7977ec3ce21a5f04be0fa686dec9b27730a)
was read on Opus: `metasystem/artifacts/agents/hcl-context/code-read-r1.md`
(hcl-cc1-20260911), fifteen findings, eleven material, four critical. The
coordinator's decisions below are binding, one per finding; the design,
metasystem/plans/human-carried-landing-carry-design.md revision 6, stays
the specification and you do not edit it (the coordinator records the two
page amendments named below in its revision record at landing). This
round is rebased onto trunk by the dispatcher; trunk moved on
internal/goal (a `Claimed` key `episodeAt`, the elapsed-budget origin) and
testing.json, so read what the rebase brought before you build.

The read's central fact: your named Go fixtures and bed legs did not test
what their names say, so five engine and wrapper defects went unnoticed
through a green run. This round's rule is therefore: **no fixture claims
what it does not drive.** A TestHCL function or a bed leg either runs the
production path and asserts the oracle its design line states, or it does
not exist. Fewer real tests beat many shells.

# Decisions, in build order

## A. Engine and wrapper

1. **HCL-C-35 (critical), reruns before staging.** `land.sh --carried`
   reads `landing carry-status` and takes its branch BEFORE the
   "stage caller paths" step; the rerun states (`local:<sha>`,
   `origin:<sha>`, `ledger:`, the counselor repair) need no staged set
   and never reach `stage_changes`. Only a fresh carried landing (state
   `open`) stages. Page amendment (coordinator's, at landing): the stop
   list's "empty staging set" excludes the rerun states.
2. **HCL-C-36 (high), the wrapper closes its own row.** Between the
   reservation and the push, every exit of land.sh that is not the push
   runs `goal carrying --abandon <row> --why <the ask or failure>`: put it
   in the EXIT trap, armed after `reserve_carry` and disarmed after the
   push succeeds, so an ask from commit.sh, a verify failure, a judge
   failure and a signal all close the row. `carried-asks` checks the
   closing row after carry-unneeded and carry-refusal-mismatch.
3. **HCL-C-37 (high), evaluator-unavailable first.** In the carried
   classification the evaluator-unavailable case (base judge, a recorded
   live failure, ordinary pass, sufficient result, word names
   `evaluator-unavailable`) is decided before carry-unneeded; a table test
   drives both orders (the word names it: lands; the word names something
   else in that state: carry-refusal-mismatch; no live failure recorded:
   carry-unneeded).
4. **HCL-C-38 (critical), the ledger check runs whatever the declaration.**
   The carried classification runs the path-class and record-ownership
   checks over the candidate's changed paths before the match, in every
   declaration mode: a change under a ledger-class path
   (`ledger-path-not-goal-verb`), a runtime path (`runtime-path-refused`),
   an unclassified path (`path-unclassified`) or a record not owned
   (`record-not-owned`) is a refusal the word cannot carry, and it refuses
   even when the word names `missing-declaration`. If the ordinary
   observer's early return for a chainless candidate hides those checks,
   move them ahead of the declaration decision (the ordinary refusal
   stays `missing-declaration` when nothing else is wrong). One word, one
   defect: a second carryable refusal beside the named one asks
   carry-refusal-mismatch naming both. Land leg: a ledger path staged
   beside a chainless payload under a `missing-declaration` word refuses
   with `ledger-path-not-goal-verb`; a `records/counselor` line beside it
   refuses with `record-not-owned` unless the wrapper wrote it.
5. **HCL-C-39 (high), the fence holds on every row.** `goal carrying` and
   `goal carried` refuse a format-1 ledger with the same
   `carry-format-required` ask as `goal carry`, and `ValidateTree` under
   format 1 reports a `carrying` or `carried` row as a problem. Fixture: a
   channel carry word bound on a format-1 ledger; the reservation asks;
   after `--raise-format` it publishes.
6. **HCL-C-40 (medium), local mode is out of scope, by name.** A carried
   landing needs a code remote. `goal carry`, `goal carrying` and the
   carried classification ask `carry-remote-required` ("a carried landing
   needs a code remote: set goal.sync-remote") when the checkout is in
   single-machine mode; the code adds the code as a Question row of the
   register (no Override) and a goal-cli scenario drives it. The anchor
   rule stays as the page states it for remote mode. Page amendment
   (coordinator's): local mode is named out of scope with that ask.
7. **HCL-C-45 (low), four asks follow the page.** (a) the workspace ask
   posts the `--kind carry` question when the word is a channel word, and
   the printed `goal carry --supersede` line is complete (with `--by` and
   `--why`); (b) `goal carrying --owner-pid` with a live non-ancestor exits
   2; (c) a second `goal carried --entry` on a confirmed entry returns
   confirmed with detail `idempotent`; (d) a failed code fetch at the start
   of the carried sequence asks with exit 3.
8. **HCL-C-46 (the ordinary gate), restore it.** In
   metasystem/cmd/metasystem/test.go the candidate engine digest for an
   ORDINARY landing is recovered only from a sufficient successful attempt,
   exactly as at the base, and the base's test case (a later failed attempt
   with a different digest does not hide the sufficient digest) returns
   unchanged. The structured red result a group word needs is reached only
   when the observation is carried: pass that fact explicitly (a flag or
   parameter from the carried path), never by widening the ordinary rule.
9. **HCL-C-48, HCL-C-49 (low).** `ParseCarryWord` reads its keyed fields
   outside the quoted `why`. The carried observer checks the Carried-By
   value against the word's actor and refuses a mismatch.
10. **The counselor line's file (the critic's gap).**
    `records/counselor/carried-landings.jsonl` is an append-only register
    exactly like `accepted-risk-register.jsonl`: listed in
    metasystem/internal/landing/registers.go's append-only registers so
    the workspace projection excludes it, tracked once it first exists,
    carried to origin by the next landing. Create it empty in this round
    so it is tracked from the start.

## B. The Go fixtures are real (HCL-C-41, HCL-C-43)

11. Every `TestHCL<nn><Name>` function drives the production path its
    design line names and asserts that oracle: the goal ones call `Carry`,
    `Carrying`, `Carried`, recovery and `done` against a fixture ledger;
    the landing ones call `observeCarried`, `CarryDebtAt`,
    `CarryConsumptionAt` and `ReadCarryStatus` with real trees; the command
    ones run the verbs. Delete the three shared shell helpers. A named
    function you cannot make real in this round is DELETED together with
    its name in the carry group's list and named under `deviations`; it
    is never left as a one-line shell. Priority inside this block: the
    HCL-21 match tests, TestHCL05WrongRefusalAsks, TestHCL05UnneededAsks,
    TestHCL33ExpiredReservationNotDebt (and its abandonment twin),
    TestHCL25ReplayMismatchRefused over all fourteen fields and the goal
    target, TestHCL02GenerationZeroRefusedInProduction,
    TestHCL03BaseJudgeBlindAsks, the TestHCL30 supersede tests,
    TestHCL06DoneRefusesOpenCarry, TestHCL27FormatRaiseAsksThenWrites.
12. `TestHCL03NoPendingAfterSlice2` reads `carry-status` from main.go's
    landing verb table, anchors `--carried)` to commit.sh's argument-loop
    case, and drives the negative table for real: for each entry point,
    a copy of the source with that entry point removed must FAIL the
    entry-point check, and the test fails if it does not.

## C. The beds drive their design lines (HCL-C-42, HCL-C-44)

13. Each carried land leg and goal-cli scenario drives what its design
    lines name, through land.sh's own reservation (the bed publishes no
    reservation itself). In this order, as many as fit: the two HCL-33
    two-seat legs (interleave; trailer-without-row as abandonment and as
    expiry: seat B asks carry-debt-unpaid naming A's word), HCL-28 peer
    debt seen, the HCL-38 ledger-path leg of decision 4, the HCL-36 row
    closed on an ask, the HCL-35 reruns of carried-crash (crash before the
    push resumes; crash after the push completes the record; counselor
    repaired), HCL-05-RED-BATTERY-LANDS (a group word with a red battery
    lands), HCL-05-CHAIN-PLUS-GROUP, the HCL-21 asks (two failures one
    name; each alone lands), HCL-05-UNNEEDED-ASKS, the superseded state,
    the cap and debt asks at the landing, the forward cases (second word,
    ledger move keeps the word, the rebase posture, lease held). Goal-cli:
    transfer, the in-flight and obligation debt asks at the word, the
    push-before-row supersede refusal, abandon, done refuses, recover
    completes, the confirmed leg, the critic discharge, accept-risk replay
    and changed-why. A leg that stays out of this round is named under
    `deviations`; a leg that exists asserts what its name says.
14. **HCL-C-44.** Before the push step land.sh (or commit.sh) prints the
    08 lines: the reservation opid and ledger tip, the judge with the live
    failure when base, the ordinary verdict, the testing result with its
    four lists, the obligation finding, the exception count after this
    one; `carried-fresh` asserts their presence and order in land.out.
    `goal carry`'s word lines after its transaction stand (the opid is
    minted inside it; coordinator's note).

## Not folded

HCL-C-47 (testing.json classified as behaviour) is intended and stays.

# Proof before you return

`go build ./... && go vet ./...`; `go test ./internal/... ./cmd/metasystem/...`
(name each sandbox-bound test); `bash scripts/agents/go-build.sh` then
`metasystem test check --root .`; `bash scripts/agents/land-fixtures.sh`,
`bash scripts/agents/goal-cli-fixtures.sh`,
`bash scripts/agents/dispatch-fixtures.sh` (the goal-cli bed is
sandbox-bound for you in two existing scenarios; say so); `bash
scripts/agents/go-gate.sh --fast`. The coordinator reruns the sandbox-bound
parts on the host.

# Return

`diffBoundary` and `files` are repository-root paths. Under `evidence`
every command with its observed result; under `deviations` every finding
or fixture left out, by id, with the reason. Return BEFORE the 120-minute
cap with blocks A and B green and as much of C as fits; what is left of C
is the next round's brief, so name it exactly. A return at 100 minutes
beats a timeout.
