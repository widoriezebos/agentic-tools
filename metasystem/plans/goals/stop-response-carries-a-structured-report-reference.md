# stop-response-carries-a-structured-report-reference

- State: queued
- Priority: 2
- Sequence: 25
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: a wrong reference would break delivery matching for every Stop, but the field is the same data the parent already holds; novelty 1: the reference exists, it moves from text to a field; exposure 3: every Stop of every seat; accumulation 2: settlement, the steward check and the beds read it"
- Tier: 2
- Intent: Delivery-efficiency phase D, delegate loop. The Stop response carries its report reference (installation, id, alias, path, digest) as a structured field, and the human line is rendered from it; settlement, the steward's delivery check and the beds resolve the report from the field and never from the console wording, so a change to the human line cannot break delivery matching or a bed's report lookup. Why: two Stop-message wording changes landed during the stop-decisions build; one broke the receipt matcher outright and four build rounds were spent on text drift. DONE: a fixture changes the human line's wording and every matcher and every bed still resolves the report; a response without the field is refused as unreadable. every mechanism in this goal lives in the Go engine, the ledger verbs and the adapter contract, never in one runtime; it is proven on at least two runtimes.
- Origin: human
- Next step: Design page: the field, who writes it, who reads it, the compatibility rule for responses without it; then build.
- OpenedAt: 2026-09-14T15:31:30Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-14T15:31:30Z CFBXKJ2RZ7MBEYMEFA4NKB3012-m1e-c6925449 open actor=human:Wido targets=stop-response-carries-a-structured-report-reference
- 2026-09-14T15:32:28Z VV8NMV2K00RHAP0G4TBDQJ6BVH-m1e-c6925449 set-priority actor=human:Wido targets=stop-response-carries-a-structured-report-reference reason=priority-order subject=stop-response-carries-a-structured-report-reference from=unranked to=2:25 requested-sequence=append
Integrity: sha256=7c27af7aac1fa6ac0518225e5493c0dfa23fddb7f9db69c4e8a4e0a7123af882
