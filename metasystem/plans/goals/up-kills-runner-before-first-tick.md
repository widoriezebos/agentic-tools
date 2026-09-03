# up-kills-runner-before-first-tick

- State: queued
- Intent: metasystem up (and the stop hook that runs it every turn end) replaces a live steward runner whose first tick has not completed: repairPinnedRunner judges the runner by tick completion, not by process liveness, kills it mid-tick, launches a replacement and waits only 10 seconds, while a real tick on a loaded Mac with a 228-file ledger takes 20 to 100 seconds (m2, 2026-09-03: twelve attempts, zero completions, every one cut by the next up). DONE means a runner that is alive and attempting is left to finish, the wait scales with the last measured tick, and a fixture proves a slow first tick survives a second up.
- Origin: main
- Next step: TIER 2 per R-54-m1 (mechanical logic inside internal/steward/runner.go): build plus one code review, no design round; box 4h/6/240m/1. Waits for human approval for execution. Related: stop-hook-wedge-on-enrollment-drift (same turn-end hook).
- OpenedAt: 2026-09-03T15:23:09Z
- Revision: 1
- Labels: robustness

History:
- 2026-09-03T15:23:09Z VVVCNKDBRF5TR9PM63VYN5E6TF-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=up-kills-runner-before-first-tick
Integrity: sha256=1efc5c0e0c214d7eac9fec505f1244885ea5f34ea23124298e329139ba5a25fb
