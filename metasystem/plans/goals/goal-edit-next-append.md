# goal-edit-next-append

- State: claimed
- Intent: goal edit --next replaces a goal's whole next step, and the seat composes the replacement by reading its LOCAL goal file, which lags the accepted tree until the next pull; on 2026-09-02 m1 edited codex-handshake-budget-load-fragile minutes after opening it, the local file did not exist yet, the read returned empty, and the ladder text was silently replaced by one measurement sentence (restored by hand two edits later). A ledger verb that lets a stale read erase a record is a robustness defect (R-33: robustness gain, under 4h). DONE means: goal edit gains an append form for the next step (or a --next-append flag) so a seat adds a fact without retyping the step, and edit refuses when the local goal file is behind the accepted tip for that goal, naming the pull that fixes it.
- Origin: main
- Next step: TIER 2 per R-54-m1 (mechanical logic inside an existing owner, the goal edit verb): build (Sol: an append form for the next step and a refusal when the local goal file is behind the accepted tip, with a fixture replaying the empty-read overwrite), one code review (Fable), land with --chain. No design round. Box stays 4h/10/240m/1.
- OpenedAt: 2026-09-02T16:27:54Z
- Revision: 5
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=4 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=0fd773c2d87cd4bb98ea7dae754408b9728e709e4287073071546eacf56d1aee
- Claimed: machine=m1c lineage=main-1788680061-17829-64951c at=2026-09-07T01:31:25Z revision=5 accountingRevision=5
- StopCapability: generation=5 revision=5 machine=m1c claimEpoch=1 fenceEpoch=0

History:
- 2026-09-02T16:27:54Z 4TPZGZPR25FVZRHWG7ADWKY5W2-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=goal-edit-next-append
- 2026-09-02T16:28:05Z 930R2N46704Z7F94JMWWE1ZEZZ-m1-7bb1546e set-budget actor=m1+main-1788333680-2840-7f79f4 targets=goal-edit-next-append
- 2026-09-02T18:36:37Z W30259HPZ24R2HAWKSEF7KF8CJ-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=goal-edit-next-append
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=goal-edit-next-append reason=sweep
- 2026-09-07T01:31:25Z CH1BGG3EZZKPFJVD71HNZTFE86-m1c-7cd0bd60 claim actor=m1c+main-1788680061-17829-64951c targets=goal-edit-next-append
Integrity: sha256=eb4ba76ab020bc28e62416a7f675fa4681d5c57948517207575e0ceb1c0bf0da
