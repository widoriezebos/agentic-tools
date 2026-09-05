# channel-local-timestamps

- State: done
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a misread timestamp costs the human a moment of arithmetic, nothing is authorized or lost by it; novelty 1: rendering an existing time value in a different location, with the only real question being which times are display and which are record; exposure 2: every message the fleet posts to the channel, read by one human; accumulation 1: first report of it"
- Tier: 2
- Intent: Telegram messages carry UTC timestamps, so Wido reads every fleet message in a timezone he is not in. internal/channel/report.go:84 renders the status headline as '<machine> status 2006-01-02 15:04Z' from c.Now.UTC(), and report.go:34 and question.go:158 set Now to time.Now().UTC(). DONE means the timestamps a human reads in the channel are actual local time for the machine's own timezone, the offset is unambiguous to the reader, and the records the inbox keeps stay whatever the ledger needs so nothing that is compared or sorted changes meaning
- Origin: main
- Next step: Split display from record before changing anything: internal/channel/report.go:84 and the question rendering are what a human reads and should be local with a visible offset; internal/channel/inbox.go:58-60 SentAt and ReceivedAt are record fields that other machines compare, and report.go:159 feeds a git --since, so those stay UTC. Then decide where 'local' comes from - the machine's own zone is the honest answer for a per-machine status line. Wido asked for this on 2026-09-05; it waits behind m2's supervisor repo-identity landing like everything else here
- Concluded: Landed on origin/main. The status headline is the posting machine's wall-clock time with a numeric offset, taken from ReportConfig.Location so a test can pin it; the inbox's SentAt and ReceivedAt and the git --since window stay UTC because other machines and git parse them. The status digest recogniser accepts both the offset-bearing headline and the legacy Z form, and the review caught that the Z branch was the one left without a test, which the fold added. Chain implementer-d8ff4e56e9453860f3e03154: build, code-critic round 1 zero material, fold, closing review zero material. Location defaults to the posting machine's zone; the fleet's Lima guests run UTC, so they will keep posting +0000 headlines until every machine is set to one IANA location. That choice is Wido's and is not this goal: the key exists and is documented, so it is a setting rather than new machinery.
- OpenedAt: 2026-09-05T10:10:41Z
- Revision: 6
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 1
- Approved: by=human:human:Wido at=2026-09-05T10:56:38Z revision=5 opid=99238QEYJ17V0VK654CF1B7QE0-m1-a4f8999f authority=relayed digest=f38f1e37f444cb84dad54934e767da39455e4ec7e28ff8c9cbdf8858b3e217e8 reviewBy=2026-09-06
- Sliced: machine=m1 lineage=main-1788594343-3833-fb64b9 revision=3 at=2026-09-05T10:44:35Z

History:
- 2026-09-05T10:10:41Z GB3XPE0X5VV0WRKZGGBT8XTEKQ-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=channel-local-timestamps
- 2026-09-05T10:13:10Z FWHRFW8MEKGN6S95PATEVKQQ7G-m1-a4f8999f approve actor=human:human:Wido targets=channel-local-timestamps authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="Instead of waiting, work on the Telegram messages: the timestamp one, and the other low-risk one that prevents a wall of text in Telegram. Maybe do these now."
- 2026-09-05T10:44:30Z QRH9S279VEDR93ZFDCPBTDZQ4Q-m1-a4f8999f claim actor=m1+main-1788594343-3833-fb64b9 targets=channel-local-timestamps
- 2026-09-05T10:44:35Z 96SMRAQS0KFEC4QN6MT23WVBNX-m1-a4f8999f slice-start actor=m1+main-1788594343-3833-fb64b9 targets=channel-local-timestamps
- 2026-09-05T10:56:38Z 99238QEYJ17V0VK654CF1B7QE0-m1-a4f8999f set-budget actor=human:human:Wido targets=channel-local-timestamps authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="So, land this now, make it happen"
- 2026-09-05T11:01:34Z GXRJB9D4HJ9HGZ8HACSBR2EQ4W-m1-a4f8999f done actor=m1+main-1788594343-3833-fb64b9 targets=channel-local-timestamps
Integrity: sha256=adb7b24467cc970e4ff70426bdc4ca8c5111d90dbd294060be66ab919246e430
