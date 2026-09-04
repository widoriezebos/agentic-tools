# channel-telegram-shared-update-pointer

- State: queued
- Tier: 2
- Intent: DUPLICATE of fleet-channel-gateway; withdrawn on Wido's correction 2026-09-04: 'ONE bot for all machines. I do not want to have a huge admin for a cluster. We agreed (already on backlog I hope) that we have all read, first come first served BUT then store centrally in the git repo where others READ my answer'. The agreed design is fleet-channel-gateway (Wido 2026-09-03): every machine polls without an offset, commits the reply to the shared git inbox, first commit wins, the offset is confirmed only after the commit is durable. The defect observed today (a machine's cursor sent as offset confirms the reply for the whole bot) is what that goal fixes; nothing new to design. This item is not to be scheduled.
- Origin: human
- Next step: None: discharged by fleet-channel-gateway. Prune when the store allows dropping a queued duplicate.
- OpenedAt: 2026-09-04T11:13:58Z
- Revision: 2
- Labels: duplicate, robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2

History:
- 2026-09-04T11:13:58Z 7V405CVRXK2B161DGJ7RVC3T5E-m3-a5da21ff open actor=human:Wido targets=channel-telegram-shared-update-pointer
- 2026-09-04T11:25:51Z VCE7XSE474R9CC1CT8M2J97JF3-m3-a5da21ff edit actor=human:Wido targets=channel-telegram-shared-update-pointer
Integrity: sha256=88ec496765185c1e6ff5d1d3ec5950974bc46df6e3f262713b0ba8d73d5ca208
