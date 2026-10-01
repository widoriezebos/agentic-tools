# landing-deploys-the-engine

- State: approved
- Priority: 1
- Sequence: 4
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="Changes how every seat gets its engine; design first"
- Tier: 2
- Intent: What: after a successful landing push, the lane runs the project's deploy step on the pushed commit. For the metasystem itself: build the engine from that commit into ~/.metasystem/engines/<sha>/, atomically repoint ~/.metasystem/bin/metasystem (on PATH) to it, seats pick it up at their next safe point and their hooks call the central binary; deploy rollback repoints to the previous engine. Other apps supply their own deploy step (docker, kubernetes) at that extension point. Why: Wido 2026-10-01 - per-checkout rebuild/restart caused today's stale engines, generation mismatches and refused proofs; one place with the newest proven engine removes that class. Pros: one engine everywhere, hard cutover, instant rollback. Cons: must replace or feed the per-checkout engine pins; a bad landing reaches every seat at once; skills/docs still come by pull.
- Origin: human
- Next step: After the lane's first real landing: one design page (core question: engine pins vs central binary), Astra at most 2 rounds, one builder, one real run Wido 2026-10-01: full control by verbs where applicable (deploy status/now/rollback/pause, which engine each seat runs) and all of it from the UI too (Fleet page: current engine, per-seat engine, deploy now, rollback, history); the design names each verb and its UI action. Wido 2026-10-01: must fit other languages, e.g. Java/Maven: the lane only calls the project's deploy adapter (build artifact from the pushed commit, place/activate it, report the version, roll back) with language-neutral input/output; the metasystem's Go engine is just one adapter; a Maven adapter (mvn package/deploy of a jar, or a container image) is the design's worked second example.
- OpenedAt: 2026-10-01T15:20:49Z
- Revision: 6
- Pinned: m1e
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-01T15:20:56Z revision=2 opid=VFJM4WBQCXHR5THQ0H9X6SD9Z4-m1e-528c72bf authority=proven digest=a0e3224f682ac8afd96f174a1e1e3257a53ce47bc8c9eb49490a58a751b29a1d episode=2

History:
- 2026-10-01T15:20:49Z VFWZW80FZ53PQW3JTWNRYTDB40-m1e-528c72bf open actor=human:Wido targets=landing-deploys-the-engine
- 2026-10-01T15:20:56Z VFJM4WBQCXHR5THQ0H9X6SD9Z4-m1e-528c72bf approve actor=human:Wido targets=landing-deploys-the-engine
- 2026-10-01T15:22:30Z Q74HE6HAW8JE9DMCXATGWJ44JQ-m1e-528c72bf edit actor=human:Wido targets=landing-deploys-the-engine
- 2026-10-01T15:26:19Z RGT8WW3PPJ2NYY20GP9P9CZ15K-m1e-528c72bf edit actor=human:Wido targets=landing-deploys-the-engine
- 2026-10-01T15:56:54Z X0K7GM6X5JRN0KQ0MSB11JF6F5-m1e-9c612d71 set-priority actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,cross-cutting-change-inventories-its-readers,design-rounds-read-less-and-repeat-less,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,host-setup-from-scratch,landing-deploys-the-engine,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,reviewers-check-the-rulings,round-proof-feeds-the-next-brief,seat-successor-continues-a-handoff-without-a-human,seats-spend-tokens-in-bounded-sessions,spend-fence-reports-tokens-per-model-and-cause,steward-acts-on-behaviour-patterns,stop-hook-never-forces-an-empty-turn,system-start-launches-your-agent,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror reason=priority-order subject=landing-deploys-the-engine from=unranked to=1:4 requested-sequence=4
- 2026-10-01T20:00:54Z V6S8MSQXJAPDVQX6PM7E9HXNAZ-m1e-9c612d71 set-pin actor=human:Wido targets=landing-deploys-the-engine
Integrity: sha256=582c9263088745e47c9bbfa4471c437492e085484d7c2202330d7567da0d54ab
