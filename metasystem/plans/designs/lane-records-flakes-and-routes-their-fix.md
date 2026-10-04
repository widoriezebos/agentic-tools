# Design for lane-records-flakes-and-routes-their-fix

- Kind: design
- Id: 01M42XE4V9XAAD3W0E7YR0AX55
- Status: accepted
- Goals: lane-records-flakes-and-routes-their-fix

Accepted 2026-10-04 10:30 by the ui seat for the build: tier 2, one Astra round and one confirmation read (m1e's order); LF-01's residual accepted by Wido (below); his two answers are in the rulings section.

Step 1 only. Facts read at `2f1f9b1c6`, 2026-10-04; paths are relative to `metasystem/`.

## The rule

A red check gets at most one repeat per tree, only when the batch cannot have caused it. A failed test that then passes is recorded and routed to a fix goal (opened when none exists) before the tree is green.

## The rulings

- R-103-m1e, "a flaky test is fixed, never retried": the repeat only decides whose fault the red is; every pass after a fail reaches a fix goal.
- R-19: `docs/flake-registry.md` ("exactly ONE solo rerun") now runs in code, routing the fix at the first sighting, not the third.
- R-93-m1e ("a seat opens only the defect that blocks its claimed goal"): **recorded exception**, Wido 2026-10-04 (`decisions-for-wido.md`, with undo): the lane may open one tier-1 priority-1 fix goal per flaky unit, never a second. Also answered: the command names what failed; the entry's fix goal is the owner.

## 1. The command contract

The check is a local shell line (`cmd/metasystem/intent_landing_prove.go:114`) whose tests may run elsewhere (here a VM script): everything crosses in its environment and output.

- **In.** `LANDING_ONLY`: empty, or one unit to run alone, by repository path (Go: the package's folder); the line forwards it (`… | ssh VM suite "$LANDING_ONLY"`).
- **Out.** The last lines printed, tab-separated:

  ```
  LANDING-FAILED   UNIT   TEST TEST…   per failed unit; on a timeout, the tests running
  LANDING-LOAD     N.NN                load where the tests ran
  LANDING-CHECKED  COUNT               all asked ran; COUNT failed lines
  LANDING-NOT-RUN  REASON              instead: no test started (snapshot, build, disk floor)
  ```
- **Exit.** The script's own status: 0 only when every unit asked passed.
- **Fixtures.** The command removes its run's temporary folder after printing, also on a timeout or signal.

The lane reads them from the output it copies to the log. The report is complete when the exit is non-zero, `LANDING-CHECKED` is last and its count matches.

## 2. Which red may repeat

Decided inside `proveInWorktree` (`internal/landing/plain/prove.go:424`) before it removes the worktree; unit repeats run there too. Three kinds of red:

- **No test ran:** `LANDING-NOT-RUN` is last, or the lane's own process died, leaving `running.json` without a result (written as this red). One more whole check may run (skill case 7). Nothing failed, so nothing is recorded: an ordinary green.
- **Tests ran, report incomplete** (no `LANDING-CHECKED` last, a wrong count, or neither line): red, no repeat of any kind; today's culprit search.
- **Report complete:** below.

Contract and register are read from main, never the batch. The batch's changed paths (`git diff --name-only origin/main...COMMIT`) go through `testpolicy.Select` (`internal/testpolicy/select.go:99`: path to surface, then `dependsOn`); a unit is affected when a surface in `AffectedSurfaces` holds one of its files, and on no contract, an error or `Uncertainty`.

- **A unit affected:** red, no repeat; today's culprit search.
- **None affected, every failed test on its unit's open register entry:** each unit runs once more, alone.
- **None affected, otherwise** (a new test or unit, none named): one more whole check may run; skill case 3 gains "when `last_proof.repeat` is `allowed`, run `landing prove` once more".

## 3. The bound

A `results.jsonl` line gains `failed`, `load` and `repeat` (`allowed` or `started`); only a tree's first line is ever `allowed`, never an incomplete report's; a no-test-ran red has no `failed`. Under the lane lock, before a repeat's command starts, the lane appends the red with `repeat: started`; a crash leaves it so. `Run` (called directly by `landing prove --wait`, `intent_landing_prove.go:167`), `Start` and `Settled` read the tree's newest line under that lock: green returns without running; a red not `allowed` refuses: "this code failed its check and gets no other; give the goal that broke it back".

## 4. The record and the fix goal

`plans/goals/trunk-red.json`, class `pending-flake` (`internal/goal/trunkred.go:573`; holds no landing, `internal/steward/trunkred.go:69`), one open entry per unit (`flaky:UNIT`): `failures` holds the tests seen; a sighting both attempts, tree, log, load and the unit's surfaces.

One new operation, `goal.RecordFlake`, one transaction:

- adds the sighting and new tests;
- when the entry names no goal, opens `fix-flaky-UNIT` (tier 1, priority 1, unapproved) and names it on the entry;
- when its goal is concluded, reopens it through the existing reopen mutation and its guards (`internal/goal/verbs.go:3530-3547`, `3582-3590`), as an archived id never opens again (`:975-976`, `internal/goal/validate.go:280-288`); a guard's refusal (an abandoned or split goal) fails the record;
- appends to the fix goal's next step "Flaky: UNIT (TESTS) failed in the lane's check of COMMIT on DATE, passed when repeated (alone | whole); load N; log PATH", leaving claim, state and approval.

It publishes through `goal.Publish` (`internal/goal/txn.go:662`) on the installation's ledger endpoint. Only a confirmed `Outcome` counts (rejected and expired return a nil error, `:891`, `:975`); without it, no green. The record moves main, so `landing push` refuses and skill case 5 merges the new main; that tree differs only in ledger files and inherits the green, repeating its reason.

## 5. What a person sees

`landing status` (`cmd/metasystem/intent_landing.go:312`):

`proven green (metasystem/internal/proofrun failed once and passed when run again alone; seen 3 times; goal fix-flaky-metasystem-internal-proofrun fixes it)`

## Units

1. `internal/landing/plain`, sections 1-3 and seams: 220 code, 380 test.
2. `internal/goal`, `RecordFlake`: 150 code, 250 test.
3. `cmd/metasystem`, `internal/testpolicy`, seams, reason, skill, `docs/flake-registry.md`: 150 code, 220 test, 30 prose.

By hand (m1e), the VM script and configured line: 30 shell.

## Tests

Written first, seen red; `ProveSeams.Now` and `.Git` stubbed, the command a stub printing the lines.

Unit 1: a `LANDING-NOT-RUN` red and a dead lane process (a red line): one whole check, nothing recorded. An incomplete report, each form: every repeat refused. An affected unit: `landing prove` refused. Known tests, unaffected: each unit once with `LANDING_ONLY`, in the worktree, one record, green. A new test: no unit repeat. A failing repeat or unconfirmed record: red. `started` precedes the repeat; killed mid-repeat, every entry refuses; `Run` returns an existing green unrun. The `--json` layout audit follows.

Unit 2: another seat's claimed goal and a parked one gain the line, keeping claim, state and approval; one goal opens for an entry naming none, never a second; a recurrence after the generated fix goal concluded reopens it, one goal still; an abandoned one fails; rejected and expired fail; an ordinary machine open stays refused.

Unit 3: any file makes a surface hold a unit; `dependsOn` consumers and the fallback count; contract and register come from main; the inherited green repeats the reason; the plain-words audits pass.

## Later, when it hurts

- The check script as a declared file of the landing checkout.
- Seats' round checks record flakes; "passes repeatedly on its own"; repeating one test.
- A flake in a unit its batch can affect: today the batch's red.
- No repeat for a unit seen often (`known-flake`); closing an entry after its fix (`countFlakePass`).
- The 787 leaked folders and the five tests: fix goals' work.

## Accepted risk

LF-01, accepted by Wido 2026-10-04 (through m1e, option a): the contract's dependency graph is not complete (a census change can break `internal/proofrun` while `testing.json` says its surface does not depend on it), so a branch that changes code a flaky unit uses without the contract declaring it, and makes one of that unit's registered flaky tests fail in the full check but pass alone, lands green. It is recorded and routed: the occurrence lands on the flake's fix goal with the commit. Goal `lane-reproves-only-what-a-change-can-affect` (m1f) makes the graph trustworthy.

## Astra

| Finding | Round 1 | Confirmation read |
|---|---|---|
| LF-01 | accepted: 2 | not closed; residual accepted by Wido (Accepted risk) |
| LF-02 | accepted: 2, 3 | folded: 1-3, tests |
| LF-03 to LF-06 | accepted: 3; 4; The rulings, 4; 1, 2 | closed |
| LF-07 | | folded: 4, tests |
