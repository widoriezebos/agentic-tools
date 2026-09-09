# correction-cost-machinery

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="Severity 3: admission and landing authority; a wrong widening lets an unproven seat land. Novelty 2: new grants and a retry envelope on existing owners. Exposure 3: every goal on every machine. Accumulation 2: admission, budget and the landing route."
- Tier: 3
- Intent: A bounded correction inside an approved budget costs one focused check and one proof, not a human terminal, a lease takeover or a ceremonial delegate chain. DONE means a second seat on the same machine can run a goal's proof, a retry within the approved envelope needs no new set-budget, and the mechanical source-copy handoff is gone from the chain landing route.
- Origin: main
- Next step: Evidence from goal coordinator-loop-prevention, 2026-09-09: the landing needed budget revisions 12, 13 and 14, each minted from Wido's enrolled terminal because a retry inside an already approved envelope had no cheaper path. The Fable seat diagnosed and fixed five defects in the candidate but could not run the proof: metasystem/cmd/metasystem/proof_run.go admitProofLaunch refuses a MAIN caller that is not the checkout-lease holder with active coordinator does not own the claimed goal reservation, while a HUMAN caller is admitted without that check, so the only routes were Codex's seat or Wido's terminal. Codex's recovery plan (/Users/wido/metasystem-evidence/agentic-tools/coordinator-loop-prevention/landing-recovery-plan-20260909.md, step 4) budgeted three native executions, one of which was an implementer job whose only work was to copy and hash preserved source so the chain landing in metasystem/scripts/agents/land.sh would have an implementer return. A 60-minute cap was one bad scheduling day away from killing a healthy 50-minute run; caps are hang bounds and the guidance for full selections belongs in metasystem/docs/project-rules.md. Mechanism to design, smallest first: (a) admission accepts a MAIN caller whose main is announced on the same machine and whose goal claim epoch matches, or a human-granted second-seat-may-prove mark recorded on the goal; (b) a typed retry decision under an unexpired revision consumes attempts and reserved minutes from that revision without a new set-budget, coordinated with breach-clock-and-budget-honesty, which owns the raise-resets-the-clock defect, and with hp-resume-takes-its-budget-from-the-ledger; (c) the chain route accepts a recertification whose delta is a reviewed patch with a critic verdict, with no byte-copying implementer job. Nothing here weakens identity or custody. Each part is design-bearing: design round, Sol critique, then build. Order after single-owner-per-decision; do not start while coordinator-loop-prevention is in flight.
- OpenedAt: 2026-09-09T20:15:56Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-09T20:15:56Z HCBXY60AEQVBHTNPZJHP1RVJV9-m1c-8d678ae8 open actor=m1c+main-1788963308-60248-b019cb targets=correction-cost-machinery
Integrity: sha256=6d0b29733594e2ca1a6251831a737e7d9174858522abaf979a6aecaaa2f2cb74
