# the-ledger-and-the-push-keep-their-own-refs

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Git ref namespaces with a one-release migration: readers accept both prefixes, writers write the new one, the push never lists the ledger's prefix; a mistake is a refused publish with the ref named, never a lost commit"
- Tier: 2
- Intent: The goal ledger's transaction refs and the branch push's transaction refs live under different prefixes, the push's sweep touches only its own blobs for one release of migration, and a ledger transaction whose owner died before pushing is terminalized and its refs cleaned by the dead-owner path (finding 432; design plans/designs/the-stewards-act-on-what-they-see.md (accepted, revision 4) D8, G5)
- Origin: main
- Next step: On m1e's build lane after a-review-keeps-every-read: opus-designer cuts the brief from the page (BRIEF-CHECKLIST, Read each round: yes, one Astra round, the reverse-dependent packages run whole); one unit, about 35 production lines: goal/branch/push.go, push_repository.go, goal/txn.go, goal/recover.go
- OpenedAt: 2026-10-27T15:28:28Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-27T15:28:28Z VSAGRBYB5HS5RKZ9T4EA5J8ATR-m1e-e06ab4ba open actor=human:Wido targets=the-ledger-and-the-push-keep-their-own-refs
Integrity: sha256=159771badd5355572ba9ab32b3a4b189e34c79ef8986ba071e99db413e9559ff
