# Design for a-hand-in-stands-on-main

- Kind: design
- Id: 01M6AXEFKYM6M2CE1MVFYGWQMN
- Status: accepted
- Goals: a-hand-in-stands-on-main
- Critique: design review round 1 (Astra, design-critic-a2839c682869b4ba3dc55866): 3 material accepted, 1 non-material accepted; folded in revision 2

Draft revision 2, written 2026-11-01 around 11:40 CET by the engine's design author (Claude Opus 5.5) from the steward's revision request of 2026-11-01 after design review round 1 ("Revision request: a-hand-in-stands-on-main, revision 2 after design review round 1", a1, written in Wido's name; the steward's dispositions are in `agentic-tools-evidence/lane-test-20261009/designs/a-hand-in-design-r1-dispositions.md`, outside the repository). Revision 2 folds the four accepted findings. Fold 1 puts the move read into D3, D2, D5, section 1.2, T1, the unit table and Moved effects. Fold 2 makes the move's reach a package-level closure, in D3 and T1. Fold 3 adds section 8, Moved effects. Fold 4 puts the round's read into D4, U3 and T3. Q1 is answered (section 4), and D4, section 1.2, U3 and T3 follow the answer. Three changes go beyond the folds. Every backticked path is now a full repository path. The angle-bracket placeholders are gone. D9 names a-red-is-classified-once's accepted design. Code was read again at main `9e26670fca`, and origin/main `9f36e302ed` changes nothing under `metasystem/internal/` or `metasystem/cmd/` since then. `metasystem design review --check-only`, run from `metasystem/` on this draft, ends with the line "moved-effects: inventory=present rows=12 problems=0".

Revision 1 was written 2026-11-01 around 08:00 CET from the steward's request (`/Users/wido/agentic-tools-evidence/lane-test-20261009/designs/a-hand-in-stands-on-main-request.md`), which carries Wido's order. Its code was read at main `9e26670fc` in the m1e checkout. Lane records of this morning were also read: the read report and return reason named in the request, and the ui seat's message to m1e at 07:26. The page does not take the request's recommended shape, a merge of main into the goal branch. It takes the existing rebase instead. D2 gives the reason, and the request allowed this "if it shows the attestations survive". Unit checks measured for D3: the 37 unit checks recorded under `~/.metasystem/unit` in the last ten days ran 4 to 91 minutes each, with a median of 27.

Accepted on 2026-11-01 by Wido's act at his enrolled pane (steward decision 114, in his name under decisions 102, 111 and 115) after design review round 1 (Astra, design-critic-a2839c682869b4ba3dc55866: 3 material and 1 non-material, all accepted and folded in this revision). The engine's check of this revision: `metasystem design review plans/designs/a-hand-in-stands-on-main.md --check-only` → "moved-effects: inventory=present rows=12 problems=0". Q1 is answered in section 4 (decision 115). U3 builds after a-red-is-classified-once's classifier is on main.

## Wido's words (the authority)

- 2026-11-01 07:3x: "Is there anything that needs a redesign, we should not patch if something needs a deeper improvement". On the steward's answer naming this redesign: "do as proposed". The goal's approved intent says the branch is brought "onto main tip (rebase or merge of main)", and that the seat "reads that merge at the seat".
- 2026-10-05, from `lane-lands-finished-goals`: "Landing is responsible for making sure the goal's test suite is green in the most efficient way, and can fix merge conflicts so it can do that. Landing exists only because the test suite is expensive and it can batch goal tests." And: "You only integrate when you are done with a goal."
- 2026-10-03, from `conflicts-resolve-unattended`: "The goal is unattended functionality."
- Standing rules: "landing order: cheap first" (2026-10-08); "land fast, fix non-breaking findings forward" (2026-09-17); "a red on main outranks every landing" (2026-09-16); "the lane acts as the seat; never cut his principle" (2026-10-10); humans are never denied a verb (2026-09-27); verb naming is his call (2026-09-28); reads answer their own questions, in his name, and he overrules (2026-10-24).

## Step 1

What is true after each unit:

- **After U1 alone.** When `work rebase G` carries a unit's read onto main, that unit's check runs only the tests that main's move can affect, not the unit's whole test scope. Main's move is measured by packages, through the import graph. Where main's move reaches the unit, the unit's reader reads the rebased commit again before its read is carried. `work rebase G` is the verb the lane names for every conflict return, and the one the ui seat ran this morning. Each such check runs as host work that the lane's proof hold can see.
- **After U2.** `work land G` moves a goal that is behind main by code onto main before handing it in. It carries each unchanged unit's read with that narrowed check, and with the move read where main's move reaches the unit. A conflict that a builder can resolve goes to the existing resolve round. The lane then receives a tip that contains main.
- **After U3.** A unit that goes red on main, or whose move read has a material finding, is corrected by a round of that unit at the seat, not by a lane return. A judgement conflict goes to the same kind of round. Each such round runs the unit's read, and that read becomes the unit's new read.
- **After U4.** Each lane return says which case caused it. A conflict return is fixed by `work land G` again, not by a separate `work rebase`.

## 0. The gaps, and what this page does with each

| Gap | Observed today | This page |
|---|---|---|
| 1. A goal is handed in on a base that is days old | Each unit's base is the branch head when the unit started (`metasystem/cmd/metasystem/intent_work.go:1204-1208`), and `work land` never moves the branch (lane-lands-finished-goals D2). a-round-survives-a-restart was handed in at 06:53 on a base two days behind main. The lane merged it onto `1ddc0a467`, resolved two files, found a semantic conflict in `metasystem/internal/lease/sweep.go` in its read, and returned it at 07:22 | D1, D2 |
| 2. The fix for a conflict return is a separate verb | The lane's reason says "run metasystem work rebase G" (`metasystem/internal/landing/plain/resolve.go:272`), and the seat's `work land` names the same command (`metasystem/cmd/metasystem/landing_plain.go:115-121`) | D8 |
| 3. The seat asks a person about conflicts the lane resolves on its own | `work rebase` refuses on a judgement conflict and asks a person (`metasystem/internal/goal/branch/rebase_conflict.go:150-157`, judgement as defined in `metasystem/internal/conflict/conflict.go:35-75`). The lane resolves every class "as the seat" (`metasystem/internal/landing/plain/resolve.go:197-206`). This morning the ui seat's `work rebase` asked Wido to choose for `metasystem/internal/dispatch/unit_launch.go` and `metasystem/internal/steward/seat_start.go`. The lane's job had already resolved both, and its read found them correct | D4, Q1 (answered) |
| 4. A red found while carrying a read has no way forward | A unit's carry check that fails ends `work rebase` with an error and a rerun command (`metasystem/internal/goal/branch/rebase.go:370-387`, `metasystem/cmd/metasystem/intent_carry_check.go:127-136`). The seat planned a new unit at the tip for `metasystem/internal/lease/sweep.go`. That cannot give U1 back its read, because U1's own rebased commit stays red | D5 |
| 5. Each carried unit re-runs its whole check | The carry check is the unit's check with its parent as the base (`metasystem/cmd/metasystem/intent_carry_check.go:46`, `:144`). Measured: 4 to 91 minutes per check, median 27. A four-unit goal pays four of these | D3 |
| 6. Carry checks are invisible to the lane | Carry checks run inside the calling process (`metasystem/cmd/metasystem/intent_carry_check.go:144`). The lane's hold counts only launch records of kind build or proof (`metasystem/internal/launch/hostcapacity.go:33-57`). A round's own check is a host-gated proof launch (`metasystem/internal/launch/unit_run.go:613`) | D3, section 8 |
| 7. A main move that adds an audit is found only by the lane | the-steward-sees was returned on 2026-10-31 for a static audit (finding 459) | Covered once a-unit-check-runs-the-audits puts audit tests in each unit's scope: D3's narrowed check then includes an audit that main added (D9) |
| 8. A carried read is never read again | The carry checks an unchanged unit but runs no reader (`metasystem/internal/goal/branch/rebase.go:348-391`). Main's move can change what an unchanged patch means: main's early return in `metasystem/internal/lease/sweep.go` made the goal's settle branch dead code, and the lane's merge read found it | D3 (the move read) |

## 1. What the person sees

### 1.1 Today

- `work land G` answers "handed to the lane" for a branch that is days behind main.
- About half an hour later, the lane returns it for a conflict that already existed when the last unit was built. Until a-lane-returns-on-its-own-proof lands, a person has to make that return.
- `work land G` then says "was returned ... run: metasystem work rebase G".
- `work rebase` asks the person to judge lines the lane had already merged. It can then stop with an error while carrying a read whose check is red on main.

### 1.2 After

Text is summarised by default. The details below each line appear under `--verbose`.

- **Behind main by code:** "goal G is N commits behind main (K of them change code); moving it onto main before the hand-in". When main moved by records only, the hand-in line is the same as today. Under `--verbose`: "main moved by N record commits; the lane folds them".
- **Moved cleanly:** "goal G moved onto main abc1234; its N builds keep their reads, each checked again on main (T tests that main's change can reach) and read again where main's change reaches it; handed to the lane". Under `--verbose`, one line per build, for example "U2: main's move reaches none of this unit's code; its read is carried with the check".
- **Conflict a builder can resolve:** "build U1 conflicts with main in internal/steward/seat_start.go; round R of U1 resolves it". When the round ends, the hand-in line adds "U1 read again in its round".
- **Red on main (U3):** "build U1 is red on main: internal/lease TestATakeoverLeavesAUnitLaunchAlone; main's c73c88124 changed internal/lease/sweep.go; round R of U1 fixes it on main".
- **Main's move changes what a build means (U3):** "build U1 was read again on main and has a material finding: main's change in internal/lease/sweep.go returns before U1's settle branch; round R of U1 corrects it on main". Before U3: "build U1 was read again on main and has a material finding; nothing was handed in", and the second line is `metasystem work review G --work U1`.
- **Judgement conflict (U3):** "main and this goal changed the same lines in P; round R of U1 resolves it and keeps both sides whole". In U2, before U3 lands: "main and this goal changed the same lines in P; the lane resolves it as before; handed to the lane as it is".
- **Main moved again during the step:** the hand-in line adds "main moved again by M commits while this was checked; the lane merges them".
- **Refusal:** "goal G cannot move onto main: REASON; nothing was handed in", where REASON says what stopped the step (D6). The second line is one command. When it is a person's way past the refusal, it is `metasystem work land G --exception GOAL_BEHIND_MAIN --reason TEXT --by NAME` (D7).
- **The Stop path's automatic hand-in:** "LANDING WAITS G: K code commits behind main; the session's next metasystem work land G moves it onto main first".
- **A lane return (U4):** the return names its case, for example "it conflicts with main in P: handed in on main abc1234, and main moved by code to def5678 since; run metasystem work land G". A conflict with another goal in the same batch stays held, with today's reason.

## 2. Problem (what the code does today)

- **The rebase already exists.** `work rebase G` is "the shared branch operation for rebase and land" (`metasystem/cmd/metasystem/intent_work_rebase.go:70-180`). It does five things:
  - It replays the branch onto origin's main (`metasystem/internal/goal/branch/rebase.go:436-470`).
  - Generated paths are rebuilt. A conflict a builder can resolve gets a resolve round, which is a counted round of that unit's run, in the rebase's scratch worktree, with an engine-written brief of the conflicting hunks and any resolution the lane wrote (`metasystem/cmd/metasystem/intent_work_rebase.go:188-236`, `metasystem/internal/goal/branch/rebase_conflict.go:158-181`, `metasystem/internal/launch/unit_revise.go:136-143`, `:469-501`). A judgement conflict asks a person (`metasystem/cmd/metasystem/intent_work_rebase.go:291-342`).
  - It carries the read of every unit whose change digest is unchanged. It checks that unit's rebased commit, then writes a new read commit that names the old one (`metasystem/internal/goal/branch/rebase.go:306-410`). No reader runs (`metasystem/internal/goal/branch/rebase.go:348-391`). A unit whose digest changed is listed as needing a read.
  - It publishes the branch with a lease and writes a "rebase" history line.
  - A repeat continues from the moved local branch (`metasystem/internal/goal/branch/rebase.go:97-275`).
- **A resolve round runs no read.** It finishes after its proof (`metasystem/internal/launch/unit_run.go:652-659`), and its test fails on a read launch (`metasystem/internal/launch/unit_revise_test.go:57-60`). Promotion needs exactly one completed, counting read (`metasystem/cmd/metasystem/intent_unit_review.go:651-663`). The unit then needs `work review G --work U`.
- **`work land` stopped calling the rebase on 2026-10-06** (lane-lands-finished-goals Decision 2, `metasystem/plans/designs/lane-lands-finished-goals.md:121-145`). The lane's merge became the integration instead, and that merge does not see a semantic conflict until the merge read or the proof.
- **The branch model is linear.** The range reader refuses a commit with two parents and names `work rebase` (`metasystem/internal/goal/branch/range.go:288-293`). The rebase linearises a branch that contains a merge (`metasystem/internal/goal/branch/rebase.go:173-183`).
- **Read attestations are per commit.** They live at `metasystem/records/reads/GOAL/COMMIT.json` on the branch (`readPath`, `metasystem/internal/goal/branch/range.go:211`). A carried read is a new attestation for the rebased commit, with a passing check of that commit's tree.
- **Impact selects tests in two ways.** `test impact` selects a test by the declarations, fields and literals it names (`metasystem/internal/landing/batch/goadapter/impact.go:133-164`, `:271-276`). The reverse-dependents reader works by packages, as a fixed point over the import graph (`metasystem/internal/landing/batch/goadapter/unitgate.go:205-235`).
- **The hand-in reads origin.** `work land` reads the goal tip from origin (`metasystem/cmd/metasystem/intent_delivery.go:425-451`). The lane fetches `refs/heads/goal/G` when it selects a hand-in. A tip that was rewritten under a waiting hand-in becomes "not on origin" (`metasystem/internal/landing/plain/handin_fetch.go:57-66`).
- **The Stop path runs `work land` in the Stop process.** It runs `work land G --json` for a landing that is due (`metasystem/cmd/metasystem/holder_step.go:27-48`, `metasystem/cmd/metasystem/goal.go:679`).
- **Two classifiers exist, for two questions.** The lane decides "records only, so no executable input changed" with `recordsOnlyPaths` (`metasystem/internal/landing/plain/scope.go:271-292`, pathclass record and ledger). The rebase decides "ledger only, so the lane's ledger merge drivers take it" with `goal.IsLedgerFile` (`metasystem/internal/goal/branch/rebase.go:410-433`, `metasystem/internal/goal/validate.go:77`).

## 3. Decisions

### D1. When `work land` moves the branch onto main (U2)

**When it runs.** On a whole hand-in through the lane, after the gates that do not depend on main and before the queue line is written. Those gates are: every declared unit built and read (`handLandingSubject`, `metasystem/cmd/metasystem/intent_delivery.go:2186`), the incident hold (`:2223`) and landing admission (`:2226`). `work land` reads main's tip. It finds the fork point and the paths main changed since then. If any of those paths is not records by the lane's classifier, the branch is behind main by code, and `work land` runs the step: the existing rebase operation with D2 to D5. If none is, the hand-in is unchanged; the lane folds a records-only move as it does today.

**After the step.** `work land` reads the branch from origin again and repeats the read gate on the new tip. The old code did this re-read before 2026-10-06 (`metasystem/cmd/metasystem/intent_delivery.go:1995-2002` of that date). It then hands in the new tip. The queue line records the main tip the hand-in stood on.

**When main moves again during the step.** The hand-in goes ahead. There is no second pass. The lane's own merge, resolution job and merge read stay as the second line.

**When the step does not run.** The branch is then handed in as it is, as today:

- a records hand-in, `--through`, `--again`;
- an exception route, which binds the tip a person saw;
- a computer without a lane (the hand route keeps its contract and refusals);
- an earlier hand-in of the goal that still waits in the lane. Rewriting the branch would strand that hand-in's commit.

**The Stop path.** The Stop path's automatic `work land` only reports a branch that is behind by code ("LANDING WAITS"). It never starts the step, because the step runs checks, reads and possibly rounds, and a Stop must not block.

**Rejected:**

- **Checking on main in every round** (the request's option d). It costs a check at every records move, and units are built on their own base on purpose ("you only integrate when you are done with a goal").
- **Chasing main until it stands still.** That has no bound. The lane is the second line.
- **The rebase's ledger-only test as the trigger.** A non-ledger records move would then start a rebase.

### D2. Rebase through the existing operation, not a merge (U2)

The step is `rebaseGoal` (`metasystem/cmd/metasystem/intent_work_rebase.go:71`), called from `work land`.

**The read attestations survive.**

- A unit whose change is unchanged keeps a read: a new attestation for its rebased commit, made by the existing carry, with a check of that commit's tree on main (D3). Where main's move reaches the unit, the unit's reader reads the rebased commit again, and the attestation names that read (D3's move read). Where main's move reaches nothing of the unit, the reader does not run again, because the code it read, and everything that code depends on, is the same.
- A unit whose change differs after the move, because a conflict was resolved or a red or a finding was corrected, gets the read of the round that changed it (D4, D5). Otherwise its hand-in stops with `work review G --work U`.

**What this page replaces.** It replaces lane-lands-finished-goals D2 ("work land never rebases") for a goal that is behind main by code. The authority is the approved intent's own words. What D2 protected still holds:

- Only the session holding the claim rewrites the branch (`branch.CheckHolder`).
- A branch is never rewritten under a waiting hand-in (D1).
- Every rewritten unit keeps a read.

**Rejected: a merge commit of main into the goal branch.** It would need:

- a fourth commit kind that the range reader accepts (`metasystem/internal/goal/branch/range.go:288-293`), and its handling in status, reads, sweep, drop, attestation and the hand route's per-unit replay;
- a new resolution path and a resolution read for one merge.

A later `work rebase` would then drop the merge and its resolution. The merge's one advantage, one check instead of one per unit, is matched by D3's narrowing.

### D3. The check on main runs only what main's move can reach, and the move is read where it reaches the unit (U1)

**Main's move.** The move is the difference between the old parent and the new parent of the carried commit. That difference is main's move, plus any change an earlier unit's resolution made.

**The move's reach, by packages.** The reach is the reverse-dependency closure, at package level, of the packages the move changed. It holds those packages and every package that imports one of them, directly or through others, with test files' imports included. It is computed from the repository's own import graph, the same reading every brief's "reverse dependents run whole" rule relies on. The implementation uses the existing reader `reverseDependents` (`metasystem/internal/landing/batch/goadapter/unitgate.go:205-235`), a fixed point over the import graph that `packageImports` reads from every Go file of the tree, tests included (`metasystem/internal/landing/batch/goadapter/unitgate.go:268-327`). The reach is never narrowed by symbol. Each path of the move counts by its package:

- A Go file counts by its directory's package, as `changedWorkingGoPackages` maps it (`metasystem/internal/landing/batch/goadapter/unitgate.go:164-203`).
- Any other path counts by its nearest enclosing package, as `unitImpact` already places it (`metasystem/internal/landing/batch/goadapter/impact.go:115-130`).
- A path that the lane's classifier calls records reaches nothing (D1's classifier).
- `go.mod`, `go.sum`, or a path with no enclosing package, reaches the whole module.

**What the narrowed check runs.** It runs the intersection, package by package, of two sets:

- the unit's selection, as today, with its parent as the base;
- the move's reach.

Inside a package that is in both, every test runs, whatever the unit's selection named.

**Why that is enough.** A test whose package is outside the move's reach imports nothing that main's move changed. It therefore has the same outcome on the new tree as on the old one, where the unit's earlier read already required its check to pass. A test outside the unit's selection is not in today's whole carry check either, so the narrowed check is never weaker than that check. The same argument holds when a read is carried more than once, because the old parent is that of the last commit that was read. An empty intersection is a check that passed and ran no test. The audits step runs as today. A test can reach code other than through imports. That is outside the unit check's own reverse-dependents rule too, and the lane's proof of the tip stays the second line (D8).

**The move read.** The move reaches the unit in either of two cases:

- the narrowed check's intersection is not empty;
- the move's reach and the unit's own reach share a package. The unit's own reach is the same closure, of the packages the unit changed.

The second case also covers new code on main that calls the unit's changed code, and a unit whose changed package no test selects.

When the move reaches the unit, the carry runs the unit's reader once more, on the rebased commit, after the narrowed check has passed. The reader gets three inputs: the unit's brief, the unit's diff against its new parent, and main's move in the packages where the two reaches meet. It runs through the existing read machinery: the commit read of `work review --commit` (`commitReview`, `metasystem/cmd/metasystem/intent_unit_review.go:898-925`). The unit's reader and brief are found through the work of the commit the read is carried from (`workOfCommit`, `metasystem/cmd/metasystem/intent_unit_review.go:1366-1388`). The carried attestation names that read's examination as its root job, beside the read it carries (`RootJob` and `Carry` of `CommitReadRequest`, `metasystem/internal/goal/branch/attest.go:125-146`). A repeat reuses the retained read, as the commit read already never starts a critic twice.

- **A clean read.** The read is carried with the narrowed check and the move read.
- **A material finding.** The carry stops at that unit, exactly as D5's red does. In U3 that is a round of the unit, with the findings as its brief (D5). Before U3, `work land` refuses the hand-in and names `work review G --work U`, and `work rebase` lists the unit as needing a read, with the findings as its reason.
- **A move that reaches nothing of the unit.** D3's own argument stands and no read runs. The read is carried with the narrowed check only, and the line says "main's move reaches none of this unit's code".

**How the move reaches the check.**

- `launch.UnitCheck` gains the move as the two commits. They are passed to the cheap command only, as `LANDING_PROOF_MOVE` beside `LANDING_PROOF_BASE` (`metasystem/internal/launch/unit_check.go:123-125`). Beside them goes `LANDING_PROOF_REACH`, the path of a file in the check's execution directory.
- `metasystem test impact` computes the move's reach. It intersects that reach with the unit's selection, package by package, runs the result, and writes the reach file as JSON. The file holds the packages where the two reaches meet, the packages it ran and the number of tests.
- A declared check that ignores these variables runs its whole unit scope and writes no reach file. That is correct and only slower, which keeps the machinery language-generic.
- An impact the tool cannot compute falls back to the whole scope and writes no reach file.
- With no reach file, the move counts as reaching the unit, so the move read runs.

**Who sees the check.** The carry check runs through the launch manager as a host-gated proof launch, as a round's check does (`metasystem/internal/launch/unit_run.go:613`). The lane's host-work count and its hold then see it (`metasystem/internal/launch/hostcapacity.go:33-57`). After the-lanes-proof-goes-first, the check yields to the lane's proof like every other check. The check's records, log, gate observation and worktree change owner as section 8 lists.

**Rejected:**

- **Full unit checks.** The measured median is 27 minutes per unit. A four-unit goal would cost more than the lane cycle it saves.
- **One check at the tip.** A read is attested per commit, and a carried read needs a passing check of its own commit.
- **No check.** `metasystem/internal/lease/sweep.go` would pass this morning's step and fail in the lane, as it did.
- **Symbol-level narrowing of the move.** `test impact` selects a test only when the test names a changed declaration, field or literal (`metasystem/internal/landing/batch/goadapter/impact.go:133-164`, `:271-276`). Take package a's `Wrap`, which calls package b's `Value`, and a test that names only `a.Wrap`. When main changes `b.Value`, the symbol rule drops that test, and the first rebase passes a red unit. By packages, a imports b, so a is in the reach and the test runs.
- **A carried read with no reader.** Main's move can change what an unchanged patch means: main's early return in `metasystem/internal/lease/sweep.go` made the goal's settle branch dead code. The lane's merge read found that, and a check alone may not.
- **A read of every carried unit.** Where the move reaches nothing of the unit, the code the reader read and everything it depends on are the same. A read would repeat the earlier one.

### D4. Conflicts at the seat (U2 and U3)

**Each class at the seat:**

- Generated paths are rebuilt by the declared commands.
- A builder-class conflict goes to the resolve round of the unit that conflicts.
- A judgement conflict goes to the same round, from U3 on (Q1, answered yes).

**A judgement conflict.** From U3, the step at `work land` gives a judgement conflict to the resolve round of the unit that conflicts, as it gives a builder-class one. The round's brief carries the lane's rule verbatim: "keep both sides whole: main's behaviour plus this goal's additions; never drop a test; a generated file is regenerated by its declared command, never hand-edited" (`metasystem/skills/landing-agent/SKILL.md:160-162`). The round's read (below) reads the resolution, as the lane's resolution job is read for the same conflict "as the seat". `work rebase` run by hand still asks a person (`metasystem/internal/goal/branch/rebase_conflict.go:150-157`).

In U2, before U3 lands, the step sends no question. The rebase is abandoned, which leaves the branch untouched (`metasystem/internal/goal/branch/rebase_conflict.go:150-157`). The goal is handed in as it is, with the paths named, and the lane resolves it as it does today.

**The round belongs to the unit that conflicts, not to the last unit.** A unit's read attests that unit's commit, so its correction must be in that commit.

**Limits.** The round is counted in that unit's run: its round limit, its counted cap and divergence (`metasystem/internal/launch/unit_revise.go:184-192`), and the box through round admission (`:279`). As for every resolve round today, the correction allowance does not apply (`:179-183`): the round corrects main's move, not the builder's work.

**Reads after a round (U3).** A resolve round, a red round and a finding round (D5) run the unit's read sequence before promotion. That sequence is the existing reader, with the round's brief, over the round's diff against its base. Today a resolve round returns before the read sequence. `metasystem/internal/launch/unit_run.go:652-659` changes for these rounds: a green proof goes on to the read sequence, while a proof that wrote or went red still finishes as today. The test at `metasystem/internal/launch/unit_revise_test.go:57-60` flips: the resolve round launches exactly one read, after its proof. U3 then promotes the round's clean read to the unit's read through the existing promotion (`metasystem/cmd/metasystem/intent_unit_review.go:651-697`). The round's base is the commit's parent, and the promotion needs exactly one completed, counting read (`:651-663`). The hand-in then needs no `work review` command. Until U3, the repeated read gate names `work review G --work U`.

**Rejected: the last unit takes every correction** (the request's shape). The conflicting unit's own commit would keep a read of a tree it no longer has.

### D5. A red on main, or a material finding of the move read, becomes a round of that unit (U3)

**How a red is caught.** A red carry check is a typed result of the carry: the unit, its rebased commit, the failing tests and the check's log. It is not an error. The red is first classified by the classifier of a-red-is-classified-once. U3 builds after that classifier is on main and adds no classifier of its own.

| Classification | What happens |
|---|---|
| Own | The replay runs again from the moved, unpublished branch and stops at that unit. The stop places the unit's change, uncommitted, on its rebased parent, so `rebasePlan`'s two checks hold unchanged. The resolve round then runs with a red brief: the unit's original brief (prepended as today), the failing tests with the tail of their output, main's commits since the fork that touched the failing tests' packages, and the rule "make this unit's check pass on main and keep main's behaviour". The round runs the unit's read sequence (D4). It commits the unit again, and the replay and the carry continue. |
| Main's own red | No round. The goal is handed in as it is, and the lane's rules for a red main apply. |
| Flake | The rerun that classifier prescribes. |
| Environment | The step stops and says so. |

**A material finding of the move read.** The carry takes the same stop as an own red. The round's brief is the unit's original brief (prepended as today), the move read's findings, main's move in the packages where the two reaches meet, and the rule "make this unit's intent hold on main and keep main's behaviour". The round runs the unit's read sequence (D4), and that read is promoted as the unit's read.

**Until U3.** In U2 a red refuses the hand-in and names the failing tests. A material finding refuses it and names `work review G --work U`. The local move stays unpublished, so origin and any hand-in are untouched. A repeat reuses the retained red check, or the retained read, without running it again. Line 2 is the person's way past (D7). `--verbose` names the existing rerun command for a suspected flake.

**Rejected:**

- **A correction unit at the tip.** The red unit's commit stays red, so its read can never be carried (`metasystem/internal/goal/branch/rebase.go:370-387`). This is the wall the ui seat's plan for `metasystem/internal/lease/sweep.go` would meet.
- **Handing in on red, as today.** It costs a lane cycle and a return for a red the seat has already seen.

### D6. `work land`'s texts (U2, U3)

The texts are those of section 1.2.

- A refusal says what stopped the step and names one command: `work build G` for a missing worktree, `work status G` for a round that is still running, `work review G --work U` for a material finding of the move read, or the person's way past (D7).
- Codes appear only under `--verbose` and `--json`, as `laneRefusalResult` does today. The one exception is the command line of the person's way past, which has to carry its code.

### D7. A person's way past uses the existing exception route (U2)

Every refusal of the step carries the code `GOAL_BEHIND_MAIN`. `work land G --exception GOAL_BEHIND_MAIN --reason TEXT --by NAME` is admitted the way the trunk-red exception already is on a lane computer (`metasystem/cmd/metasystem/intent_delivery.go:1922-1933`). The person's proof is checked at the enrolled terminal. The step is skipped, and the queue line records who carried the hand-in past it and why.

**Rejected: a new flag.** Flag names are Wido's call, and the exception route already exists for exactly this: a person carrying a landing past one refusal code.

### D8. What the lane still does (U4)

**Unchanged.** The lane proves the tip it was handed. The step makes that proof the common case, because the merge of a tip that stood on main is clean unless main moved since. Its merge, its resolution job and its merge read stay unchanged, for two cases:

- two goals in one batch;
- a code move between the hand-in and the merge.

**Return reasons name their case.**

- A conflict with main says either "handed in on main X; main moved by code to Y since" or "handed in K code commits behind main" (exception, judgement or records route). The reason comes from the queue line's recorded main.
- A batch conflict stays held, with today's reason.
- A red on the batch keeps its cause.

**The command in a conflict return.** It names `metasystem work land G` in three places. Two of them name `work rebase G` today: the conflict and regeneration returns (`metasystem/internal/landing/plain/resolve.go:272`, `:419`) and the resolve verb's help (`metasystem/cmd/metasystem/intent_landing_resolve.go:20`). The third says "review and hand in again" today: the resolution read's return (`metasystem/internal/landing/plain/resolution_read.go:77`). On a returned entry with a conflict with main, `work land G` runs the step and hands in the new tip instead of refusing (`metasystem/cmd/metasystem/landing_plain.go:115-121`). An own-red return refuses as today, because its fix is the seat's.

### D9. Relations to other goals

- **a-records-only-move-keeps-mains-green (landed).** Its classifier is exported from `plain` as `RecordsOnly`, a rename of `recordsOnlyPaths` (`metasystem/internal/landing/plain/scope.go:271`), and used by D1 and by D3's reach. The rebase's ledger-only test stays. It answers a different question: whether the lane's ledger merge drivers take a ledger-only move without a replay.
- **the-lanes-proof-goes-first (approved, ui).** D3 makes the step's checks host work that the lane sees. That goal's admission then orders them behind the lane's proof. This page adds no admission of its own.
- **a-unit-check-runs-the-audits (approved, m1e).** It puts the audit tests in each unit's scope, so an audit that main added falls in D3's intersection (gap 7).
- **a-red-is-classified-once (approved, m1e, design accepted 2026-11-01).** D5 calls its classifier, `loopstop.ClassifyRed`. U3 builds after it is on main.
- **a-lane-returns-on-its-own-proof (finding 464).** Separate, and out of scope here.
- **a-push-folds-a-records-only-move (finding 460).** Separate, and out of scope here.

## 4. Answered questions

**Q1. Judgement conflicts at the seat: yes.** Answered 2026-11-01 by the steward (a1) in Wido's name, under his rule of 2026-10-24 that reads answer their own questions; he overrules.

The question was whether the seat's resolve round should also take a judgement conflict. A judgement conflict is one where both sides changed the same lines, or where there is no common original.

The answer is yes. The seat's resolve round takes a judgement conflict with the lane's keep-both rule, and the unit's reader reads it. That is exactly what the lane's resolution job already does for the same conflict "as the seat" (conflicts-resolve-unattended: "the goal is unattended functionality"). The person's call stays only for `work rebase` run by hand. The accepted rule "a person is asked only for a judgement" (conflicts-resolve-unattended D4, `metasystem/plans/designs/conflicts-resolve-unattended.md:62-68`) then holds only there. D4's judgement paragraph and section 1.2's judgement line follow the answer. U3 adds about 10 lines in `metasystem/internal/goal/branch/rebase_conflict.go`, with a test (T3). U2 keeps today's outcome until U3 lands: no question at the hand-in, and the goal handed to the lane as it is.

**Rejected alternative: no.** The goal would be handed in as it is and the lane would resolve it, so it could come back the way a-round-survives-a-restart did. `work rebase` by hand would also keep asking a person about lines the lane had already merged. This morning's question 6230KPV9B8HYR8CVPMPXQEYVGC waited more than four hours.

## 5. Units

Production lines count additions plus deletions; tests are separate. All units are pinned to m1e and land in this order: U1, U2, then U3 once a-red-is-classified-once's classifier is on main. U4 may go after U2, alongside U3. There are four units, not the request's two or three, because the narrowed check and its read (U1) are their own seam and have value alone. The estimates run double in practice (2026-10-25), so every brief names its split seam.

| Unit | Decisions | What it builds | Production lines (estimate) | Seam | Step value |
|---|---|---|---|---|---|
| U1 `the carry check runs what main's move can reach` | D3 | The move's reach by packages through `reverseDependents`; the move on the unit check and the reach file; `test impact` intersecting the reach with the unit's selection package by package; an empty selection that passes and says so; the carry passing the old and new parents; the carry check as a host-gated proof launch, with its records, log, gate observation and worktree as section 8 lists; the move read on a reached unit, its stop on a material finding, and the attestation's read name | About 195. U1a: `metasystem/internal/landing/batch/goadapter/unitgate.go` 25 (the exported move reach), `metasystem/cmd/metasystem/test_impact.go` 40, `metasystem/internal/launch/unit_check.go` 8. U1b: `metasystem/internal/goal/branch/rebase.go` and `metasystem/internal/goal/branch/read.go` 30, `metasystem/internal/gittree/detached.go` 10 (the commit worktree at a fixed path), `metasystem/cmd/metasystem/intent_carry_check.go` 55, the move read call and the attestation's read name 25 (`metasystem/internal/goal/branch/rebase.go` 10, `metasystem/cmd/metasystem/intent_carry_check.go` 15) | Split: the reach and `test impact`'s move (U1a) from the carry wiring, the launch and the move read (U1b) | `work rebase` carries reads with checks that take minutes, not hours, reads them again where main's move reaches them, and the lane sees the checks |
| U2 `work land moves the branch onto main` | D1, D2, D4 (no-question path), D6, D7 | The trigger after the gates, with the skip list; the behind-by-code count through `plain.RecordsOnly`; `rebaseGoal` with questions off; re-reading the branch and repeating the gates on the new tip; the queue line's recorded main; the red and finding refusals; the exception code; the Stop path's report | About 140: `metasystem/cmd/metasystem/intent_delivery.go` 45, a new `metasystem/cmd/metasystem/land_on_main.go` 55, `metasystem/cmd/metasystem/intent_work_rebase.go` 12, `metasystem/internal/landing/plain/scope.go` 3, `metasystem/internal/landing/plain/queue.go` 5, the exception route 20 | Split: the texts and the exception route | A goal behind main by code reaches the lane standing on main |
| U3 `a conflict, a red or a finding on main is a round of its unit` | D4 (round reads, promotion, judgement conflicts per Q1), D5 | The read sequence for resolve, red and finding rounds; promoting the round's read; judgement conflicts to the round with the keep-both rule; the typed red from the carry; the replay stopping at the red or found unit; the red and finding briefs; the classifier call | About 185. U3a: `metasystem/internal/launch/unit_run.go` 15 (the read sequence for these rounds), the promotion wiring in `metasystem/cmd/metasystem/intent_work_rebase.go` 15, `metasystem/internal/goal/branch/rebase_conflict.go` 10. U3b: `metasystem/internal/goal/branch/rebase.go` 70, `metasystem/cmd/metasystem/intent_work_rebase.go` 40 (with the finding stop and its brief, 15 of them), `metasystem/cmd/metasystem/land_on_main.go` 20 | Split: round reads, promotion and judgement conflicts (U3a) from the red and finding stops (U3b) | A semantic conflict is a round at the seat, not a lane return |
| U4 `the lane names the case` | D8 | The entry's recorded main; return reasons that name the case and `work land G`; the resolve verb's help; `work land` running the step on a conflict return | About 62: `metasystem/internal/landing/plain/resolve.go` 15, `metasystem/internal/landing/plain/resolution_read.go` 8, the return verb's reason 12, `metasystem/cmd/metasystem/intent_landing_resolve.go` 2, `metasystem/cmd/metasystem/landing_plain.go` 25 | None needed | A return says whether main moved after the hand-in, and its fix is `work land G` |

Every brief follows BRIEF-CHECKLIST, including item 17. Each has `Read each round: yes` and one Astra round. Each runs the reverse-dependent packages whole, by name from the module root `metasystem/`: the Go packages `./internal/goal/...`, `./internal/launch`, `./internal/landing/...` and `./cmd/metasystem`. U1b also runs `./internal/gittree`, which it extends.

## 6. Tests

These rules apply to every test:

- Only new or changed tests. Each uses `t.Parallel()` and artificial clocks, with no sleeps.
- Each mutation turns a named test red by assertion.
- Each behaviour has one test through the component that runs it in production, in production's order.
- Source comments and test diagnostics carry no design, decision, ruling or finding ids.

### T1 (U1)

Through `test impact` and the carry, over a fixture repository with packages a, b and c:

- A unit changes a. Main's move changes c, which neither a nor any package that imports a imports, and c imports nothing the unit changed. The selection is empty, the check passes and says so, no read runs, and the line says "main's move reaches none of this unit's code".
- The move-read case. Main changes `b.Value`, which the unit's change in a calls. The check runs every test of a, which is in the reach, and the reader runs once on the rebased commit. The carried attestation names that read.
- The wrapper case. Package a's `Wrap` calls package b's `Value`, a test of a names only `a.Wrap`, and main changes `b.Value`. The test runs.
- Main adds code in c that calls the unit's changed function in a, and no selected test reaches c. The reader runs.
- Main adds a test file to a: it runs.
- An earlier unit's resolution changes b, which a imports: a's tests run, because the move is parent to parent.
- A read carried twice is narrowed to the second move only.
- A records path in the move reaches nothing. A `go.mod` change reaches the whole module.
- A move read with a material finding stops the carry at that unit. `work rebase` lists the unit as needing a read, with the findings as its reason, and no later unit is carried past it.
- A carried unit's gate records the narrowed command. While the check runs, its execution is a proof launch that `HostWork` counts. Its execution holds `result.json` and `output.log` in the carry's records for the subject commit. The gate observation's run id is the launched execution.
- A repeat while the check's launch still runs starts no second launch and waits for it. A repeat after it finished reuses its result.
- A carry interrupted during its launch leaves the worktree at its fixed path. The repeat closes it once the launch is terminal, then plans its own.
- A declared check that ignores the move runs its whole unit scope, writes no reach file, and the reader runs.
- A move whose impact cannot be computed runs the whole scope, and the reader runs.

Mutations:

- Drop the intersection: the count assertion goes red.
- Take the move as fork point to main tip instead of parent to parent: the earlier-resolution case goes red.
- Treat an empty selection as red.
- Run the check inside the process: `HostWork` sees nothing.
- Narrow the move to direct symbols: the wrapper case's test is dropped and the case goes red.
- Drop the read when the move reaches the unit: the move-read case's read count goes red.
- Treat a missing reach file as no reach: the ignoring-check case runs no read.
- Start a new check on a repeat: the running-launch case counts two launches.

### T2 (U2)

Through public `work land` over real Git, in the `TestWorkLandHandsInOverRealGit` fixture:

- Behind main by a code commit: origin's branch is rewritten onto main; the queue line holds the new tip and records main; the unit's read is carried; one rebase history line is written.
- Behind by a records-only commit: nothing is rewritten. Today's assertions are kept for this case.
- A second code move of main during the step: the goal is handed in, and the text says main moved again.
- `--through`, `--records`, `--again`, an exception route, or an earlier hand-in still waiting: no rewrite.
- A builder-class conflict: the resolve round of the conflicting unit runs in the step. Until U3, nothing is handed in, and the line names `work review G --work U`.
- A judgement conflict: no channel question; handed in as it is, with the paths named. This case flips in U3 into T3's judgement case.
- A red carried check: nothing is handed in; the failing test is named; origin's branch is untouched; a repeat runs no second check.
- A material finding of the move read: nothing is handed in; the line names `work review G --work U`; origin's branch is untouched.
- `--exception GOAL_BEHIND_MAIN` from the enrolled terminal: handed in as it is, and the line names the person.
- The Stop path on a branch behind by code: it reports, with no rewrite and no check.

Mutations: use the ledger-only test as the trigger (a non-ledger records move rewrites); drop the re-read (the queue holds the old tip); leave questions on (a question is sent); hand in on red.

### T3 (U3)

- A fixture where main returns early in a function where U1 settles a case, so U1's test goes red on main (this morning's shape):
  - the replay stops at U1, with U1's change on its rebased parent;
  - the round's brief names the failing test and main's commit;
  - the round counts in U1's run, and the round limit stops one round too many;
  - the round has exactly one completed read step, launched after its proof and before the promotion;
  - U1's new commit carries the round's promoted read;
  - U2 is carried with a narrowed check;
  - the goal is handed in with no `work review` command.
- The same fixture where U1's check stays green but the move read has a material finding: the replay stops at U1, the round's brief carries the findings and main's move in the meeting packages, and the round's read is promoted.
- A red the classifier calls main's own: no round; handed in as it is.
- A builder-class conflict: the resolve round runs its read, the read is promoted, and no `work review` command is printed.
- A judgement conflict: it goes to the round of the unit that conflicts, the round's brief carries the keep-both rule, the round's read is promoted, and no question is sent. `work rebase` by hand still asks (`TestIntentWorkRebaseJudgementAsksPerPath`, unchanged).

Mutations:

- Correct at the tip: U1's gate stays red and the hand-in refuses.
- Skip the classifier: main's own red starts a round.
- Skip the round's read sequence: the read-exists assertion goes red, and the promotion refuses.
- Skip the promotion: `work review` is named.
- Send a judgement conflict to a person at `work land`: a question is sent.

### T4 (U4)

- A conflict-with-main return of a hand-in that stood on main names "handed in on main X; main moved by code to Y since" and `metasystem work land G`.
- The same return of a hand-in made through the exception route names "handed in K code commits behind main".
- A batch conflict stays held, with today's reason.
- `work land G` on a conflict return runs the step and hands in a new tip.
- `work land G` on an own-red return refuses as today.

Mutations: name `work rebase` again; run the step on an own-red return.

### Pinned tests that change

- **`TestWorkLandHandsInOverRealGit`** (`metasystem/cmd/metasystem/intent_delivery_owner_test.go:340`). The "behind main" case's three no-rebase assertions (`:395-414`) flip back for a code move; lane-lands-finished-goals D2 flipped them the other way. The fixture's main commit `main-extra.md` must classify as code, or the case gains a records twin.
- **`TestWorkLandKeepsBranchBehindMain`** (`metasystem/cmd/metasystem/landing_rebase_test.go:118`). Kept for a records-only move; flipped for a code move.
- **Tests asserting `work rebase` in a return** (U4): `TestWorkLandReturnedConflictNamesRebase` (`metasystem/cmd/metasystem/landing_rebase_test.go:147`), `metasystem/cmd/metasystem/landing_plain_handin_test.go:342`, `metasystem/cmd/metasystem/landing_conflict_test.go:101` and `:145`, and `metasystem/internal/landing/plain/resolve_test.go:109`.
- **`TestUnitCheckBaseOverridesInheritedValueOnlyForCheap`** (`metasystem/internal/launch/unit_check_base_test.go:9`). The move and the reach file travel with the base, to the cheap command only.
- **`metasystem/internal/goal/branch` rebase tests** (`metasystem/internal/goal/branch/rebase_test.go` `TestRebasePolicy`, `metasystem/internal/goal/branch/rebase_resolve_test.go` `TestRebaseResolveRound`). The red stop and the finding stop are added beside the conflict stop.
- **`TestRevisionOnStoppedRebase`** (`metasystem/internal/launch/unit_revise_test.go:47`), in U3. Its check that a resolve round launches no read (`:57-60`) flips: the round launches exactly one read, after its proof.
- **T2's judgement case**, in U3, as T2 says.

### Tests that stay unchanged

- `TestIntentWorkRebaseJudgementAsksPerPath` (`metasystem/cmd/metasystem/intent_work_rebase_test.go`; `work rebase` by hand still asks).
- The hand route's host-contract tests (`metasystem/cmd/metasystem/goal_landing_host_contract_test.go`, `metasystem/cmd/metasystem/goal_landing_portable_test.go`, `metasystem/cmd/metasystem/goal_landing_portable_file_test.go`, `metasystem/cmd/metasystem/goal_landing_workers_public_test.go`).
- The unit plan's base tests (`metasystem/internal/launch/unit_plan.go` does not change).
- The lane's scope tests (the classifier only gains an exported name).
- `test impact`'s unit selection tests (`UnitImpact` does not change; the move's reach is a new function beside it).

## 7. Later, in a future iteration (named, not built)

- **A "behind main by code" line in each round's brief** (option d). It is cheap. Add it only if seats are surprised at the hand-in.
- **One merge commit instead of a rebase.** Only if the per-unit carry ever costs more than D3 makes it. It needs a commit kind that the range reader accepts and a resolution read like the lane's.
- **A resolve round that survives an interrupted step.** A round is bound to the scratch worktree's path and the stop's commits, and every replay makes new ones (`metasystem/internal/goal/branch/rebase.go:436-470`, `metasystem/internal/launch/unit_revise.go:149-152`). An interrupted `work rebase` or `work land` therefore starts the round again.
- **The step as a launched job that `work wait` follows.** Only if the step, run in the foreground, proves too long for a seat's turn. Measure first: with D3, most carried checks run few tests and most carried units need no read.
- **Computers without a lane moving onto main at `work land`.** Their replay already refuses when main touched a unit's files, and names `work rebase`.

## 8. Moved effects

This table lists every effect this page moves to a new owner. From is the old owner and its lines. To is the new owner, the unit that moves it and the test that proves the new owner. Code is the code that performs the effect today; it names existing files only, and the new `metasystem/cmd/metasystem/land_on_main.go` appears only in the To column.

| Effect | From | To | Code |
|---|---|---|---|
| the carry check's execution | the calling process: `carrySubjectCheck` runs `UnitCheck.Run` itself, unseen by the lane's hold (`metasystem/cmd/metasystem/intent_carry_check.go:143-148`) | a host-gated proof launch the carry starts, as a round's check is started (`metasystem/internal/launch/unit_run.go:613`); its process runs the frozen check through the unit-check command (`metasystem/cmd/metasystem/intent_unit_check.go:93-142`), and `HostWork` counts it (U1b, D3; T1's launch case and the in-process mutation) | `metasystem/cmd/metasystem/intent_carry_check.go:143-148`, `metasystem/internal/launch/unit_check.go:87-171`, `metasystem/internal/launch/hostcapacity.go:33-57` |
| the carry check's retained result and its reuse by a repeat | the calling process: each run leaves a `check-*` execution with its `result.json` in the carry's records for the subject commit, and a repeat reuses the newest with the same commands, or refuses it when red (`metasystem/cmd/metasystem/intent_carry_check.go:44`, `:93-142`) | the launched check, which writes its execution into the same records; a repeat reuses a finished launch's result, waits on a running launch through its record, and never starts a second check of the same commit (U1b, D3; T1's repeat case and the second-launch mutation) | `metasystem/cmd/metasystem/intent_carry_check.go:93-142`, `metasystem/internal/launch/unit_check.go:165-171` |
| the carry check's log | the calling process: `UnitCheck.Run` writes `output.log`, and nobody can see it while the check runs (`metasystem/internal/launch/unit_check.go:98-103`) | the launched process, which writes the same file; the proof launch's record names it, so `work status G` and `work wait` show it as a running check (U1b, D3; T1's launch case) | `metasystem/internal/launch/unit_check.go:98-103` |
| the carry's gate observation | the calling process, from its own run's `result.json` right after the run, written to `gate.json` (`metasystem/cmd/metasystem/intent_carry_check.go:149-190`) | the carry, after the launch's terminal record, from the launched execution's `result.json` and after the workspace-unchanged check; its run id is the launched execution (U1b, D3; T1's gate case) | `metasystem/cmd/metasystem/intent_carry_check.go:149-190` |
| the detached worktree's lifetime | `resolveReadGate`: a temporary detached worktree, opened before the subject check and closed as soon as the in-process check returns (`metasystem/internal/goal/branch/read.go:219-226`, `metasystem/internal/goal/branch/read_repository.go:44-50`) | the check's launch: the worktree is a commit worktree at a scratch path fixed by the subject commit, which U1b adds in the commit form beside `PlanDetachedWorktreeAt`, the existing fixed path for a tree worktree (`metasystem/internal/gittree/detached.go:198-233`); it is the launch's working directory, the carry closes it once the launch is terminal, and a repeat closes a leftover whose launch is terminal before planning its own (U1b, D3; T1's interrupted-carry case) | `metasystem/internal/goal/branch/read.go:219-226`, `metasystem/internal/goal/branch/read_repository.go:44-50`, `metasystem/internal/gittree/detached.go:198-233` |
| the carried attestation's witness, and finding what main's move changes in an unchanged unit | the check alone: the carry commits a read that names the read it carries and the passing gate, and runs no reader (`metasystem/internal/goal/branch/rebase.go:365-405`); a semantic conflict is found only by the lane's merge read after the hand-in (`metasystem/skills/landing-agent/SKILL.md:152-165`) | the check and, where main's move reaches the unit, the move read by the unit's reader through the commit read (`metasystem/cmd/metasystem/intent_unit_review.go:898-925`); the carried read names that read's examination as its root job (U1b, D3; T1's move-read case and the dropped-read mutation) | `metasystem/internal/goal/branch/rebase.go:365-405`, `metasystem/internal/goal/branch/attest.go:125-146`, `metasystem/cmd/metasystem/intent_unit_review.go:898-925`, `metasystem/skills/landing-agent/SKILL.md:152-165` |
| moving a goal that is behind main by code onto main before the hand-in | the lane's merge after the hand-in, since lane-lands-finished-goals D2, "work land never rebases" (`metasystem/plans/designs/lane-lands-finished-goals.md:121-145`); `work land` hands in the tip it reads from origin (`metasystem/cmd/metasystem/intent_delivery.go:425-451`) | `work land`'s step, after the gates and before the queue line, through the existing `rebaseGoal` (`metasystem/cmd/metasystem/intent_work_rebase.go:70-180`) and the new `metasystem/cmd/metasystem/land_on_main.go` (U2, D1 and D2; T2's behind-by-code case) | `metasystem/plans/designs/lane-lands-finished-goals.md:121-145`, `metasystem/cmd/metasystem/intent_delivery.go:425-451`, `metasystem/cmd/metasystem/intent_work_rebase.go:70-180` |
| resolving a builder-class conflict with main for a lane hand-in | the lane's resolution job "as the seat", after the hand-in (`metasystem/internal/landing/plain/resolve.go:197-206`) | the resolve round of the conflicting unit at the seat, in `work land`'s step (`metasystem/internal/goal/branch/rebase_conflict.go:158-181`); the lane's job stays for a conflict that arises after the hand-in (U2, D4 and D8; T2's builder-class case and T3's builder-class case) | `metasystem/internal/landing/plain/resolve.go:197-206`, `metasystem/internal/goal/branch/rebase_conflict.go:158-181` |
| resolving a judgement conflict with main for a lane hand-in | the lane's resolution job "as the seat" (`metasystem/internal/landing/plain/resolve.go:197-206`), after a hand-in on which the seat asked nobody, or asked a person when the seat ran `work rebase` (`metasystem/internal/goal/branch/rebase_conflict.go:150-157`) | the conflicting unit's resolve round, with the lane's keep-both rule in its brief, read by the unit's reader; `work rebase` by hand still asks (U3, D4 and Q1; T3's judgement case) | `metasystem/internal/landing/plain/resolve.go:197-206`, `metasystem/internal/goal/branch/rebase_conflict.go:150-157` |
| the read of a resolve round | the seat's later `work review G --work U`, because a resolve round finishes after its proof with no read (`metasystem/internal/launch/unit_run.go:652-659`) | the round's own read sequence before promotion, through the existing promotion (U3, D4; T3's read-exists assertion and the skipped-read mutation) | `metasystem/internal/launch/unit_run.go:652-659`, `metasystem/cmd/metasystem/intent_unit_review.go:651-697` |
| correcting a red on main, or a material finding of the move read | the lane: its proof names the goal own (`metasystem/internal/landing/plain/replay.go:398-405`), the goal is returned, and the seat plans a new unit at the tip; at the seat, a red carry check ends `work rebase` with an error (`metasystem/internal/goal/branch/rebase.go:370-387`, `metasystem/cmd/metasystem/intent_carry_check.go:127-136`) | a round of that unit at the seat, in `work land`'s step, with a red or finding brief; before U3 the hand-in refuses and names the failing tests or `work review G --work U` (U3, D5; T3's red and finding cases, T2's red and finding refusals) | `metasystem/internal/landing/plain/replay.go:398-405`, `metasystem/internal/goal/branch/rebase.go:370-387`, `metasystem/cmd/metasystem/intent_carry_check.go:127-136` |
| the conflict return's command | `work rebase G`, named by the lane's conflict and regeneration returns (`metasystem/internal/landing/plain/resolve.go:272`, `:419`), by the resolve verb's help (`metasystem/cmd/metasystem/intent_landing_resolve.go:20`) and by `work land` on a returned conflict (`metasystem/cmd/metasystem/landing_plain.go:115-121`); "review and hand in again" in the resolution read's return (`metasystem/internal/landing/plain/resolution_read.go:77`) | `work land G`, which runs the step on a conflict return (U4, D8; T4) | `metasystem/internal/landing/plain/resolve.go:272`, `metasystem/internal/landing/plain/resolution_read.go:77`, `metasystem/cmd/metasystem/intent_landing_resolve.go:20`, `metasystem/cmd/metasystem/landing_plain.go:115-121` |

## Dispositions (critique design-critic-a2839c682869b4ba3dc55866)

Written by metasystem design review when critique design-critic-a2839c682869b4ba3dc55866 closed: every answered round's decisions, as the author made them.

| Round | Finding id | Finding | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- | --- | --- |
| 1 | RULING-a-hand-in-stands-on-main | The approved goal requires the engine to “reads that merge at the seat”, but the design replaces examination of the integrated result with carried reads. This changes the required read contract and fails the safety test by removing a human-required witness without authorization. | accepted | The goal's approved intent says the seat "reads that merge at the seat"; the page carries an unchanged unit's read with a check only (D2, D3; metasystem/internal/goal/branch/rebase.go:348-391 carries without a reader). This morning's defect was found by the lane's merge read, and main's move can change what an unchanged patch means (the sweep.go early return made the goal's settle branch dead code). The witness is the machine's independent read, and the page may not drop it where the move reaches the unit. Decided by the steward in Wido's name (his rule of 2026-10-24): the move is read where it reaches the unit; where it reaches nothing of the unit (D3's empty intersection) no read runs, by D3's own argument. | Revision 2: D3 gains the move read: when main's move reaches the unit (a non-empty intersection), the carry runs the unit's reader once more on the rebased commit with the unit's brief, the unit's diff and main's move in the reached packages (the existing read machinery of metasystem/cmd/metasystem/intent_unit_review.go), and the carried attestation names that read; a material finding stops the carry at that unit as D5's red does (a round of the unit in U3; before U3 the hand-in refuses and names work review G --work U); an empty intersection carries the read with the check only and says so. T1 gains the move-read case and the mutation that drops the read when the move reaches the unit. The unit table moves the lines (about 25) into U1b and U3. Section 1.2's "moved cleanly" line names the reads. |
| 1 | IMPACT-TRANSITIVE-READERS | U1's intersection can silently pass a failing carried unit. If a unit test calls a wrapper whose dependency changed on main, direct-symbol selection can exclude that test. Both materiality tests fail: the selection and its fixtures must change, and the first rebase can produce a false passing check. | accepted | D3 intersects the unit's selection with the move's selection computed by test impact, whose symbol-level matching (metasystem/internal/landing/batch/goadapter/impact.go:133-164, :271-276) can omit a test that reaches main's change through a wrapper, so the intersection drops a failing test and the first rebase passes a red unit. | Revision 2: the move's reach is computed at package level as the reverse-dependency closure of the packages main's move changed (the repository's own import graph, as the reverse-dependents rule of every brief already does), never by symbol; the intersection is package by package with the unit's selection, and inside a selected package every test runs. T1 gains the wrapper case (a.Wrap calls b.Value; a test references only a.Wrap; main changes b.Value; the test runs) with the mutation that narrows the move to direct symbols. D3's "Rejected" names symbol-level narrowing of the move and why. |
| 1 | MOVED-EFFECTS-CARRY-AND-INTEGRATION | The required Moved effects inventory is absent. U1 moves carry-check execution into proof launches without fully assigning retained results, logs, gate observations and detached-worktree lifetime. This changes the ownership contract and leaves the first launched check's complete behavior unproven. | accepted | The page contract requires a Moved effects table and check-only reports no inventory; U1 moves the carry check's execution from the calling process (metasystem/cmd/metasystem/intent_carry_check.go:93-190) into a host-gated proof launch (metasystem/internal/launch/unit_check.go:87-171), with its retained result, log, gate observation and the detached worktree's lifetime, and later units move integration and correction duties from the lane to the seat (metasystem/internal/goal/branch/read.go:217-241; lane-lands-finished-goals D2). | Revision 2: a Moved effects section with every moved effect, each row with the effect, the old owner (file:line), the new owner, the unit that moves it and the test that proves the new owner: the carry check's execution, retained result, log and gate observation (U1); the detached worktree's lifetime (U1); moving a goal onto main before the hand-in, from lane-lands-finished-goals D2's "never" to work land (U2); the conflict return's command (U4); the correction of a red on main, from a lane return to a round of the unit (U3); the carried attestation's witness (the move read, U1b). design review plans/designs/a-hand-in-stands-on-main.md --check-only passes before revision 2 is published and its last line is quoted in the provenance. |
| 1 | LATER-RESOLVE-READ | U3 assumes a resolve-round read exists to promote, but current resolve rounds deliberately omit reads. Defer this correction until U3: U1 works and remains safe without changing that later transition. | accepted | Not material to U1, as the critic says, but U3's text is wrong as written: a resolve round runs no read today (metasystem/internal/launch/unit_run.go:652-659 returns before the read sequence; metasystem/internal/launch/unit_revise_test.go:57-60 rejects a resolve-round read) and promotion needs a completed read (metasystem/cmd/metasystem/intent_unit_review.go:651-663). Folded now so U3's brief is cut from a true page. | Revision 2: D4's "Reads after a round (U3)" says that a resolve round and a red round run the unit's read sequence (the existing reader, the round's brief) before promotion, that unit_run.go:652-659 changes for those rounds and that the test at unit_revise_test.go:57-60 flips; U3's estimate and owner files gain those lines; T3 gains the assertion that the round's read exists before promotion, with the mutation that skips the read sequence. |
