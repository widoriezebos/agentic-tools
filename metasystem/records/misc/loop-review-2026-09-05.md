# Loop review, 2026-09-04 09:30Z to 2026-09-05 (m2 seat)

Written for Wido at his request: where the time went, whether the critique stopped at the right moment, and what to change in the metasystem. Facts first, judgements after, each marked ran, read, or inferred.

## 1. The span and its shape

The m2 seat ran from 2026-09-04 09:30Z to 2026-09-05 15:00Z, about twenty-nine hours, with the human at the console for most of it. In that span the seat landed 33 changes of its own on main (git log, read) among 77 landings by all machines, opened 22 goals, and drove 69 delegate jobs (job records, read): 55 implementer rounds that completed (518 job-minutes), 1 that timed out at its cap (120), 4 that were refused at setup (16), and 9 critic rounds (97). Job time is 751 minutes, twelve and a half hours; the seat's own wall time around those jobs was the rest.

Seat-side verification that the delegate sandbox cannot do (known issue 15): 50 fixture-suite runs from the scratch logs (13 dispatch, 23 supervision, 14 channel), each 1 to 35 minutes, plus 6 long Go test runs of 18 to 24 minutes. Landing attempts: 36 logs; 6 were refused on the narrator-digest race and retried; 3 on other gate refusals (coverage, staticcheck, untracked files).

## 2. Where the time went, by kind

Inferred from the logs and the timestamps; the split is approximate to the quarter hour.

| Kind | Hours | Notes |
| --- | --- | --- |
| Implementer rounds (wall time waited) | 9 | 60 rounds; median 8 minutes; the tail is 35 to 60 minutes and one 120-minute cap. |
| Fixture suites seat-side | 8 | The dispatch suite alone is 25 to 35 minutes per run and ran 13 times; the supervision suite 3 minutes, 23 times. |
| Landing gates | 3 | A landing that touches Go runs the goal package's 18-minute tests; six first attempts lost the digest race and reran fast. |
| Critic rounds | 1.5 | 9 rounds, 6 to 15 minutes each. |
| Seat orchestration | 5 | Reading returns, writing briefs and records, staging, cleaning litter, re-arming, diagnosing refusals, and the human's questions. |
| Lost to mistakes | 2 | Listed in section 4. |

Two things dominate: waiting on suites, and rounds that ended one assertion further than the last. The custody item alone (section 3) took nine hours of the twenty-nine.

## 3. The custody item, round by round

The chain that fixed a fixture scenario killing the seat's supervision (goal supervision-custody-per-checkout and its landing member). Times from the job records (read).

| Round | Started | Minutes | Outcome |
| --- | --- | --- | --- |
| scc-build1 | 05:51Z | 120 | Timed out at its cap while emitting its return; work preserved on a branch. |
| scc-build2 (carry) | 07:53Z | 49 | Returned. |
| critic 1 | 08:42Z | 11 | Two material: the guard compared the git scope with the recorded state root and would refuse re-arming this very repository; the self-check missed the `metasystem up` path. |
| scc-build2-r2 | 08:54Z | 49 | Corrected. |
| critic 2 | 09:44Z | 11 | Two material: selection still by a tag prefix, not the recorded path; the reduction's fail-closed change was an unnamed contract change. Its register could not fold: both critic rounds numbered SCC-02. |
| scc-build3 (fresh chain) | 09:57Z | 60 | Corrected. |
| critic 3 | 10:59Z | 14 | Three material: owners now recorded the git top-level while every existing row held the state root, a lockout of every armed checkout; dead owners without a row un-takeoverable; the self-check accepted the seat's agent pid. Review box spent. |
| pause | 11:15Z | 15 | The seat stopped one question short of asking for a raise; the human asked why. |
| scp-build1 (member goal) | 11:23Z | 35 | Corrected. |
| critic 4 | 12:00Z | 7 | Three material: the guard read checkouts from reservation rows production never writes; the test seeded rows the previous binary never wrote; shutdown guarded before liveness. |
| scp-build1-r2 | 12:08Z | 57 | Corrected; two suite scenarios went red from it. |
| critic 5 | 13:07Z | 15 | One material: the live-owner gate also disabled the foreign-repository veto. |
| scp-build1-r3 | 13:23Z | 43 | Corrected. |
| critic 6 | 14:08Z | 14 | One material, in the fixture self-check only; accepted as risk by the human, scheduled. |
| coverage round | 14:26Z | | Tests only, for the registry package's ratchet; then the landing. |

Six critic rounds, each finding something new and each of them right. That is the fact to explain.

## 4. Did the critique stop at the right moment?

The criterion in force is: a material finding holds the chain until it is fixed or a human accepts the risk; a finding is material only if it changes what gets built and names the artifact. Every critic applied that bar honestly, and no round was spent on a finding that did not change the code. By that criterion the critique never ran too long: each round it found a defect that would have shipped.

It also never stopped too early. The one finding accepted as risk at the end was in the fixture's self-check, not in the custody code, and the human accepted it explicitly.

So the criterion is right and it worked. The nine hours came from somewhere else:

1. **The design was made inside the review.** Three of the six rounds rejected a different selection rule: git scope, then owner-tag prefix, then reservation claims, before the rule the design already stated (the canonical checkout path from the rows production writes) was implemented. A tier-3 change to the custody law should have started with a design round that named the recorded row shape, the path form, and the liveness order. The design gate at dispatch exists; the seat skipped it under "immediately". Cost: about four rounds, three and a half hours.

2. **Verification the sandbox cannot do sat with the seat.** Every implementer round ended with "the orchestrator must run the suite"; every seat-side run took 3 to 35 minutes and often exposed the next defect that a critic then formalised. The critic and the suite each saw half the picture. Cost: the suite hours in section 2.

3. **The review box is per goal and per round count, not per defect.** Three rounds per goal is a fair appetite failsafe, but its correct outcome is a raise to the human at the line. The seat instead stopped, and the raise then needed a member goal because the configured maximum is three rounds per goal. Cost: an hour and the human's patience.

4. **Rounds lingered after their return.** Four implementer rounds emitted their return and then kept running their sandbox gate for 15 to 25 minutes before the dispatcher saw completion; one hit its cap that way and lost its return entirely. Cost: about ninety minutes.

5. **Small law frictions, each costing a round or a retry:** finding ids must be unique across every critic round of one subject (a second round restarting at 01 blocked the fold and forced a fresh chain); the narrator digest changes under a long gate (six refused first landings); returns naming paths from the metasystem directory (two lost rounds on 2026-09-04); briefs citing runtime paths; the seat's shell cwd drifting into a worktree (four wasted commands).

## 5. What to change in the metasystem

Ordered by the hours they would have saved here.

1. **A design round before any tier-3 build**, mandatory at dispatch, naming the invariant, the data the code will read, and the order of checks. Estimated saving here: half the custody rounds.
2. **Let the node run the suites.** The delegate sandbox's process-visibility limit (known issue 15) makes the seat the only place a fixture suite runs. Either lift the limit for a suite-running role, or give the implementer a verb that queues a seat-side suite run and waits for its verdict inside the round. Saving: most of section 2's suite hours and one round per defect the suite found.
3. **Cut a round when its return is emitted.** A return message ends the round; the sandbox gate the implementer starts afterwards is not the round's business. Saving: ninety minutes here and one lost round.
4. **Make the review box escalate, not stop.** When the last round of a box still holds a material finding, the engine posts the raise question to the human itself, with the finding, rather than leaving the seat to decide. Saving: the pause and the member-goal detour.
5. **Critic ids per subject.** The brief template states the id range and the register refuses a round that restarts numbering. Saving: one fresh chain.
6. **Land the digest last.** The commit gate re-stages the narrator digest after its long tests, or the steward pauses appends during a landing. Saving: six retries.
7. **Path form in the return protocol** landed on 2026-09-04 (b7d119b3); keep it.

## 6. What the seat did wrong

Named so the learnings are honest: the seat played node and brain at once and put judgement into critic briefs; opened three regressions of its own by landing channel changes without running the channel suite; stopped one question short when the box ran out; and hallucinated a fact about the fleet's bot tokens in a report. Each is recorded in the memory the seat keeps, and each is a reason for the headless node with a single brain rather than a person at a console.
