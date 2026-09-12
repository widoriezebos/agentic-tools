Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal stop-message-truth)
Date: 2026-09-10

# Fold round four: CLAIM HELD never contradicts STILL WORKING

Follow-up round on chain stop-truth-build1-20260910. The third critic
(stop-truth-crit3b-20260910) found one material defect in round three
and one proof gap; both fold here, and nothing else.

## SMT-10 (medium): the CLAIM HELD line can be false

In internal/goal/turnverdict.go the idle rule returns early when a
delegate job of this checkout, a live proof attempt of this checkout or
a held landing lock is in flight (hasScannedWorkInFlight looks only at
the proof-attempt and landing-lock kinds). Otherwise, with a claim held,
it appends "CLAIM HELD: <goal>; nothing in flight". But decide() prints
"STILL WORKING: ..." from the scan's whole busy list, which also names a
running mission, a gate run and any scanner-visible job or open delegate
chain. So with a claim held and a gate run live the display says STILL
WORKING and then "nothing in flight". Fix: print the CLAIM HELD line
only when the scan's busy list is empty as well (a held claim with any
busy item prints nothing extra; the block decision is unchanged: only
the three suppressors reset the idle counter, exactly as round three
left it). Add one assertion in the existing idle tests: with a claim
held, nothing claimable and a busy item of kind mission, gate or job,
the display carries STILL WORKING and no CLAIM HELD line; with the busy
list empty it carries the CLAIM HELD line as before.

## SMT-11 (note, proof gap): the execution-root comparison has no negative case

In internal/report/scan_test.go, the own-attempt test's foreign case
varies only the control root. Add the case that keeps our control root
and names a different execution root (another repository root) and
expects the attempt to be ignored, so deleting the execution-root
comparison in internal/report/scan.go would fail a test.

## Mandate

1. SMT-10 and SMT-11 as above.
2. Nothing else changes; the printed shape is unchanged except that the
   CLAIM HELD line is absent whenever STILL WORKING prints.

## Proof

internal/goal (idle, brain, render, world tests), internal/report;
gofmt, vet. Report the round as your own.

## Constraints

Wall-clock budget: 15 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
