# The lane recovers and pauses: a human verb for every scenario, the goal's clock stops in the lane, the lane owns its fix rounds

- Kind: design
- Id: 01M4MSA883YK2CM2W2BXXFCT5H
- Status: accepted
- Critique: Astra round 1 (2026-10-11 08:08) 1 material: first slice undefined (R-121-m0), Step 1 added; round 2 (08:14) 2 material on the extra fix round (LRP-01 no recovery state after the automatic return, LRP-02 a second repair commit fails the batch-history check): the extra round leaves Step 1 and becomes Step 3 with both findings as its obligations; round 3 (08:27) 1 material (LRP-03 a re-hand-in's fresh round collides with the first round's build identity) folded into Step 1, LRP-04 (`--cause other` is not a cause) corrected
- Goals: lane-recovers-and-pauses
- Requested: by Wido on 2026-10-11 08:00 ("I follow your recommendation … there should always be a verb that the human can use to recover … Every scenario we should support with a verb … Now make it flawless"), after the overnight lane run (lane test findings 98-113)

Author: m1e (Fable), 2026-10-11 08:05. Tier 3; a generic lane goal, top priority per R-146-m1e.

## Problem, measured

The lane ran a goal end to end unattended (resolve as the seat, merge, read, full proof, detached classification to `own`, a correct fix brief) and then its fix round was refused by admission: the goal's elapsed budget (8 h) had run out while it waited in the lane and its box allowed zero review rounds; the lane returned the goal (finding 109). The merge read took 71 minutes for three resolved files (findings 98, 109). After a hot-fix reached main the agent asked for a trunk-red exception instead of proving main (finding 105). Twice the only way forward was a steward improvising person acts from the enrolled pane (S42, S43, S47). Scenarios without a verb cost hours.

## Step 1: the smallest thing that works first (R-121-m0)

Step 1 is D2 alone: the lane admits ONE fix round per batch proof under its own allowance (`landing.fix-rounds`, default 1; `landing.fix-minutes`, default 120), independent of the goal's box. Nothing else changes: no clock pause, no read scope change, no new verb. It works without the rest: tonight's refused fix round (finding 109) would have run, and a second red returns the goal exactly as today; the person's recovery after that return is the existing path (fix on the branch, `work land G` again, which re-enters the lane with a fresh allowance for its next batch proof). The fix job's named-unit identity is bound to the repair episode: the unit is `lane-fix-<batch proof attempt>-1` (the attempt id of the red proof it repairs), so a returned goal that is changed and handed in again gets a distinct fix job for its new batch proof while a repeat within the same episode stays idempotent (`UNIT_NAMED_INPUT_CHANGED` never fires across episodes). Its tests: repair, return, changed hand-in, distinct repair job (fixture); the real refusal "not approved with a budget yet" never appears for an own red while the lane's allowance is unspent (fixture seams on admission, the real fix record shape); a green batch admits no fix round; one rehearsal of the green scenario.

Deferred, in order: Step 2 = D1 (clock pause); Step 3 = the extra round `landing fix G --reason` with the two obligations the critique set: (LRP-01) the command's destination after the automatic return — it restores the returned hand-in and its proof checkpoint on the same repair tree, or holds the goal instead of returning while an allowance can still be granted; (LRP-02) a second repair commit on `merge → repair-1 → repair-2` stays recognised by the batch-history check (every authorised repair commit keeps its review evidence) and by `currentFix`; both proven by a two-round history fixture covering proof and push admission; Step 4 = D3 (resolution-only read); Step 5 = the rest of D4 (`landing retract`, the two verbs in every status line and question, the skill rule for a moved main).

## Decisions

### D1 The goal's clock pauses in the lane
A goal's elapsed budget does not run while the goal is in the lane: from the hand-in's acceptance (`work land`) to its push or return, the budget record carries the pause (`lane_paused_at`, accumulated `lane_paused_minutes`), the breach-stop custodian excludes paused time, `goal show`/`goal budget` print "in the lane since HH:MM, clock paused (Nh so far)". A goal returned by the lane resumes its clock at the return. The lane's own waiting time is bounded separately: `landing.wait-hours` (default 12) over which status prints a finding line ("waiting 13 h in the lane") — never a stop.

### D2 The lane owns its fix rounds
A red batch proof attributed to a goal (`own`) admits a lane fix round under the lane's allowance: `landing.fix-rounds` (default 1 per batch proof) and `landing.fix-minutes` (default 120 per round), checked by the lane's admission, not the goal's review rounds or attempts; the fix job's provenance and commit (`Goal-Unit: G/lane-fix-<batch proof attempt>-1`, the identity Step 1 defines) land on the batch tree as today; the goal's box is untouched by the round. When the allowance is spent and the proof is still red, the lane returns the goal with the brief and the rounds' evidence (as today's return). A person may grant one more round (D4).

### D3 The merge read covers the resolution
The read of a lane merge (`lane-merge-N`) judges the diff between the merge commit and git's automatic merge of the same parents (the hunks the resolution wrote), with the conflict record, the paths' classes and the check result as context, and nothing else of the goal (the goal's units were read on their branch). The brief states the scope and the bound `landing.read-minutes` (default 20); the read's wall time is recorded next to the fix record; an overrun is recorded as a finding line in status, not a stop.

### D4 A human verb for every scenario
Every stop, return, question and hold the lane produces prints, in `landing status` and in the question text, the one verb that resumes the goal in the lane and the one that retracts it. Inventory (existing verbs keep their names):

| Scenario | Resume in the lane | Retract from the lane |
| --- | --- | --- |
| Goal returned by the lane (own red, failed fix rounds, conflict it could not resolve) | fix on the branch, `work land G` (exists) | nothing to retract; the goal is with its seat |
| Fix allowance spent, person wants the lane to try once more | today: fix on the branch and `work land G` (exists); Step 3: `landing fix G --reason` (new: one extra lane fix round for the current batch proof, with the LRP-01/02 obligations) | `landing return G --cause other --reason` (exists; a proven person may return any accepted cause) |
| Proof red attributed to main | the lane runs `landing prove --trunk` itself when main moved since the incident; a person may start it too (exists; runs detached, unit u9) | `landing return G --cause main` (exists) |
| A question waits for a person | the question's own answer verb; `question withdraw` makes the agent decide again (exist) | `landing return G --cause other` |
| Merge conflict | `landing resolve --as-seat` (exists) | `landing resolve` without the flag returns it (exists) |
| Lane stopped or paused | `landing start` (exists) | `landing unset` (exists) |
| Goal's budget lapsed (D1 makes this rare) | `goal budget G BOX`, `goal resume G` (exist) | `landing return G --cause other` |
| Agent dead, looping or silent | `landing run`; `landing stop` then `landing start` (exist) | — |
| Goal must leave the queue or batch now (wrong hand-in, superseded) | — | `landing retract G --reason` (new: a person removes the goal from the queue or the running batch; a running proof of a batch that held it is abandoned and the batch rebuilt without it; the goal returns to its seat as "retracted by NAME") |

`landing retract` and `landing fix` are person acts (terminal-grade proof or power of attorney). The landing agent's skill gains one rule: after a `main` cause, when fetched main moved since the incident, run the trunk proof (detached) before any question; ask only if that proof is red.

## Units (each at most 250 production lines, each rehearsed with `landing rehearse` before it lands)

- U1 (D1): the lane pause in the budget record, the custodian's exclusion, the two print lines, `landing.wait-hours`. Tests: a goal handed in with 1 h left waits 3 h in the lane and is not breach-stopped; its return resumes the clock; the custodian's real record shape.
- U2 (D2, Step 1): lane fix allowance and admission, `landing.fix-rounds`/`landing.fix-minutes`, the fix job runs under the lane's box, the return after the allowance names the rounds. Tests: fixture seams for admission; the real refusal text of finding 109 never appears for an own red with allowance left; a green batch admits no round.
- U2b (Step 3): `landing fix G --reason` with the LRP-01/LRP-02 obligations and the two-round history fixture.
- U3 (D3): the resolution-only read brief (diff against the automatic merge), the recorded wall time, the bound as a finding line. Tests: a merge with three resolved files produces a brief naming only their hunks; the real fix record shape.
- U4 (D4): `landing retract`, status and questions print the two verbs per scenario, the skill rule for a moved main. Tests: every stop/return/question path in internal/landing/plain names a resume and a retract verb (audit test); retract during a running proof abandons and rebuilds.

## Threat model and stop rule

Threats: (1) a paused clock hides a goal that never lands → `landing.wait-hours` finding line, and the lane's queue is visible in status; (2) a lane fix round changes a goal behind its seat's back → the commit carries Goal-Unit, lands on the goal's branch history through the merge, and the read judges it; (3) a resolution-only read misses a semantic conflict → the batch proof covers behaviour, the read covers intent; (4) retracting during a proof wastes the proof → accepted, the person chose it; (5) `landing fix` as an infinite escape → each grant is one round, recorded with the person's name. Stop rule for this design: Astra critique until no material finding (cap 3 rounds), then a shape pass; findings with no implementation impact end the loop (R-139-m1e family).

## Estimates
U1 2 h, U2 3 h, U3 2 h, U4 3 h build+read each; one real batch per unit through the lane.

## Amendment 2026-10-11 12:45 (Wido: "fixed immediately before we do anything else"): D7 records-only main moves never void the lane's work

Measured twice today (findings 92 and 125): the goal ledger's commits land on main (claim, hand-in, slice-start, incident clear), the lane treats every main move as a reason to drop its pending merge and rebuild the batch, so the machinery's own bookkeeping voided the pcc batch at 23:36 and the lane-first-run-fixes resolution at 12:33. The lane already treats a proven tree that differs only in goal ledger files as proven; the merge, read and push phases lack the same rule.

- D7a Classification of a main move. The move is measured from the main parent of the work in flight (the first parent of the pending merge or batch commit — what the resolution, read or proof was built on), not from the original batch base, to fetched main. It is records-only when every changed path of every commit in that range belongs to the lane's coordination-path set — exactly the set the proof already treats as proof-neutral (prove.go ~938), extracted into one shared list and not widened (Astra LRP-N1: reads are not in it); anything else (code, metasystem.conf, testing.json, and plans or docs that audits read, such as plans/README.md) is a code move. `landing status` reports `main_move`: none, records-only (with the count) or code.
- D7b A records-only move changes nothing in flight: the pending merge, the resolution job, the read and the running proof stay valid; the batch base stays; status prints "main gained N record commits since the batch base; the batch stands". The landing agent's skill fetches and rebuilds the batch only when status reports a code move or no batch; on a records-only move it continues the step it is in.
- D7c Push after a records-only move: `landing push` folds the record commits into the batch commit with one merge (the batch tree never touches those paths), checks that the resulting tree differs from the proven tree only in coordination paths (the same shared list), and pushes that merge; the push record names the folded commits. A fold that conflicts or differs in a unit path falls back to today's rebuild.
- D7d A code move keeps today's behaviour: rebuild and prove again.

Threats: a commit misclassified as records-only that changes behaviour — only the proof's own coordination-path list is neutral (Astra LRP-05: plans/README.md is an audit input, so 'non-unit' is not neutral), everything else rebuilds; measuring from the original batch base would re-count code already rebuilt on (LRP-06), so the move is measured from the in-flight work's main parent. Critique: Astra round 1 (12:44) 2 material (LRP-05 non-unit is not proof-neutral; LRP-06 measure from the in-flight parent) folded; round 2 (12:49) 0 material (LRP-N1 non-material: do not widen the list) — closed. Stop rule: no material finding (cap 2), then build.

Unit U0 (lands FIRST, before U2; at most 250 production lines; rehearsed on a candidate that carries the rehearsal verb): D7a-D7d, the skill lines, tests: a pending resolution survives a slice-start commit on the scratch origin (fixture seams), a push after a records-only move produces one fold merge and lands, a code move rebuilds; real shapes: today's slice-start commits 823722cc1f27 and 9ae0a0ff2 as fixtures of a records-only move.
