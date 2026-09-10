# dispatch-admission-refuses-on-a-fenced-sibling

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: after breach-stop-wedges-seat lands, a seat can claim the next goal beside a breach-stopped one but cannot delegate any work on it until a human resumes the stopped goal, so the wedge moves from claim to dispatch; novelty 1: one predicate in the machine-wide claim scan of dispatch admission; exposure 3: every seat whose held goal ever breach-stops; accumulation 2: each stranded claim freezes delegation on that machine until cleared"
- Tier: 3
- Intent: EvaluateGoalAdmission in internal/dispatch/admission.go scans every claim this machine and lineage hold and, for a breach-stopped one, adds a refusal with the live stop reason and exits 10; dispatch.sh then records REFUSED-BUDGET and runs the breach-stop routes, which skip a fenced goal whose stop batch is complete, so the dispatch dies with 'goal admission required breach-stop but supplied no stoppable route'. Found by the closing critic of chain bsws-build1b-20260909 on 2026-09-10 as BSW-12, at the three-read ceiling (R-42-m0), so it is carried here instead of a fourth read: breach-stop-wedges-seat lands with the claim wedge removed and this goal removes the dispatch wedge. The fold is already built and tested as round 7 of that chain (worktree bsws-build1b-20260909, the round-7 diff is retained in its artifacts): the machine-wide scan skips fenced claims when the dispatch is for another goal; dispatch against the fenced goal itself stays refused by the per-goal revision admission; three fixtures (live beside fenced passes; the fenced goal itself refused; a second fenced claim changes nothing). DONE means that fold lands with one independent read, and the live proof is claim AND dispatch on this machine while a fenced goal is held.
- Origin: main
- Next step: Appetite: 2h. Adopt the round-7 diff of chain bsws-build1b-20260909 (admission.go and its tests) into a fresh chain's round 1 on current trunk, run one independent code-critic read, land, then prove live: hold a fenced claim on this machine, claim another goal, dispatch a job on it, watch it start.
- OpenedAt: 2026-09-10T08:18:11Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T08:18:11Z QK0QJFZXFE0QWHTPRCHNF3C2S1-m1-c6925449 open actor=human:Wido targets=dispatch-admission-refuses-on-a-fenced-sibling
Integrity: sha256=3971c082dfbf6ca8428c26730e5d912e45404cdca4205d005a6725f84dff455a
