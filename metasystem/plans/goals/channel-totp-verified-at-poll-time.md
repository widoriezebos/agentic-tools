# channel-totp-verified-at-poll-time

- State: queued
- Tier: 1
- Intent: internal/channel/poll.go verifies a reply's TOTP code against the poll's clock (VerifyTOTP(secret, code, c.Now), window one step either side, 30 s) and ignores the message's own send time, which the inbound carries as Inbound.At. Any reply older than about a minute when the poll runs is rejected 'bad code' and the rejection is posted back to the thread: on m3 2026-09-04 13:08 Wido's approval on question SQPBWDE2HK0Q6WHV0SK6D94ERM (thread 24) was rejected that way after sitting three minutes. DONE means the code is verified against the message's send time (the step the human saw), the replay register still refuses a reused step, a message whose send time is older than a bounded age (ten minutes) is rejected 'stale' with that word, and a test pins a reply sent two minutes before the poll being accepted.
- Origin: main
- Next step: TIER 1 per R-54-m1 (one call site and its test in internal/channel): build, go test ./internal/channel/, scripts/agents/channel-fixtures.sh, land as a declared direct fix; box 1h/3/60m/1. Waits for human approval for execution.
- OpenedAt: 2026-09-04T11:09:19Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0

History:
- 2026-09-04T11:09:19Z WJ0TDS0TEEGNCZEBB54G2H79ZP-m3-a5da21ff open actor=m3+mac-m3 targets=channel-totp-verified-at-poll-time
Integrity: sha256=57d70d400779b77636294f84180beca012df2d277b3b6c46979ff98a59942323
