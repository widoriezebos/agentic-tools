# Design brief: lane-lands-finished-goals, revision after Astra round 1

Working Mode: Design
Revise the draft at plans/designs/lane-lands-finished-goals.md (attempt 2). One Astra round found 11 material findings; all are accepted. This is the one revision; no second critique round follows unless a finding is critical. Keep everything not named here.

## Findings to fold (Astra, gpt-6-astra, on main 9eb87cd15)

1. Decisions 4, 7: the full rung may still be a scoped or reused proof (internal/landing/plain/scope.go:169-172, :345-350). Require a fresh full execution for changed executable inputs and for clearing an incident; reuse only across proven ledger-only changes. Test: a recent scoped green cannot clear an incident.
2. Decision 1: a goal without a Units table can still hand in a partial goal (cmd/metasystem/intent_delivery.go:2224-2246). Require a declared completion boundary for goals without a design table (tier-1 goals need no design page; say what declares their end). Test an unfinished goal on this route.
3. Decision 7: a red cheap gate after a green baseline is not yet `own`. Apply the cause and retry rules (mechanism 3) to gate failures; return only a demonstrated `own`. Test a lost process and a registered flake after a green baseline.
4. Decision 2: on the hand route (no lane) `landCandidate` -> `PrepareLanding` (cmd/metasystem/intent_delivery.go:355-356, internal/goal/branch/land.go:659, :713, preimage check :267-272) rewrites unit commits. Design the hand route's merge explicitly; revise the assertions of cmd/metasystem/intent_delivery_owner_test.go:359-380 that require rewritten tips, while keeping "a hand-in never moves main"; re-estimate the unit.
5. Decision 4: `full-due` alone does not run a periodic trunk check (prove.go:217, :392-394, scope.go:353-360, SKILL.md:61 runs it only when nothing waits). Give the scheduled trunk check an explicit fresh-full path that runs when only held lines remain. Test unchanged green and red trees at the next interval with a held queue.
6. Decision 4 and departure 1: identity by failed unit hides a different red in the same unit; and RecordTrunkRed appends and publishes on every call (internal/goal/trunkred.go:688-700). Identify a red by failing test, independent of the main commit; suppress duplicates inside the publishing transaction (no commit for a repeat sighting). Test two different reds in one unit and repeated sightings of one red.
7. Decisions 2, 3: a conflict against an earlier batch member, not against main (plain/resolve.go:94, :170-179; SKILL.md:29-30), cannot be fixed by `work rebase`. Separate conflicts with main from conflicts with a predecessor in the batch; the latter wait until that predecessor lands, then retry. Test two branches that each merge cleanly with main but conflict together.
8. Decision 3: the regeneration failure path returns goals for environment failures (plain/resolve.go:205-207, :224-244). Classify regeneration failures; environment or unclassified holds and asks. Test a generated-only merge whose regeneration cannot start.
9. Decision 3: replay can grant a repeat after today's repeat is spent (prove.go:693-735). Carry one explicit repeat allowance through the existing proof logic and the replay; once spent, isolated greens with a persistent full red hold. Test "full red, repeat red, isolated green" asserts no third full execution.
10. Decisions 3, 7: the return guard cannot be met after a red cheap gate (gates are stored apart). Let `landing return` accept an attributable gate result bound to the goal's queued sha; gate greens never satisfy a push.
11. Decision 1: the next-step function must share one predicate with admission: "built, and read clean or ReadsWaived" (intent_selection.go:280-281, intent_manual_submit.go:334-339). Test an unfinished and a fully built tier-1 goal.

## Open questions, as decided

- Question 1 (which command finishes a goal): option A (both the author and Astra recommend it): the finished goal's next step is `work land`, the goal's one hand-in; `goal done` follows after it lands. Write the line.
- Question 3 (departures): accept departures 2 and 3; replace departure 1 by finding 6's test-level identity.
- Question 2 (this repository's proof.full and proof.cheap) stays open for Wido. Design the ladder so it does not depend on the answer: the committed keys hold portable commands (a committed script for proof.full that runs a VM suite only when a computer fact in local settings names the VM, and the plain package suite otherwise; `metasystem test run` for proof.cheap); state that the exact commands are Wido's to confirm.

Re-estimate units and the total; keep each unit at or under about 130 lines, splitting if needed. Keep the page's Wido's words, threat model, acceptance table and deferred list.
