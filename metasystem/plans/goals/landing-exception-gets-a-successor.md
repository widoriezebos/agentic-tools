# landing-exception-gets-a-successor

- State: parked
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Removes an override path for persons; a mistake strands a person behind a wrong refusal, so the replacement must land first; the deletion itself is mechanical"
- Tier: 2
- Intent: Delete work land --exception and --using-exception, the override a person uses when the commit step refuses, once something replaces it (Wido 2026-10-03, answering question WKK79C96: 'not-now and explain in the new goal why this is delayed and why we should do it'). WHY IT WAITS: seat-path-lands-without-help deletes the other ways a seat lands by itself but keeps these, because they still work with a landing lane and every refusal of the commit step names them as its way past (internal/refusal/register.go, Override CarriedLanding); deleting them first would leave a person no way past a wrong refusal, against R-142-m1e (NO HAL9000). WHY DO IT: they are the last user of the carried landing (landpath land.go and carried.go, intent_exception*.go, internal/landing/carried*.go, branch.PrepareLanding, register rows 436-442), a second landing mechanism beside the lane, against Wido's 2026-10-01 target of one landing mechanism.
- Origin: main
- Next step: Decide what a person does when the commit step refuses once --exception is gone (for example: the person's decision is recorded with its plain-English impact and the commit goes through, per R-142-m1e and R-143-m1e); then delete D3 of the seat-path-lands-without-help design page under its deletion rule.
- OpenedAt: 2026-10-03T07:46:28Z
- Revision: 1
- BlockedBy: seat-path-lands-without-help
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Parked: by=human:Wido at=2026-10-03T07:46:28Z blocker=seat-path-lands-without-help because=blocked by seat-path-lands-without-help; returns when it is done

History:
- 2026-10-03T07:46:28Z 2MJBH9M0JGJCBWME8YVBCQ73WX-m1e-718ba0eb open actor=human:Wido targets=landing-exception-gets-a-successor,seat-path-lands-without-help
Integrity: sha256=633b5f3a8802d6c9264739d90b4f570738d20109d53f8fd199479e77b36491a2
