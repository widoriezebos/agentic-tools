# channel-code-verified-at-poll-time

- State: queued
- Tier: 1
- Intent: A human answer on the fleet channel is rejected as 'bad code' when the steward's poll comes late: internal/channel/poll.go verifies the six-digit code against the poll time (c.Now) with one 30-second step of slack either way, while the steward polls once per tick (about two minutes), so any reply sent more than about 45 seconds before the next poll fails even with a correct code. Wido's first answer on 2026-09-04 12:28 was rejected; the second passed only because a manual poll ran seconds later. Wido: 'we probably have an issue with the polling and the expiry of the token; this seems to be very tight on reading in time before expiry'. DONE means the code is verified against the message's own send time (the Telegram update carries date; the inbound message type carries it to the poll) with the same one-step slack, the replay guard still keys on the code's step, a reply older than the poll interval plus one step is still refused with a reason that names the age, and the channel test pins a reply sent 100 seconds before the poll being accepted.
- Origin: main
- Next step: TIER 1 per R-54-m1 (one field carried through the adapter, one time argument, a test): build, go test ./internal/channel/..., land through a chain; box 1h/3/60m/1. Waits for human approval for execution; Wido raised it on 2026-09-04.
- OpenedAt: 2026-09-04T10:31:42Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0

History:
- 2026-09-04T10:31:42Z 90DC681DSG1N93T2VV23S41VMV-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=channel-code-verified-at-poll-time
Integrity: sha256=cf240544d89f764bf104795103e97ea0bdcdc80d0c307a1a67dc298d4ee59e7f
