Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal breach-clock-and-budget-honesty)
Date: 2026-09-02

# Goal

Revision 6 of metasystem/plans/breach-clock-and-budget-honesty-design.md
(revision 5 landed, in your worktree). Sol's build breach-build-1b stopped at
three Fix 3 gaps, decided in
metasystem/records/misc/breach-build-1b-gaps.md (landed; read it first, the
decisions are binding and quoted there with their wording). Fold the three,
then the consistency pass over Fix 3 (state model, verb mechanics, the
one-claim section, the consumer table), the proof plan and the disposition
table (add a "build gaps" row). Edit in place; diffBoundary is that one
file. Keep it under twelve minutes; verify against the tree lines named
here, no wider reading.

# The folds

1. Gap 1: the state-model transition sentence. From the claimed-and-fenced
   shape `goal resume` keeps the claim and starts a fresh episode
   (`State: claimed`, today's path, metasystem/internal/goal/stop.go
   lines 378-411 with `bindClaim`); from parked-with-breach it binds no
   claim (`State: queued`). Say it once in the state model and once in the
   package rule, identically.
2. Gap 2: the only-claim invariant is restated at the quota's unit
   (metasystem/internal/goal/validate.go lines 250-283 count one arc under
   one claimant once). New rule and its exact wording from the record. State
   the consequence for `CloseStop` on an arc member (never conflicts), for a
   different-arc or unarced second claim (refused as before), for the
   orientation branch (every claim of the machine is checked for a fence,
   the way-out line names the fenced goal), and for release of the fenced
   member (siblings stay claimed). Record the two rejected alternatives in
   one sentence each. Proof plan: a validate test with an arced fenced goal
   and a claimed sibling (accepted), the same with a different-arc claim
   (refused, exact wording), an unarced fenced goal with any second claim
   (refused); a `CloseStop` test on one member of a two-member claimed arc
   (the fence closes, the sibling stays claimed); the orientation test with
   the fenced goal not first in the claimed list.
3. Gap 3: the consumer table gains the row
   metasystem/internal/steward/delivery.go (lines 56-79): UNCHANGED, with
   the reason from the record; note the legacy reader
   `goal.ParseWorkingDuration` in the day-token inventory's legacy-reader
   class if it is not already there (add the row if missing, say so).

Bump the header to revision 6 naming the build's gaps. R-31: no benchmarks.

# Constraints

Wall-clock budget: 12 minutes. Design only; edit nothing but the design
file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the design file.

# Gap Rule

stop and report a gap; never fill it silently.
