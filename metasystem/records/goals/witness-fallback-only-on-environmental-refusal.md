# witness-fallback-only-on-environmental-refusal

- State: done
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: each item removes measured waste inside one test or fixture and changes no law; novelty 1: known shapes; exposure 1: one group's duration; accumulation 1: none"
- Tier: 1
- Intent: The witness gate falls back to the plain full gate only for environmental refusals (snapshot extraction, witness write, witness_refusal reasons); a test failure or a coverage-ratchet refusal inside the snapshot is terminal. Today scripts/agents/witness-gate.sh lines 157 to 159 re-run everything on any snapshot failure, which cost 21 minutes for a deterministic answer on 2026-08-30. DONE means a deterministic red inside the snapshot ends the gate red at once with the reason readable, and an environmental refusal still falls back.
- Origin: human
- Next step: Slice 4 item 2 of plans/suite-speed-plan.md. Classify the refusal reasons the snapshot gate can produce before changing the branch; a fixture proves both paths. Code critique only.
- Concluded: Implemented by the coordinator on the m1e seat and landed 15af7625 by a human commit from the enrolled terminal in Wido's name (2026-09-11). Inside its clean snapshot the gate's own refusal (exit 3, environmental) now falls back to the plain gate when WITNESS_GATE_FALLBACK is plain, while a test or coverage-ratchet red ends the gate at once with its reason and no re-run; preparation failures fall back as before. Verified once through the engine: section/witness-gate-fixtures passed as a diagnostic proof of the candidate tree in 38 s with its canaries.
- OpenedAt: 2026-09-10T12:02:42Z
- Revision: 4
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-11T14:32:38Z revision=2 opid=TEXC4V8FVGWZ30S7MEG83851XA-m1-f47a9d40 authority=proven digest=5bf88cfbea8c1985b39ecbb0399b867747a191b7219f615f328d7421814d70e9

History:
- 2026-09-10T12:02:42Z E906XJ9V80KJBQ3NZ6QCDD5HJ2-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=witness-fallback-only-on-environmental-refusal
- 2026-09-11T14:32:38Z TEXC4V8FVGWZ30S7MEG83851XA-m1-f47a9d40 approve actor=human:Wido targets=witness-fallback-only-on-environmental-refusal
- 2026-09-11T15:02:30Z 2VMTGJ8N67T3CC3A6ZYKPPBCYT-m1e-892cdaec claim actor=m1e+main-1789030447-51011-5722fc targets=witness-fallback-only-on-environmental-refusal
- 2026-09-11T15:11:47Z 89NG332C6S02WSCZJ7PYP3NSD1-m1-c6925449 done actor=human:Wido targets=witness-fallback-only-on-environmental-refusal displaced=m1e+main-1789030447-51011-5722fc@2026-09-11T15:02:30Z
Integrity: sha256=6546f55ccad024abfc24e895e451b8744f1458862ee69b0b65aa0f8e45db61d7
