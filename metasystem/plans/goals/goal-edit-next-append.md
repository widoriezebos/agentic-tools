# goal-edit-next-append

- State: approved
- Intent: goal edit --next replaces a goal's whole next step, and the seat composes the replacement by reading its LOCAL goal file, which lags the accepted tree until the next pull; on 2026-09-02 m1 edited codex-handshake-budget-load-fragile minutes after opening it, the local file did not exist yet, the read returned empty, and the ladder text was silently replaced by one measurement sentence (restored by hand two edits later). A ledger verb that lets a stale read erase a record is a robustness defect (R-33: robustness gain, under 4h). DONE means: goal edit gains an append form for the next step (or a --next-append flag) so a seat adds a fact without retyping the step, and edit refuses when the local goal file is behind the accepted tip for that goal, naming the pull that fixes it.
- Origin: main
- Next step: SURVEYED, NOT STARTED, released 2026-09-07 02:05Z (m1c stops here on Wido's word). What the survey found for whoever resumes: goal edit is runGoalEdit in cmd/metasystem/goalsync_mutations.go (line 1227, runSyncOnly('edit', ...)) and goal.Edit in internal/goal/verbs.go line 1418; the next step is one whole field on the goal file, and the seat composes the replacement text itself, so a stale local read yields an empty base and the ladder text is lost. The append form therefore belongs in the VERB (a --next-append flag that reads the accepted tree's current NextStep, not the local file, and appends), plus a refusal when the local goal file is behind the accepted tip. Fixture home: scripts/agents/goal-cli-fixtures.sh has the edit legs at lines 350-372 (label add/remove, no-op revision bump, contradictory refusal); add the empty-read overwrite replay beside them. Tier 2 per R-54-m1: one Sol build, one Fable review, land with --chain.
- OpenedAt: 2026-09-02T16:27:54Z
- Revision: 7
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=4 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=0fd773c2d87cd4bb98ea7dae754408b9728e709e4287073071546eacf56d1aee

History:
- 2026-09-02T16:27:54Z 4TPZGZPR25FVZRHWG7ADWKY5W2-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=goal-edit-next-append
- 2026-09-02T16:28:05Z 930R2N46704Z7F94JMWWE1ZEZZ-m1-7bb1546e set-budget actor=m1+main-1788333680-2840-7f79f4 targets=goal-edit-next-append
- 2026-09-02T18:36:37Z W30259HPZ24R2HAWKSEF7KF8CJ-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=goal-edit-next-append
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=goal-edit-next-append reason=sweep
- 2026-09-07T01:31:25Z CH1BGG3EZZKPFJVD71HNZTFE86-m1c-7cd0bd60 claim actor=m1c+main-1788680061-17829-64951c targets=goal-edit-next-append
- 2026-09-07T05:26:09Z T628J4G3H0Z1S348XK3G6DHB7R-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=goal-edit-next-append
- 2026-09-07T05:26:13Z VE3SK83H4ER6AMJBQ51DWMW82D-m1c-7cd0bd60 release actor=m1c+main-1788680061-17829-64951c targets=goal-edit-next-append
Integrity: sha256=bb31822e88ac79f895b96bc15079ad635ffed8f17bbb57a02e331ac43a0fe7f8
