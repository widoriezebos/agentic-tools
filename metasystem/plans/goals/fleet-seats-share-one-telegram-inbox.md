# fleet-seats-share-one-telegram-inbox

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: a human's answer or approval is silently lost and the asking seat waits or times out, nothing granted or destroyed; novelty 2: reply routing across checkouts is new, the poll and question records exist; exposure 3: every question any of the four seats asks from now on; accumulation 2: every lost reply is another timeout and another re-ask until someone notices"
- Tier: 3
- Intent: Since 2026-09-10 08:35Z the four seats on the m1 Mac (m1, m1b, m1c, m1d) all carry the same Telegram bot token and chat in metasystem.conf.local, so each steward tick polls getUpdates with its own per-checkout cursor (internal/channel/poll.go). Telegram confirms every update below the highest offset any poller sends, so whichever seat polls first consumes a human reply; a reply meant for another seat's open question is filed in the first seat's unmatched.jsonl and the asking seat never sees it, while concurrent pollers get 409 terminated by other getUpdates request (mapped to ErrBusy in internal/channel/telegram). Posting is unaffected. Done means: a human reply reaches the seat that asked, with four seats polling one bot, either through one bot token per seat (a config-level answer, documented) or one shared inbox under the fleet destination that routes each reply to the seat whose question thread it answers, with a test that plays two seats and one reply.
- Origin: main
- Next step: Wido decides between one bot per seat (four tokens, a documented config step, no code) and a routed shared inbox (code in internal/channel: the poller that consumes an update forwards replies whose thread belongs to another checkout's question). Until then, treat any question a seat asks over Telegram as possibly unanswered and re-ask from the seat that needs it.
- OpenedAt: 2026-09-10T08:43:42Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T08:43:42Z 1GTABT8HW8TMPVXVX2P1BWWMG1-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=fleet-seats-share-one-telegram-inbox
Integrity: sha256=282e2a513afee360c5ed428fbf304060151f0c1260c6f5ee1ff9bb8831a05a58
