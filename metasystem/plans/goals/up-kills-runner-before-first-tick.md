# up-kills-runner-before-first-tick

- State: approved
- Tier: 2
- Intent: metasystem up and the supervision watcher replace a live steward runner whose first tick has not completed: the runner is judged by tick completion, not process liveness, killed mid-tick and relaunched (the watcher every 60 seconds, up on every call including the stop hook's), while a real tick on this Mac spends over 210 seconds inside the goal ledger projection (goal.Project -> loadTree -> ReadCommitGoals forks one git cat-file per goal file, 228 files, repeated per caller) under load average 3 (m2, 2026-09-03: fifteen attempts, zero completions). DONE means a runner that is alive and attempting is left to finish, the watch and the up wait scale with the last measured tick, the ledger is read in one git cat-file --batch call, and a fixture proves a slow first tick survives both a second up and a watcher cycle.
- Origin: main
- Next step: LANDED 2026-09-04 11:2x local (m2): 1f1aba13 'A live steward runner is left to finish its tick' (chain ukr-build1; one Fable review, zero material; records/misc/up-kills-runner-before-first-tick-critique-cc1.md). On main: liveness by process plus an attempt-in-progress mark, steward.tick-patience-sec (default 120, bound max(3 x last tick, 120)), up counts an attempting runner as verified, ReadCommitGoals reads the ledger in one git cat-file --batch. Residuals: a first tick that genuinely exceeds 120 seconds still loops at a 120-second cadence (UKR-03); no named equivalence test for the batched reader (UKR-02); the timing comparison is unmeasured (UKR-01). Remaining before goal done: the journey chapter, batched with the other 2026-09-04 fixes. The seat releases now.
- OpenedAt: 2026-09-03T15:23:09Z
- Revision: 9
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- Approved: by=human:Wido at=2026-09-04T06:13:30Z revision=5 opid=7C9Y8FXPB9SY9CTTB110NZ49KR-m2-5fcf08ab authority=relayed digest=1d8bf3096d231a8f1cb03eefd766dbd4078c29b0518c56d84c2ccc14a4defecf reviewBy=2026-09-06
- Sliced: machine=m2 lineage=main-1788441779-14484-82d6ed revision=6 at=2026-09-04T07:35:01Z

History:
- 2026-09-03T15:23:09Z VVVCNKDBRF5TR9PM63VYN5E6TF-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=up-kills-runner-before-first-tick
- 2026-09-03T15:24:41Z VSKZJPGF1VKZ5ZDM4PQCTY8VVS-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=up-kills-runner-before-first-tick
- 2026-09-03T15:28:34Z FZDNHFAF2Q842Z06NXZECDW7NH-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=up-kills-runner-before-first-tick
- 2026-09-04T06:13:06Z NNQH0ZVHBNQ26NWMDEXH9A20HT-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=up-kills-runner-before-first-tick
- 2026-09-04T06:13:30Z 7C9Y8FXPB9SY9CTTB110NZ49KR-m2-5fcf08ab approve actor=human:Wido targets=up-kills-runner-before-first-tick authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="the bugs yu mentioned are approved to fix too"
- 2026-09-04T07:32:38Z 0N43ZFBJ51XK6PY2YAQS2V53XV-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=up-kills-runner-before-first-tick
- 2026-09-04T07:35:01Z 4E7TMNQVXF6BGD2G99XJ3XQQ6R-m2-5fcf08ab slice-start actor=m2+main-1788441779-14484-82d6ed targets=up-kills-runner-before-first-tick
- 2026-09-04T09:21:05Z YSCJQ7PQ8N9M9MSVH3VVHB1790-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=up-kills-runner-before-first-tick
- 2026-09-04T09:21:38Z DJ091K1XYZ69GV2KWC20W701RE-m2-5fcf08ab release actor=m2+main-1788441779-14484-82d6ed targets=up-kills-runner-before-first-tick
Integrity: sha256=fe1a593e216d690cba0491333fdb24bcde8191a383c329a053b172c1d788fd27
