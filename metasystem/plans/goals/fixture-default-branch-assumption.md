# fixture-default-branch-assumption

- State: claimed
- Intent: Five missionrunner wall-scope test beds (TestScopeCleanBedPasses and siblings) run 'git init' without '-b main' and assume the machine's init.defaultBranch is main - red on any host without that global config (git's own default is master through at least 2.39). Found by m0 (Debian guest, 2026-09-01) during the two-bars joint-round verification: the failures predate the round (proven by stash-revert at HEAD) and vanish when the guest sets init.defaultBranch=main. m0 healed its own environment; the portable fix is the beds passing -b main explicitly like the nested bed at wallscope_test.go:471 already does. R-33: robustness gain, well under 4h. Second sighting of the environment-assumption class after supervise-start-gate-linux-red.
- Origin: main
- Next step: One small slice: add '-b main' (or equivalent) to every bed init in internal/missionrunner tests that asserts a branch name; prove by running the TestScope family with init.defaultBranch unset. Budget tuple is Wido's word at claim
- OpenedAt: 2026-09-01T09:50:49Z
- Revision: 5
- Budget: elapsedLimit=2d attemptLimit=4 reservedJobMinutesLimit=240 activeJobLimit=1
- Sliced: machine=m0 lineage=main-1788178136-1684505-4ffe42 revision=4 at=2026-09-01T21:16:15Z
- Claimed: machine=m0 lineage=main-1788178136-1684505-4ffe42 at=2026-09-01T21:16:09Z revision=4
- StopCapability: generation=4 revision=4 machine=m0 claimEpoch=1 fenceEpoch=0

History:
- 2026-09-01T09:50:49Z 96K61N4VHV674ZV5SFQ188AYCF-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=fixture-default-branch-assumption
- 2026-09-01T20:26:45Z YZVJT8TF4AEWR65BD8BYHDQJ80-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=fixture-default-branch-assumption
- 2026-09-01T21:16:05Z C5FMCAR4C8Y4AF9WT022Z811WB-m0-c5dbf036 set-budget actor=human:Wido targets=fixture-default-branch-assumption
- 2026-09-01T21:16:09Z J3111RM9TPRD71MY26V9PJMANN-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=fixture-default-branch-assumption
- 2026-09-01T21:16:15Z 5FYJSQRY56KP9VYHCF8134KYAP-m0-c5dbf036 slice-start actor=m0+main-1788178136-1684505-4ffe42 targets=fixture-default-branch-assumption
Integrity: sha256=d5ee665b19f2563c9fceef71919d75a7a371796d9a9d1f9b418a8f7a2528980f
