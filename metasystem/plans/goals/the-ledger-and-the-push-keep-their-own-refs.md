# the-ledger-and-the-push-keep-their-own-refs

- State: approved
- Priority: 1
- Sequence: 84
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Git ref namespaces with a one-release migration: readers accept both prefixes, writers write the new one, the push never lists the ledger's prefix; a mistake is a refused publish with the ref named, never a lost commit"
- Tier: 2
- Intent: The goal ledger's transaction refs and the branch push's transaction refs live under different prefixes, the push's sweep touches only its own blobs for one release of migration, and a ledger transaction whose owner died before pushing is terminalized and its refs cleaned by the dead-owner path (finding 432; design plans/designs/the-stewards-act-on-what-they-see.md (accepted, revision 4) D8, G5)
- Origin: main
- Next step: On m1e's build lane after a-review-keeps-every-read: opus-designer cuts the brief from the page (BRIEF-CHECKLIST, Read each round: yes, one Astra round, the reverse-dependent packages run whole); one unit, about 35 production lines: goal/branch/push.go, push_repository.go, goal/txn.go, goal/recover.go
- OpenedAt: 2026-10-27T15:28:28Z
- Revision: 5
- Pinned: m1e
- Budget: elapsedLimit=2d attemptLimit=8 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=A9VDW2P446TYHY55CGX3B193BB-m1e-e06ab4ba minutes=1200 reviewRounds=20 goalRevision=2
- Approved: by=human:Wido at=2026-10-27T15:29:51Z revision=3 opid=A9VDW2P446TYHY55CGX3B193BB-m1e-e06ab4ba authority=proven digest=8cc83570babcf538a987c37e1bf84f08853deb385a921e9230a3fadd2fcefd54 episode=3

History:
- 2026-10-27T15:28:28Z VSAGRBYB5HS5RKZ9T4EA5J8ATR-m1e-e06ab4ba open actor=human:Wido targets=the-ledger-and-the-push-keep-their-own-refs
- 2026-10-27T15:29:19Z 77TBXPFR74H2NYNS3SNGDCFDCG-m1e-e06ab4ba approve actor=human:Wido targets=the-ledger-and-the-push-keep-their-own-refs
- 2026-10-27T15:29:51Z A9VDW2P446TYHY55CGX3B193BB-m1e-e06ab4ba approve actor=human:Wido targets=the-ledger-and-the-push-keep-their-own-refs
- 2026-10-27T15:30:04Z 00TFMXZM60003CZZJZANHM26BD-m1e-e06ab4ba set-pin actor=human:Wido targets=the-ledger-and-the-push-keep-their-own-refs
- 2026-10-27T15:30:21Z WVXKJE2HFZXRZYXX8476FXA0GP-m1e-e06ab4ba set-priority actor=human:Wido targets=the-ledger-and-the-push-keep-their-own-refs reason=priority-order subject=the-ledger-and-the-push-keep-their-own-refs from=unranked to=1:84 requested-sequence=append
Integrity: sha256=f3822c9dd06249946afcf291ed21f080d700ff5f0dcc4188624ed4e487539b58
