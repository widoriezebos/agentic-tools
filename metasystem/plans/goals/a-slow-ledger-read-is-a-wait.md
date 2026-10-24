# a-slow-ledger-read-is-a-wait

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Fable's addendum 2026-10-24 10:50 (row 10): the fresh ledger read is bounded by 4 s of wall clock (internal/goal/project.go:45-46, fresh.go:41-121); at load 16-20 goal next on m1e failed twice (git show-ref context deadline exceeded); a seat's turn verdict counts a failed read as an idle refusal with backlog (turnverdict.go:1252-1275) and alerts at three; 75 of ui's 129 open alerts are this sentinel."
- Tier: 2
- Intent: A timed-out fresh ledger read under load is a wait with a count, never an idle refusal: the next tick retries, and after the count the generic stop-and-tell applies; a person's goal next reads without the 4-second bound. About 40 production lines; brief to cut.
- Origin: human
- Next step: Cut the brief (opus-designer, BRIEF-CHECKLIST, Read each round: yes, --last), one Astra round, then the machine route on ui after the-elapsed-clock-excludes-waits.
- OpenedAt: 2026-10-24T09:02:21Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-24T09:02:21Z 2C6NY68NNCNY9RRDKYH7C8YY9M-m1e-294bbb33 open actor=human:Wido targets=a-slow-ledger-read-is-a-wait
Integrity: sha256=e45ff7b6e324d4cd46e53aa106fcd9a3730c0f52cc645745beb6f6c244a2b23b
