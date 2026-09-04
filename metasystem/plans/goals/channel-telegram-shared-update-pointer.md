# channel-telegram-shared-update-pointer

- State: queued
- Tier: 2
- Intent: Telegram keeps one getUpdates confirmation pointer per bot, and every machine polls the same bot: a getUpdates call with offset N confirms every update below N for the whole bot, so the first machine to poll past a human reply takes it away from every other machine (the reply is dropped server-side, never seen by the seat that asked). Each machine keeps its own cursor (internal/channel/poll.go writes it per repository) as if the pointer were private; it is not. Wido on m3 2026-09-04: 'there is clearly a polling problem with telegram. I need to tell you to look. And when I did it late the token already expired and I had to do it again. So something is off in the background poller. I know there is an issue with many machines polling because the channel has only 1 message pointer for the bot identifier; that could also play a role. In any case: we are not yet in a stable state with the telegram functionality'. This is the third of three receive defects; the other two are channel-totp-verified-at-poll-time and channel-poll-not-automatic. DONE means a reply reaches the machine that asked regardless of which machine polls first, proven by a fixture with two repositories polling one fake bot; the design chooses between one bot token per machine (offsets become private), one receiving machine relaying replies through the ledger, or offset-free reads deduplicated locally by update_id (bounded by Telegram's 100-per-call window and 24-hour retention).
- Origin: human
- Next step: Design first (tier 2): a short design under plans/ choosing the receive architecture, critiqued once, then one build round with the two-repository fixture; land the three receive goals together as one stable-channel arc. Waits for human approval for execution.
- OpenedAt: 2026-09-04T11:13:58Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2

History:
- 2026-09-04T11:13:58Z 7V405CVRXK2B161DGJ7RVC3T5E-m3-a5da21ff open actor=human:Wido targets=channel-telegram-shared-update-pointer
Integrity: sha256=fbe7c2c38ee04c44c4d825435087e02e1982c05d1cffd430ff23f824e61ee702
