# round-proof-feeds-the-next-brief

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="severity 2: a wrong findings file could refuse a legitimate follow-up or let a failing bed through; novelty 2: the round proof gains an output the dispatcher reads; exposure 2: every follow-up round; accumulation 2: later rounds build on the recorded findings"
- Tier: 2
- Intent: Delivery-efficiency phase D, delegate loop. After a round returns, the seat's round proof runs the beds the round's diff touches and records every failing assertion, with the bed's own words, in a findings file beside the round. A follow-up dispatch whose brief neither cites that file nor states that its findings are cleared is refused, so the seat stops copying bed output into briefs by hand and the delegate reads the failures the proof observed. Why: for fifteen rounds of the stop-decisions build the seat was a relay between the bed and the brief. DONE: a fixture round with a failing bed produces the findings file; a follow-up brief that ignores it is refused naming the file; a follow-up that cites it proceeds; a round with green beds produces an empty file and the follow-up proceeds. every mechanism in this goal lives in the Go engine, the ledger verbs and the adapter contract, never in one runtime; it is proven on at least two runtimes.
- Origin: human
- Next step: Design page: the findings record, its location beside the round, the dispatch check and the fixture; then build.
- OpenedAt: 2026-09-14T15:32:06Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-14T15:32:06Z HBZ6CGVD976G3XCTXEECV20PEX-m1e-c6925449 open actor=human:Wido targets=round-proof-feeds-the-next-brief
Integrity: sha256=1408e4ba442f4a3791412cc0eb236f34630190781475365c400b2c812f4dab84
