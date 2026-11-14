# a-slow-ledger-read-is-a-wait-2

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="Split from a-slow-ledger-read-is-a-wait so its test fix (U2) and its truthful refusal (U1) could land on 2026-11-14 while three long proofs had failed on the 4 s ledger read."
- Tier: 2
- Intent: The rest of a-slow-ledger-read-is-a-wait, whose first two units landed as its whole on 2026-11-14: unlocked command reads of the goal ledger opt into a 35 s outer and 26 s transport bound while reads inside the commit's lease lock, the tick, the channel duty and the Stop hook stay short (U3), and a timed-out read at the Stop hook is a counted wait, never an idle refusal, telling the person at three without an idle incident (U4).
- Origin: main
- Next step: Build U3 (bound 60) from /Users/wido/agentic-tools-evidence/lane-test-20261009/briefs/a-slow-ledger-read-is-a-wait-brief.md section U3; then U4 (--last) from a brief of its own that a1 writes when U3 is published (persisted count, a notification path that is not the idle incident, a whitelisted verdict class). Fold the two low notes of U1's read into U3: a resumed commit's recheck timeout must not say 'nothing was committed', and the message names the bound that actually expired.
- OpenedAt: 2026-11-14T08:08:17Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-14T08:08:17Z 0KECAYCPJJQ4ZWK0FZZEQXFFJ4-ui-2e4e153f open actor=human:Wido targets=a-slow-ledger-read-is-a-wait-2
Integrity: sha256=74ff254b3c60405393539efa3b3d0b9b54d223755e3dacb23724106c0f73bd89
