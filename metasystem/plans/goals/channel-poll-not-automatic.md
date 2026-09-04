# channel-poll-not-automatic

- State: queued
- Tier: 1
- Intent: Nothing on a seat's machine polls the Telegram channel on its own: 'channel wait' only watches the question record on disk and 'channel poll' is the sole receiver of replies, run by hand or by fixtures. A question asked over the channel is answered only when someone happens to poll, and with the poll-time TOTP check (goal channel-totp-verified-at-poll-time) the answer must be polled within a minute of being sent. On m3 2026-09-04 Wido asked 'did you get the answer from telegram? This feels like it is not automatically being picked up!' after his approval sat unread. DONE means the steward tick runs channel poll for the repository whenever a question is open, and 'channel wait' polls at its own interval instead of only reading the record; a fixture proves a reply is dispositioned without a manual poll.
- Origin: main
- Next step: NEXT PICK for m3 (seat's choice under R-76-m3, 'a properly working channel (telegram)'): the smallest channel fix, the steward tick polls whenever a question is open and channel wait polls at its own interval. TIER 1 per R-54-m1 (one call in the steward tick and one in channel wait, with their tests): build, go test ./internal/steward/ ./internal/channel/ ./cmd/..., channel-fixtures.sh, land as a declared direct fix; box 1h/3/60m/1. Waits for Wido's execution approval: the status post asks for it with the token 'start channel-poll-not-automatic'. Coordinates with fleet-channel-gateway (the resident listener replaces the tick poll later; the one-shot tick keeps a short poll under the gateway design).; ASKED X7V708S00S0ECRTSS59CYB3F9E (reserved-decision): Your priority R-76: a properly working channel. This is the smallest channel fix: the steward tick polls Telegram whenever a question is open, and channel wait polls at its own interval, so a reply is picked up without anyone running a poll by hand (your words 2026-09-04: 'This feels like it is not automatically being picked up!').
- OpenedAt: 2026-09-04T11:09:25Z
- Revision: 3
- Labels: next, robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-04T11:09:25Z A3Q5J6APA2MKDG1G7ATTMMS0DD-m3-a5da21ff open actor=m3+mac-m3 targets=channel-poll-not-automatic
- 2026-09-04T15:56:03Z 1Z30E4WFSR3780XENRSFEQDEVJ-m3-a5da21ff edit actor=m3+mac-m3 targets=channel-poll-not-automatic
- 2026-09-04T15:56:25Z J2N3ZA93W207CXBKJMYCBADFCJ-m3-587cb0f1 ask actor=m3+main-1788172645-85501-aa86ee targets=channel-poll-not-automatic
Integrity: sha256=07fde88957bca62b87b140cf3f01ba2b025fc3322223fc0687d6759d5115da2c
