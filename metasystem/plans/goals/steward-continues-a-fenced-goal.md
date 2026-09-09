# steward-continues-a-fenced-goal

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: an unrunnable continuation is minted on every stop refusal, and if one were ever notified it would dispatch a job at a goal no verb can continue; novelty 1: one predicate in the steward's choice of continuation target; exposure 3: every seat's steward; accumulation 2: one orphan intent per refusal with no cancel verb, unbounded"
- Tier: 3
- Intent: The steward picks a breach-stopped goal as its continuation target. On m1d, 2026-09-09, every stop-hook refusal selected the machine's held goal account-provenance, which was breach-stopped with a stale stop batch that no verb could resume (records/misc/account-provenance-resume-is-wedged.md), and minted a steward continuation intent for it: 1f0a5fd7ca962a31, 554ffe2e728e5813, a8bcb766c7acaa4e, d625e34b74f10868, f97ad5b246d3c086, all in artifacts/agents/steward/intents, all unnotified and undispatched, all stamped fenceAtMint=19 against a capability at 21. None can ever run, none can be cancelled, and each refusal added one. Sibling of steward-revives-a-done-goal, which refuses revival of a done goal or one whose holder is alive; this is the fenced case. DONE means the steward never selects a goal with a standing StopFence as a continuation target, an intent minted under a fence that then advances is discarded rather than kept, and an operator can cancel a minted intent by name; a fixture proves a fenced held goal yields no intent and the seat's idle verdict says why.
- Origin: main
- Next step: Appetite: 2h. If breach-stop-wedges-seat lands the shared predicate (a claim under a StopFence is not this machine's live work) in the turn verdict, the selection half of this is done and only the discard-on-mint and the cancel verb remain. Read the steward's target selection and the intent record writer; add a discard on mint-time fence mismatch and a cancel verb that moves an intent to artifacts/agents/steward/cancelled with a reason. Canary: one fixture with a breach-stopped held goal, three refusals, zero intents, one verdict line naming the fence. Live specimens: the five intents above.
- OpenedAt: 2026-09-09T09:31:08Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-09T09:31:08Z TSM0NGHNQHSKGT31NZ27AEMXMH-m1-c6925449 open actor=human:Wido targets=steward-continues-a-fenced-goal
Integrity: sha256=5534e1c995d3e070b7781c430f97d83434928f6635b4c0aed6d6f1bd8cdbb07b
