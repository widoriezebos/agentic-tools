# gap-rule-deletes-built-work

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: wasted job-minutes and a stalled chain, no wrong code; novelty 1: text and one schema field; exposure 2: every implementer round on every chain reads the gap rule; accumulation 1: one rule, one field, one fixture"
- Tier: 2
- Intent: Two consecutive build rounds on fleet-coordinator-brain (2026-09-07, Sol implementer) produced no code: each built part of section 1 to its fixture boundary, met one specification gap (a route the design misnamed; then a header bound 20 bytes under the sum of its own caps), and DELETED the passing work under the brief's standing gap rule ('Stop and report a gap; never fill it silently'), returning a clean tree and an empty diffBoundary. 240 job-minutes bought two gap reports. The gap rule must mean: stop ADDING at the gap, keep what is built and green, report the gap with the resolution proposed; a numeric contradiction the page's own numbers settle is resolved and reported, not a stop. DONE means: (1) the standing gap-rule text every brief carries (the role packets under scripts/agents/roles and the brief templates) says keep-and-report, never delete; (2) the implementer return schema carries a 'partial' marker with the sections built so a round that stops at a boundary is still a round with a diff; (3) the dispatcher's conformance stage accepts a partial round's diff as reviewable; (4) a fixture: a fake-runtime round that returns partial with a non-empty diff is recorded as such, not as failed.
- Origin: main
- Next step: Tier 2, MECHANICAL: edit the role packets' gap rule text, add the partial marker to the v2 implementer schema (internal/schema), teach validate conformance to accept it, fixture in dispatch-fixtures.sh. Sol builds, Fable reviews, land. Interim: every brief m1 writes carries the corrected gap rule verbatim (see scratch brain-build3-brief.md).
- OpenedAt: 2026-09-06T22:39:47Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T22:39:47Z CCWYBVHJDBB25N492VQZ0AKKKG-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=gap-rule-deletes-built-work
Integrity: sha256=582737ae69580a69c8340b87adc3aac5227928acd08b620936e89a5a27736344
