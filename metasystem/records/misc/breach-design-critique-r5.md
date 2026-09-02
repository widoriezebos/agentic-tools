# Sol design-critique of the breach-machinery design — round 5 (2026-09-02)

Job breach-design-crit5 (design-critic, codex gpt-5.6-sol), reviewed commit
ec16d741, design revision 6 (plans/breach-clock-and-budget-honesty-design.md),
brief plans/breach-clock-critique-r5-brief.md. One material finding
(wording), two non-material notes. Full return:
artifacts/agents/breach-design-crit5/rounds/1/return.json.

## What held

Sol's binding test: would an implementer build something different or
wrong? The restated invariant admits every quota-admitted same-arc
`CloseStop` shape and refuses claims outside the fenced goal's arc, so the
breaker's own fence commit is never refused; resume's claimed path rebinds
the same machine and lineage, the parked path binds no claim; Fix 1
(including the inherit rule) and Fix 2 intact.

## BCD-R5-001 (medium, material)

The fenced-claim diagnostic's format string takes machine, offending goal,
fenced goal, stop id; the design's substitution list said "the offending
goal, the fenced goal, the fenced goal, the stop id" and the proof plan
"C, A, A, and A's stop id", omitting the machine. Exact refusal wording and
its asserted test are implementation contracts.

## BCD-R5-002 (low, not material)

The delivery-consumer rationale said a fenced claim "has failed to deliver";
delivery.go:105-144 can return alive for a fenced claim. Leaving the consumer
unchanged was unambiguous.

## BCD-R5-003 (low, not material)

"One arc under one claimant" misdescribes validate.go:252-281, which groups
claims by machine and arc and never compares lineage; the invariant itself
matches the validator's actual unit.

## Orchestrator decision (m0b, 2026-09-02 19:30Z)

All three folded by hand as revision 6a (wording only: the substitution
order in the rule and the proof plan; the delivery rationale; the quota's
unit stated as machine and arc). No sixth Sol design round: the change is
wording, the register is the record, and the code critique checks the built
diagnostic against the format string. The build proceeds from
plans/breach-clock-build-brief.md against revision 6a.
