# Design for lane-records-flakes-and-routes-their-fix

- Kind: design
- Id: 01M42XE4V9XAAD3W0E7YR0AX55
- Status: draft
- Goals: lane-records-flakes-and-routes-their-fix

Step 1 only. Facts read at `2f1f9b1c6`, 2026-10-04; paths are relative to `metasystem/`.

## The rule

A red check gets at most one repeat per tree, only when the batch cannot have caused it. A passing repeat is recorded and routed to a fix goal (opened when none exists) before the tree is green: no repeat ends green unrecorded.

## The rulings

- R-103-m1e, "a flaky test is fixed, never retried": the repeat only decides whose fault the red is; every pass after a fail reaches a fix goal.
- R-19: `docs/flake-registry.md` ("exactly ONE solo rerun") now runs in code, routing the fix at the first sighting, not the third (the intent).
- R-93-m1e ("a seat opens only the defect that blocks its claimed goal"): **recorded exception**, Wido 2026-10-04 through m1e (`decisions-for-wido.md`, with undo): the lane may open one tier-1 priority-1 fix goal per flaky unit, never a second; "a goal is a request to a person, not a decision over one". If overturned, the lane only says "nobody fixes it yet". Also answered: the command names what failed; the entry's fix goal is the owner.

## 1. The command contract

The check is a local shell line (`cmd/metasystem/intent_landing_prove.go:114`) whose tests may run elsewhere (here a VM script): everything crosses in its environment and output. m1e applies this by hand:

- **In.** `LANDING_ONLY`: empty, or one unit to run alone, named by its repository path (Go: the package's folder); the line forwards it to the script (`… | ssh VM suite "$LANDING_ONLY"`).
- **Out.** The last lines printed, tab-separated:

  ```
  LANDING-FAILED   UNIT   TEST TEST…   per failed unit; on a timeout, the tests running
  LANDING-LOAD     N.NN                load where the tests ran
  LANDING-CHECKED  COUNT               all asked ran; COUNT failed lines
  ```
- **Exit.** The script's own status: 0 only when every unit asked passed.
- **Fixtures.** The command removes its run's temporary folder after printing these lines, also on a timeout or signal.

The lane reads them from the output it copies to the log (no file path, no Mac load). The list is complete when the exit is non-zero, `LANDING-CHECKED` is last and its count matches.

## 2. Which red may repeat

Decided inside `proveInWorktree` (`internal/landing/plain/prove.go:424`) before it removes the worktree; unit repeats run there too.

Contract and register are read from main, never the batch. The batch's changed paths (`git diff --name-only origin/main...COMMIT`) go through `testpolicy.Select` (`internal/testpolicy/select.go:99`: path to surface, then `dependsOn`); a unit is affected when a surface in `AffectedSurfaces` holds one of its files, and on no contract, an error or `Uncertainty`. `failuresBelongToGoal` (`internal/goal/branch/red.go:122`) takes contract group ids, not these units: not reused.

- **No complete list** (the check itself broke; a dead `running.json` without a result is written as this red): one more whole check may run (skill case 7), as unit `the-check-itself`.
- **A unit affected:** red, no repeat; today's culprit search.
- **None affected, every failed test on its unit's open register entry:** each unit runs once more, alone.
- **None affected, otherwise** (a new test or unit, none named): one more whole check may run; skill case 3 gains "when `last_proof.repeat` is `allowed`, run `landing prove` once more".

## 3. The bound

A `results.jsonl` line gains `failed`, `load` and `repeat` (`allowed` or `started`); only a tree's first line is ever `allowed`. Under the lane lock, before a repeat's command starts, the lane appends the red with `repeat: started`; a crash leaves it so. `Run` (called directly by `landing prove --wait`, `intent_landing_prove.go:167`), `Start` and `Settled` read the tree's newest line under that lock: green returns without running; a red not `allowed` refuses: "this code failed its check and gets no other; give the goal that broke it back".

## 4. The record and the fix goal

`plans/goals/trunk-red.json`, class `pending-flake` (`internal/goal/trunkred.go:573`; holds no landing, `internal/steward/trunkred.go:69`), one open entry per unit (`flaky:UNIT`): `failures` holds the tests seen; a sighting both attempts, tree, log, load and the unit's surfaces.

One new operation, `goal.RecordFlake`, one transaction:

- adds the sighting and new tests;
- appends to the fix goal's next step "Flaky: UNIT (TESTS) failed in the lane's check of COMMIT on DATE, passed when repeated (alone | whole); load N; log PATH", leaving claim, state and approval (`goal edit` refuses a machine there, `internal/goal/verbs.go:3786`);
- when the entry names no goal, or a concluded one, opens `fix-flaky-UNIT` (tier 1, priority 1, unapproved) and names it on the entry.

It publishes through `goal.Publish` (`internal/goal/txn.go:662`) on the installation's ledger endpoint (here origin's main). Only a confirmed `Outcome` counts (rejected and expired return a nil error, `:891`, `:975`); without it, no green. The record moves main, so `landing push` refuses and skill case 5 merges the new main; that tree differs only in ledger files and inherits the green, repeating its reason.

## 5. What a person sees

`landing status` (`cmd/metasystem/intent_landing.go:312`):

`proven green (metasystem/internal/proofrun failed once and passed when run again alone; seen 3 times; goal fix-flaky-metasystem-internal-proofrun fixes it)`

## Units

| Unit | Lines |
|---|---|
| 1. `internal/landing/plain`: sections 1-3, seams | 220 code, 380 test |
| 2. `internal/goal`: `RecordFlake` | 150 code, 250 test |
| 3. `cmd/metasystem`, `internal/testpolicy`: seams, reason, skill, `docs/flake-registry.md` | 150 code, 220 test, 30 prose |
| By hand (m1e): VM script, configured line | 30 shell |

## Tests

Written first, seen red; `ProveSeams.Now` and `.Git` stubbed, the command a stub printing the lines.

Unit 1: no lines, a wrong count, or lines not last: `allowed`; a green second check leaves one record under `the-check-itself`. An affected unit: red, `landing prove` refused. Known tests, unaffected: each unit once with `LANDING_ONLY`, in the worktree, one record, green. A new test: no unit repeat. A failing repeat or unconfirmed record: red. `started` precedes the repeat; killed mid-repeat, every entry refuses; `Run` returns an existing green unrun; a dead check becomes a red line. The `--json` layout audit follows.

Unit 2: another seat's claimed goal and a parked one gain the line, keeping claim, state and approval; one goal opens for an entry naming none or a concluded one, never a second; rejected and expired fail; an ordinary machine open stays refused.

Unit 3: any file makes a surface hold a unit; `dependsOn` consumers and the fallback count; contract and register come from main; the inherited green repeats the reason; the plain-words audits pass.

## Later, when it hurts

- The check script as a declared file of the landing checkout.
- Seats' round checks record flakes; "passes repeatedly on its own"; repeating one test.
- A flake in a unit its batch can affect: today the batch's red.
- No repeat for a unit seen often (`known-flake`); closing an entry after its fix (`countFlakePass`).
- The 787 leaked folders and the five tests: fix goals' work.

## Astra round 1

| Finding | Disposition | Fold |
|---|---|---|
| LF-01 | accepted | 2 |
| LF-02 | accepted | 2, 3 |
| LF-03 | accepted | 3 |
| LF-04 | accepted | 4 |
| LF-05 | accepted | The rulings, 4 |
| LF-06 | accepted | 1, 2 |
