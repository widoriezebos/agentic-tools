# every-launched-agent-waits-inside-its-turn

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="One instruction sentence added to launch briefs; prevents lost work"
- Tier: 1
- Intent: Every agent the machinery launches headless (seat sessions already; also builders, critics, readers and the landing agent) is told that its process ends when its turn ends and must wait inside the turn (Wido 2026-10-03, lessons from m1f: attempt 4 of root-structs ended its turn while its full test run was still going, so the run died with it; the rule exists only in the seat brief, internal/steward/seat_start.go).
- Origin: main
- Next step: Find every launch brief template (unit builds, critic and read delegates, landing agent) and add the one-sentence rule in the shared place they are assembled; keep the message and always-loaded budgets green; test that each launched kind's brief carries it.
- OpenedAt: 2026-10-03T07:55:58Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-10-03T07:55:58Z FP9QK48TH80TX90Z65RW40T7PY-m1e-718ba0eb open actor=human:Wido targets=every-launched-agent-waits-inside-its-turn
Integrity: sha256=22b3bcbf392b4085cc4375639ece2764cce1fd5c813b8809cd604263d8bfc915
