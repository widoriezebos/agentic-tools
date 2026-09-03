# up-kills-runner-before-first-tick

- State: queued
- Intent: metasystem up (and the stop hook that runs it every turn end) replaces a live steward runner whose first tick has not completed: repairPinnedRunner judges the runner by tick completion, not by process liveness, kills it mid-tick, launches a replacement and waits only 10 seconds, while a real tick on a loaded Mac with a 228-file ledger takes 20 to 100 seconds (m2, 2026-09-03: twelve attempts, zero completions, every one cut by the next up). DONE means a runner that is alive and attempting is left to finish, the wait scales with the last measured tick, and a fixture proves a slow first tick survives a second up.
- Origin: main
- Next step: TIER 2 per R-54-m1 (mechanical logic inside internal/steward/runner.go and the watcher component): build plus one code review, no design round; box 4h/6/240m/1. Waits for human approval for execution. CORRECTION 2026-09-03 17:25 (m2): the killer is the supervision WATCHER as much as up: the watcher's 60-second cycle (supervise component --component watcher, pid 98600 here) relaunches the steward runner every minute because checkStewardRunner reads tick completion, not process liveness, so a tick longer than one watch interval can never complete (runner pids 18919, 41870, 49569, 56698 in four minutes). A hand-run tick on this Mac spends over 100 seconds in the ledger-attention phase (ValidateCommit forks one git cat-file per goal file, 228 files, plus the acceptance gates), under load average 3. Fix shape: liveness by process plus a tick-in-progress mark; the wait scales with the last measured tick; and ReadCommitGoals reads the ledger in one git cat-file --batch call. Related: stop-hook-wedge-on-enrollment-drift.
- OpenedAt: 2026-09-03T15:23:09Z
- Revision: 2
- Labels: robustness

History:
- 2026-09-03T15:23:09Z VVVCNKDBRF5TR9PM63VYN5E6TF-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=up-kills-runner-before-first-tick
- 2026-09-03T15:24:41Z VSKZJPGF1VKZ5ZDM4PQCTY8VVS-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=up-kills-runner-before-first-tick
Integrity: sha256=22abe567ea5cb71df5ed49e3f9d117926791f260c0299685c64b3b8144365952
