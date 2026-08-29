# goal-git-stderr-pollution

- State: done
- Intent: goalGit captures CombinedOutput (internal/goal/txn.go:68) and Project trims that combined stream as the accepted tip (internal/goal/project.go:38), so any git warning on stderr corrupts object-name parsing — reproduced twice on 2026-08-27 under codex read-only sandboxes where macOS confstr() warnings surface (goal next printed the warning as part of the tip)
- Origin: main
- Next step: Appetite: 1h. Split stdout from stderr in goalGit (and audit sibling git captures in internal/goal for the same CombinedOutput shape): parse stdout only, surface stderr in the error path; fixture proves a git wrapper that prints a warning on stderr leaves the parsed tip clean.
- Concluded: Landed fb86c4c: goalGit and hashObject split channels — success returns parse-clean stdout (wrapper-warning pin), errors carry both voices for CAS push-rejection classification (the race suite caught the one-voice draft). Full goal suite green. Well inside the 1h appetite.
- OpenedAt: 2026-08-27T09:50:59Z
- Revision: 4
- Budget: elapsedLimit=2h attemptLimit=4 reservedJobMinutesLimit=30 activeJobLimit=1

History:
- 2026-08-27T09:50:59Z CA965F8RXHCRNDKBSAFEXV4KFA-m1-bf243850 open actor=m1+coordinator targets=goal-git-stderr-pollution
- 2026-08-29T18:50:57Z 0YB11KKBBWKWB5ZSFN6E9BZPT0-m2-bc1be9cb set-budget actor=human:wido targets=goal-git-stderr-pollution
- 2026-08-29T18:51:11Z 842BQJQWJ0CFPQ1ZRC2HY24ZZZ-m2-bc1be9cb claim actor=m2+mac-coordinator targets=goal-git-stderr-pollution
- 2026-08-29T19:27:54Z 0A1S6FZ4AFRKHZF9D1AZGN09ZR-m2-bc1be9cb done actor=m2+mac-coordinator targets=goal-git-stderr-pollution
Integrity: sha256=4a2440f50e7d69d798371c542e234217b232de19b2d776d24523890787fb6fc2
