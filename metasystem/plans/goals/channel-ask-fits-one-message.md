# channel-ask-fits-one-message

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: nothing is authorized wrongly, but a token pushed into a later chunk is a decision the human cannot act on without scrolling a wall, and the channel is the only path for the human's word when no terminal is at hand; novelty 1: the bound and the trimming primitive both already exist in report.go and are being applied to a second renderer; exposure 2: every ask the fleet posts, read by one human; accumulation 1: first report, though it shares a cause with the four channel defects of 2026-09-04 recorded on fleet-channel-gateway"
- Tier: 2
- Intent: An ask posts a wall of text in Telegram. internal/channel/question.go:231 renderQuestion prints every fact, every option's full consequence, and the recommendation verbatim, and this repo's facts are goal-record prose that runs to thousands of characters; internal/channel/telegram/telegram.go:117,152 then splits the result at a 4000-rune chunkLimit, so one ask becomes several giant messages. The status report already bounds itself to twelve lines at report.go:80-96 and trims with oneSentence at report.go:144; the ask has no such bound. DONE means an ask a human reads is one bounded message with its reply token intact, long material trimmed with the trim visible rather than silently dropped, and the token instruction never separated from the token by a chunk boundary
- Origin: main
- Next step: Bound renderQuestion at internal/channel/question.go:231: cap the number of facts and the length of each, trim option consequences the way report.go:144 oneSentence already does, and keep the reply instruction and the verbatim token whole and last so a chunk boundary can never separate them. Wido asked for this on 2026-09-05 alongside channel-local-timestamps; both are internal/channel only and touch neither repo identity nor arming, so neither waits on m2's supervisor landing
- OpenedAt: 2026-09-05T10:12:52Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-05T10:12:52Z EQKY33K9R2T59R93YK4D2WQS62-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=channel-ask-fits-one-message
Integrity: sha256=2e8aedb4d6d21bb51807c36c793178d7a5b4131deef91ddcc75f049e905f7759
