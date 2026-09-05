# channel-local-timestamps

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a misread timestamp costs the human a moment of arithmetic, nothing is authorized or lost by it; novelty 1: rendering an existing time value in a different location, with the only real question being which times are display and which are record; exposure 2: every message the fleet posts to the channel, read by one human; accumulation 1: first report of it"
- Tier: 2
- Intent: Telegram messages carry UTC timestamps, so Wido reads every fleet message in a timezone he is not in. internal/channel/report.go:84 renders the status headline as '<machine> status 2006-01-02 15:04Z' from c.Now.UTC(), and report.go:34 and question.go:158 set Now to time.Now().UTC(). DONE means the timestamps a human reads in the channel are actual local time for the machine's own timezone, the offset is unambiguous to the reader, and the records the inbox keeps stay whatever the ledger needs so nothing that is compared or sorted changes meaning
- Origin: main
- Next step: Split display from record before changing anything: internal/channel/report.go:84 and the question rendering are what a human reads and should be local with a visible offset; internal/channel/inbox.go:58-60 SentAt and ReceivedAt are record fields that other machines compare, and report.go:159 feeds a git --since, so those stay UTC. Then decide where 'local' comes from - the machine's own zone is the honest answer for a per-machine status line. Wido asked for this on 2026-09-05; it waits behind m2's supervisor repo-identity landing like everything else here
- OpenedAt: 2026-09-05T10:10:41Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-05T10:10:41Z GB3XPE0X5VV0WRKZGGBT8XTEKQ-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=channel-local-timestamps
Integrity: sha256=5de86036ec36c18fcd99744fdb1a8df3164213b6293cc953b1b233fd1aff58fd
