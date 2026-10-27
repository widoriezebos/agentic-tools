# a-settled-reply-names-its-machine

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Fills two fields from the same resolver the channel duty already uses, and refuses an unusable record; no authority changes; without it no phone answer can act"
- Tier: 2
- Intent: A reply the steward's settlement pass matches is recorded under this machine's name and lineage, so its operation id is valid and the chosen act runs on the next tick; a channel config without a machine refuses to record an answer instead of writing an unusable one (finding 435: Wido's first live tap, 2026-10-27 16:46, stalled at 'an answer needs a valid recorded operation id' because channel/phase/phase.go SettleReplies builds PollConfig with no Machine)
- Origin: main
- Next step: Build first on m1e's build lane (before a-question-reminds U4a and a-launched-job-is-never-a-main): opus-builder fixes SettleReplies to resolve the machine (goal.ResolveMachine) and lineage as run() does (phase.go:187 against :205-214), adds the refusal for an empty machine at the answer record (inbox.go:693-708), and tests the steward's real order (settlement pass before the channel poll, runner.go:371 then :455) for a tap and a typed reply on a contract question through to the act; about 10 production lines; Read each round: yes; the steward submits with work review --changes --last
- OpenedAt: 2026-10-27T16:00:22Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-27T16:00:32Z revision=2 opid=KM4V1ZQS6QKK04B8CCA7ZS1E2P-m1e-55cdfc19 authority=proven digest=37e5d4f7a5ad4104c30966f643d045c34dc7fe207dad4f427396178409d0b6db episode=2

History:
- 2026-10-27T16:00:22Z 11AKW3J08BWMFGEHHAA8CMW1RG-m1e-55cdfc19 open actor=human:Wido targets=a-settled-reply-names-its-machine
- 2026-10-27T16:00:32Z KM4V1ZQS6QKK04B8CCA7ZS1E2P-m1e-55cdfc19 approve actor=human:Wido targets=a-settled-reply-names-its-machine
Integrity: sha256=f4b910b1b64186bbab8e619437274260c2072e9e5b766511b8e9ca53eb6e4504
