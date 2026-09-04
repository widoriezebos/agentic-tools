# channel-totp-verified-at-poll-time

- State: parked
- Tier: 1
- Intent: internal/channel/poll.go verifies a reply's TOTP code against the poll's clock (VerifyTOTP(secret, code, c.Now), window one step either side, 30 s) and ignores the message's own send time, which the inbound carries as Inbound.At. Any reply older than about a minute when the poll runs is rejected 'bad code' and the rejection is posted back to the thread: on m3 2026-09-04 13:08 Wido's approval on question SQPBWDE2HK0Q6WHV0SK6D94ERM (thread 24) was rejected that way after sitting three minutes. DONE means the code is verified against the message's send time (the step the human saw), the replay register still refuses a reused step, a message whose send time is older than a bounded age (ten minutes) is rejected 'stale' with that word, and a test pins a reply sent two minutes before the poll being accepted.
- Origin: main
- Next step: DUPLICATE of channel-code-verified-at-poll-time, which m2 landed as 4b919708 (the code is checked against the moment the reply was sent; internal/channel/poll.go now reads Inbound.SentAt). Nothing left to build here; to be closed by whoever holds the closing word.
- OpenedAt: 2026-09-04T11:09:19Z
- Revision: 3
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Parked: by=m3+mac-m3 at=2026-09-04T15:35:15Z because=duplicate of channel-code-verified-at-poll-time, landed 4b919708

History:
- 2026-09-04T11:09:19Z WJ0TDS0TEEGNCZEBB54G2H79ZP-m3-a5da21ff open actor=m3+mac-m3 targets=channel-totp-verified-at-poll-time
- 2026-09-04T15:35:06Z NTQXE8BVJEH2D7CHZ605SHFKRZ-m3-a5da21ff edit actor=m3+mac-m3 targets=channel-totp-verified-at-poll-time
- 2026-09-04T15:35:15Z 66EFJCXTY8K1K3WR0D9NMV6DC4-m3-a5da21ff park actor=m3+mac-m3 targets=channel-totp-verified-at-poll-time reason=duplicate of channel-code-verified-at-poll-time, landed 4b919708
Integrity: sha256=7e297b807f64317278fadfdb70b7dcdf41b49f07574961a4d5419cf065c9a2b2
