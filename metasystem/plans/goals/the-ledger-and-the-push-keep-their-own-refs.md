# the-ledger-and-the-push-keep-their-own-refs

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Git ref namespaces with a one-release migration: readers accept both prefixes, writers write the new one, the push never lists the ledger's prefix; a mistake is a refused publish with the ref named, never a lost commit"
- Tier: 2
- Intent: The goal ledger's transaction refs and the branch push's transaction refs live under different prefixes, the push's sweep touches only its own blobs for one release of migration, and a ledger transaction whose owner died before pushing is terminalized and its refs cleaned by the dead-owner path (finding 432; design plans/designs/the-stewards-act-on-what-they-see.md (accepted, revision 4) D8, G5)
- Origin: main
- Next step: On m1e's build lane after a-review-keeps-every-read: opus-designer cuts the brief from the page (BRIEF-CHECKLIST, Read each round: yes, one Astra round, the reverse-dependent packages run whole); one unit, about 35 production lines: goal/branch/push.go, push_repository.go, goal/txn.go, goal/recover.go
- OpenedAt: 2026-10-27T15:28:28Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-27T15:29:19Z revision=2 opid=77TBXPFR74H2NYNS3SNGDCFDCG-m1e-e06ab4ba authority=proven digest=60dd7e887fee0ac6bb87d571d75b4ff1d20c4efc8fa99fd4b68b1f35ce399750 episode=2

History:
- 2026-10-27T15:28:28Z VSAGRBYB5HS5RKZ9T4EA5J8ATR-m1e-e06ab4ba open actor=human:Wido targets=the-ledger-and-the-push-keep-their-own-refs
- 2026-10-27T15:29:19Z 77TBXPFR74H2NYNS3SNGDCFDCG-m1e-e06ab4ba approve actor=human:Wido targets=the-ledger-and-the-push-keep-their-own-refs
Integrity: sha256=6057304e6d9509f8ce7549dfbec8af6afe290e0b542f83b772fd4dd27f476127
