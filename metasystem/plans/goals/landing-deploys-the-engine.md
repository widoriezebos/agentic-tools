# landing-deploys-the-engine

- State: approved
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="Changes how every seat gets its engine; design first"
- Tier: 2
- Intent: What: after a successful landing push, the lane runs the project's deploy step on the pushed commit. For the metasystem itself: build the engine from that commit into ~/.metasystem/engines/<sha>/, atomically repoint ~/.metasystem/bin/metasystem (on PATH) to it, seats pick it up at their next safe point and their hooks call the central binary; deploy rollback repoints to the previous engine. Other apps supply their own deploy step (docker, kubernetes) at that extension point. Why: Wido 2026-10-01 - per-checkout rebuild/restart caused today's stale engines, generation mismatches and refused proofs; one place with the newest proven engine removes that class. Pros: one engine everywhere, hard cutover, instant rollback. Cons: must replace or feed the per-checkout engine pins; a bad landing reaches every seat at once; skills/docs still come by pull.
- Origin: human
- Next step: After the lane's first real landing: one design page (core question: engine pins vs central binary), Astra at most 2 rounds, one builder, one real run Wido 2026-10-01: full control by verbs where applicable (deploy status/now/rollback/pause, which engine each seat runs) and all of it from the UI too (Fleet page: current engine, per-seat engine, deploy now, rollback, history); the design names each verb and its UI action.
- OpenedAt: 2026-10-01T15:20:49Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-01T15:20:56Z revision=2 opid=VFJM4WBQCXHR5THQ0H9X6SD9Z4-m1e-528c72bf authority=proven digest=a0e3224f682ac8afd96f174a1e1e3257a53ce47bc8c9eb49490a58a751b29a1d episode=2

History:
- 2026-10-01T15:20:49Z VFWZW80FZ53PQW3JTWNRYTDB40-m1e-528c72bf open actor=human:Wido targets=landing-deploys-the-engine
- 2026-10-01T15:20:56Z VFJM4WBQCXHR5THQ0H9X6SD9Z4-m1e-528c72bf approve actor=human:Wido targets=landing-deploys-the-engine
- 2026-10-01T15:22:30Z Q74HE6HAW8JE9DMCXATGWJ44JQ-m1e-528c72bf edit actor=human:Wido targets=landing-deploys-the-engine
Integrity: sha256=5a0f21322a776a08cb9951645e169d986cce006ab36f8f9e121a86ccc0301c98
