# channel-tells-me-when-something-lands

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: nothing is authorized or lost wrongly, but the human's only view of fleet delivery is silent for up to four hours, which is what makes him ask whether the machinery is working at all; novelty 2: the trigger point is new - a landing completes in scripts/agents/land.sh while the poster lives on the steward tick - and where it lives, how it stays exactly-once across machines, and what happens when the post fails are real choices; exposure 3: every landing on every machine, into the one channel Wido reads; accumulation 2: the third channel defect he has reported today after the wall of text and the UTC stamps, all three sharing one cause - the channel was designed as a periodic digest and is being asked to be a notifier"
- Tier: 3
- Intent: A landing does not tell Wido anything. The channel was built to digest, not to notify: internal/channel/phase/phase.go composes a status on the steward tick and posts only when ShouldPost agrees, and internal/channel/report.go:259 requires BOTH that channel.status.interval-minutes (default 240) has elapsed AND that the content changed. So two landings on 2026-09-05 sat unsent for over an hour behind a four-hour floor, and Wido had to ask why he heard nothing. DONE means a landing produces a message to the channel promptly and exactly once, naming what landed and the goal it belongs to; a post that fails never blocks or slows the landing itself and is retried rather than lost; and the four-hourly status stays what it is, so the fleet does not gain a second wall of text
- Origin: main
- Next step: Wido 2026-09-05: 'the moment something lands, I want a message of that'. Decide where the trigger lives before building: land.sh posting directly after a successful push is immediate but couples landing to the channel, while a steward-tick sweep over commits since a cursor is decoupled but arrives a tick late; the likely answer is both, the landing posting best-effort and the cursor sweep catching whatever it missed, which is also what makes it exactly-once and what makes a failed post harmless. The message must respect the 1600-rune bound landed today in b52711d3a. m1's status.json shows the mechanism clearly: lastPost 09:44:22Z, two landings at 12:43 and 12:5x local, nothing sent until the verb was run by hand at 11:05Z
- OpenedAt: 2026-09-05T11:06:28Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-05T11:06:28Z 4KW9GG9Y073BKWR0WJZ5BGQTRS-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=channel-tells-me-when-something-lands
Integrity: sha256=a4869d4b81813532b6b09582d94ffe3fe315de989fe9640d277bd02644362c98
