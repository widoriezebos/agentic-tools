# goal-git-stderr-pollution

- State: queued
- Intent: goalGit captures CombinedOutput (internal/goal/txn.go:68) and Project trims that combined stream as the accepted tip (internal/goal/project.go:38), so any git warning on stderr corrupts object-name parsing — reproduced twice on 2026-08-27 under codex read-only sandboxes where macOS confstr() warnings surface (goal next printed the warning as part of the tip)
- Origin: main
- Next step: Appetite: 1h. Split stdout from stderr in goalGit (and audit sibling git captures in internal/goal for the same CombinedOutput shape): parse stdout only, surface stderr in the error path; fixture proves a git wrapper that prints a warning on stderr leaves the parsed tip clean.
- OpenedAt: 2026-08-27T09:50:59Z
- Revision: 1

History:
- 2026-08-27T09:50:59Z CA965F8RXHCRNDKBSAFEXV4KFA-m1-bf243850 open actor=m1+coordinator targets=goal-git-stderr-pollution
Integrity: sha256=25402381299347240def72dd0406fece4d9979a49c4483acc4fd000abc6069de
