# dispatch-admission-refuses-on-a-fenced-sibling

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: after breach-stop-wedges-seat lands, a seat can claim the next goal beside a breach-stopped one but cannot delegate any work on it until a human resumes the stopped goal, so the wedge moves from claim to dispatch; novelty 1: one predicate in the machine-wide claim scan of dispatch admission; exposure 3: every seat whose held goal ever breach-stops; accumulation 2: each stranded claim freezes delegation on that machine until cleared"
- Tier: 3
- Intent: EvaluateGoalAdmission in internal/dispatch/admission.go scans every claim this machine and lineage hold and, for a breach-stopped one, adds a refusal with the live stop reason and exits 10; dispatch.sh then records REFUSED-BUDGET and runs the breach-stop routes, which skip a fenced goal whose stop batch is complete, so the dispatch dies with 'goal admission required breach-stop but supplied no stoppable route'. Found by the closing critic of chain bsws-build1b-20260909 on 2026-09-10 as BSW-12, at the three-read ceiling (R-42-m0), so it is carried here instead of a fourth read: breach-stop-wedges-seat lands with the claim wedge removed and this goal removes the dispatch wedge. The fold is already built and tested as round 7 of that chain (worktree bsws-build1b-20260909, the round-7 diff is retained in its artifacts): the machine-wide scan skips fenced claims when the dispatch is for another goal; dispatch against the fenced goal itself stays refused by the per-goal revision admission; three fixtures (live beside fenced passes; the fenced goal itself refused; a second fenced claim changes nothing). DONE means that fold lands with one independent read, and the live proof is claim AND dispatch on this machine while a fenced goal is held.
- Origin: main
- Next step: Appetite: 2h. Adopt the round-7 diff of chain bsws-build1b-20260909 (admission.go and its tests) into a fresh chain's round 1 on current trunk, run one independent code-critic read, land, then prove live: hold a fenced claim on this machine, claim another goal, dispatch a job on it, watch it start.
- OpenedAt: 2026-09-10T08:18:11Z
- Revision: 9
- Arc: fenced-claim-wedge
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-10T09:05:38Z revision=2 opid=9QG50YD61TG6WDQH6RPDW5W9GJ-m1-c6925449 authority=proven digest=2573697b22b5c7aef90b62309318320e91921760d8db98c1cf7d1b9a0c383701
- Sliced: machine=m1d lineage=main-1788941004-20871-6e7a43 revision=3 at=2026-09-10T09:06:42Z
- Claimed: machine=m1d lineage=main-1788941004-20871-67f3a0 at=2026-09-10T12:21:43Z revision=9 accountingRevision=9
- StopCapability: generation=9 revision=9 machine=m1d claimEpoch=3 fenceEpoch=0

History:
- 2026-09-10T08:18:11Z QK0QJFZXFE0QWHTPRCHNF3C2S1-m1-c6925449 open actor=human:Wido targets=dispatch-admission-refuses-on-a-fenced-sibling
- 2026-09-10T09:05:38Z 9QG50YD61TG6WDQH6RPDW5W9GJ-m1-c6925449 approve actor=human:Wido targets=dispatch-admission-refuses-on-a-fenced-sibling
- 2026-09-10T09:06:03Z 3CV1WQ8SFJTT99MFV6EMFDQ9D5-m1d-a38bcdde claim actor=m1d+main-1788941004-20871-6e7a43 targets=dispatch-admission-refuses-on-a-fenced-sibling
- 2026-09-10T09:06:42Z F33G4RS31MRHCPNWX48SWS0Q4W-m1d-a38bcdde slice-start actor=m1d+main-1788941004-20871-6e7a43 targets=dispatch-admission-refuses-on-a-fenced-sibling
- 2026-09-10T09:20:16Z X0RHBR4RYT2BFXN8RJ7EVK7P03-m1d-a38bcdde release actor=m1d+main-1788941004-20871-6e7a43 targets=dispatch-admission-refuses-on-a-fenced-sibling
- 2026-09-10T09:20:22Z E5DWEEBV5ZHF4ECKEPHX58Z2MX-m1d-a38bcdde set-arc actor=m1d+main-1788941004-20871-6e7a43 targets=dispatch-admission-refuses-on-a-fenced-sibling
- 2026-09-10T09:20:47Z N0EVC8WJ0YEPBC2Z3J4DGMXA57-m1d-a38bcdde claim actor=m1d+main-1788941004-20871-6e7a43 targets=breach-stop-wedges-seat,dispatch-admission-refuses-on-a-fenced-sibling
- 2026-09-10T12:21:39Z MN6B82HBNXG3X52PCNY17K73WH-m1d-a38bcdde release actor=m1d+main-1788941004-20871-6e7a43 targets=breach-stop-wedges-seat,dispatch-admission-refuses-on-a-fenced-sibling
- 2026-09-10T12:21:43Z 4KHQZ9SK4EERJY25Y929DM0BQJ-m1d-8651d169 claim actor=m1d+main-1788941004-20871-67f3a0 targets=breach-stop-wedges-seat,dispatch-admission-refuses-on-a-fenced-sibling
Integrity: sha256=c48c6f5c3379823a12a9097cf25c9e825b6b2eb4d75ea9f1074b4a7bd0c28b19
