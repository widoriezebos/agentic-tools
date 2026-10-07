# Machinery findings still open, and their solutions

Status: accepted
Critique: with `plans/designs/machinery-mechanisms.md` as one set, two Astra rounds on the set (12 material, then 6 with 4 class repeats), all folded; stopped under the stop rule of this plan. Open: Wido's naming pass; the headless-session ruling (finding 7).
Written: 2026-10-06 05:10 CEST by m1e in Wido's word; revised 09:15 after the week's deep dive (`agentic-tools-evidence/night-review-20261003/week-deep-dive-2026-09-29-to-2026-10-06.md`), which mined main's history, the goal ledger, the lane's queue, the launch and unit records and every seat's session transcripts. Every number here comes from that mining.
Sources: the deep dive; `assessment-2026-10-05.md` (sections 2 and 4); the decisions log of 10-04 to 10-06; the unit reads of the -04 and health goals; the landing discussion of 10-06 04:50.

This page lists every machinery finding that has no solution or only a partial one, in plain words, ordered by the harm it does. For each it designs the solution. The designs are generic: they must fit any application the metasystem runs, so nothing in them names Go, a package, or this repository's tests. Where a solution needs something only the adopter knows, it is a declaration the adopter writes, and the machinery reads.

## How the order was made

Worst first. A finding ranks by what it costs when it strikes, under the circumstances in which it strikes:

1. Main red or work lost, every seat builds on a broken base.
2. An unattended run or the whole fleet stalls: nothing moves until a person looks.
3. Wrong work is accepted, or right work is refused.
4. Hours and tokens wasted on rounds that change nothing.
5. Noise and confusion a person has to clean up.

## Principles every solution follows

- **Declarations, not code.** Anything application-specific (how to run the cheap proof, the full proof, the audits; which paths a goal owns; how many builds a host carries) is a declaration in the adopter's repository. The machinery runs what is declared and never guesses.
- **Three values for every policy: auto, capped, person.** `auto` means the machinery decides by rule. `capped` means auto within a number a person set. `person` means the machinery prepares and waits for a person's act. Taking the helm sets every policy of the seat (and of the lane) to `person` at once; releasing it restores the configured values. The helm is therefore not a separate mode of any mechanism; it is the person value on all of them.
- **No HAL 9000.** A person's act always takes effect at once; the machinery may say why it would have decided otherwise, never refuse.
- **The record holds the outcome, never a person's memory.** Every decision the machinery or a person takes (a unit stopped, a finding split, a goal dropped from a batch, a risk accepted, a seat paused for a limit) is a word in the record that the next verb reads and the UI shows.
- **An environment failure is never a round.** A provider limit, a lost process, a denied sandbox, a busy path, a host over capacity: the machinery records the cause, retries once on its own, and stops with the reason on the second failure. It never counts against the unit's rounds or the goal's box.
- **Every loop has a stop.** Any activity the machinery repeats declares its attempt budget, its progress measure, its class-repeat detector and its handoff (the section below); a loop without a declared stop does not ship.
- **Smallest step first.** Each solution names the first unit that gives most of the value.

## A stop criterion for every loop

The rule m1e applied by hand from 10-05 11:20 (one build, at most two corrections, material must fall every round, a repeated class stops the unit whatever the count, decide the stop when the evidence arrives and before composing the next attempt, then drop or split and never "one more round") is not a rule about unit reviews. It is the shape of a stop for anything the machinery repeats, and the week shows the same loop without a stop in at least nine places. The machinery gets one primitive and applies it in each.

**The primitive.** Every repeated activity declares:

1. **An attempt budget**: how many attempts the loop may take before it must stop (two corrections after the first read; one environment retry; two full proofs per batch; two revivals an hour; one raise per unit).
2. **A progress measure that must change every attempt**: the material count falls; the set of red tests shrinks; the cause differs from the last attempt; the diff differs; the finding class is new. An attempt whose measure did not change is the last one.
3. **A class-repeat detector**: the same class twice (the same test red, the same conflict hunk, the same refusal code, the same finding class in a new place, the same environment cause) stops the loop at once, whatever the budget says.
4. **A decision before the next attempt**: the stop is decided by the machinery at the moment the evidence arrives (the read's return, the proof's result, the revival's outcome) and recorded as a word, before anything composes the next attempt.
5. **A handoff at the stop, never a retry**: the loop ends in one of split, drop, return, hold or ask, each a recorded outcome the next level reads (the goal, the lane, the helm holder, the person). The handoff names the evidence list as the next brief.

Every stop is a policy with the three values: auto (the primitive decides), capped (a person's numbers), person (the machinery stops and asks at every attempt). The helm sets person.

**Where it applies, and what the week paid without it:**

| Loop | Budget | Progress measure | Class repeat | Handoff | The week |
| --- | --- | --- | --- | --- | --- |
| Unit correction rounds (finding 6) | 2 corrections | material count falls | finding class repeats in a new place | split or drop | 18, 10, 10, 9, 6 rounds on single units |
| Design critique rounds (finding 20) | rounds continue while the last round found material; at most 4 rounds | the material count over the whole page falls every round; a round with 0 material accepts the design | a class already fixed recurs in the same section | accept; or accept the sections clean in the last round and split the sections still carrying material into a follow-up goal with its own design loop, queued next in the plan; after the last round, every remaining finding that names a concrete change is folded and becomes an acceptance item of its unit, checked by that unit's code read (as the code stop rule carries findings into a fix unit), and only a section whose finding names no concrete change, or whose clean sections cannot be built without it, leaves the goal; and at any round, when a split would leave no buildable unit (the clean sections depend on a section carrying material), the remaining findings that name concrete changes are folded as acceptance items and the design is accepted; never ask a person (refined 2026-10-07 on 1b and on lane-reads-its-policies, where a literal split left nothing buildable) | 6 critique rounds on one design; "17 critic rounds"; 1a stopped after 1 round with 11 findings and paid about 6 material code-read findings that traced to the design |
| Lane re-proofs and returns (findings 1, 2) | 2 full proofs per batch; 2 returns per goal | the red set shrinks; the cause class differs | same test red, same hunk | hold the batch and ask; return names the cause | a goal returned 8 times; proven red three times on the same test; 5 re-proofs of one green branch |
| Environment retries (findings 5, 14) | 1 retry | the cause differs | same cause twice | stop the unit with the cause | 7 read failures in a row on one path; capacity kills counted as rounds |
| Steward revivals and restarts (findings 4, 7) | 2 an hour | the seat makes progress after revival | same death reason | stop the seat and alert | 15 failed revivals of one seat; 71 seat restarts in a day |
| Budget raises (finding 10) | 1 auto raise per unit | a unit was admitted since the last raise | a raise for the same unit twice | ask | 2 to 7 raises per goal; four by hand in a night |
| Hot-fixes (finding 2) | 1 per red | main is green after it | a hot-fix to a hot-fix | the second fix is a goal with a design and a review | push 49 needed push 50; three hot-fixes in ten hours |
| Fix-forward on one component | N fixes in a day without a green end-to-end run (N from the tier) | an end-to-end run passes | the same component again | step back: open a design | the lane of 09-30: 11 stacked defects, 14 fixes, 5 arms, nothing landed |
| Refusals that need a person (finding 11) | N per hour across seats | the refusal code differs | same code again | file a machinery finding, route to the helm holder | 6 person-needing refusals an hour at night |
| The helm holder's own supervision | hourly judgment | the judgment names what changed | the same intervention twice | ask why the steward did not; open a finding | m1e's rounds beyond "last round"; slices by habit |

**What counts as material in a design round, decided by the critic's return, not by judgment.** A finding is material only when it names the section or unit and the concrete change to what is built or to a test; a finding without one is recorded and does not count. Every round answers the same five questions for every new function, record and act: who calls it in production; how fresh is every state a decision reads; is the actor a person or an agent; can every refusal's remedy succeed when followed; does every unreadable input fail safe for the agent and never refuse a person. An unanswered question is a material finding. (Wido 2026-10-06 22:40 CEST: the design stop is an automated rule, not a question to him.)

**What this changes in the findings above.** Finding 6 holds the primitive and the review loop; findings 1, 2, 4, 5, 7, 10, 11, 14 and 20 each declare their budget, measure, class and handoff in the terms of the primitive rather than their own counters. One implementation, one record shape (`stop: <loop> <attempt> <measure> <class> <handoff>`), one board column.

## The findings, worst first

### 1. The landing lane admits unfinished goals and fights itself

**Problem.** `work land` hands in whatever the goal branch holds. Nothing checks the branch against the design's list of units. After every unit read the machinery prints `work land` as the next step, so an agent lands slices. Before each hand-in the seat rebases its branch, which rewrites every unit commit and voids the reads bound to them. Several goals editing the same files then re-open each other's conflicts at every landing.

**Impact and circumstances.** In four days the lane took 102 hand-ins for 28 goals, landed 53 and returned 45: a return rate of 44 %. On 10-03 and 10-04 it returned about as many as it landed (16 against 18, 20 against 21). Two thirds of the returns were red proofs, several of them main's own red ("not this branch's fault: main itself is red") or another goal's landing ("red on latest main merged with this branch", two goals at once); one third were conflicts, eight of them "source conflicts need resolution on the goal branch". The night of 10-04/05: nine pushes, twenty units, zero finished goals in nine and a half hours. It strikes whenever two or more seats work in the same area and land before they finish. Rank 1.

**What exists.** Wido's rule (10-05): a goal integrates once, when finished; the lane batches finished goals' proofs and fixes the small conflicts batching causes. Conflicts-resolve-unattended and correction-carries-reads are on main. The rule itself lives in people and memory, not in a verb. Item AV (a hand-in frees the claim quota) is parked because it writes to main's ledger at every hand-in. The landing-lane-runtime-redesign goal Wido asked for on 09-30 was abandoned on 10-02; the lane was fixed piecemeal instead (lane checks, flakes, reproves, conflicts).

**Solution.**

- **Admission is whole-goal only.** `work land` refuses a goal whose accepted design has a unit not built or not read clean on the branch; the refusal names the units. The next-step hint after a published read names the next unit, or `goal done` when none is left; never `work land`. One hand-in per goal. A card never says "landed N units".
- **No rebase before hand-in.** The lane merges the branch onto main with a merge commit and proves that commit. Unit commits and their reads keep their identity. Conflicts are resolved in the merge commit by the resolve round, and the resolution is remembered (a reuse store per lane) so a returned branch does not resolve the same hunk twice.
- **The lane's four policies** (each auto, capped or person):
  - *Batch size.* Auto: everything finished and waiting when a proof starts. Capped: at most N; 1 is "one by one". Person: a person picks the batch.
  - *Proof ladder.* After every merge in the batch: the cheap gate (declaration `proof.cheap`: static plus impacted). Once per batch, on the exact tree to be pushed: the full proof (`proof.full`). On a timer (`proof.trunk-every`, default 4 h): the full proof on main even when nothing landed.
  - *On red.* Every red is classified before anything is returned: the goal's own change, main's own red, another goal in the batch, a flake on the register, the lane's environment. Only the first class returns to the seat, with the red attached. Main's own red opens the trunk-red entry and the batch holds. Another goal's red returns that goal. A flake re-proves once. An environment failure retries once. Auto: replay the batch's merges one at a time with the cheap gate, full proof only where the cheap gate passes, return the goal that breaks, land the rest; if that costs more than two full proofs, hold the batch and ask. Person: the lane stops at the red and asks.
  - *Trunk red.* While the trunk check is red, the lane admits only a goal marked as the fix (the fix goal names the red it clears). A person can override; the machinery cannot.
- **Drain.** `landing drain`: stop admitting, finish the queue, hold (`landing stop` pauses at once). Taking the helm on the lane checkout drains the lane. A recorded stop or drain is honoured by every launcher: nothing starts a proof or an agent against a stopped lane (09-30 18:26: `landing stop` was recorded and a diagnostic proof launched anyway at load 16).
- **Quota and hand-in (AV).** A hand-in is recorded in the lane's queue, not in the goal ledger. The claim quota counts a goal in the lane's queue as held by its seat until it lands or is returned; a returned goal counts again. No ledger commit per hand-in, no session needed to hand in. A hand-in is one act with one record: the queue entry and the claim's hand-in mark are written together or not at all, so the lane never holds a goal the ledger does not count as handed in (assessment C11).
- **One design, one Astra round, then build.** This is the landing redesign of 09-30, done as one page (the lane's charter: batch finished goals' proofs, fix the small conflicts), not as a kernel.

**First unit.** The admission refusal and the next-step hint (two readers, one test through `work land`). Then the no-rebase merge. Then the red classification. Then the policies, batch size first.

### 2. Integration proofs ran selections; main was red eight times and the register saw none

**Problem.** The hand integrations of 10-05 and the lane's proof ran a selection of tests chosen by the integrator. A goal changes rules that tests elsewhere pin (an audit allow-list, a precondition of a verb, a verdict text); the selection never names them. Nothing on main checks the trunk on its own. The trunk-red register is written by nothing that finds reds.

**Impact and circumstances.** Eight reds on main in the week: 09-30, 10-01, 10-02 (40 min), 10-03 (5.9 h and 2.6 h), 10-05 (1.3 h) and the six-test red from pushes 47, 48 and 51 that lasted up to 18 hours until push 52. Every seat built on a red main during those hours; the lane returned good branches for main's red. The register holds two entries for the whole week, both on 09-30, both "not a red". Rank 1.

**What exists.** A memory rule for m1e (full package before every push). Lane-reproves-only-what-a-change-can-affect narrows the per-change proof, which is right for the cheap gate and wrong as the only gate.

**Solution.** The proof ladder of finding 1, declared by the adopter:

```
proof.cheap       = <command: static + impacted>
proof.full        = <command: the whole suite>
proof.audits      = <commands: the repository's audits>
proof.deadline    = <minutes per command>
proof.trunk-every = 4h
```

The lane runs `proof.full` once per batch on the exact tree it pushes, and the trunk timer runs it on main. A red from either writes the trunk-red register entry (test, commit, time) that `work land`, `goal done`, the steward's health and the UI read; the fix goal clears it by proving green. The cheap gate is feedback inside a batch, never a substitute. Every proof command has a deadline (three lane proofs hung 36 to 46 minutes on 10-03/04 without one). A register entry carries the failing test's own output, not only its name (a lane proof log of 10-04 printed only FAIL lines). The trunk check writes one entry per red and never a commit per tick; any component that pushes to main more than N times in an hour for the same reason stops itself and alerts (09-30: the cadence pushed 24 trunk-red commits in eleven minutes).

**First unit.** The trunk timer writing the register and the lane reading it. It needs no new proof command; it needs the full proof to run on a clock and its result to be recorded.

### 3. The goal ledger lives on main and moves it hundreds of times a day

**Problem.** Every ledger act (edit, set-priority, approve, claim, done, budget) is a commit pushed to main: 1,280 of the week's 1,760 commits, 609 pushes on 10-03 alone (347 edits, 120 priorities, 119 approvals in the week). Every push moves main under every open branch; a seat's rebase then rewrites its units and voids reads; the lane's proof is voided when a records or design page lands during it; a hand-in that writes to the ledger (AV) "moves main" and the landing tests forbid it.

**Impact and circumstances.** The lane livelock of 10-03 03:30 to 05:30 (four green proofs, no push, because ledger-only commits kept moving main); two green lane proofs voided by m1e's design-page pushes on 10-03 (about 40 minutes); reads re-run on unchanged code under a new SHA (at least nine between 10-03 and 10-05); AV dropped. Rank 1 for the voided proofs and reads, rank 4 for the churn.

**What exists.** Lane proofs "inherit green" when only ledger files changed (10-03); records-land-through-the-lane (10-05) routes records pushes through the lane. Both treat the symptom.

**Solution.** Separate the ledger's movement from the code's. Two shapes, to decide in the landing redesign:

- *A records ref.* Ledger and records commits go to a branch or ref of their own, merged into main by the lane at each landing (one commit per batch). Code branches never see ledger moves; a goal branch is "behind main" only by code.
- *Batched ledger pushes.* The ledger stays on main but acts are batched by the lane (every N minutes or at every landing), so main moves a few times an hour, not hundreds.

Either way: the seat's rebase ignores ledger-only moves (no rewrite, no re-read); a proof never re-runs for a ledger move; a hand-in records to the lane's queue (finding 1). The first shape is cleaner; the second is smaller.

**First unit.** The seat's rebase treats a ledger-only move as no move (no rewrite); the lane's proof inherits green across it (exists). Then the records ref.

**Seen again 10-06, starting goal 1a.** The seat's view of the accepted ledger was seven hours old; `goal claim` acted on it until a person ran `goal list --fetch`. Then `session start` was deferred because the checkout was two ledger and settings commits past the engine's stamp, and only a person's `system restart` from the pane cleared it. Both are ledger-only moves treated as if they were code. Added to the solution: a reader of the ledger fetches it itself before it decides, and an engine re-arm compares code, never ledger or settings commits (with finding 17's stamp). In goal 1b (unit U6 of its brief: a ledger-only move is no move for any reader) and goal 7 (the stamp).

### 4. Provider limits and outage marks take the whole fleet down

**Problem.** A provider session limit stops every Claude seat at once; the stewards then hold the seats on an "outage mark" that rolled to the next day (or "resets Oct 8") and did not clear at the reset; budget clocks kept running; a person restarted eight checkouts by hand.

**Impact and circumstances.** 10-03 03:20: six seats reaped by the limit. 10-04 04:01 to 05:10: all eight seats down, the lane agent died mid-proof. 10-04 08:35 to 10:10: a second limit; stewards held seats on stale marks after the reset. 10-04 13:44, 16:29 and 21:11: marks rolled forward, about 4 h 40 min of fleet down, three hand recoveries. 10-04 15:20: a weekly-limit message took every Opus seat down for 40 minutes. It strikes whenever the fleet runs against one provider account. Rank 2, the whole fleet.

**What exists.** The probe that clears a mark (push 36) and the roll-forward fix (push 38) landed on 10-04/05.

**Solution.** A provider limit is an environment condition with a reset time. The steward records it once, fleet-wide (one mark per provider, in the lane's state, not per seat), pauses every budget clock that depends on that provider, and revives at the reset on its own by probing, never by a date it computed. A mark older than its reset plus a grace is a defect, not a hold: the steward clears it and alerts. A seat still down N minutes after a reset is an alert to the person with the one act that revives it. A seat's own re-arm never invalidates the authorization its steward needs to revive it (the ui seat's steward failed fifteen revivals on 10-03 for that reason). A seat that is waiting on a job it launched is making progress and is never reaped as idle (three seats were, 10-03 00:05). The fleet's provider use is a number on the board (calls and tokens per hour against the account's limits) so the limit is seen before it strikes.

**First unit.** One fleet-wide mark with a probe at reset; budget clocks pause on it.

### 5. Runs do not advance without a driver

**Problem.** `work wait` returns after about four minutes with "still running". When a step ends, nothing starts the next unless a verb runs again. With the seats paused, builds sat until a person re-attached; before the pause, seats drove their own runs by polling.

**Impact and circumstances.** 59 gaps of over ten minutes between a run's steps, 38.2 idle hours in four days; the ten largest gaps (240, 198, 169, 155, 118, 84, 62, 60, 52, 48 minutes) are all "read ended, next build not started". The coordinator session made 16,900 model calls in the week, 3,400 a day at the peak, mostly to drive rounds and re-arm watchers by hand. Rank 2.

**What exists.** The steward tick runs on every seat. Nothing in it drives open unit runs. Loops in shells pile up (65 stale loops in 2.5 days on 09-29).

**Solution.** The steward tick drives every open run of its seat: when a step ended, start the next; when a round ended green, run the review that commits and dispatches the read; when a read is published clean, start the next unit of the design; when the last unit is clean, hand in (finding 1). `work wait` waits until the round ends or the caller's `--timeout`, whichever first, and says which. An environment failure re-runs the step once on its own and counts no round; the second failure stops the unit with the reason. The driver is a policy: auto (the tick drives), person (the tick reports what is next and waits). Helm sets person.

**First unit.** The tick drives build, proof and review steps; `work wait` waits to the end.

### 6. The review chain cannot record a stop, re-runs green units, and does not record what the stop needs

**Problem.** When a read finds a material defect, the only machinery outcomes are "correct it in another round on this unit" or "a person accepts the risk". Wido's stop rule has no word in the records. A revise is admitted after a green round with no finding at all. An empty build is recorded green. The material count the stop rule needs is not recorded where the stop would read it (`materialCount` is zero on all 408 read launches of the week; the unit rounds' `material` is unset in most). A round's staged tree binds to the base it was built on, so any move of the branch under it leaves the round uncommittable; a revise after an empty round cannot commit; a read commit whose unit landed under another SHA refuses the whole branch; a hand-made unit commit needs an undocumented trailer.

**Impact and circumstances.** 40 of 150 units ran three rounds or more. One unit ran 18 rounds (7.6 h, eight proof-reds) and its goal was parked; two units ran ten green rounds each (one with nine revisions and no decisions file; 7.1 h and 4.6 h); one unit retried the same machinery fault seven times in a row; one ran six rounds because every read found another reader. Up to five accepted risks per goal (three goals) were stops in disguise. Eight green rounds had an empty diff. Rank 3, with a slice of rank 4.

**What exists.** Unit-rounds-converge-or-stop landed a round cap, a material count per round, and machinery causes that do not count. Correction-carries-reads landed. The dispositions are accepted, refuted, out-of-scope, noted, accepted-risk.

**Solution.**

- **Two new dispositions.** `split: <unit>`: the finding moves to a named unit whose brief lists it and carries its stop history (a second stop on the same unresolved finding asks instead of splitting again); only the original review closes; the finding stays open against the named unit, that unit joins the goal's completion set, and `goal done` and `work land` stay blocked until its read is clean. `dropped: <reason>`: the unit leaves the goal; the machinery reverts its commits on the branch (a revert commit, never a rewrite) and the review closes. `goal done` sees no open material finding in either case: a drop has none, a split has it against a unit that must clear first. Both are person acts under the helm, auto under the stop rule.
- **The stop rule as a policy.** `review.corrections` (default 2), `review.material-must-fall` (default on), `review.class-repeat-stops` (default on). At the stop the machinery proposes split or drop with the finding list as the brief; auto takes split when the goal's design needs the unit and drop otherwise; person asks.
- **A green read closes the unit.** A revise needs an accepted material finding in a decisions file or a person's recorded reason. A repeat build of a green unit is a new unit, never a round.
- **The read records what the stop reads.** The reader's return carries the material count and a class for each finding (regression, weakened test, incomplete item, false premise, faked seam, reader of a changed invariant); the launch record and the unit round store them; the stop reads them from there.
- **An empty build is stopped, not green.** A build whose diff is empty records "stopped (gap)" with the builder's last message as the reason; no proof runs; no review is offered.
- **A build far over its declared size is held.** A diff larger than twice the unit's declared lines stops before the proof for a person's look (lane-agent-restarts round 1: 484 changed lines against a 120-line row, with a behaviour deleted).
- **The round binds to its own tree.** A round records the tree it produced, not the base; a branch move under it is reconciled by replaying the round's diff onto the new tip, and only a conflict refuses. A revise after a stopped round binds to the plan. A correction's commit is gated alone (static gate on that commit) before it binds, so a unit commit that only compiles together with the next never reaches the branch (10-05 04:45). The carry after a correction covers by-commit reads as well as unit reads (10-05 05:52: three by-commit reads redone because the carry read only the unit reads' files). `goal done` on a goal with an open review chain closes the chain with the recorded reason instead of leaving it open behind a `--force`. A read whose unit landed under another SHA is re-keyed or dropped by the range rule, never a refusal of the branch. `work rebase` compares the branch with main's tip, not with a remembered base, so it never answers "already on main" for a branch 84 commits behind (assessment C1); when it has nothing to do it says how far behind the branch is.
- **One action per worktree.** A worktree holds a lock while a round builds, proves or awaits review; a publish, a build of another unit or a rebase waits for it and says so.
- **`work commit --work NAME`** commits a hand change as a unit with the trailer, for the repair case.

**First unit.** The material count and the class per finding produced by the reader and stored in the read record, together with the two dispositions, the stop policy and "a green read closes the unit" (a stop cannot read inputs that are not recorded, so they ship in the same unit). Then the empty build. Then the tree binding and the lock.

### 7. Seat sessions have no lifetime and no ceiling

**Problem.** The headless seat sessions the steward starts run until they die. On 10-03 eight of them ran 10 to 24 hours each with peak contexts of 460K to 967K tokens; one read 730 million tokens in 22 hours. 75 of the week's 104 seat launches ended with "result-error" (all 75 provider limits: 64 session limits, 11 weekly), 8 were cancelled, 20 completed; on 10-04 there were 71 seat launches of 85 minutes mean, sessions dying and restarting. By 17:20 on 10-03 the ui seat had read 515 million tokens and m1j 479 million; by the end of that day 1.0 billion and 751 million against a 250 million ceiling that nothing enforced.

**Impact and circumstances.** 3.35 billion tokens of seat sessions in two days; 22.5 billion cached tokens across all seats' Claude sessions in the week; provider limits reached twice a day, which is finding 4. It strikes whenever seats run unattended for more than a few hours. Rank 4 by harm, first by cost.

**What exists.** Wido's rule of 09-20, verbatim: "Until further notice, I do not want any claude code to have a cap set for the context; we will use the defaults." Its reason: one absolute token number was applied across runtimes whose windows differ five-fold (a 200K cap cut Fable's 1M window to a fifth and was impossible for Codex's 258K), the harness compacts at its window minus 34K anyway, and a seat cap also capped every delegate of that seat. The seat-sessions-stay-small goal is open.

**How this finding relates to that rule.** It does not put a cap back. A cap is a number that truncates or compacts a session's context mid-work, lossy, with nothing written down. A lifetime is the moment a headless session ends cleanly, at a unit boundary, with its handoff written by the verb, and the successor starting from that handoff. Two other limits were first proposed as safety nets (a fraction of the runtime's own window, and hours); Wido's ruling of 10-06 removed them, so the unit boundary is the only end. The person's own session has no limit at all. What the week measured without a lifetime: 104 seat sessions, 75 ended with an error and 20 completed; of the 56 with a measured peak, the median context was 265K, the ninetieth percentile 625K, the largest 967K; the harness compacted once in the whole week, so contexts simply grew; the six sessions over 600K read 191 to 728 million tokens each, because every call re-reads the whole context and the cost of a session grows with the square of its length. A session that hands off at each unit boundary starts its successor at the size of a handoff, tens of thousands of tokens, and pays that instead. The rule of 09-20 stands for what it said: no cap, defaults everywhere. The amendment asked for was one sentence: a headless seat session is a worker with a lifetime, not a mind that runs until it dies. Wido's ruling (10-06 12:50): rely on the vendors' defaults; no context or compaction cap until the metasystem is stable and hard data allows a proper decision. The lifetime is therefore the unit boundary only; the context fraction and the hours are dropped.

**Solution (as ruled by Wido, 10-06 12:50: no context or compaction cap of any kind until the metasystem is stable and hard data exists; vendor defaults govern).** A headless seat session is a worker with a lifetime, never an always-on mind, and its lifetime is work-shaped: it ends at a unit boundary by writing its handoff through the verb (`session handoff`), and the steward starts the successor from that handoff. No context fraction, no hour limit, no token ceiling: those are numbers without data and are not built. A session that dies without a handoff is classified by its exit before any restart: every one of the week's 75 error deaths was a provider limit (64 session limits, 11 weekly), which is finding 4's hold-and-probe, never a restart; any other death is a revival loop under the stop rule (the same death reason twice stops the seat), with two restarts an hour as the backstop. A stop on a death loop, not a cap. Peak context, calls and tokens per session and per seat are board numbers, measured now so the later decision has data. A person's own session is untouched.

**First unit.** The end at a unit boundary with the handoff verb, the restart bound, and the per-session measurements on the board.

### 8. The host runs beyond its capacity and nothing isolates one run from another

**Problem.** Nothing limits how many builds and proofs run at once on a host, no proof has a deadline, and a kill by pattern reaches every run.

**Impact and circumstances.** 10-04: load 16 to 17 on 18 cores; four proofs timed out at process start and two lost their supervisor; the mean proof time rose from 2.7 to 7.6 minutes over the week; one build ran 211 minutes; three lane proofs hung 36, 46 and 44 minutes without a deadline; eight builders ran in parallel at "max agents" during the flaky-test wave; a seat ran `pkill -f metasystem.test` and killed every seat's test binary; seven idle per-unit Go caches held 16.7 GB; a builder ran the whole cmd package per round (40 minutes) at load 16. Rank 2 when it stalls the lane, rank 4 otherwise.

**What exists.** The rule "one full proof at a time per host" (09-28) and "one landing lane per host" (09-29), in memory. `-timeout 25m` patched into the VM script by hand.

**Solution.** A host declares its capacity: `host.builds` (parallel builds), `host.proofs` (parallel proofs, default 1 full), `host.load-max`. The steward refuses to start a step beyond it and queues it with the reason (a board number). Every proof and build command runs under the declared deadline (finding 2) in its own process group, so a stop reaches only its run; the kill verb is the only killer. Build caches are per host, not per unit, and the sweep removes a unit's leftovers at its end. Every launch except a seat session runs under a deadline from its declaration (design critiques, Codex jobs: two stale jobs ran 35 and 46 hours on 09-30); a session's only end is its unit boundary (finding 7). The machinery never writes into a code tree it proves or builds in: logs, digests and state go to the state root or an ignored artifacts directory (the steward wrote narrator-digest.log into the checkout, and three proofs failed "the tree changed during the proof").

**First unit.** `host.proofs = 1` and the deadline on every proof; the steward queues beyond it.

### 9. Remedies that do not clear, and a health phase that stalls on them

**Problem.** When the steward's automatic healing ends, it names a person's act. Twice the act named was a read-only verb that cannot clear the condition. On 10-03 at 11:35 the health verdict was "unhealthy" on all eight seats permanently (four to six dead roles each) and the stop's health phase held everything. The steward runner also burned half a core per seat for hours on 10-04; the cause is not in the passes the brief suspected.

**Impact and circumstances.** An unattended run stalls on a red that no automatic act and no named act clears; a person is sent on an errand. Rank 2 for the remedy, rank 4 for the burn.

**What exists.** Health-is-green landed: remedies apply, fixture mode never probes, breaker remedies name the person's act. The ledger-attention remedy is an accepted risk (assessment item 12). Runner-idles landed a per-tick timing line.

**Solution.**

- **A remedy is a verb whose effect is proven.** Each health role declares its clearing act as a verb and the condition that proves the clear. The health tests run the declared act in the bed and assert the role clears. A remedy that does not clear in the bed does not ship.
- **For ledger attention:** a person verb that records the fetch, or the steward accepting the examined tip on a person's acknowledgment. Design question first; the test above decides it.
- **The burn.** When a runner exceeds a CPU share for N ticks, the timing line names the phase and the runner writes a profile of its next tick to the artifacts; the alert carries the path. Rotate the runner log. The noted follow-ups of the health reads stay on this finding's list: the session-main role's guards, and the growth of arming.log (rotate with the runner log).

**First unit.** The remedy-proves-clear test for the roles that have a remedy today.

### 10. Budget boxes close admission in the middle of a goal

**Problem.** A goal's box is set at claim with the tier's default; the design's unit count plays no part. Goals reach 75 % and 90 % of their box with units left; admission closes; a person raises the box.

**Impact and circumstances.** The seven machinery goals of the week needed 2 to 7 raises each (design-gate-at-dispatch 7, review-findings 5); three goals hit 75 % at the same hour on 10-03; m1e raised four boxes by hand on 10-05/06. Rank 2.

**What exists.** The goal goal-budget-follows-its-plan exists, unseated. Working-hour days (memory of 09-16).

**Solution.** At claim, the box is derived from the accepted design: units × a per-unit allowance (tier-dependent) for attempts, rounds and tokens; elapsed from the estimate the design carries. When a unit is admitted within the box, the machinery raises the remaining allowance by the unit's share if a correction consumed it. A person's raise stays the exception and is recorded. Clocks pause on a provider limit (finding 4). Policy values: auto (derive and raise), capped (derive, never raise), person (ask at 75 %).

**First unit.** Derivation at claim from the Units table.

**Seen again 10-06, starting goal 1a.** The review-round maximum of 20 stood before the first build of a 15-unit design and had to be raised by a person in the committed configuration. Finding 10's derivation from the Units table covers it (goal 4).

### 11. Refusals and questions that need a person, at night

**Problem.** A seat that is refused a verb either fights the refusal, bypasses it, or asks a person; many refusals are wrong or name the wrong act.

**Impact and circumstances.** Between 23:00 on 10-03 and 06:00 on 10-04 there were about six person-needing refusals an hour (tier dance, frozen brief, worktree engine, placeholders, lane wake), about five seat-hours lost; three accept-risk acts could not be recorded for two hours on 10-03 (worktree review unreadable, terminal not enrolled); nine questions reached Wido that were not his; `goal claim` without a lineage said "not by a person"; a read could not be closed from its goal worktree; m1e hit eight blocks in one afternoon and asked about none. Rank 2.

**What exists.** Blocked-agent-asks-the-human (10-02), the worktree-dispatch goal (10-05), R-145 (hot-fix exception). Questions-reach-the-person is open; the claim lineage message is open.

**Solution.** Every refusal that needs a person names the act, the pane and the person; the escape hatch routes to the helm holder of the fleet first (the coordinator seat under its grant) and to the person only when the act is outside the grant. A refusal class that repeats N times in an hour across seats becomes a machinery finding on its own (the machinery files it, with the refusal code and count), so the coordinator sees "six seats refused on X" instead of six questions. A refusal's text is a line a person can act on, audited like every message.

**First unit.** The routing: helm holder first, person second; the repeated-refusal finding.

### 12. Seats claim overlapping areas, rebase late, and are returned for others' faults

**Problem.** Goals that edit the same files run in parallel for days; a branch rebases only at hand-in, onto a main that moved hundreds of commits; the lane returns a branch for a red that is main's own or another goal's.

**Impact and circumstances.** 16 conflict returns in four days; the -04 branch lived two days while main moved 122 code commits and needed nine hand resolutions; one slice was 731 commits behind main at return; the fifth conflict return of 10-03 was the same files as the four before. Rank 3 when the lane returns the goal, rank 4 for the hand work.

**What exists.** Conflicts-resolve-unattended (the resolve round); rerere in the m1e repository with nine resolutions. The goal seats-claim-disjoint-areas exists, unseated.

**Solution.** A goal claim records the areas its design names (paths or globs, in a declaration the design page carries). A second claim whose areas overlap is sequenced behind the first (auto), or allowed with a warning (person). The lane's resolve round reuses recorded resolutions across batches (one reuse store per lane). Integration is the lane's merge of finding 1, which keeps unit commits and their reads as they are; a read-carrying replay happens only for a correction (the carry of finding 6) or a rebase a person asks for by name, never as a step of integration and never for a ledger-only move (finding 3). Returns are classified (finding 1): only the goal's own fault returns. The resolve round's answer reaches the builder: a returned branch's brief carries the lane's resolution and the conflict record, so the seat does not resolve the same hunk again (the read of conflicts-resolve-unattended found the resolution rarely reached the builder, N-4); the lane's lease lock is released while the model builds the resolution and taken again to apply it (N-5). A verb that moves a checkout's branch (pull, rebase) says which branch the checkout is on and refuses a rebase of a checkout that is not on main unless asked for that branch by name (the ui checkout on `ui-development`, assessment C16).

**First unit.** Areas on the claim and the overlap check, built in the landing redesign (1b) on the claim-and-queue view of finding 1: a goal keeps its areas excluded while it is claimed or waiting in the lane's queue, and releases them when it lands or is dropped.

### 13. The builder's green and the proof's red disagree

**Problem.** A builder reports its checks green; the machinery's proof of the same tree is red. The builder ran in a sandbox that denied its cache, could not run the repository's audits, or ran a different command than the proof.

**Impact and circumstances.** 56 rounds ended proof-red after a builder's green (50 proofs exited 1), each after a full build of 20 to 30 minutes and each skipping its read; on 10-04 nine of 27 rounds on one seat were built blind in the sandbox; a lane batch was returned three times for an audit the builder's check did not run. Rank 4.

**What exists.** Codex jobs run unsandboxed on a trusted host (10-04); the unit gate runs reverse dependents (landed, but the checks that use it do not always include the audits).

**Solution.** The builder's check is the declared `proof.cheap` plus `proof.audits`, run in the same environment the proof step uses; the proof step reruns the same command only to attest. A proof-red whose command the builder could not run (a denied path, a missing tool) is an environment cause, not a round. The brief never composes its own check; it names the declaration. A check runs through the verb, which reports every command's exit; a person's shell chain that pipes a test to `tail` is not a check (assessment E5).

**First unit.** The unit's check composed from the declarations, by the scaffold.

### 14. Reads collided on a shared path and were retried without bound

**Problem.** Per-package reads of one unit wrote to one temporary path; parallel reads failed "declared-output-busy"; the machinery retried the same failure round after round.

**Impact and circumstances.** 195 read launches failed this way on 10-02 and 10-03, most of the 69 read-failed rounds of the week; one unit retried it seven rounds in a row (7.8 h). The per-package fan-out of 10-03 ran 228 read launches in a day, 0.5 billion tokens, three minutes each, mostly useless. Rank 4; fixed in part.

**What exists.** One committed read per unit (Wido's roster, 10-03) replaced the fan-out; unit-rounds counts the collision as a machinery cause (10-05). No recurrence after 10-03.

**Solution.** Every launch owns its output path (per launch, never per unit). An environment failure retries once and then stops the unit with the cause (the principle above); the same cause twice in a row on one unit is never a third attempt. Fan-out of reads only with per-launch isolation and a declared parallelism (finding 8).

**First unit.** The retry bound on environment causes.

### 15. Briefs with false premises, readers not enumerated, beds that fake the seam

**Problem.** Briefs written from memory asserted code that did not exist; briefs did not list the readers of the invariant they changed; builders' beds stubbed both sides of a seam so proofs passed and reads found production breaks; a unit's check did not run the repository's audits.

**Impact and circumstances.** Three false premises in m1e's briefs on 10-05 (records-scoped, done-over-notes, U3e's `goal sync`), each a build and a read; the six-round unit; two accepted risks that a grep would have avoided; reads returning "fix first" 76 times against "land" 80 in the week. Rank 4.

**What exists.** The rules live in memory and in the briefs m1e writes by hand. The `work brief` scaffold has none of them.

**Solution.** `work brief` emits the Readers section with the grep list filled for every name the brief cites, the public-verb section, the check composed from the declarations (finding 13), and refuses dispatch when a cited `file:line` does not exist at the base commit (premise check). The reader marks the class of each finding (finding 6). When a unit deletes files or data, the scaffold adds the five fail-closed deletion rules (empty ids refuse, errors are never discarded, paths never widen, the protected set is explicit, a dry run precedes the delete) that the disk goals of 09-29 learned from twenty data-loss findings.

**First unit.** The premise check and the Readers grep in `work brief`.

**Seen again 10-06, starting goal 1a.** `design write` refused a brief whose code sites were written as short paths; a person rewrote every path. A builder appended to `memory/receipts.log`, outside its brief, in three of eleven builds. Added to the solution: the brief resolver accepts any path that resolves from the repository root and names the one it cannot; a build whose diff touches `memory/` or `records/` without its brief naming them is refused at the check (goal 6, briefs-carry-their-rules).

### 16. Settings changes strand units in flight; ledger acts re-bind running claims

**Problem.** A roster or settings change applies at once to units mid-round (their digest froze the old settings) and strands them; a person's ledger act on a claimed goal (unapprove and approve to re-arm, a set-budget) re-binds the claim epoch and refuses the holder; approvals were withdrawn in bulk so waiting goals became unapproved.

**Impact and circumstances.** 10-03 19:33: a roster switch froze units on five seats, two rounds and two accepted risks lost, rolled back by hand. 10-03 00:40: m1e's re-arm released three seats' claims; they took other goals with stale work staged. 10-03 05:15: set-budget acts refused the holders. 10-03 14:25: twelve waiting goals found unapproved. `settings set` accepted a non-existent key and critics ran on the wrong model all night. Rank 3.

**What exists.** The approval-stays design is accepted (cutover pending). `settings set` key validation was corrected on 10-04. Item O is open.

**Solution.** A settings change takes effect per seat at its next unit boundary, never mid-round; a running round keeps its frozen settings to its end and says so. A ledger act on a claimed goal never changes its claim epoch unless the act is release or steal. Approval is agreement: once given it stays until intent drifts (the accepted design). `settings set` refuses unknown keys and shows the effective roster per seat.

**First unit.** Settings at the unit boundary.

**Seen again 10-06, starting goal 1a.** `goal claim` by the seat refused because no session of the seat was live, and the seat could not start one until the engine was rebuilt. A person's act needs no live session (No HAL 9000); an agent's claim starts the session it needs. Added to goal 1b, unit U6, with finding 3's readers.

### 17. Engine versions drift across seats

**Problem.** After a lane push, each seat kept its old engine until its own fetch and rebuild; critics launched on a stale engine were lost; the engine-skew preflight refused a branch rebased exactly onto the engine's commit.

**Impact and circumstances.** 10-04 01:44: five reads lost on stale engines; 10-05 03:30: a false preflight refusal cost an hour and three reads. Rank 3.

**What exists.** Rearm-after-fetch landed the boundary refresh (10-06). The preflight false refusal is open.

**Solution.** One engine version per host, stamped at the lane's push, armed on every seat at its next unit boundary by the boundary refresh; a read or build records the engine it ran on; the preflight compares stamps, not commits, and refuses only a lower stamp.

**First unit.** The stamp comparison in the preflight.

### 18. Load-fragile tests and proofs without deadlines

**Problem.** Tests fail under load, not under defect (a ledger fetch with a four-second wall-clock bound, a cancel race, a frozen-corpus test under the default ten-minute package timeout); a proof without a deadline hangs.

**Impact and circumstances.** The ledger-fetch test timed out twice on 10-05 and once more at the -04 gate; the frozen-corpus test failed three times under load on 10-04; the lane returned health for a wall-clock flake; three proofs hung 36 to 46 minutes. Rank 3 when a branch is returned.

**Solution.** Wido's rule of 09-12: injectable clocks and fixture ledgers, never wall-clock bounds in tests. Every proof runs under the declared deadline (finding 2). A test that fails only under load goes to the flake register with its fix as a unit, and the lane re-proves a registered flake once before returning.

**First unit.** The three named tests.

### 19. Housekeeping that can lose data or leave stale state

**Problem.** Finding ids are not unique across split reads. The sweep removes a worktree without repacking first, which can destroy objects other worktrees reach through alternates. The census needs a person's act after an engine rebuild. `goal done`'s sweep keeps a worktree because of ignored binaries. Idle build caches fill the disk.

**Impact and circumstances.** Data loss on a sweep (rank 1 when it strikes, rare); stale census, kept worktrees, 16.7 GB of caches (rank 5).

**Solution.** Repack before any worktree removal, and refuse the removal when the repack fails. Finding ids carry the read id. The engine rebuild re-arms the census through the boundary refresh. The sweep ignores ignored files when judging a worktree clean and removes a unit's cache at its end.

**First unit.** The repack-before-remove rule.

### 20. Token and model discipline

**Problem.** Effort, reader caps and the number of reads are set by hand per brief; the preliminary read step dispatches a read after every proof; critics ran on the wrong model for a night; design critiques ran six rounds.

**Impact and circumstances.** 45 preliminary reads cancelled by hand (34 on 10-05); critics on Fable all night on 10-04; a six-round design critique on 10-02; "17 critic rounds" on 10-04 (about 25 seat-hours of reads that night). Rank 4.

**What exists.** Wido's roster (10-03): Codex builds, one Opus read per unit, one Astra round per design. The critique stop goal.

**Solution.** Settings, not code: builder effort `high` for units under 150 lines and `xhigh` only for design-level units; reader cap 20; one read per unit; one Astra round per design, a second only for a critical finding and only within the stop predicate of the mechanisms page (declared improvement, no class repeat); a rising material count is a stop, never a reason for another round. The preliminary read step is removed from the round plan. The roster per role is shown by `settings show` and audited against the launches (a launch on a model the roster does not name is a finding).

**First unit.** Remove the preliminary read step; the roster audit.

### 21. Records fragility

**Problem.** A note after a design page's status word turns main red; tests added by a unit are not always registered; an accept-risk refuses until the dispositions file has been run once; message and instruction edits fail audits after the push.

**Impact and circumstances.** A red main from a records edit (10-04 06:42, 30 minutes); two reds from message and instruction edits on 10-03; a test that never runs; a person act that fails the first time. Rank 5.

**Solution.** The records push verb validates design page status and runs the message and instruction audits before it commits. Test registration is part of the unit's check (the register is generated; the check regenerates and diffs). `goal accept-risk` registers the findings it needs from the critic root itself.

**First unit.** The audits in the records push.

## Human at the helm

Wido's question of 10-06 04:55: should "human at the helm" be one of the lane's policies? Yes, as the `person` value every policy has, not as a separate policy. Taking the helm on a seat sets its driver (finding 5), its review stop (finding 6) and its budget raise (finding 10) to `person`; taking the helm on the lane drains it (finding 1) and sets batch, on-red and trunk-red to `person`. Releasing the helm restores the configured values. While the helm is held the machinery prepares every step, says what it would do, and waits; a person's act takes effect at once. This is what this week's repair was in practice, done by hand from panes; the helm makes it a recorded state the UI shows.

## The way of working, from the week's decisions

Not machinery, but the plan is incomplete without it. The decisions that cost the most this week, in Wido's words and the rule each set:

- "Maybe it's time to take a step back, properly design and build instead of just fixing forward" (09-30): the lane redesign is one design page and one Astra round, then built smallest-first; it was abandoned on 10-02 and the week paid for it. Finding 1 is that design.
- "We're not building a nuclear reactor" (10-01): smallest thing that works; no kernel.
- "YOU ONLY INTEGRATE WHEN YOU ARE DONE WITH A GOAL" (10-05): no slices, ever; finding 1 makes the machinery refuse them.
- "17 critic rounds is nuts" and "be critical on the review cycle" (10-04, 10-05): finding 6 is the stop rule in the machinery.
- "why is that not done HOURS after I asked" (10-04): a broken machine is fixed through the hot-fix exception, not through the broken machine; the trunk-red policy of finding 1 is the machinery's form.
- "I NEVER ASKED FOR THAT" (10-03): no scope added by the coordinator.
- "No HAL 9000" (10-03): every policy has the person value; a person's act is never vetoed.

## Overlaps: the mechanisms several findings share

Read together, the twenty-one solutions use eleven mechanisms more than once. Their shapes, rules, verbs and first consumers are designed once in `plans/designs/machinery-mechanisms.md` (twelve mechanisms there: the ask record is its own); this section says which findings share them and where each is built. The design page and this plan are one set: a change to a shape is made on the page and referenced here, never the other way round. Each is designed once, with one record shape, and built in the first goal of the order that needs it; the later findings reuse it instead of growing their own counter, register or lock. No "primitives" goal up front: that is the kernel of 09-30 again. The mechanism is built as the smallest thing its first consumer needs, with the shape the others will fit.

| Mechanism | One design | Used by findings | Built first in |
| --- | --- | --- | --- |
| **Policy store and the helm** | One store of named policies, each `auto`, `capped N` or `person`; the existing `settings set/show` for the values and the existing `helm take/return` for the person value on all of them; the UI shows the effective value. | 1 (batch, proof ladder, on red, trunk red, drain), 5 (driver), 6 (stop), 7 (lifetime, ceiling), 10 (raise), 11 (routing), 16 (settings boundary) | landing redesign (goal 1) |
| **The stop primitive** | Budget, progress measure, class-repeat, decision before the next attempt, handoff; one record `stop: <loop> <attempt> <measure> <class> <handoff>`; one board column. | the stop section; 1, 2, 4, 5, 6, 7, 10, 11, 14, 20 | the minimal stop (record, close, budget, handoff) in landing 1a for the lane's on-red; the review chain (goal 2) adds the round loop |
| **Cause taxonomy** | One classification of why an attempt failed: the work's own change, main's own red, another goal in the batch, a registered flake, an environment cause (provider limit, capacity, lost process, busy path, denied sandbox, stale engine). Every red, return, failed step and retry carries one. | 1 (on red), 2 (register), 4, 5, 8, 13, 14, 17, 18 | landing redesign (goal 1): the lane classifies its reds; the unit proof reuses it in goal 2 |
| **One register of what the machinery found** | The trunk-red register, the flake register and the repeated-refusal findings are one register with kinds (red, flake, refusal, stop), written only by the component that found the thing (a proof, the trunk timer, the steward), read by `work land`, `goal done`, health and the UI. | 2, 6, 11, 18, 21 | goal 1 (trunk red and the minimal stop kind), extended in goals 2 and 5 |
| **Declarations** | One file per adopter, one reader: `proof.cheap`, `proof.full`, `proof.audits`, `proof.deadline`, `proof.trunk-every`, `host.builds`, `host.proofs`, `host.load-max`, the remedies per health role, the areas a design names, the deletion rules. Nothing application-specific anywhere else. | 2, 8, 9, 12, 13, 15, 18, 21 | goal 1 (the proof ladder), each later goal adds its keys |
| **The proof ladder, one owner per rung** | Builder: `proof.cheap` plus `proof.audits`, same environment as the proof step. Unit proof step: the same command, to attest. Lane, per merge: `proof.cheap`. Lane, per batch, on the pushed tree: `proof.full`. Trunk timer, on main: `proof.full`. Every rung under its deadline. Nothing else runs a suite. | 2, 8, 13, 18, 20 | goal 1 (lane and trunk), goal 2 (builder and unit step) |
| **The unit boundary** | One event the steward owns: no step running, no critic job, no revise pending. Consumers: the driver starts the next unit; a seat session may end; settings take effect; the engine re-arms; a budget raise is admitted. | 5, 7, 10, 16, 17 | the minimal boundary event in the fleet goal (goal 3) for sessions; runs-advance (goal 4) adds the driver as its consumer |
| **Host state** | One record per host, owned by the host's lane steward: provider marks with reset times, the capacity queue, the engine stamp, the trunk-red entry, the token and call counts per seat and provider. The board reads it. | 2, 4, 7, 8, 17 | fleet goal (goal 3), trunk red from goal 1 |
| **Tree ownership** | A tree (worktree, checkout, lane merge tree) has one owner at a time (a round, a proof, a merge) holding a lock; nothing writes into an owned tree except its owner; machinery logs and state never live in a code tree; the lane's lease is released while a model builds and taken to apply. | 6, 8, 12, 19 | review chain (goal 2) for rounds; goal 1 for the lane's lease |
| **The read record** | One shape for every read: its own id, the subject tree, the engine stamp, the material count, a class per finding, its own output path; finding ids carry the read id; the carry re-keys the record to a replayed commit, for unit reads and by-commit reads alike. | 3, 6, 14, 17, 19, 20 | review chain (goal 2) |
| **The scaffold** | `work brief` composes the check from the declarations, fills the Readers grep, adds the public-verb section, the deletion rules when the unit deletes, the decisions section for a correction, the effort from the unit's size, and refuses a brief whose cited `file:line` does not exist. | 6, 13, 15, 20 | briefs goal (goal 6), the check composition earlier in goal 2 |
| **Ask** | One way for the machinery to ask: a question record with the act it needs, routed to the helm holder first and to the person only outside the grant; repeated questions of one class become a register entry, not more questions. | 1 (on red, person), 6 (stop, person), 10 (raise at 75 %), 11 | the minimal ask record in landing 1a (the lane's hold); remedies (goal 5) adds the routing and the repeat rule |

**Where the findings would otherwise contradict each other, the resolution:**

- **Who drives, and what a seat session is for.** Finding 5 makes the steward tick drive every step; finding 7 bounds the seat session. Together: the steward drives steps and boundaries; the seat session is a worker that judges at boundaries (writes the brief, decides a read's findings, answers a question) and ends when nothing of that is pending. A seat session never polls a run.
- **Where a hand-in lives.** Finding 1 puts the hand-in in the lane's queue; finding 3 takes the ledger off main. Together: the lane's queue is the authority for "this goal is in the lane"; the ledger holds claims and states and reads the queue for the quota; the records ref carries both to main at each landing. AV's work is re-cut to that shape or dropped.
- **Which proof runs where.** Findings 2, 13 and 20 each name a suite. The ladder above is the only answer: the builder and the unit step run the cheap rung; only the lane's batch and the trunk timer run the full one; a builder that runs the full suite is a defect of the scaffold.
- **What "ask" means.** Finding 1's "hold and ask", finding 6's "person asks", finding 10's "ask at 75 %" and finding 11's routing are one mechanism, not four prompts; the helm holder is the first addressee of all of them.
- **Where fleet-wide state lives.** Finding 4's provider mark "in the lane's state", finding 8's capacity queue "the steward refuses", finding 17's engine stamp: one host record, owned by the host's lane steward, read by every seat's steward.
- **Registers.** Finding 2's trunk-red register, finding 18's flake register and finding 11's repeated-refusal findings are one register with kinds; `work land`, `goal done` and health read one thing.
- **Records push versus the records ref.** Finding 21 validates records pushes; finding 3 may move the ledger to a ref. If the ref is chosen, the records push becomes a commit on the ref and the validation runs there; the landed records-land-through-the-lane is the bridge until then.

## Practices tried by hand, promoted to machinery when proven

Wido (2026-10-06 22:45 CEST): "what you apply here should be the best solution. And if that proves to be true; that should be promoted to machinery." Each practice below is applied by hand from goal 1b on, measured per goal against 1a's baseline, and becomes a unit of the named goal only when its measure holds on at least two goals; a practice whose measure does not hold is dropped and the drop recorded here.

| Practice (by hand now) | Measure, 1a baseline | Promoted into |
| --- | --- | --- |
| Design critique to convergence under the stop table's design row, with the five fixed questions | material code-read findings that trace to the design or brief per unit (1a: about 6 of 30) | goal 2 (review chain: replaces design admission's "critical finding for a second round") |
| Every builder brief hand-written with read, cited code sites; the four recurring defect classes named in it | corrections per unit (1a: 13 of 15 units corrected, 7 split) | goal 6 (briefs carry their rules: the scaffold) |
| Builders run only the tests their change affects (new and changed tests, their mutations, the touched packages) | wall time per build and host load (1a: full cmd run 55 min under load against about 8 unloaded) | goal 4 (runs advance: the builder's check) |
| Each unit built from the goal branch's current tip and integrated right after its read | cherry-pick conflicts per goal (1a: 6, one needing a merge job) | goal 4 (the driver) |
| The full gate (`go test ./internal/...` and the whole cmd package) after every batch of integrated units and on the exact pushed tree | reds found only at integration (1a: main's internal red, 12 bed reds, 3 audit reds) | goal 1 follow-up (the lane's proof.full already runs both) and goal 7 |
| A bounded waiter armed with every delegate job | idle minutes with a finished job unread (1a: about 120) | goal 4 (the driver consumes the job's end) |

## Suggested goals and order

| Order | Goal | Findings | Mechanisms it builds | Why first |
| --- | --- | --- | --- | --- |
| 0 | the mechanisms page (`plans/designs/machinery-mechanisms.md`, design only: one Astra round, Wido's naming pass, no build) | the overlaps section | the shapes of all twelve | Every later goal builds to it; nothing is built without a consumer |
| 1 | landing-admits-finished-goals (the landing redesign, designs 1a and 1b) | 1, 2, 3, 12 (merge, returns, areas on the claim), AV | 1a: cause taxonomy, the register (trunk red), declarations (proof ladder), the lane's rungs, tree ownership for the lease, the minimal stop and ask records; 1b: policy store and helm, drain, areas on the claim | Rank 1 three times; the lane is the switch-on path |
| 2 | review-chain-stops-and-records | 6, 15 (class marks), 13 (the check from declarations) | the stop primitive, the read record, tree ownership for rounds, the builder and unit rungs | Removes every hand workaround of the week |
| 3 | fleet-survives-its-providers | 4, 7, 8 | host state, the minimal unit boundary event, the session's end at the boundary and the restart bound | The fleet went down for seven hours in two days and cost 22 billion tokens |
| 4 | runs-advance-on-their-own | 5, 10, 16 | the driver as the boundary's consumer, the driver policy | Unattended runs stall without them |
| 5 | remedies-prove-they-clear | 9, 11 | ask routing, the refusal kind of the register | The health phase and the refusals hold the stop |
| 6 | briefs-carry-their-rules | 15, 20 | the scaffold | Cuts rounds and reads |
| 7 | housekeeping (with plans/engine-inputs-design-brief.md, dropped from lane-drain-and-fresh-claims 10-07) | 14, 17, 18, 19, 21 | the flake kind of the register, the engine stamp | Small units, bundled |


The landing redesign is split in two designs, 1a (the lane's charter: whole-goal admission, merge instead of rebase, red classification, trunk timer and register) and 1b (the policies and the helm, drain, areas on the claim over the claim-and-queue view), each with a consumer from its first unit; the minimal stop and ask records are 1a's because its on-red hold needs them. The machinery stays off until every goal of this plan (0 to 7) is on main; then one small real goal goes from claim to `goal done` with no hand step while a person holds nothing, and only then is it switched on (Wido, 2026-10-06 22:15 CEST: "no machinery does not switch on until all is landed. I want a reliable machine before we switch it on"). Until then every goal is delivered by hand with delegates. Finding 7 needs Wido's word first, because it amends his rule of 09-20 for headless sessions.
