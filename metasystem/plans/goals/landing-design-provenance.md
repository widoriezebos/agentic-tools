# landing-design-provenance

- State: queued
- Priority: 2
- Sequence: 9
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="A wrong acceptance gate can admit unreviewed changes across every node; it extends existing evidence binding but crosses dispatch and landing."
- Tier: 3
- Intent: A design-bearing landing is refused unless its implementation names the certified design revision and independent design critique admitted before the build, with evidence bound to the candidate being landed.
- Origin: human
- Next step: INTENT: deliver the deferred design-provenance slice of two-bars-for-changes as one independently verifiable gate. CONSTRAINTS: consume the designChain and critic identity produced by design-gate-at-dispatch; keep one existing landing owner; missing, stale or substituted evidence refuses; a design change invalidates the old binding; preserve the existing mechanical design exemption and any explicitly authorized waiver without weakening critique-always. Activation follows the existing observed-then-authorized promotion rule. FREEDOMS: record shape and placement within the existing landing evaluator. PROOF: omitted design, changed design blob, wrong critic, and post-review candidate change refuse; a matching certified chain passes. This goal owns this slice alone; caller classification, witness and growth-fuse work remain with two-bars-for-changes. ROSTER: current configured development builder and independent critics; no fixed model copied from historical handoffs.
- OpenedAt: 2026-09-07T21:10:16Z
- Revision: 3
- BlockedBy: design-gate-at-dispatch, path-class-manifest
- Labels: headless-fleet, headless-process
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-07T21:10:16Z VRAAJCC9NXEXBP708G8AWM8JQN-m1-76f67331 open actor=human:Wido targets=landing-design-provenance
- 2026-09-07T21:12:01Z RPKM7WBBCDYH5MC47RN3ZCNKX0-m1-76f67331 edit actor=human:Wido targets=landing-design-provenance
- 2026-09-08T15:57:40Z DVA3RH85GEHAGDFGWEWCTRX68M-m1-7cd0bd60 set-priority actor=human:Wido targets=landing-design-provenance reason=priority-order subject=landing-design-provenance from=unranked to=2:9 requested-sequence=9
Integrity: sha256=cbcfd7dc2d15bfd9f5b8e2f5627d2ce3afac1926c04c5a1ac54e47da91c482f8
