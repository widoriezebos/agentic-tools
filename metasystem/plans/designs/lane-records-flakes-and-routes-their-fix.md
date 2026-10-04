# Design for lane-records-flakes-and-routes-their-fix

- Kind: design
- Id: 01M42XE4V9XAAD3W0E7YR0AX55
- Status: draft
- Goals: lane-records-flakes-and-routes-their-fix

Step 1 only. Facts read at `42d76a1d0`, 2026-10-04; paths are relative to `metasystem/`.

## The rule

A red check in the lane gets at most one repeat per tree, and no repeat ends green unrecorded. When the check names what failed and every failed unit (here a Go package) is a known flaky one, the lane runs each once more, alone, on the same tree. All pass: it records the occurrence, adds a line to the goal that fixes the flake, then writes green; otherwise red, as today. An unknown failure is today's path; when the agent then repeats that tree's whole check and it ends green, the lane records each unit that failed first as newly flaky. No tree is checked a third time.

## The rulings

- R-103-m1e, "a flaky test is fixed, never retried": the repeat is not the remedy. It decides whose fault the red is, and every pass after a fail leaves a record and a line on a fix goal.
- R-19 calls the flake protocol "the working model": "A listed leg earns exactly ONE solo rerun", "An unlisted leg gets no rerun benefit of the doubt" (`docs/flake-registry.md`). Step 1 is that protocol run by code, except that the fix is routed at the first sighting (the approved intent), not the third.
- R-104-m1e, R-35-m3: each record carries the load, so a load-shaped failure reads as the test's defect.
- R-93-m1e conflicts with the intent on who opens a goal: question 1.

## 1. Recognition is in Go, and the check says what failed

Go cannot learn what failed from the log: it is free text from a local command (`cmd/metasystem/intent_landing_prove.go:25`), passing tests print failure-shaped lines into it, and repeating one unit proves the tree only when nothing else failed. Only the command knows that. It gains two optional parts:

- `LANDING_FAILED` names a file. The command writes it last and whole, one failed unit per line (optionally a tab and the failed tests), only when everything else ran and passed.
- `LANDING_ONLY=UNIT`: the same command runs only that unit.

No file is today's red. Nothing in Go knows a language; the agent decides only whether to repeat a whole check once (flow step 5).

## 2. The register

`plans/goals/trunk-red.json`, through `goal.RecordTrunkRed` (`internal/goal/trunkred.go:534`), class `pending-flake` (`:573-576` already demands both runs on one tree). One entry per unit (identity `flaky:UNIT`). A sighting holds the red attempt, commit, tree, log and time; `sample` the failed tests and host load (`internal/hostload`); `rerun` the repeat's attempt and log. `incident claim I --goal G` sets `fixGoal`. Such entries hold no landing (`internal/steward/trunkred.go:69`).

The lane's process writes it as a goal-ledger act before it writes green; a ledger path (`internal/landing/plain/prove.go:320`) needs no new check.

Not `memory/flake-registry.md` (no code can safely append to a hand table; it gets a closing line pointing here), not a lane-folder file (seats must read it).

## 3. The flow on a red

In `plain.Run` (`prove.go:261`), after a non-zero exit:

1. Read `LANDING_FAILED` into the result (`failed`). None: red as today.
2. On the tree's first check, when every unit is an open flake entry in the checked commit's register: run the command once per unit with `LANDING_ONLY`, same worktree, same log.
3. All exit 0: record, route, write green with the reason. No record, no green.
4. Otherwise red; `failed` stays, so the agent returns the culprit with it.
5. A whole second check ending green after a red that named units: each unit is recorded and routed. Skill case 3 gains one sentence: when no waiting branch touches the failed units' code, run `landing prove` once more.
6. `Start` and `Run` refuse a tree with two reds, or one red that repeated a unit: "this code failed its check twice; give the goal that broke it back".

A timed-out unit gets the repeat like any other: the lane cannot tell a hang, the command's own limit (25 minutes here) bounds it, and returning the batch costs its seats more.

## 4. Routing the fix

An entry with an open fix goal: the lane appends one line to that goal's next step, as `goal edit G --next-append` does (it admits a machine): "Flaky: UNIT (TESTS) failed in the lane's check of COMMIT on DATE and passed when run again alone; load N; log PATH; seen N times." One entry per unit, so one goal per flake, extended on every sighting, never a second.

No open fix goal: step 1 opens nothing (R-93). The status line says nobody fixes it yet; `incident claim` names the goal.

## 5. What a person sees

The last check's reason in `landing status` (`cmd/metasystem/intent_landing.go:312`) becomes:

`proven green (internal/proofrun failed once and passed when run again alone; seen 3 times; goal fix-proofrun-race fixes it)`

or it ends "nobody fixes it yet".

## Units

| Unit | Lines |
|---|---|
| 1. `internal/landing/plain`: failed list, repeat, two-check bound; seams for known units and the record | 120 code, 220 test |
| 2. `cmd/metasystem`: the seams on the register and the next step; the reason; skill; `docs/flake-registry.md` | 130 code, 200 test, 30 prose |
| By hand (m1e, no repository): the VM script writes the list and honours `LANDING_ONLY` | 20 shell |
| 3. If question 1 is yes: the lane opens the goal | 70 code, 100 test |

## Tests

Written first, seen red; clock `ProveSeams.Now`, Git stubbed by `ProveSeams.Git`.

Unit 1: a red with a list keeps the units; without one nothing is repeated; known units passing alone give one record, then green; a failed record gives red; a unit failing again, or an unknown unit, gives red and no record; each repeat runs once with `LANDING_ONLY`; a green second check records the first check's units and repeats nothing; a third is refused. The `--json` layout audit follows.

Unit 2: only open flake entries are known units; a new unit opens one entry, a known one gains a sighting and no second entry; a fix goal gains exactly one line; without an open goal nothing is appended and the reason says so; messages pass the plain-words audits.

## Open questions for Wido

1. **May the lane open a goal by itself when nobody fixes a flaky test yet?** The approved intent opens "a new tier-1 goal when none owns it". R-93-m1e lets a machine open only a goal that blocks its own; the code refuses anything else (`internal/goal/verbs.go:838-886`). Recommended: yes, as a recorded exception to R-93: one goal per flaky unit, tier 1, priority 1, never a second because the register entry names it. Otherwise each new flake waits for you while the lane keeps repeating it. If no: no unit 3; each landing's channel message names the flake nobody fixes.
2. **May the project's own check command, not the testing contract, name what failed?** The intent says "the test identity and the owning surface come from the testing contract". The lane's check is a local command that never runs the contract, and the contract names no owner (`internal/testpolicy/contract.go:70-79`). Recommended: yes; the owner is the fix goal on the register entry.

## Later, when it hurts

- Seats' own round checks record flakes the same way (the third flake hit a seat).
- "Passes repeatedly on its own" as a second way to recognise a flake.
- Stop repeating a unit seen many times a day: one that always fails in the full check but passes alone keeps landing with a growing count (the register's `known-flake` allowance exists for this).
- Close an entry after its fix and three green checks (`countFlakePass`, unwired).
- An owner per surface in the testing contract.
- Repeat one test, not one package; cap the units repeated per check.
- The 787 leftover fixture folders and the five tests: fixes this mechanism routes.
