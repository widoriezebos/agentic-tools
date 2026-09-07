# fleet-pull

- State: queued
- Intent: An idle machine picks up claimable shared-backlog work by itself: fleet liveness is the steward's duty, never a human's memory
- Origin: main
- Next step: RESIDUAL RESTORED by Wido 2026-09-07 during headless-fleet backlog reconciliation. The earlier merge into ledger-attention did not finish this intent: that delivered owner notices ledger changes and explicitly does not grant claims; idle-with-backlog-alarm adds only a repeated-Stop-triggered claim/continuation path. INTENT: a running headless node repeatedly selects eligible approved work, atomically claims it, starts a mission under that goal approval, observes its durable completion or lawful park, and selects the next eligible item without a supervising chat or a hook-triggered rescue. CONSTRAINTS: one repository and its shared ledger; reuse goal claim/release, missionrunner and steward ownership rather than add another scheduler or leader. Unapproved, blocked and foreign-pinned goals never execute; an empty queue is a durable wait that notices later approvals; stop and capacity limits remain authoritative; retries cannot duplicate a live claim or completed effect. one-approval-gate owns the goal-to-contract authority conversion; backlog-ordered-by-priority owns optional future ranking, not this loop. FREEDOMS: the smallest connection among existing owners and notification/wait mechanisms. Before implementation, settle transition ownership and crash/retry behavior against the original FP-R3-01..07 findings and current code; if the approved box cannot contain the result, split before claiming more scope. The independent consecutive-feature acceptance is headless-continuous-delivery-proof. No budget or execution approval is supplied by restoring this queued item.
- OpenedAt: 2026-08-23T08:06:12Z
- Revision: 16
- BlockedBy: one-approval-gate
- Labels: headless-execution, headless-fleet
- BudgetExceptions: 0

History:
- 2026-08-23T08:06:12Z B9R7HVR9Z1H2XQ1C9GGHX63018-widos-m5-pro-bf243850 open actor=widos-m5-pro+coordinator targets=fleet-pull
- 2026-08-23T08:06:31Z KDK3WC7GZ75KDA6DPFYDN1WYQ9-widos-m5-pro-bf243850 claim actor=widos-m5-pro+coordinator targets=fleet-pull
- 2026-08-23T08:13:18Z CS6TRWQKADPT3S22HKSKS5TH11-widos-m5-pro-bf243850 release actor=widos-m5-pro+coordinator targets=fleet-pull
- 2026-08-23T08:16:21Z 97YYDE2BR2DX4NNP111C3NQ5NZ-widos-m5-pro-bf243850 claim actor=widos-m5-pro+coordinator targets=fleet-pull
- 2026-08-23T08:17:58Z QM2KW7V24PHYDJJBGXAD22SBXD-widos-m5-pro-bf243850 release actor=widos-m5-pro+coordinator targets=fleet-pull
- 2026-08-23T10:31:21Z 81MNRPXTSD15RD4CXHW7943YEJ-m1-bf243850 claim actor=m1+coordinator targets=fleet-pull
- 2026-08-23T11:26:43Z QWVMBGWJQMXAFWKV6FAREKW90Y-m1-bf243850 edit actor=m1+coordinator targets=fleet-pull
- 2026-08-23T11:26:47Z A3CM12JAXQPFTMHCQW1QNG2C8E-m1-bf243850 release actor=m1+coordinator targets=fleet-pull
- 2026-08-23T13:34:08Z AX2T6QBNVSYBXMA2X7TGBT44EJ-m1-bf243850 edit actor=m1+coordinator targets=fleet-pull
- 2026-08-23T19:38:37Z NVH1BHW0GG9CK6JHH1PA3PVBFQ-m1-bf243850 edit actor=human:wido targets=fleet-pull
- 2026-08-23T19:38:59Z CHYYSAFJQWJM4FW507826PHZDM-m1-bf243850 edit actor=human:wido targets=fleet-pull
- 2026-08-23T19:43:33Z SP0DCX5170EA643KVZ73C0XEXQ-m1-bf243850 edit actor=human:wido targets=fleet-pull
- 2026-08-24T13:14:35Z QGWDYRDQ7KZ3FR3K41BBH4N9PV-m1-bf243850 edit actor=m1+coordinator targets=fleet-pull
- 2026-08-31T19:09:47Z JG0KTREHQ6WY5ZGGM39M9GFSSQ-m0-c5dbf036 park actor=m0+main-1788178136-1684505-4ffe42 targets=fleet-pull reason=R-33 triage (Wido 2026-08-31, replayed at reconciliation): merged into ledger-attention - one steward-tick mechanism notices ledger changes AND picks claimable work
- 2026-09-07T21:12:01Z RPKM7WBBCDYH5MC47RN3ZCNKX0-m1-76f67331 unpark actor=human:Wido targets=fleet-pull
- 2026-09-07T21:12:01Z RPKM7WBBCDYH5MC47RN3ZCNKX0-m1-76f67331 edit actor=human:Wido targets=fleet-pull
Integrity: sha256=38f3076e261f4b2240864f78aceac8d2376796cefa389f5dec2b76ac603b7b62
