# channel-code-verified-at-poll-time

- State: approved
- Tier: 1
- Intent: A human answer on the fleet channel is rejected as 'bad code' when the steward's poll comes late: internal/channel/poll.go verifies the six-digit code against the poll time (c.Now) with one 30-second step of slack either way, while the steward polls once per tick (about two minutes), so any reply sent more than about 45 seconds before the next poll fails even with a correct code. Wido's first answer on 2026-09-04 12:28 was rejected; the second passed only because a manual poll ran seconds later. Wido: 'we probably have an issue with the polling and the expiry of the token; this seems to be very tight on reading in time before expiry'. DONE means the code is verified against the message's own send time (the Telegram update carries date; the inbound message type carries it to the poll) with the same one-step slack, the replay guard still keys on the code's step, a reply older than the poll interval plus one step is still refused with a reason that names the age, and the channel test pins a reply sent 100 seconds before the poll being accepted.
- Origin: main
- Next step: TIER 1 per R-54-m1 (one field carried through the adapter, one time argument, a test): build, go test ./internal/channel/..., land through a chain; box 1h/3/60m/1. Waits for human approval for execution; Wido raised it on 2026-09-04.
- OpenedAt: 2026-09-04T10:31:42Z
- Revision: 2
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- Approved: by=human:Wido at=2026-09-04T13:18:20Z revision=2 opid=WXFQZK97VRZZR5MS7KD9FRTKB6-m2-5fcf08ab authority=relayed digest=dddd1986c09a1d1c846783cb678d87559580e51355be5a6717c1a70ec785a5da reviewBy=2026-09-06

History:
- 2026-09-04T10:31:42Z 90DC681DSG1N93T2VV23S41VMV-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=channel-code-verified-at-poll-time
- 2026-09-04T13:18:20Z WXFQZK97VRZZR5MS7KD9FRTKB6-m2-5fcf08ab approve actor=human:Wido targets=channel-code-verified-at-poll-time authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="When all that is done picking the next thing off fof the backlog is approved too"
Integrity: sha256=c92307026d91693a3a4405dc13608addf7b204cfd0373c3f80144b12b03d154e
