# channel-poll-not-automatic

- State: done
- Tier: 1
- Intent: Nothing on a seat's machine polls the Telegram channel on its own: 'channel wait' only watches the question record on disk and 'channel poll' is the sole receiver of replies, run by hand or by fixtures. A question asked over the channel is answered only when someone happens to poll, and with the poll-time TOTP check (goal channel-totp-verified-at-poll-time) the answer must be polled within a minute of being sent. On m3 2026-09-04 Wido asked 'did you get the answer from telegram? This feels like it is not automatically being picked up!' after his approval sat unread. DONE means the steward tick runs channel poll for the repository whenever a question is open, and 'channel wait' polls at its own interval instead of only reading the record; a fixture proves a reply is dispositioned without a manual poll.
- Origin: main
- Next step: NEXT PICK for m3 (seat's choice under R-76-m3, 'a properly working channel (telegram)'): the smallest channel fix, the steward tick polls whenever a question is open and channel wait polls at its own interval. TIER 1 per R-54-m1 (one call in the steward tick and one in channel wait, with their tests): build, go test ./internal/steward/ ./internal/channel/ ./cmd/..., channel-fixtures.sh, land as a declared direct fix; box 1h/3/60m/1. Waits for Wido's execution approval: the status post asks for it with the token 'start channel-poll-not-automatic'. Coordinates with fleet-channel-gateway (the resident listener replaces the tick poll later; the one-shot tick keeps a short poll under the gateway design).; ASKED X7V708S00S0ECRTSS59CYB3F9E (reserved-decision): Your priority R-76: a properly working channel. This is the smallest channel fix: the steward tick polls Telegram whenever a question is open, and channel wait polls at its own interval, so a reply is picked up without anyone running a poll by hand (your words 2026-09-04: 'This feels like it is not automatically being picked up!').
- Concluded: Landed 1f8c6221 as a tier-1 direct fix: channel wait polls the channel every --poll-seconds (default 30, first poll at once) through the steward's channel duty while the record has no answer; the steward tick already ran that duty every tick; the channel fixture's Telegram leg proves a verified reply is dispositioned by the wait alone, with no manual poll.
- OpenedAt: 2026-09-04T11:09:25Z
- Revision: 7
- Labels: next, robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-04T17:13:38Z revision=4 opid=A4FQ8SN3MPZNVHZSAP339FBV7C-m3-587cb0f1 authority=relayed digest=ebef9603afd916fda9486a2b71b6d68de3eab9fc2aae218107a95961eec24be8 reviewBy=2026-09-06
- Sliced: machine=m3 lineage=mac-m3 revision=5 at=2026-09-05T03:10:21Z

History:
- 2026-09-04T11:09:25Z A3Q5J6APA2MKDG1G7ATTMMS0DD-m3-a5da21ff open actor=m3+mac-m3 targets=channel-poll-not-automatic
- 2026-09-04T15:56:03Z 1Z30E4WFSR3780XENRSFEQDEVJ-m3-a5da21ff edit actor=m3+mac-m3 targets=channel-poll-not-automatic
- 2026-09-04T15:56:25Z J2N3ZA93W207CXBKJMYCBADFCJ-m3-587cb0f1 ask actor=m3+main-1788172645-85501-aa86ee targets=channel-poll-not-automatic
- 2026-09-04T17:13:38Z A4FQ8SN3MPZNVHZSAP339FBV7C-m3-587cb0f1 approve actor=human:Wido targets=channel-poll-not-automatic authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="All approved, also said this on Telegram"
- 2026-09-05T03:10:06Z 1QWA2B7DA65J28CNFZ17WRBSB5-m3-a5da21ff claim actor=m3+mac-m3 targets=channel-poll-not-automatic
- 2026-09-05T03:10:21Z FEB1YK579WZF56N6JEBAAEQZM3-m3-a5da21ff slice-start actor=m3+mac-m3 targets=channel-poll-not-automatic
- 2026-09-05T03:29:27Z J53D54A4FABS83AGQJBD60GWQM-m3-a5da21ff done actor=m3+mac-m3 targets=channel-poll-not-automatic
Integrity: sha256=4d297ab42b3dec06a3993068035d6b5c8532f2096f5a7ef857f12f2d655647c6
