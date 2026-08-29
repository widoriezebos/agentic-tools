# goal-git-stderr-pollution

- State: queued
- Intent: goalGit captures CombinedOutput (internal/goal/txn.go:68) and Project trims that combined stream as the accepted tip (internal/goal/project.go:38), so any git warning on stderr corrupts object-name parsing — reproduced twice on 2026-08-27 under codex read-only sandboxes where macOS confstr() warnings surface (goal next printed the warning as part of the tip)
- Origin: main
- Next step: Appetite: 1h. Split stdout from stderr in goalGit (and audit sibling git captures in internal/goal for the same CombinedOutput shape): parse stdout only, surface stderr in the error path; fixture proves a git wrapper that prints a warning on stderr leaves the parsed tip clean.
- OpenedAt: 2026-08-27T09:50:59Z
- Revision: 2
- Budget: elapsedLimit=2h attemptLimit=4 reservedJobMinutesLimit=30 activeJobLimit=1

History:
- 2026-08-27T09:50:59Z CA965F8RXHCRNDKBSAFEXV4KFA-m1-bf243850 open actor=m1+coordinator targets=goal-git-stderr-pollution
- 2026-08-29T18:50:57Z 0YB11KKBBWKWB5ZSFN6E9BZPT0-m2-bc1be9cb set-budget actor=human:wido targets=goal-git-stderr-pollution
Integrity: sha256=7637c5049590cfc7d15ddde65c8e67cc32cd303dac3b61a7760f3d4c1c37f07f
