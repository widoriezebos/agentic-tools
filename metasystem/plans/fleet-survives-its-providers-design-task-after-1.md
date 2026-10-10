# Task: revise the design fleet-survives-its-providers after critique round 1

Working Mode: Design
Revise plans/designs/fleet-survives-its-providers.md in place (keep Status: draft). Round 1 (Opus) found 6 material findings; the split was wrong. Re-cut and fold, with a "Round 1 findings and changes" table. Cap: unit <=250 production lines, <=5 units; one paragraph per unit Decision, exact behaviour and sites, so a builder cannot read more scope in.

Decided by m1e for Wido (record under "Decided by m1e for Wido", 2026-10-08, reversible): the goal keeps its approved intent, provider survival. Units:
- P1 host view (U1, filling a provider section from P2; ~190).
- P2 one mark per provider in the registered owner's state, probed at reset and cleared by the first success; move the existing outage writers and readers (internal/steward/seat_start.go:300, internal/missionrunner/loop.go:2104, :2163, internal/steward/runner.go:497, internal/steward/revive.go:169) (~230).
- P3 dependent clocks pause on that mark (internal/dispatch/budget.go:271, :867, internal/steward/tick.go:515); a stale mark is cleared and alerted (~200).
- P4 the minimal unit-boundary event; a headless session ends there through the existing `session handoff` verb (cmd/metasystem/context_verbs.go:75) (~240). Goal 4 (runs-advance-on-their-own) consumes it.
- P5 the restart bound in revive.go (~110) plus `host.builds=auto|N|person` checked in Manager.Start against current load and a declared `host.load-max` (no suite ratio) (~130); suite serialization is only a declaration over the existing admission cap (proof.admission.top-level-max, internal/proofrun/admission.go:31; one lane full proof per host already, internal/landing/plain/proof_admission.go:82), at most ~40 lines, inside P5 or P1.
Moved out, each with its destination written into that brief: the suite comparison against the unloaded baseline and the integration observations (old U2 and U5) to plans/process-changes-cover-declarations-and-interventions-design-brief.md (they feed machinery-measures-its-own-process's cost reader, internal/processmeasure); the pending-step queue, resume and recovery (old U4 part) to plans/runs-advance-on-their-own-design-brief.md; re-arm authorization, the unrecovered-seat ask and provider usage numbers stay in the follow-up brief (plans/fleet-provider-and-session-recovery-design-brief.md), which says it will be opened as a ledger goal after this design is accepted.
Findings 4-6 fold into the above (U3's premise; admit on current load, the ratio is a measure only; no production consumer for the comparison here).

Do not touch code, memory/, records/ or the ledger. Never open any metasystem.conf.local. Return: units with estimates and what changed per finding.
