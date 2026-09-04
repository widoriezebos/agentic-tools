# up-kills-runner-before-first-tick

- State: claimed
- Tier: 2
- Intent: metasystem up and the supervision watcher replace a live steward runner whose first tick has not completed: the runner is judged by tick completion, not process liveness, killed mid-tick and relaunched (the watcher every 60 seconds, up on every call including the stop hook's), while a real tick on this Mac spends over 210 seconds inside the goal ledger projection (goal.Project -> loadTree -> ReadCommitGoals forks one git cat-file per goal file, 228 files, repeated per caller) under load average 3 (m2, 2026-09-03: fifteen attempts, zero completions). DONE means a runner that is alive and attempting is left to finish, the watch and the up wait scale with the last measured tick, the ledger is read in one git cat-file --batch call, and a fixture proves a slow first tick survives both a second up and a watcher cycle.
- Origin: main
- Next step: TIER 2 per R-54-m1 (mechanical logic inside internal/steward/runner.go and the watcher component): build plus one code review, no design round; box 4h/6/240m/1. Waits for human approval for execution. CORRECTION 2026-09-03 17:25 (m2): the killer is the supervision WATCHER as much as up: the watcher's 60-second cycle (supervise component --component watcher, pid 98600 here) relaunches the steward runner every minute because checkStewardRunner reads tick completion, not process liveness, so a tick longer than one watch interval can never complete (runner pids 18919, 41870, 49569, 56698 in four minutes). A hand-run tick on this Mac spends over 100 seconds in the ledger-attention phase (ValidateCommit forks one git cat-file per goal file, 228 files, plus the acceptance gates), under load average 3. Fix shape: liveness by process plus a tick-in-progress mark; the wait scales with the last measured tick; and ReadCommitGoals reads the ledger in one git cat-file --batch call. Related: stop-hook-wedge-on-enrollment-drift.
- OpenedAt: 2026-09-03T15:23:09Z
- Revision: 7
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- Approved: by=human:Wido at=2026-09-04T06:13:30Z revision=5 opid=7C9Y8FXPB9SY9CTTB110NZ49KR-m2-5fcf08ab authority=relayed digest=1d8bf3096d231a8f1cb03eefd766dbd4078c29b0518c56d84c2ccc14a4defecf reviewBy=2026-09-06
- Sliced: machine=m2 lineage=main-1788441779-14484-82d6ed revision=6 at=2026-09-04T07:35:01Z
- Claimed: machine=m2 lineage=main-1788441779-14484-82d6ed at=2026-09-04T07:32:38Z revision=6
- StopCapability: generation=6 revision=6 machine=m2 claimEpoch=2 fenceEpoch=0

History:
- 2026-09-03T15:23:09Z VVVCNKDBRF5TR9PM63VYN5E6TF-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=up-kills-runner-before-first-tick
- 2026-09-03T15:24:41Z VSKZJPGF1VKZ5ZDM4PQCTY8VVS-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=up-kills-runner-before-first-tick
- 2026-09-03T15:28:34Z FZDNHFAF2Q842Z06NXZECDW7NH-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=up-kills-runner-before-first-tick
- 2026-09-04T06:13:06Z NNQH0ZVHBNQ26NWMDEXH9A20HT-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=up-kills-runner-before-first-tick
- 2026-09-04T06:13:30Z 7C9Y8FXPB9SY9CTTB110NZ49KR-m2-5fcf08ab approve actor=human:Wido targets=up-kills-runner-before-first-tick authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="the bugs yu mentioned are approved to fix too"
- 2026-09-04T07:32:38Z 0N43ZFBJ51XK6PY2YAQS2V53XV-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=up-kills-runner-before-first-tick
- 2026-09-04T07:35:01Z 4E7TMNQVXF6BGD2G99XJ3XQQ6R-m2-5fcf08ab slice-start actor=m2+main-1788441779-14484-82d6ed targets=up-kills-runner-before-first-tick
Integrity: sha256=e8d81f1d1eedaeafcfdc013598409cf2beeb7d59950542649f92a6fe3773edd7
