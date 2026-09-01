# fixture-default-branch-assumption

- State: queued
- Intent: Five missionrunner wall-scope test beds (TestScopeCleanBedPasses and siblings) run 'git init' without '-b main' and assume the machine's init.defaultBranch is main - red on any host without that global config (git's own default is master through at least 2.39). Found by m0 (Debian guest, 2026-09-01) during the two-bars joint-round verification: the failures predate the round (proven by stash-revert at HEAD) and vanish when the guest sets init.defaultBranch=main. m0 healed its own environment; the portable fix is the beds passing -b main explicitly like the nested bed at wallscope_test.go:471 already does. R-33: robustness gain, well under 4h. Second sighting of the environment-assumption class after supervise-start-gate-linux-red.
- Origin: main
- Next step: BUILT AND CERTIFIED, LANDING BLOCKED (correcting the premature DONE note): the one-line fix is chain-closed (fixture-branch-v1, reviewedTree 9e8baef9) and proven under an emptied global config, but the landing gate runs the whole missionrunner package and the terminate-flake family is now CONSISTENTLY red on m0 (twice blocking in a row - the identity-drift symptom hardening). The certified diff is durable in the chain artifacts and staged in m0's checkout; it lands the moment the root fix (vm-epoch-identity-drift) un-reds the package. No further work needed on this goal itself.
- OpenedAt: 2026-09-01T09:50:49Z
- Revision: 8
- Budget: elapsedLimit=2d attemptLimit=4 reservedJobMinutesLimit=240 activeJobLimit=1
- Sliced: machine=m0 lineage=main-1788178136-1684505-4ffe42 revision=4 at=2026-09-01T21:16:15Z

History:
- 2026-09-01T09:50:49Z 96K61N4VHV674ZV5SFQ188AYCF-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=fixture-default-branch-assumption
- 2026-09-01T20:26:45Z YZVJT8TF4AEWR65BD8BYHDQJ80-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=fixture-default-branch-assumption
- 2026-09-01T21:16:05Z C5FMCAR4C8Y4AF9WT022Z811WB-m0-c5dbf036 set-budget actor=human:Wido targets=fixture-default-branch-assumption
- 2026-09-01T21:16:09Z J3111RM9TPRD71MY26V9PJMANN-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=fixture-default-branch-assumption
- 2026-09-01T21:16:15Z 5FYJSQRY56KP9VYHCF8134KYAP-m0-c5dbf036 slice-start actor=m0+main-1788178136-1684505-4ffe42 targets=fixture-default-branch-assumption
- 2026-09-01T21:30:26Z DT802804WH5XHZQ1EWNR3ZCQQ1-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=fixture-default-branch-assumption
- 2026-09-01T21:30:31Z RR4W4C7QE9Z8A1NWQ1Z9VY188N-m0-c5dbf036 release actor=m0+main-1788178136-1684505-4ffe42 targets=fixture-default-branch-assumption
- 2026-09-01T21:35:32Z 7H3J6J9NFSRCRJP5H5AEV9NVAF-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=fixture-default-branch-assumption
Integrity: sha256=450cfb62230ca8435c8e5e94ef15cd18bc20d013334c0a5c56a1bbf5ef5a63dd
