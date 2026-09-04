# channel-poll-not-automatic

- State: queued
- Tier: 1
- Intent: Nothing on a seat's machine polls the Telegram channel on its own: 'channel wait' only watches the question record on disk and 'channel poll' is the sole receiver of replies, run by hand or by fixtures. A question asked over the channel is answered only when someone happens to poll, and with the poll-time TOTP check (goal channel-totp-verified-at-poll-time) the answer must be polled within a minute of being sent. On m3 2026-09-04 Wido asked 'did you get the answer from telegram? This feels like it is not automatically being picked up!' after his approval sat unread. DONE means the steward tick runs channel poll for the repository whenever a question is open, and 'channel wait' polls at its own interval instead of only reading the record; a fixture proves a reply is dispositioned without a manual poll.
- Origin: main
- Next step: TIER 1 per R-54-m1 (one call in the steward tick and one in channel wait, with their tests): build, go test ./internal/steward/ ./internal/channel/ ./cmd/..., channel-fixtures.sh, land as a declared direct fix; box 1h/3/60m/1. Waits for human approval for execution.
- OpenedAt: 2026-09-04T11:09:25Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0

History:
- 2026-09-04T11:09:25Z A3Q5J6APA2MKDG1G7ATTMMS0DD-m3-a5da21ff open actor=m3+mac-m3 targets=channel-poll-not-automatic
Integrity: sha256=feab1c47363f210389e5194de6af5000309c4ee7ddbbd97732d7fc72074f6edb
