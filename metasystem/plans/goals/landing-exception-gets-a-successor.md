# landing-exception-gets-a-successor

- State: parked
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Removes an override path for persons; a mistake strands a person behind a wrong refusal, so the replacement must land first; the deletion itself is mechanical"
- Tier: 2
- Intent: Delete work land --exception and --using-exception, the override a person uses when the commit step refuses, once something replaces it (Wido 2026-10-03, answering question WKK79C96: 'not-now and explain in the new goal why this is delayed and why we should do it'). WHY IT WAITS: seat-path-lands-without-help deletes the other ways a seat lands by itself but keeps these, because they still work with a landing lane and every refusal of the commit step names them as its way past (internal/refusal/register.go, Override CarriedLanding); deleting them first would leave a person no way past a wrong refusal, against R-142-m1e (NO HAL9000). WHY DO IT: they are the last user of the carried landing (landpath land.go and carried.go, intent_exception*.go, internal/landing/carried*.go, branch.PrepareLanding, register rows 436-442), a second landing mechanism beside the lane, against Wido's 2026-10-01 target of one landing mechanism.
- Origin: main
- Next step: Decide what a person does when the commit step refuses once --exception is gone (for example: the person's decision is recorded with its plain-English impact and the commit goes through, per R-142-m1e and R-143-m1e); then delete D3 of the seat-path-lands-without-help design page under its deletion rule. FOLDED IN 2026-10-03 from human-carried-landing: apply ruling R-142-m1e (NO HAL9000) to the landing refusal codes a person can hit (the nine tier1-* rows in internal/refusal/register.go:313-321, bound to internal/landing/tierone.go): a person's landing decision is recorded with its plain-English impact (R-143-m1e) and goes through; this is the replacement that must exist before --exception and --using-exception are deleted.
- OpenedAt: 2026-10-03T07:46:28Z
- Revision: 3
- BlockedBy: seat-path-lands-without-help
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-03T12:25:38Z revision=3 opid=XY8E711TK25DGQC71WCX5E6FXT-m1e-718ba0eb authority=proven digest=ec11b336c536c2883eafb0b3ecaed0a3d0a6cd91f48f99680b2295375535085d episode=3
- Parked: by=human:Wido at=2026-10-03T07:46:28Z blocker=seat-path-lands-without-help because=blocked by seat-path-lands-without-help; returns when it is done

History:
- 2026-10-03T07:46:28Z 2MJBH9M0JGJCBWME8YVBCQ73WX-m1e-718ba0eb open actor=human:Wido targets=landing-exception-gets-a-successor,seat-path-lands-without-help
- 2026-10-03T09:57:30Z 1T43CMJ5Z2HMVM36BY7YQ5AYY6-m1e-718ba0eb edit actor=human:Wido targets=landing-exception-gets-a-successor
- 2026-10-03T12:25:38Z XY8E711TK25DGQC71WCX5E6FXT-m1e-718ba0eb approve actor=human:Wido targets=headless-fleet-coordination-proof,landing-design-provenance,landing-exception-gets-a-successor,live-proof-needs-a-dispatchable-verifier,next-adoption-follow-ups,recovery-rehearsal,seam-declaration-convergence,terminal-enrollment-per-computer,token-spend-fence,trunk-versus-candidate-is-a-verification-step,uncapped-delegate-fanout
Integrity: sha256=8d0df2745e15f1f95001259bcaf58d04942ee38121e223621340389aa517aed3
