# channel-telegram-shared-update-pointer

- State: done
- Tier: 2
- Intent: DUPLICATE of fleet-channel-gateway; withdrawn on Wido's correction 2026-09-04: 'ONE bot for all machines. I do not want to have a huge admin for a cluster. We agreed (already on backlog I hope) that we have all read, first come first served BUT then store centrally in the git repo where others READ my answer'. The agreed design is fleet-channel-gateway (Wido 2026-09-03): every machine polls without an offset, commits the reply to the shared git inbox, first commit wins, the offset is confirmed only after the commit is durable. The defect observed today (a machine's cursor sent as offset confirms the reply for the whole bot) is what that goal fixes; nothing new to design. This item is not to be scheduled.
- Origin: human
- Next step: None: discharged by fleet-channel-gateway. Prune when the store allows dropping a queued duplicate.
- Concluded: Withdrawn duplicate, reconciled 2026-09-07 on Wido's request. The record already says not to schedule it and names fleet-channel-gateway as its owner. Removing this approved duplicate from the live queue corrects scheduling state; it makes no claim that the shared channel is finished. The original defect, user correction and first-commit-wins receive/commit/confirm requirement remain preserved here and with fleet-channel-gateway.
- OpenedAt: 2026-09-04T11:13:58Z
- Revision: 4
- Labels: duplicate, robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=3 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=231951a2e5cb8c42c74e0442304883a554e6f64d2c363af86827947d80a8e0bf

History:
- 2026-09-04T11:13:58Z 7V405CVRXK2B161DGJ7RVC3T5E-m3-a5da21ff open actor=human:Wido targets=channel-telegram-shared-update-pointer
- 2026-09-04T11:25:51Z VCE7XSE474R9CC1CT8M2J97JF3-m3-a5da21ff edit actor=human:Wido targets=channel-telegram-shared-update-pointer
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=channel-telegram-shared-update-pointer reason=sweep
- 2026-09-07T21:12:01Z RPKM7WBBCDYH5MC47RN3ZCNKX0-m1-76f67331 done actor=human:Wido targets=channel-telegram-shared-update-pointer
Integrity: sha256=eb95bb65d3367ae15b9ca7568e71494eb63b799d44e562c0d8b78915252a9428
