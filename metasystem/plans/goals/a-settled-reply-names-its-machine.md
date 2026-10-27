# a-settled-reply-names-its-machine

- State: approved
- Priority: 1
- Sequence: 88
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Fills two fields from the same resolver the channel duty already uses, and refuses an unusable record; no authority changes; without it no phone answer can act"
- Tier: 2
- Intent: A reply the steward's settlement pass matches is recorded under this machine's name and lineage, so its operation id is valid and the chosen act runs on the next tick; a channel config without a machine refuses to record an answer instead of writing an unusable one (finding 435: Wido's first live tap, 2026-10-27 16:46, stalled at 'an answer needs a valid recorded operation id' because channel/phase/phase.go SettleReplies builds PollConfig with no Machine)
- Origin: main
- Next step: Build first on m1e's build lane (before a-question-reminds U4a and a-launched-job-is-never-a-main): opus-builder fixes SettleReplies to resolve the machine (goal.ResolveMachine) and lineage as run() does (phase.go:187 against :205-214), adds the refusal for an empty machine at the answer record (inbox.go:693-708), and tests the steward's real order (settlement pass before the channel poll, runner.go:371 then :455) for a tap and a typed reply on a contract question through to the act; about 10 production lines; Read each round: yes; the steward submits with work review --changes --last
- OpenedAt: 2026-10-27T16:00:22Z
- Revision: 5
- Pinned: m1e
- Budget: elapsedLimit=2d attemptLimit=8 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=A9WRVGZKQVRW2E4H1FWAKQTHZ6-m1e-55cdfc19 minutes=1200 reviewRounds=20 goalRevision=2
- Approved: by=human:Wido at=2026-10-27T16:00:42Z revision=3 opid=A9WRVGZKQVRW2E4H1FWAKQTHZ6-m1e-55cdfc19 authority=proven digest=d5750a2e36b819a1e6735b505803a67389413dd1657e6f985c732ffb86f60c62 episode=3

History:
- 2026-10-27T16:00:22Z 11AKW3J08BWMFGEHHAA8CMW1RG-m1e-55cdfc19 open actor=human:Wido targets=a-settled-reply-names-its-machine
- 2026-10-27T16:00:32Z KM4V1ZQS6QKK04B8CCA7ZS1E2P-m1e-55cdfc19 approve actor=human:Wido targets=a-settled-reply-names-its-machine
- 2026-10-27T16:00:42Z A9WRVGZKQVRW2E4H1FWAKQTHZ6-m1e-55cdfc19 approve actor=human:Wido targets=a-settled-reply-names-its-machine
- 2026-10-27T16:00:51Z KKWRTB7FMC8BM3F1E2SSQPKMDQ-m1e-55cdfc19 set-pin actor=human:Wido targets=a-settled-reply-names-its-machine
- 2026-10-27T16:01:02Z 3BSQ40ZH3Z4XDJRQ6CC198WZB6-m1e-55cdfc19 set-priority actor=human:Wido targets=a-settled-reply-names-its-machine reason=priority-order subject=a-settled-reply-names-its-machine from=unranked to=1:88 requested-sequence=append
Integrity: sha256=721575eae00287ca3e0e1d472c9c1f6d4bbc29e0c3ee9a982b137105b44cf869
