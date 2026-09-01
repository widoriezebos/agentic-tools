# proof-harness-process-custody

- State: queued
- Intent: Seat-run proof harnesses leak their load generators: twelve CPU-hog busy loops from m2's 2026-08-31 proof legs orphaned to init at 95-100 percent CPU for 12-13 hours (pids preserved in the session records; m3's twelve from the prior afternoon were the same class), because 'kill $LOADPIDS' from a jobs list races shell detachment under the background-execution wrapper - the kill fires in a shell whose job table no longer owns the loops. The leaked load then poisoned the very load diagnoses the harnesses served, and starved m3's steward startup confirmation. Same disease the delegate machinery already cured for its own children (process-group custody, dab1dbd family) - seat-run harnesses have no custody at all.
- Origin: main
- Next step: Appetite: 2h, full ladder per R-38-m2 (backlog, design, design critique, build, code critique, tests). Direction for the designer: proof harnesses own their processes DETERMINISTICALLY - a harness runs its load generators in one process group it kills whole on every exit path (trap on EXIT, kill by negative pgid), or better: a small engine verb (proc load-generate --seconds N --workers K) whose group the existing kill-through machinery owns, so no shell job table is ever the custodian. Fixture: a harness killed mid-run leaves zero orphans; the census sees nothing unowned.
- OpenedAt: 2026-09-01T07:21:28Z
- Revision: 1

History:
- 2026-09-01T07:21:28Z 9WGMX9SF5S5CXHZYNZ4SY0ZMXY-m2-bc1be9cb open actor=m2+mac-coordinator targets=proof-harness-process-custody
Integrity: sha256=e488d4c12feb2ff70308ec5bc7b8d90bdec775bca2328a3e82802dffcb750e1c
