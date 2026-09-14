# stop-response-carries-a-structured-report-reference

- State: approved
- Priority: 2
- Sequence: 25
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: a wrong reference would break delivery matching for every Stop, but the field is the same data the parent already holds; novelty 1: the reference exists, it moves from text to a field; exposure 3: every Stop of every seat; accumulation 2: settlement, the steward check and the beds read it"
- Tier: 2
- Intent: Delivery-efficiency phase D, delegate loop. The Stop response carries its report reference (installation, id, alias, path, digest) as a structured field, and the human line is rendered from it; settlement, the steward's delivery check and the beds resolve the report from the field and never from the console wording, so a change to the human line cannot break delivery matching or a bed's report lookup. Why: two Stop-message wording changes landed during the stop-decisions build; one broke the receipt matcher outright and four build rounds were spent on text drift. DONE: a fixture changes the human line's wording and every matcher and every bed still resolves the report; a response without the field is refused as unreadable. every mechanism in this goal lives in the Go engine, the ledger verbs and the adapter contract, never in one runtime; it is proven on at least two runtimes.
- Origin: human
- Next step: Design page: the field, who writes it, who reads it, the compatibility rule for responses without it; then build.
- OpenedAt: 2026-09-14T15:31:30Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T18:53:59Z revision=3 opid=B39ANC154E1FM3MF7A24V0R51V-m1e-c6925449 authority=proven digest=b1291aeb353351c5d395b3474fd1d47a1f6cffe54c2ee67aad776f9d54105220

History:
- 2026-09-14T15:31:30Z CFBXKJ2RZ7MBEYMEFA4NKB3012-m1e-c6925449 open actor=human:Wido targets=stop-response-carries-a-structured-report-reference
- 2026-09-14T15:32:28Z VV8NMV2K00RHAP0G4TBDQJ6BVH-m1e-c6925449 set-priority actor=human:Wido targets=stop-response-carries-a-structured-report-reference reason=priority-order subject=stop-response-carries-a-structured-report-reference from=unranked to=2:25 requested-sequence=append
- 2026-09-14T18:53:59Z B39ANC154E1FM3MF7A24V0R51V-m1e-c6925449 approve actor=human:Wido targets=stop-response-carries-a-structured-report-reference
Integrity: sha256=1b525ea425d88ca696f76f6d16a985a8525de084e64aa81a7e884b1304e8d5a9
