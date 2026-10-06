# Design for lane-lands-finished-goals

- Kind: design
- Id: 01M484T41SDEHDHR1VSE4YWRVF
- Status: accepted
- Goals: lane-lands-finished-goals
- Critique: closed after 1 round with 11 findings folded (Codex Astra); stopped early under the old one-round rule, since replaced by the convergence rule in the machinery plan's stop table

Landing redesign 1a. Written 2026-10-06 from `plans/lane-lands-finished-goals-design-brief.md`, and revised the same day from `plans/lane-lands-finished-goals-design-brief-after-1.md` after one Astra round (gpt-6-astra, 11 material findings, all accepted and folded; the table at the end of "What the code says today" says where each one landed). Against main `9eb87cd15`. Paths are relative to `metasystem/`. Nothing is built yet. Every site cited below was read at that commit. It consumes mechanisms 2, 3, 4, 5, 6, 9 and 12 of `plans/designs/machinery-mechanisms.md`, each in the smallest form this goal needs.

## Wido's words (binding)

> "Landing is responsible for making sure the goal's test suite is green in the most efficient way, and can fix merge conflicts so it can do that. Landing exists only because the test suite is expensive and it can batch goal tests. No goal slice should ever end up there." (2026-10-05)

> "You only integrate when you are done with a goal." (2026-10-05)

## What this goal delivers

The landing lane lands finished goals only, by merging them, and says why a check went red before it gives anything back. Five things change for the people and agents who use it:

1. `work land` takes a goal only when every unit it declares is built and read. A goal with a design declares its units in the design's Units table; a goal without one declares its last unit when it builds it. A goal lands once.
2. `work land` never rebases and never rewrites the goal branch. In the lane the merge commit is the integration, so unit commits and their reads keep their identity.
3. Every red check and every return carries a cause. Only a goal's own demonstrated defect sends it back to its seat.
4. A red on main is an incident per failing test that `incident list` shows. Landing holds while one is open, except for the goal that fixes it.
5. A seat whose goal waits in the lane can claim its next goal.

Not in this goal: the lane's policies and the helm, `landing drain`, areas on the claim (design 1b); the records ref (finding 3 of the plan); the resolve round inside the lane's merge commit; proof deadlines and process groups (mechanism 8, the fleet goal); `proof.audits` and the builder's rung (the review-chain goal); a bound on returns per goal (see Deferred); the hand route integrating by a merge commit (question 4).

## Threat model and stop rule

The threat is our own agents and operators making mistakes and crashing, as the plain lane already assumes (`internal/landing/plain/queue.go:1-11`). Nothing here defends against an adversary. The risks this design answers are the week's: slices handed in (102 hand-ins for 28 goals), good branches returned for someone else's red (29 red returns, several of them main's own), and reds on main that no record saw (eight in the week).

Each unit follows the plan's stop rule: one build, at most two corrections, the material count must fall, a class repeat stops the unit.

## What the code says today, and where the brief's premises need correcting

The first brief's ten premises hold at `9eb87cd15`. Two line numbers moved: `Entry` is at `internal/landing/plain/queue.go:79` (the brief says `:76`), and `runIntentLandingProve` is at `cmd/metasystem/intent_landing_prove.go:118` (the brief says `:119`). Thirteen further facts change what is built. Facts 1 to 7 are the first draft's; facts 8 to 13 were read for this revision.

1. **`goal done` refuses unlanded work.** Its sweep answers "has commits that never landed ... run: metasystem work land G" (`internal/goal/branch/sweep.go:282`). So the finished goal's next step is `work land`, and `goal done` follows the landing (question 1, decided).
2. **The claim quota has an at-rest half that cannot read a queue.** `ValidateTree` (`internal/goal/validate.go:205`, the quota at `:399-490`) runs over the committed ledger at every publish, on every computer (`validateCommitFor`, `:696`). The lane's queue is a file on one computer. Decision 5 therefore keeps the at-rest rule untouched and changes only the claim.
3. **`StageReturned` ends a card's life on the board.** It is a terminal stage (`internal/board/card.go:63`), and `LiveCard` (`:454`) finds only non-terminal cards. A returned card must be found again by the next hand-in (decision 3).
4. **`RecordTrunkRed` appends a sighting, and a ledger commit, every time it is called for an open entry** (`internal/goal/trunkred.go:688-700`). The two closed entries in `plans/goals/trunk-red.json` show the result: twelve sightings each. Main moves hundreds of times a day by ledger commits alone, so a red keyed by main commit would open a new incident per ledger move, and a recorder that publishes per sighting would add a commit per sighting (decision 4).
5. **`work land --exception` does not go through the lane today.** It is the carried route, which lands main from the seat's own checkout (`cmd/metasystem/intent_exception.go:39`, reached at `cmd/metasystem/intent_delivery.go:1734` before any lane code). Decision 4 routes one code through the lane and leaves every other code as it is.
6. **The lane's proof already repeats a red once, and the allowance is one field.** A dead check, a command that reports it did not run, and failed units the batch cannot affect get one repeat, written as `repeat: allowed` and spent as `repeat: started` (`internal/landing/plain/prove.go:205-209`, `:399-401`, `:669-700`); `checkBound` (`:217`) refuses any further check of that tree. The proof command already accepts `LANDING_ONLY=<unit>` to run one failed unit alone (`:597`, `:718`). Decision 3 builds the replay on that call and carries that one allowance.
7. **The lane's merge tree already holds every prefix of the batch.** The landing agent merges each waiting line with `git merge --no-ff` onto a detached `origin/main` (`skills/landing-agent/SKILL.md:29-30`), so the first-parent commits between `origin/main` and HEAD are "main plus goal 1", "main plus goals 1 and 2", and so on. Each merge's second parent is a handed-in sha, which the queue maps to its goal. The replay needs no new merges.
8. **The hand route replays; it does not merge.** On a computer without a lane, `work land` composes its candidate through `PrepareLanding` (`cmd/metasystem/intent_delivery.go:355-356`, `:2112`): in a scratch worktree at main's tip it applies each unit's patch and commits it anew, one landing commit per unit, authored by the approving person and naming its source (`internal/goal/branch/land.go:659-723`, the commit at `:713`, the `Goal-Source` trailer at `:389`). It refuses a unit when main changed any file that unit touches (`:247-275`) and when the applied change differs from the change that was read (`:695-700`). The goal branch itself is never written. The sweep and a verifier read those landing commits (`internal/goal/branch/sweep.go:97-109`, `verify.go:20-58`, `landing_event.go:19`). Decision 2 says what 1a does with this route.
9. **A green in `results.jsonl` is one of three things.** The command ran whole on that tree (`scope: full`); it ran only the groups whose inputs cover what changed (`scope: scoped`, `internal/landing/plain/scope.go:169-172`); or nothing ran and the green is inherited from a tree that differs only in ledger paths (`prove.go:428-434`, `scope.go:376-394`). `fullCurrent` (`scope.go:345-350`) accepts the last two for an hour, and accepts a line without any scope for ever. Decisions 4 and 7 name which of these each rung may rest on.
10. **The trunk check cannot run when it is most needed.** A check of a tree that already has a green is answered from the record (`prove.go:280`, `:392-394`, `scope.go:353-360`); a tree whose last result is red without an allowance is refused (`prove.go:217-222`); and the skill proves `origin/main` only when nothing waits (`SKILL.md:61`). While main is red every line waits, so nothing would ever clear the incident by evidence (decision 4).
11. **A conflict in a batch can be against an earlier member, not against main.** `landing resolve` records the checkout's HEAD as "main" (`internal/landing/plain/resolve.go:94`, `:170`), and HEAD is main plus the batch members merged before. A goal returned for such a conflict cannot fix it by `work rebase`, which rebases onto origin's main (decision 3).
12. **A failed regeneration returns the goal whatever failed.** A log folder that cannot be made, a command that cannot start and a generator that rejects the merged sources all end in the same return (`resolve.go:185-209`, `:220-245`). Decision 3 classifies them.
13. **A goal without a Units table has nothing that says it is finished.** `handLandingSubject` lands whatever the branch holds once its units are read, or unread for a goal whose reads are waived (`cmd/metasystem/intent_delivery.go:2224-2246`), and two of the eight next-step sites print `work land` after every commit of such a goal (`cmd/metasystem/intent_selection.go:280-281`, `cmd/metasystem/intent_manual_submit.go:334-339`). Decision 1 gives these goals a declared end.

**Where each Astra finding landed.**

| Finding | Folded in |
| --- | --- |
| 1 the full rung may be scoped or reused | Decision 7, "What counts as the full rung"; decision 4, "Clearing it" |
| 2 a goal without a Units table | Decision 1, "What declares a goal's end"; unit `boundary` |
| 3 a red gate is not yet `own` | Decision 3, the cause table (one classifier, three callers); decision 7, "The merge gate" |
| 4 the hand route rewrites unit commits | Decision 2, "The hand route"; question 4 |
| 5 `full-due` runs no periodic trunk check | Decision 4, "The trunk check" and "The clock" |
| 6 identity by unit, a commit per sighting | Decision 4, "Writing it" and "One entry per failing test" |
| 7 a conflict against a batch member | Decision 3, "A conflict is classified before anything is returned" |
| 8 regeneration failures return goals | Decision 3, the same section |
| 9 a repeat after the repeat is spent | Decision 3, "One repeat per tree" |
| 10 the return guard after a red gate | Decision 3, "`landing return`"; decision 7, "The merge gate" |
| 11 one predicate for admission and the next step | Decision 1, "One predicate" |

## Decision 1: a goal is handed in whole, and once (mechanism: none; the charter)

**What declares a goal's end.** Every goal that builds units has one of two declarations.

- **A goal with a Units table.** The table of its accepted design, found with the lookup build admission uses (`acceptedDesignPaths`, `cmd/metasystem/intent_work.go:1024`; `launch.DeclaredUnits`, `internal/launch/admit.go:458`; the first accepted page that has a table). A commit's unit name matches a row by the rule `sizesFromTable` applies (`admit.go:251`).
- **A goal without one** (a tier-1 goal needs no design page, and a design may have no table). Its end is the unit built with `--last`: `work build G --work UNIT --last`, and the same flag on the hand-committed route (`cmd/metasystem/intent_manual_submit.go`). The flag adds one trailer, `Goal-Whole: G`, to that unit's commit (`commitMessage`, `internal/goal/branch/commit.go:228-239`). The branch reader keeps taking a commit's kind from the three trailers it knows (`internal/goal/branch/range.go:141`) and reports the mark on the unit (`UnitStatus`, `internal/goal/branch/status.go:10`). The mark is on the branch, so every checkout reads the same end and `work rebase` carries it. Units committed after the marked one (a correction a read asked for) are part of the goal. When the last unit was built without the flag, a person declares the end at hand-in: `work land G --whole --by NAME`, proven as every `--by` act is (`actingAs`, as `incident close` uses it at `cmd/metasystem/intent_planning.go:2263`); the queue line records who declared it.

**One predicate.** A unit is *finished* when it has a commit on the branch and either a clean read or the goal's reads are waived (`goal.ReadsWaived`, `internal/goal/landgate.go:663`). One function, `goalProgress`, reads the declaration and the branch (`state.Status.Units`, `state.Status.Prefix` and `state.ReadsWaived`, `cmd/metasystem/intent_delivery.go:298-306`) and answers one of four things: no end is declared; the first declared unit, in table order, that is not finished, and whether it lacks its build or its read; a unit on the branch outside the declaration that lacks its read; or finished. `work land` and the next-step function both call it and nothing else, so they cannot disagree. It replaces the `landable` arithmetic of `handLandingSubject` (`intent_delivery.go:2238-2246`) for a goal hand-in.

**The rule.** `work land G` refuses when:

- `goalProgress` answers anything but finished;
- `--through COMMIT` names anything but the branch's last unit commit: a goal lands whole;
- the goal's newest hand-in already landed and the branch has commits beyond it: a goal lands once. A waiting line may still be replaced by a newer tip (`queue.go:189-194`), and a returned goal hands in again; both are the same whole goal.

A unit on the branch that the table does not declare is allowed and needs its read as today. A records hand-in (`--records`) is untouched, and so is a branch that holds only plan commits of a goal without a table (`intent_delivery.go:2225`). A person who decides a unit is no longer part of the goal removes its row from the design page; the record then says what the goal is.

The check sits in `landGoalRoute` beside `handLandingSubject` (`intent_delivery.go:2030`), before the route splits, so the lane route and the hand route share it.

**What a person reads.**

```
goal G is not finished: its design declares 9 units; `replay` and `hold` are not built, `quota` has no clean read. A goal lands whole, once.
run: metasystem work build G --work replay --brief FILE --check COMMAND
```

```
goal G has no Units table and no unit built with --last, so nothing says it is finished. A goal lands whole, once.
run: metasystem work build G --work NAME --last --brief FILE --check COMMAND
```

```
goal G already landed at 3f2a9c1d04be; a goal lands once, so the commits since then were not handed in.
run: metasystem goal done G --reason TEXT
```

**The next step after a unit.** One function answers "what comes after this unit" for all eight sites that print `work land` today (`cmd/metasystem/intent_selection.go:281`, `:285`, `:358`, `:627`; `cmd/metasystem/intent_unit_review.go:399`; `cmd/metasystem/intent_manual_submit.go:339`, `:363`; `cmd/metasystem/intent_delivery.go:1894`). It prints what `goalProgress` answers:

- a declared unit that is not built: `work build G --work UNIT --brief FILE --check COMMAND` (the form `work build` documents, `cmd/metasystem/intent_work.go:226`);
- a unit that is built and lacks its read: `work review G --work UNIT`;
- no end declared: `work build G --work NAME --brief FILE --check COMMAND`, with the reason "goal G has no Units table; this builds its next unit, and `--last` marks its last one";
- finished: the goal's one hand-in (question 1, decided as option A):

```
goal G is finished: every unit it declares is built and read. This is its one hand-in; `goal done` follows when it has landed.
run: metasystem work land G
```

For a goal whose reads are waived the same function runs: after the commit of a unit that is not the last, the two tier-1 sites (`intent_selection.go:281`, `intent_manual_submit.go:339`) print the next unit's build, never `work land`.

The hint at `intent_delivery.go:1894` stays: it follows a successful hand-in and repeats that same hand-in with `--delivered`. The prose at `cmd/metasystem/landing_plain.go:67` is rewritten by decisions 2 and 3. `work land` for a landed goal keeps naming `goal done` (`landing_plain.go:59`).

## Decision 2: `work land` never rebases (mechanism 9, the lane's lease)

`landGoalRoute` loses the rebase (`intent_delivery.go:1948-1966`), the lines it printed (`:1967-1994`) and the re-read of the branch after it (`:1995-2002`). `cmd/metasystem/landing_rebase.go` is deleted: what `landRebaseSkip` protected (a waiting hand-in's commits, a person's word at a tip) cannot be harmed by a verb that rewrites nothing. `work rebase G` stays as it is (`cmd/metasystem/intent_work_rebase.go:27`), a verb a person or a driver names.

**A returned conflict.** The lane keeps its conflict rule for a conflict with main: `landing resolve` regenerates generated paths and returns a source conflict with its record (`internal/landing/plain/resolve.go:70`; decision 3 says which conflicts are with main). The seat's `work land G` then shows the return and names one command:

```
goal G at 3f2a9c1d04be was returned: it does not merge with main (cmd/metasystem/intent.go: both sides inserted at one place).
run: metasystem work rebase G
```

`work rebase` runs the existing resolve round and carries the reads of unchanged units (`resolveRebaseRound`, `intent_work_rebase.go:139`; `carryReviewsWith`, `internal/goal/branch/rebase.go:263`). The rebased tip is a new commit, so the next `work land G` is a new hand-in. `returnedPathsResolved` (`landing_plain.go:73`) and the rebase argument of `laneQueueState` (`:40`) go; `--again` remains for a return that needed no change.

**The lease.** In 1a no model builds while the lane's lock is held: the lock covers the merge and the regeneration (`resolve.go:70`), a source conflict leaves the batch by its return, and the resolution is built on the seat. That is mechanism 9's rule in its smallest form. Moving the resolve round into the lane is deferred.

**The hand route.** A computer without a lane has no hand-in: `work land` lands the goal itself, by the replay of fact 8. In 1a that route keeps its composition, and three things are stated so that nothing about it is inferred:

- **What changes.** `work land` no longer rebases before the replay. The four refusals of the composition that say a commit no longer applies to main (`internal/goal/branch/land.go:272`, `:298`, `:678`, `:689`), and the one that says the change would land as other changes than were read (`:700`), name `metasystem work rebase G` as their one command; today they name `work review` or `work status`, which do not bring the branch onto main.
- **What holds on both routes.** `work land` never writes the goal branch, so its unit commits and their reads keep their identity until a person or a driver runs `work rebase`.
- **What differs on the hand route.** Main does not receive the unit commits themselves: it receives the landing commits the replay composes, one per unit, each checked to carry exactly the change that was read (`land.go:695-700`). A branch behind main lands untouched when main changed none of the files its units touch; otherwise it is refused and rebased by the named verb, where the reads of unchanged units are carried. That is stricter than a merge, and it is this route's existing contract.

Making the hand route integrate by a merge commit, as the lane does, replaces that composition, the landing commits' manifest and the two readers of it. It is designed in outline under question 4, with its size, and is not part of the units below unless Wido chooses it.

**The pinned test.** `TestWorkLandHandsInOverRealGit` (`cmd/metasystem/intent_delivery_owner_test.go:311`) asserts for a branch behind main that the tip was rewritten, that the rewritten branch contains main, and that the goal's history gained one rebase line (`:355-383`). Those three assertions are replaced by their opposites: the remote branch tip is unchanged, the queue line holds exactly that tip, and the history has no rebase line. The assertion "a hand-in moved main" (`:403-405`) stays and loses its `!behind` condition: with no rebase and no history line, a hand-in moves main in neither case.

This replaces one sentence of the accepted design `plans/designs/conflicts-resolve-unattended.md` (decision 2: "`work land` runs it first, refuses a branch behind main"). The rest of that design stands.

## Decision 3: every red and every return has a cause (mechanisms 3 and 2)

**The record.** One type in `internal/landing/plain`:

```
Cause { kind, goal, sha, name, tests[], evidence }
```

`kind` is one of mechanism 3's six words: `own`, `main`, `other`, `flake`, `environment`, `unclassified`. `goal` and `sha` name the hand-in an `own` cause is about. `name` is the test for a flake and the environment kind for an environment cause. `evidence` is the path of the log that shows it. `Result` (`prove.go:48`) gains `cause` on every red, and `goals`: the goal and sha of each merge in the checked tree, as `proving` already derives the goals (`internal/landing/plain/status.go:217`). `Line` and `Entry` (`queue.go:51`, `:79`) gain `cause` on a return.

A check's subject is a batch, so in a `Result` the kind `own` names the goal whose change it is. Read from another goal's line the same record is `other <goal>`; `landing status` prints it that way for the batch mates.

**One classifier, three callers.** The cause of a red is found in the check's own process, after the red and before the result line is written (`proveInWorktree`, `prove.go:632`). The same function serves the batch proof, the merge gate (decision 7) and the trunk check (decision 4). Each caller gives it the trees on which a failed unit can be run alone, oldest first:

| Caller | Trees |
| --- | --- |
| batch proof | `origin/main`, then each prefix of the batch in merge order (fact 7) |
| merge gate | the tree before the merge, then HEAD |
| trunk check | `origin/main` |

First match wins:

| What the check saw | Cause | What happens |
| --- | --- | --- |
| The check's process died, or the command reported it did not run (`prove.go:205`, `:669`) | `environment lost-process` | The tree's one repeat, when unspent. It counts as no attempt. Spent, or a second such failure: hold and ask. |
| The command exited red without a complete report (`internal/landing/plain/report.go:39`) | `unclassified` | Hold and ask. An adopter whose command only exits non-zero gets this, never a wrong return. |
| Every failed unit is one the change cannot affect and every failed test is a registered flake (the judge, `cmd/metasystem/intent_landing_flake.go:17`) | `flake <test>` | When the tree's repeat is unspent: today's repeat of those units alone (`prove.go:699-735`); green records the flake as today and the check is green. Spent, or red again: the rows below. |
| The failed units, run alone on the first tree, fail | `main` when that tree is `origin/main`; otherwise `unclassified` | `main`: decision 4 registers it and the batch holds. Nothing is returned. |
| The failed units, run alone on each later tree in order, first fail at the merge of goal G | `own`, goal G at its sha | G is returned. The rest is checked again. |
| The failed units pass alone on every tree | none yet | When the tree's repeat is unspent: the result carries `repeat: allowed`, and one repeat of the whole check on the same tree decides. Green: `flake`, recorded as today's whole repeat. Red: this table again with the repeat spent. Spent: `unclassified`, hold and ask. |

Each "run alone" is the existing call (`runCheck` with the unit as `LANDING_ONLY`, `prove.go:718`) in a worktree at that tree's commit, with its own log under the lane's `proofs/` folder. That log is the evidence. A run alone that does not complete ends the replay as `unclassified`. The runs alone attribute a red; they never turn it green and never grant a repeat. The observer never guesses `own`: an `own` is always "passes without this goal's merge, fails with it".

**One repeat per tree.** The allowance is the field that exists (fact 6): `repeat: allowed` is written only on a tree's first result, `repeat: started` spends it, and `checkBound` refuses any check after that. The classifier reads it before it grants anything, so a repeat is granted at most once per tree whether a dead process, a flake or the last row of the table asks for it. A full red, a red repeat and isolated greens therefore end as `unclassified` with a question, and no third execution of the whole command starts. The gate keeps its own allowance per tree in its own file (decision 7).

**The budget.** A batch gets at most two counted full proofs. A counted full proof is an execution of the whole command that ended with a complete report, green or red; a repeat after a check that did not run is not counted, and a repeat of units alone is not a full proof. The third start is refused where a proof starts, under the lane lock, beside `checkBound` (`prove.go:217`). A red full proof belongs to the open loop when its goals are a subset of the loop's first red proof's goals; a push closes the loop, and so does a person's `landing run`. A batch of one never reaches the budget; a batch of three whose first two proofs each exposed a culprit is held with a question before a third. Until decision 6 lands, the refusal names `metasystem landing run` and the landing agent asks through the skill's last case.

**A conflict is classified before anything is returned.** `landing resolve` keeps regenerating generated paths (`resolve.go:70`). For everything else it now answers two questions first.

*With whom does it conflict?* When HEAD is `origin/main` (the first merge of a batch), with main. Otherwise the verb asks Git whether the handed-in sha merges with `origin/main` alone (`git merge-tree --write-tree`, no checkout involved):

- It does not: the conflict is with main. The merge is aborted and the goal is returned with cause `own`, as today, and the conflict record's `main` is the commit of `origin/main`, not HEAD (`resolve.go:94`, `:170`). The seat's one command is `work rebase G` (decision 2).
- It does: the conflict is with a batch member merged before it. The merge is aborted and nothing is returned. The line stays waiting and gains `after`: the goals and shas merged in HEAD since `origin/main`. It is held (shown by `landing status` with that reason, skipped by the skill like any held line, decision 4) while one of those lines still waits. When they have landed or were returned, the line is merged again in the next batch; a conflict then is a conflict with main and is returned as one. The seat does nothing meanwhile, and `work land G` says so:

```
goal G at 3f2a9c1d04be waits in the landing lane. It conflicts with goal A, which is in the same batch; it is merged again when A has landed.
```

*What failed, when a regeneration fails?* Today every failure returns the goal (fact 12). Classified:

| What failed | Cause | What happens |
| --- | --- | --- |
| The regeneration did not end with an exit status of its own: its log could not be made, main's side could not be taken, the command could not start, or it was killed, by `landing stop` too (`resolve.go:220-245`; `commandExit`, `internal/landing/plain/regeneration.go:91`, answers below zero for a command that did not start and 128 plus the signal for one that was killed) | `environment lost-process` | The merge is aborted and the checkout restored, as today (`:185-204`). Nothing is returned. The line is retried once at the agent's next turn; a second such failure holds and asks. |
| The command ran and exited non-zero, and the same commands pass on the tree before the merge | `own` | Returned with the regeneration's log as evidence; the seat's command is `work rebase G`, whose resolve round runs the declared commands on the rebased sources. |
| The command ran and exited non-zero, and fails on the tree before the merge too | `unclassified` | Hold and ask: the generator fails without this goal. |

The run on the tree before the merge happens after the abort, in the same lock, and removes only the outputs of the sets it ran, by the rule the abort already follows (`resolve.go:185-200`).

**`landing return GOAL --cause CAUSE [--reason TEXT]`.** `--cause` is required (`cmd/metasystem/intent_landing_return.go:28`). Without `--reason`, the reason is the failing tests and the evidence path from the result. For the landing agent the verb refuses anything but a demonstrated `own`: the newest line that names this goal, in `results.jsonl` or in the gate's `gates.jsonl` (decision 7), must carry cause `own` for this goal at the sha of its waiting line. A red about an older hand-in of the goal therefore returns nothing. `landing resolve` writes its `own` with the conflict or regeneration record itself. A person's return, proven as every `--by` act is, takes any cause and always takes effect.

```
the lane's last proof shows main itself is red (internal/steward TestTick), not goal G; a goal goes back to its seat only for its own defect. Nothing was returned.
run: metasystem landing status
```

**The card.** A return writes `board.StageReturned` on the goal's live card, as `writeJoinedCard` writes `joined` (`landing_plain.go:147`). Because that stage is terminal, `writeJoinedCard` also takes the goal's card when its stage is `returned`; otherwise a second hand-in would leave the board saying returned while the goal waits.

**The skill.** `skills/landing-agent/SKILL.md` case 3 becomes: read `last_proof.cause`; for `own`, run `landing return GOAL --cause own`, merge the rest on latest main and prove; when `last_proof.repeat` is `allowed`, run `landing prove` once more; for anything else, end the turn. Case 4 no longer assumes a return: after `landing resolve` it reads the outcome (resolved, returned, or held with its reason). Case 7 no longer returns the waiting goals after a check that stopped twice: that is an environment cause, and it holds.

## Decision 4: main's red is an incident, and landing holds on it (mechanisms 4 and 6)

**Writing it.** Three finders, one writer. The batch's replay, the first merge gate of a batch and the trunk check all reach cause `main` through the classifier of decision 3, each with the log of the failed unit run alone on `origin/main`. All call one new seam beside the flake recorder (`landingFlakeRecorder`, `intent_landing_flake.go:100`), which calls `RecordTrunkRed` (`trunkred.go:538`) once, with one group per failing test: identity `red:<unit>:<test>`, the unit as the group, the test as its one failure, that log as `logPath` with its digest, the main commit and tree as the sighting's base, and the check's attempt as the batch the register asks for (`:540`). A unit that failed without naming a test (it did not build) is one group, `red:<unit>`.

**One entry per failing test.** The identity is the test, never the main commit and never the unit alone: a second, different failing test in a unit that already has an incident opens its own entry, with its own owner and fix goal, and is not hidden behind the first. The duplicate is suppressed where it is safe from a race, inside the publishing transaction: in the mutation of `trunkRedRecordRequest` (`trunkred.go:623-700`) a group whose identity already has an open entry of class trunk-red adds nothing, and when no group of the call adds or changes anything the mutation returns `AlreadyApplied`, the path a repeated operation takes today (`:629-631`), so no commit is made. A red that lasts a day is one entry and one commit per failing test, whoever finds it and however often main moves. A first red on main still turns an open flake entry of that identity into a trunk red, once (`:688-692`). The recorder reads nothing before it publishes.

**Clearing it.** Only a fresh full green clears (decision 7 defines it: the whole command ran on that exact tree in that attempt). It clears every open `red:` entry through `ClearTrunkRed` (`trunkred.go:744`, bound to each entry as read). There are two moments: the trunk check goes green on `origin/main`; or `landing push` puts a tree on main whose own newest result is a fresh full green. A scoped green, an inherited green and a line without a scope clear nothing, however recent the full proof they rest on; the next trunk check decides. `incident close` remains the person's way.

**The trunk check.** `landing prove --trunk` is the scheduled check's own path. It fetches and proves `origin/main` itself, in the proof's own worktree (`proveInWorktree` already makes one at the commit it is given, `prove.go:641`), so the lane checkout's HEAD is not moved. It is always a fresh full execution: it does not answer from an earlier green of that tree (`prove.go:280`, `:392`), does not inherit (`:428`), does not scope (`:436`), and is not refused because that tree's last result was red (`:217`). Its bound is its clock, below. Its result line carries `trunk: true`. A red goes through the classifier with `origin/main` as its one tree; a green clears, above. One check runs at a time, as today (`Busy`, `prove.go:171`).

**The clock.** `proof.trunk-every` (a duration, default `4h`) is a new settings key. `fullProofDue` (`internal/landing/plain/wake.go:66`) keeps its present rule, the full proof a scoped push owes within the hour, and adds: due when neither a trunk check, green or red, nor a push of a tree with a fresh full green of its own ended within `proof.trunk-every`. Skill case 8 becomes: woken with `full-due` and no line you may merge (nothing waits, or every waiting line is held), run `landing prove --trunk` and end your turn. While main is red the check therefore runs again every interval, on an unchanged tree too, and clears the incident by evidence. A line that may be merged goes first; its own fresh full proof, once pushed, pays the clock.

**What `incident list` shows.** The entry, as today (`intent_planning.go:2168`), plus the failing test and the evidence path on its line. The steward's health role already turns an unowned entry into an alert that names `incident claim` (`internal/steward/trunkred.go:45`); nothing more asks.

**The seat's hold.** `work land G` refuses while main's register, as the seat's projection reads it (`projection.Tree.TrunkRed`, as at `intent_planning.go:2177`), has an open entry of class trunk-red. It passes when G is the fix goal of an open entry (`incident claim ID --goal G` sets it, `trunkred.go:892`). The refusal reuses the registered code `GOAL_LAND_TRUNK_RED` (`internal/goal/branch/red.go:13`):

```
main is red since 14:05 (incident red:internal/steward:TestTick), and landing holds until it is fixed. Nothing was handed in.
run: metasystem incident list
```

**A person's override.** `work land G --exception GOAL_LAND_TRUNK_RED --reason TEXT --by NAME` hands G in with the exception on its queue line. For this code alone, and only where a lane is registered, the exception form is a hand-in. The line passes the hold once: it ends when that line lands or is returned. The incident stays open.

**The lane's hold.** A waiting line is held while an open red entry exists, unless its goal is a fix goal or its line carries the exception; and while a batch member it conflicts with still waits (decision 3). `landing status` shows the reason on each held line, and the skill merges only lines that are not held. The guard is in code where it cannot be skipped: `landing push` refuses, through the `before` hook `PushChecked` already has (`internal/landing/plain/push.go:90`), a HEAD that merges a held goal. The lane reads the register from `origin/main` through a seam the command supplies, as the flake judge does (`intent_landing_flake.go:51`). The wake counts only lines that are not held (`wakeReasons`, `wake.go:40-53`): a queue of held lines gives no `queued` reason, so the agent is not launched to do nothing and the keeper's barren count does not run; `full-due` is what wakes such a lane.

## Decision 5: a goal waiting in the lane frees its seat's claim (the quota)

The lane's queue is the authority for "this goal is in the lane" (the plan, overlaps). The ledger is not written at hand-in. The seat's next claim reads the queue and does the bookkeeping in its own commit.

`goal claim` passes the claim transaction the newest lane state of each goal its machine holds, read through `laneCheck` and `laneInstallOf` (`intent_delivery.go:1751`, `landing_plain.go:25`) and `latestLaneEntry` (`:171`). `claimQuotaRefusal` (`internal/goal/verbs.go:1390`) then treats a held goal by that state:

- **waiting:** not counted. If the goal is not yet in the land-ready slot, the same commit that makes the new claim enters it there: the record `work land --queue-only` writes today (`verbs.go:2274`). A seat can reach this exact ledger state by hand today, so the at-rest rule, the board and every other reader of that record already know it.
- **returned:** counted, even with the slot record present. The refusal names the goal and `work land G`, which shows the return.
- **landed:** counted until concluded. The refusal names `goal done G --reason TEXT`.
- **no lane entry, or no lane on this computer:** today's rule.

The at-rest rule "one landing slot per machine" (`validate.go:461`) stays. The consequence is a bound, stated here as the smallest assumption: a seat has at most one goal waiting in the lane while it works the next. A seat with two finished goals waiting is refused a third claim, with both named. Lifting that bound belongs with the lane's policies in 1b.

This meets the intent of the parked unit (`origin/parked/m04-handin-leaves-quota-av`) without its `RecordHandIn`: no commit per hand-in, and "a hand-in never moves main" (`cmd/metasystem/intent_delivery_owner_test.go:403`) stays true.

## Decision 6: the lane's stop is a record, and its question is one command (mechanisms 2 and 12)

**The stop record.** One JSON line per decision in the lane's records, `artifacts/agents/landing/stops.jsonl`, in mechanism 2's shape: loop, subject, attempt n of budget, measure (name, previous, now), class, decision, handoff, cause, evidence, at.

| Loop | Subject | Budget | Measure | Class | Handoff at a stop |
| --- | --- | --- | --- | --- | --- |
| `lane-proof` | the batch's goals, or `main` for the trunk check | 2 counted full proofs | the red set shrinks | the failing test | `return <goal> own`, `hold <incident>`, or `ask <question>` |
| `lane-return` | the lane | 2 launches (`barrenLimit`, `internal/landing/lane/agent.go:42`) | the lane's fingerprint changes (`wake.go:139`) | "lane unchanged" | `ask <question>` |

The check's process writes a `lane-proof` line at every red, when the cause is known and before anything composes the next attempt, and a `close` line at the push that ends an open loop. A red gate and a failed regeneration that hold write the same line, with their own cause and evidence and without counting an attempt. The keeper writes the `lane-return` line where it holds today (`barrenHold`, `agent.go:338`); the hold itself and what clears it do not change. `landing status` shows the newest stop that no later line closed, in two lines: what stopped and why, then the one command.

```
the lane stopped: 2 full proofs of this batch went red and the second showed no smaller red set (internal/steward TestTick). Goals a, b wait.
run: metasystem landing return GOAL --cause own --reason TEXT
```

**The question.** A stop whose handoff is `ask` puts one question to the person through the channel, the way `question ask --about lane` does. Its first line is the command that resolves the stop; its facts are the stop line and the evidence path. For an unclassified red the command is the return above, with the batch's goals listed for the person to choose from, pre-filled when the replay or a failed regeneration points at one goal. For an environment cause that repeated, and for the barren hold, it is `metasystem landing run`. The keeper's existing hold waits on it (`questionHold`, `cmd/metasystem/landing_agent.go:185`).

The act closes the question, not an answer: at its tick the keeper withdraws the lane's own question once a later record ended the stop (a return, a hand-in, a push, a person's `landing run`). This is the minimal ask record of mechanism 12; its fields are the channel question's own. Routing to the coordinator seat first is the remedies goal's.

A stop for `main` asks nothing: its handoff is `hold <incident id>`, and the incident has its own alert.

## Decision 7: the proof ladder's keys (mechanisms 5 and 6)

**The keys.** `proof.full` and `proof.cheap` join the settings table (`internal/config/defaults.go:62`) as repository declarations with no default. They are read from the committed file only, with the call that already does that (`config.CommittedLookup`, `internal/config/resolve.go:456`). `settings set` writes them to `metasystem.conf`. `settings show` names the source and says when a local or environment value is ignored. `settings check` validates that each is one line and reports a required one that is missing. `system adopt` removes the template's own `proof.*` lines from the adopter's file, as it removes the template-mode line (`dropTemplateMode`, `internal/adopt/adopt.go:730`), so the adopter's keys are explicitly missing until supplied.

**The batch proof and the trunk check.** `landing prove` reads `proof.full` where it reads `landing.prove.command` today (`intent_landing_prove.go:123`). The lane checkout's HEAD is the merged batch, so the command is the one the proven tree declares; `--trunk` reads it from `origin/main`'s tree. The key `landing.prove.command` is removed (`intent_landing_prove.go:25`, `internal/config/read_settings_generated.go:28`, `docs/concepts.md:153`); a lane that still sets it is told the new key by name. The command's contract does not change: the `LANDING_*` environment and the report lines of `report.go`.

**What counts as the full rung.** Declaring the command is not enough, because the lane decides how much of it runs (fact 9). Three words, each read from the result line that exists:

- A **fresh full** result: the whole command ran on that exact tree in that attempt. On the record this is `scope: full`, which the lane writes only when it asked for everything (`describe`, `scope.go:363-374`). A line without a scope is not one.
- An **inherited** green: nothing ran; the tree differs from a fresh full green under an hour old only in ledger paths (`ledgerOnlySinceGreen`, `prove.go:483`). This stays, and it is the only reuse without a run.
- A **scoped** green: only the groups whose declared inputs cover the changed paths ran. From this goal on it is allowed only when every path changed since the fresh full green is a record or ledger path, the classes the records hand-in already tests for (`recordsOnlyPaths`, `scope.go:178`). `decideScope` (`scope.go:47`) returns `full`, with the reason "an executable input changed" and the path, as soon as one changed path is of any other class; the closure and coverage rules behind it stay as further reasons for `full`.

So: a tree whose executable inputs changed since its last fresh full green is proven whole before it is pushed; a push may rest on an inherited or a records-scoped green for an hour, as today; the trunk check is always fresh full; and only a fresh full green clears an incident (decision 4). This narrows step 1 of the accepted design `plans/designs/lane-reproves-only-what-a-change-can-affect.md`: after main moves under a proven batch by a file no code unit owns that is not a record (a script, a fixture), the batch is proven whole again instead of by its covering groups. In this fleet main moves under a batch almost only by ledger and record commits, which keep their shortcut.

**The merge gate.** `landing prove --gate` runs `proof.cheap` on HEAD the way a proof runs, with the tree before the merge as `LANDING_PROOF_BASE`, and writes its result to `gates.jsonl`, a file of its own. `landing push` reads `results.jsonl` alone (`provenGreen`, `push.go:105`), so a gate's green can never satisfy a push. The skill's step 3 runs the gate after every merge.

A red gate is not yet anyone's. It goes through the classifier of decision 3, with the tree before the merge and HEAD as its two trees, and its own repeat allowance per tree in `gates.jsonl`:

- a gate whose process was lost, or that reports it did not run, is `environment`: one repeat, no return;
- a red whose failed tests are registered flakes the merged goal cannot affect is repeated alone; green records the flake and the gate is green;
- failed units that fail alone on the tree before the merge are not this goal's: `main` when that tree is `origin/main` (registered by decision 4, so main's red is found at the first gate, in minutes), otherwise `unclassified`;
- failed units that pass alone before the merge and fail alone after it are `own` for the goal just merged: the agent steps back to the tree before the merge, returns the goal with `landing return GOAL --cause own`, which accepts this gate result (decision 3), and goes on with the next line;
- anything else holds and asks.

The baseline is a green gate recorded for the tree before the merge. When none is recorded (the first merge of a batch, on a main the gate has not seen), the gate runs there first. A command that does not write the report lines still gates, and every red of it is `unclassified`.

**Missing keys.** Without `proof.full`, `landing prove` refuses and names the key. Without `proof.cheap`, the gate is refused the same way.

**This repository's own declarations.** The machinery above does not depend on what the two commands are. For this repository the committed lines hold commands that run on any checkout of it, so no seat proves a different "full" than another:

- `proof.full` names a committed script (proposed path `proof/full.sh`, plumbing only). It runs the suite inside a virtual machine only when a computer fact names one, and the plain package suite on the host otherwise; both write the report lines. The fact is a new `host.*` key (proposed name `host.proof-vm`) in the lane checkout's local settings, where mechanism 5 puts computer facts. The script asks `settings show` for that one key and never reads the local file, which holds secrets.
- `proof.cheap` is `metasystem test run`, wrapped only as far as the report lines need.

The exact commands, the path and the key's name are Wido's to confirm (question 2). Unit `repo-proof` commits them and is built last among the ladder's units, so nothing else waits on the answer.

## Moved effects

| Effect | From | To |
| --- | --- | --- |
| Deciding a goal is ready to land | whoever ran `work land` | the design's Units table, or the unit built with `--last`, checked by `work land` |
| Saying what comes after a unit | eight sites, each with its own rule | one function over the same predicate as the hand-in |
| Integrating a goal with main, in the lane | the seat's rebase before every hand-in | the lane's merge commit |
| Bringing a branch onto main, on the hand route | a rebase inside every `work land` | `work rebase`, named by the refusal when the replay needs it |
| Finding which goal broke a batch | the landing agent, by hand (skill case 3) | the check's own process |
| Deciding who gets a red back | the landing agent's free text | the cause, checked by `landing return` |
| Deciding whose a conflict is | `landing resolve`, always the goal's | `landing resolve`, by asking whether the goal merges with main alone |
| Recording a red on main | nobody | the lane's checks, into the register, once per failing test |
| Asking about a red on main | the landing agent (skill case 8) | the incident and its health alert |
| Checking main on a clock | the skill, only when nothing waits | `landing prove --trunk`, when no line may be merged |
| Freeing the claim slot of a waiting goal | a person, or `work land --queue-only` by hand | the next `goal claim` |
| The command that proves a batch | each lane's uncommitted setting | the repository's committed `proof.full` |
| Saying the lane is stuck | a line in the keeper's log | a stop record and one question |

## Units

| Unit | Content | Lines |
| --- | --- | ---: |
| admission | Decision 1: `goalProgress` over the Units table, the refusals of `work land`, `--through`, lands once | 110 |
| next-step | Decision 1: the next-step function at the eight sites, the finished goal's line | 70 |
| boundary | Decision 1: `--last` on both build routes, the `Goal-Whole` trailer and its reading, a person's `--whole` | 70 |
| no-rebase | Decision 2: remove the rebase from `work land`, the conflict return's next step, the hand route's five refusals name `work rebase` | 75 |
| cause | Decision 3: the record in `Result` and the queue line, `goals` with shas, `landing return --cause` and its guard, the returned card, skill cases 3 and 7 | 120 |
| replay | Decision 3: the classifier and its table in the proof's process, one repeat per tree, the two-proof budget | 125 |
| conflict | Decision 3: conflicts with main or with a batch member, `after` on the line, regeneration failures by class, skill case 4 | 95 |
| incident | Decision 4: the recorder, one entry per failing test, no commit for a repeat sighting, clearing on a fresh full green, the `incident list` line | 120 |
| hold | Decision 4: the seat's refusal, the fix goal, the exception line, held lines, the wake, the push guard | 120 |
| fresh-full | Decisions 4 and 7: `landing prove --trunk`, `proof.trunk-every` and the clock, skill case 8, `decideScope` proves whole when an executable input changed | 90 |
| quota | Decision 5 | 80 |
| ladder | Decision 7: the two keys, committed-only, `settings set/show/check`, `system adopt`, `landing prove` reads `proof.full` | 95 |
| gate | Decision 7: `landing prove --gate`, `gates.jsonl`, the baseline, the classifier as its second caller, skill step 3 | 90 |
| repo-proof | Decision 7: this repository's committed script and its two declarations (question 2) | 45 |
| stop | Decision 6 | 95 |

Changed production lines, estimated: 1,400 in fifteen units, against 940 in nine before the critique and the brief's 690 in seven. The growth is the eleven findings: a declared end for goals without a table, conflicts and regenerations that are classified, a trunk check with a path of its own, a gate that attributes before it returns, and three units cut in two so that none is over about 130 lines (`admission` into three, `incident` into `incident` and `fresh-full`, `ladder` into `ladder`, `gate` and `repo-proof`).

Build order as listed. Each unit has a consumer the day it lands: `admission` checks goals with a table and leaves the others under today's rule until `boundary`; `replay` works with today's proof command and does not wait for `ladder`; `incident` is written by the batch's replay before `fresh-full` adds the scheduled finder and `gate` the early one; `stop` turns the refusals and the skill's asking of the earlier units into records.

**One test through the public verb per unit**, in the command package, with Git stubbed through the existing seams (`ProveSeams`, `ResolveSeams`), as the local rules require:

1. admission: `work land` on a goal whose design declares three units and whose branch has two is refused and names the third; with three it hands in; `--through` of the first unit is refused; a new commit after it landed is refused. A tier-1 goal with a table of two units and one built is refused; with both built it hands in without a read.
2. next-step: `work review` of the second of three units prints the third's build as the next step; of the third it prints `work land G`. For the tier-1 goal, the commit of its first unit prints the second's build and not `work land`; the commit of the second prints `work land G`.
3. boundary: a goal without a Units table with one unit built without `--last` is refused by `work land`, which names the build with `--last`, and the next step after that unit is the next build; with a unit built with `--last` it hands in; `work land G --whole --by NAME` hands in the unmarked branch and the queue line names who declared it.
4. no-rebase: on the lane route `work land` on a branch behind main queues exactly the branch's tip, rewrites no commit, adds no rebase line to the goal's history and does not move main; a returned conflict names `work rebase`. On the hand route a branch behind main whose units touch nothing main changed lands with the branch's commits unchanged; one that touches a file main changed is refused, names `work rebase G`, and nothing is rewritten.
5. cause: `landing return G` without `--cause` is refused; with `--cause own` after a red that names G at its waiting sha it is recorded on the line and the card says returned; when the last proof says `main` it is refused; when the red names G at an older sha than the waiting line it is refused; a second hand-in moves the card to joined.
6. replay: `landing prove --wait` over a batch of two with a command that fails one unit only when the second goal's file exists records `own` for that goal with the unit's own log; failing on main alone records `main`; a full red, then a red repeat, then units that pass alone records `unclassified`, the whole command ran exactly twice, and a third `landing prove` is refused.
7. conflict: of two branches that each merge cleanly with main and conflict with each other, the second is not returned and is held after the first; once the first is pushed the second is merged again, conflicts with main and is returned as `own` naming `work rebase`. A generated-only merge whose regeneration cannot start is aborted, nothing is returned, the line still waits, and `landing status` shows the cause `environment` on it; a second failure of the same kind holds with a question. A regeneration that exits non-zero and passes on the tree before the merge returns the goal as `own`.
8. incident: after a red check of `origin/main`, `incident list` shows one entry per failing test; a second, different failing test of the same unit found later opens a second entry; the same red found again after a ledger-only move, and by a second finder, leaves main's tip unchanged; a fresh full green clears; a recent scoped green and an inherited green clear nothing.
9. hold: `work land G` is refused and names the incident; after `incident claim ID --goal G` it hands in; with `--exception GOAL_LAND_TRUNK_RED` it hands in once; `landing push` refuses a HEAD that merges a held goal; a queue of only held lines gives `landing status` no `queued` reason.
10. fresh-full: with a held line waiting and `proof.trunk-every` passed, `landing status` reports `full-due`; `landing prove --trunk --wait` runs the whole command on main's unchanged tree when its last result was green, and again when it was red, where today the first is answered from the record and the second refused; the green one clears the incident and the line is held no more. A batch whose tree moved by a script after its full green is proven whole; one that moved by a record runs only the covering groups.
11. quota: `goal claim` of a second goal succeeds while the first waits in the lane, in one commit that also enters the first in the land-ready slot; after `landing return` of the first, a third claim is refused and names it.
12. ladder: `landing prove` runs the committed `proof.full` and ignores a local value, which `settings show` says; `settings check` reports a missing key; an adopted file has no `proof.*` line.
13. gate: after a green baseline, a gate whose process is lost is `environment`, repeated once, and nothing is returned; a red whose failing test is a registered flake the merged goal cannot affect is repeated alone, recorded, and the gate is green; a red that fails alone after the merge and passes alone before it is `own` for the merged goal, and `landing return G --cause own` accepts it; `landing push` refuses a HEAD that has only a gate's green.
14. repo-proof: `settings check` passes in this repository; `landing prove --wait` in a lane whose local settings name no virtual machine runs the package suite and ends with the report lines.
15. stop: after the second red, `landing status` shows the stop and its command; `question show` prints the same command; after `landing run` the question is withdrawn.

Units that edit the skill or a message run the instruction and message audits before their proof.

**Tests that pin today's behaviour and change:** `cmd/metasystem/landing_plain_handin_test.go` (hand-ins of part of a goal), `cmd/metasystem/landing_rebase_test.go` (replaced by the no-rebase test), `cmd/metasystem/intent_delivery_owner_test.go:355-383` (the three assertions that a branch behind main was rewritten, replaced as decision 2 says), `internal/goal/branch/refusal_words_test.go` (the hand route's refusals), `cmd/metasystem/landing_plain_cards_test.go` (the returned card), `cmd/metasystem/landed_notice_test.go`, `cmd/metasystem/intent_records_test.go`, `internal/goal/landing_test.go` (the claim's new input), `internal/goal/trunkred_test.go` and `cmd/metasystem/goal_trunkred_test.go` (a sighting per call), the lane's resolve tests (a failed regeneration returns) and scope tests (a scoped proof after a non-record change). In `cmd/metasystem/intent_delivery_owner_test.go` the assertion "a hand-in moved main" (`:403-405`) stays and now also covers the branch behind main; `internal/goal/validate_test.go` must pass unchanged.

## Estimates

About 70 minutes of machinery time per unit when the first read is clean (a 20 to 30 minute build, a 10 minute proof, a 20 minute read); a correction adds about 50. Fifteen units: 17.5 hours clean, about 22.5 with six corrections, one goal in flight.

Against the brief's proposed box (3d/40/3000m/2/20: three working-hour days elapsed, 40 attempts, 3,000 reserved minutes, 2 active jobs, 20 review rounds): the minutes hold (about 1,350 of 3,000) and so do the attempts (about 21 of 40). The elapsed time is close (22.5 of 24 working hours), and the review rounds do not hold once more than five units need a correction (fifteen first reads plus one per correction, against 20). The box is Wido's to set (question 5).

## Where this page departs from the mechanisms page

Decided with this revision (the second brief): departure 1 is replaced by the test-level identity; departures 2 and 3 are accepted. For the mechanisms page's next revision:

1. **The key of a red on main** (mechanism 4 says `(main commit, test)`). Here the key is the test alone, as unit and test name; the main commit is the entry's first sighting. Keyed by commit, every ledger commit under a standing red would open another incident.
2. **Where a stop record lives** (mechanisms 2 and 4 say a stop is a register entry of kind `stop`). Here the lane's stops are lines in the lane's own `stops.jsonl` with the register's fields. Putting them in the ledger would move main at every stop. Where a unit's stop lives is the review-chain goal's to decide.
3. **`own` in a batch's result** names the goal; `other <goal>` is how the same record reads from a batch mate's line. The page lists both as values of one field.
4. **The batch proof "on the exact tree it pushes"** (mechanism 6). Here a push may also rest on an inherited green across ledger-only changes, or on a records-scoped green, for an hour, as the lane does today; decision 7 says exactly when, and that nothing but a fresh full green clears an incident. This one is new in this revision and is put to Wido with the others when the mechanisms page is revised.

## Deferred

- The lane's four policies, the helm, `landing drain`, areas on the claim: design 1b.
- More than one goal of a seat waiting in the lane: 1b.
- A bound on returns per goal (the plan's "2 returns per goal"): with causes, only a goal's own defect returns it; the bound belongs with the on-red policy in 1b.
- The resolve round inside the lane's merge commit, and a reuse store for resolutions.
- Clearing one incident by a red check in which its test passed: the report names failures only, so 1a clears on a fresh full green or by a person.
- Proof deadlines, process groups and the host record: the fleet goal.
- The carried exception route for every other code: untouched.

## Acceptance of the goal, by decision

| The brief's acceptance | Met by |
| --- | --- |
| An unfinished goal cannot be handed in; the refusal names the unit; the next step is the next unit, and for a finished goal its one hand-in | Decision 1 |
| A hand-in rewrites no unit commit; reads survive integration | Decision 2: in the lane by the merge commit; on the hand route `work land` writes no branch commit either, and what reaches main is checked against each read (question 4 for a merge there) |
| Every return carries a cause; main's or another goal's red never returns the goal; main's red is an incident and landing holds except for its fix goal | Decisions 3 and 4 |
| A seat whose goal waits can claim its next; a returned goal holds the slot again | Decision 5 |
| The full `cmd/metasystem` package is green on the integrated tree | The whole-goal proof at hand-in, by the lane's `proof.full`, as a fresh full execution |

## Open questions for Wido

**Decided by the second brief, kept here for the record.** Question 1 (which command finishes a goal): option A, the finished goal's next step is `work land G` and `goal done` follows the landing; decision 1 carries the line. Question 3 (the departures): 2 and 3 accepted, 1 replaced by the test-level identity.

**2. This repository's own `proof.full` and `proof.cheap` (decision 7).** Decided by Wido on 2026-10-06 ("agreed on all"): option A. The lane's proof command lives today in the lane checkout's uncommitted settings, which hold secrets and which neither this design nor a builder may read. The design no longer depends on the answer: the keys, the gate and the trunk check are built against any declared command, and unit `repo-proof` alone commits this repository's two lines.

- **A.** As decision 7 proposes: `proof.full` is a committed script that runs the suite in the virtual machine a `host.proof-vm` fact names, and the plain package suite where none is named; `proof.cheap` is `metasystem test run`. Wido confirms the two commands, the script's path and the key's name; m1e supplies the lane's current command as the script's virtual-machine branch.
- **B.** Wido names other commands. They must run on any checkout of this repository.

**4. Should the hand route integrate by a merge commit too (decision 2, Astra's finding 4)?** Decided by Wido on 2026-10-06: option A, not in this goal. New in this revision, and a question of this goal's size, which is why it is asked and not decided here. Today that route replays each unit as a new landing commit and verifies it against its read (fact 8).

- **A.** Not in this goal. 1a makes the route explicit and coherent without the rebase, as decision 2 says (about 15 of unit `no-rebase`'s 75 lines); a merge there becomes a goal of its own if lane-less computers need it. Recommended: the charter is about the lane, this fleet's computers land through one, and the replay already guarantees that what lands is what was read.
- **B.** In this goal, as two more units after `no-rebase`, about 210 lines. `hand-merge` (about 120): `PrepareLanding` composes one `git merge --no-ff` of the branch tip onto main's tip in its scratch worktree, with the merge drivers it already loads (`land.go:580`), in place of the per-unit loop (`:659-723`); the one landing commit is the merge, authored as today, carrying the receipt row; a conflict refuses and names `work rebase G`; the preimage and digest checks go, because the unit commits land as themselves. `hand-landed` (about 90): the sweep, the last-landing check and the verifier recognise a merge landing by ancestry instead of by the `Goal-Source`, `Goal-Digest` and `Goal-Last` trailers (`sweep.go:97-109`, `verify.go:20-58`, `landing_event.go:19`, `cmd/metasystem/goal_branch.go:340`). It replaces most of the tests of `internal/goal/branch/land*_test.go`, and adds about 2.5 hours clean.

**5. The box (Estimates).** Decided by Wido on 2026-10-06: the box is raised to 4d with 30 review rounds; one goal, fifteen units. Fifteen units no longer fit the proposed 20 review rounds once more than five units need a correction, and leave about 1.5 working hours of the three days. Either the box is raised when the goal is approved (for example 4d and 30 review rounds), or the goal is cut after `quota` into two goals, the second holding `ladder`, `gate`, `repo-proof` and `stop`. No recommendation changes what is built; the first is one approval, the second lands the first eleven units sooner.
