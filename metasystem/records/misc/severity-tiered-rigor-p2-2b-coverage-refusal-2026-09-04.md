# Tiering machinery, slice 2b: the landing gate refused the closed chain on goal-package coverage

Date: 2026-09-04, 10:17 local. Chain str-p2-build-2d closed on a clean closing review (str-p2-build-2d-cc3, bound to round 2). The landing of its certified diff (46 files) was refused by the commit gate's coverage ratchet:

```
coverage delta: packages below floor:
  ./internal/goal: measured 79.6%, floor 80.0%
agent commit refused: staged Go package coverage check failed
```

Every other touched package is above its floor (counselor 84.8 / 83.7, critique 96.2 / 96.0, dispatch 77.0 / 75.9, returnschema 82.1 / 76.3, validate 80.8 / 79.9).

Cause, on the seat: the carry brief (plans/severity-tiered-rigor-p2-2b-carry-brief.md) named coverage floors for dispatch and validate only, and the seat's own gate run measured those two; the slice's goal-package additions (`goal accept-risk`, `goal discharge-review-obligation`, the review-obligation records) were never measured against the goal floor before the commit gate. Not a defect in the gate or the chain.

Remedy: a closed chain takes no follow-up, so a fresh tier-bound chain str-p2-build-2e carries the same certified diff and adds a tests-only round in internal/goal (plans/severity-tiered-rigor-p2-2b-carry2-brief.md), followed by a closing critic bound to that round. Reserved: 40 + 30 minutes against the goal's budget.

Two earlier landing attempts of the same diff failed before the gate on the seat's mechanics, not on the metasystem: the foreground shell's ten-minute cap ended the commit gate's goal-package run, and a detached shell was refused by the checkout lease as an outsider (`OWNED-ELSEWHERE`, caller `UNTRUSTED`), which is the one-writer law working as designed. The landing cfb1b3f7 under this slice's name carries only its review record.

## Wido's words this morning (verbatim)

- On the docs of part four, whose "PENDING Part Two" markers must be lifted once slices 2b and 2a are on main: "part 4: just finish yourself". The docs re-touch is m3's, with 2a's landing.
- On the budget: "Budget 2a: is this already doing proper accounting i.e. real time spent summed?" Answer given: no; the minutes member sums each dispatch's reserved cap (`ReservedJobMinutes += capMinutes`, internal/dispatch/budget.go), and the elapsed member is wall-clock since the claim against the elapsed limit. Whether the minutes member should mean real time spent is a change to the tuple's law and his to order; no order given.
