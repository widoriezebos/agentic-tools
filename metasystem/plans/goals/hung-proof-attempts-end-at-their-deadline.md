# hung-proof-attempts-end-at-their-deadline

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: a hung attempt holds a reservation and a seat's landing for a day; novelty 1: the deadline and watchdog exist and only their coverage changes; exposure 3: every attempt on every seat; accumulation 1: no new mechanism"
- Tier: 3
- Intent: Five m1b attempts hung 1.8 to 26.8 hours after a section died with exit 141 and the 120-minute deadline never ended them (proof-mtwiqdlq, proof-mtwlmbl5, proof-mtwp0iuu, proof-mtwvjs0d, proof-mtvdviee, 10 to 11 September); an m1e attempt stayed live 24.5 hours after its launcher died (proof-mtvl0alu); 23 process records still say running; 16 attempts ended launcher-killed without a terminal (proof-attempts.md section 6 of the delivery deep dive). DONE means: an attempt whose deadline has passed, whose launcher pid is dead, or whose section process ended without an end event is terminalized as failed with the cause within one watchdog tick; its reservation is released; metasystem status shows no reservation older than its deadline; a fixture kills a section mid-run and the attempt ends at the deadline with its evidence preserved. Goal 3 of plans/delivery-efficiency-plan.md.
- Origin: human
- Next step: Read the watchdog and launcher in internal/proofrun and the section adapter's end-event handling, design the three terminalization paths, write the fixture, land with its own battery.
- OpenedAt: 2026-09-11T15:44:57Z
- Revision: 2
- Pinned: m1e
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-11T15:44:57Z G00GBWKD64KA1ZT4J6WDGWFCSB-m1e-5083721b open actor=m1e+main-1789141490-27414-b9c0d2 targets=hung-proof-attempts-end-at-their-deadline
- 2026-09-11T15:46:56Z 7ZCH1E0KCPY6ZZ18JTDK66Q29T-m1-c6925449 set-pin actor=human:Wido targets=hung-proof-attempts-end-at-their-deadline
Integrity: sha256=b2ee1ed02e686dace5fb8ab7f18349e4eae95ac18c08bed67b89d63d41a32efb
