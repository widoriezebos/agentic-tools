# proof-harness-process-custody

- State: claimed
- Intent: Seat-run proof harnesses leak their load generators: twelve CPU-hog busy loops from m2's 2026-08-31 proof legs orphaned to init at 95-100 percent CPU for 12-13 hours (pids preserved in the session records; m3's twelve from the prior afternoon were the same class), because 'kill $LOADPIDS' from a jobs list races shell detachment under the background-execution wrapper - the kill fires in a shell whose job table no longer owns the loops. The leaked load then poisoned the very load diagnoses the harnesses served, and starved m3's steward startup confirmation. Same disease the delegate machinery already cured for its own children (process-group custody, dab1dbd family) - seat-run harnesses have no custody at all.
- Origin: main
- Next step: Appetite: 2h, full ladder per R-38-m2 (backlog, design, design critique, build, code critique, tests). Direction for the designer: proof harnesses own their processes DETERMINISTICALLY - a harness runs its load generators in one process group it kills whole on every exit path (trap on EXIT, kill by negative pgid), or better: a small engine verb (proc load-generate --seconds N --workers K) whose group the existing kill-through machinery owns, so no shell job table is ever the custodian. Fixture: a harness killed mid-run leaves zero orphans; the census sees nothing unowned. SECOND SPECIMEN (m1, 2026-09-02): 325 orphaned fixture processes on the m1 Mac, two to four days old, 303 of them steward runners from agent-fixture beds under the user's temp directory (plus fixture-battery-owner supervise components, a revocation-race battery loop and a fake-adapter handshake loop), still ticking and writing narration logs at 34 files per minute while Apple's fseventsd sat at 100 percent CPU and 12.7 GB resident for 17 days; 949 stale temp beds totalling 5.6 GB remain on disk. No engine verb owns leaked fixture processes, so the seat had nothing lawful to run. DONE for this goal now also means a sweep the seat may run: reap fixture beds whose processes are orphaned and older than a bound, under the census's eye.
- OpenedAt: 2026-09-01T07:21:28Z
- Revision: 4
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1
- Claimed: machine=m1 lineage=main-1788333680-2840-7f79f4 at=2026-09-02T17:38:39Z revision=4
- StopCapability: generation=4 revision=4 machine=m1 claimEpoch=4 fenceEpoch=0

History:
- 2026-09-01T07:21:28Z 9WGMX9SF5S5CXHZYNZ4SY0ZMXY-m2-bc1be9cb open actor=m2+mac-coordinator targets=proof-harness-process-custody
- 2026-09-01T20:29:15Z 9E5FFRAKCWM2WP3PZEDNCESZ7G-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=proof-harness-process-custody
- 2026-09-02T12:55:19Z EW5J220JFZSJ177EQC0B639M07-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=proof-harness-process-custody
- 2026-09-02T17:38:39Z 2WFJQGYPYFCD4SCA4MV9QSV106-m1-7bb1546e claim actor=m1+main-1788333680-2840-7f79f4 targets=proof-harness-process-custody
Integrity: sha256=64a1b1946cc14407382355ceac2f10cbf0a9e83a4cdcb70584a0d0191b8b9c9c
