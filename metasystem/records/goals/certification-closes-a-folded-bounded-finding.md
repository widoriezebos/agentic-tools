# certification-closes-a-folded-bounded-finding

- State: abandoned
- Risk: severity=3 novelty=3 exposure=3 accumulation=2 basis="severity 3: a certification that binds an unread change to a clean read lands code no critic read; novelty 3: a new proof contract (change units, counterfactual test runs, one closure object) with no precedent in the engine; exposure 3: every chain close and landing that uses it; accumulation 2: the register, the final-tree gate, the hazard close and landing all consume it"
- Tier: 3
- Intent: Split from critique-closes-on-folded-proof on Wido's word of 2026-09-13 (option (a) of that page's section 13): low and medium findings folded with a named passing test close on the join through an explicit certification record that binds the reviewed subject, the attributed correction, the discharged findings and the test evidence to the final tree, and every gate (register close, the final-tree gate in internal/validate/conformance.go, the hazard close, landing) recomputes and verifies it; a correction beyond the recorded boundary takes another read. DONE means: a certification unit is a change unit keyed by path, kind and blob (not a textual hunk); each certified finding is discharged by a test whose relevance is proven by a counterfactual run (the correction absent fails, present passes) or, for a proof-gap finding, by an additive test; one closure object on the implementation root is the single authority every gate reads; the write is idempotent under a fixed lock order; bar (a) of plans/two-bars-for-changes-design.md is amended to admit a verified certification; proven by Go tests for each gate and a land-fixtures leg. Design first on the design lane, starting from records/misc/critique-closes-on-folded-proof-design-critique-r1.md and -r2.md (the eight findings each), never from revision 2's textual-hunk boundary.
- Origin: human
- Next step: Design first (seat writes, Codex gpt-5.6-sol reads): the certification record around change units and counterfactual test runs, one closure object shared with critique-closes-on-folded-proof's prior-clean-read closure, then slices with Opus reads. Queued behind critique-closes-on-folded-proof's narrowed slices; tierless until ranked.
- OpenedAt: 2026-09-13T07:45:37Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Abandoned: by=human:wido at=2026-09-30T19:02:58Z revision=2 opid=2GZY3QMMAGKDS9SSZ87EP79QJ1-m1e-b6a4eb0a because=No longer needed: non-breaking findings do not gate a merge anymore (Wido 09-17, land fast and fix forward). Its parent critique-closes-on-folded-proof is concluded in the same pass.

History:
- 2026-09-13T07:45:37Z ABW4ZY0AKR1F32QG5BZ0CSCF31-m1e-c6925449 open actor=human:Wido targets=certification-closes-a-folded-bounded-finding
- 2026-09-30T19:02:58Z 2GZY3QMMAGKDS9SSZ87EP79QJ1-m1e-b6a4eb0a abandon actor=human:wido targets=certification-closes-a-folded-bounded-finding reason=No longer needed: non-breaking findings do not gate a merge anymore (Wido 09-17, land fast and fix forward). Its parent critique-closes-on-folded-proof is concluded in the same pass.
Integrity: sha256=45a2bf05b7e94737da36a9cf9bfb4ec2d522cf2adc877d689712c04ff23f30eb
