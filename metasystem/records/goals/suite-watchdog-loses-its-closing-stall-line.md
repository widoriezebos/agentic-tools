# suite-watchdog-loses-its-closing-stall-line

- State: done
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: a stalled suite's verdict line is the operator's only record of why a proof run was killed; novelty 1: the watchdog already prints the reason, the loss is in the tail of its cleanup; exposure 2: every proof run's watchdog, seen once in five bare launches of the chatty fixture; accumulation 1: none"
- Tier: 2
- Intent: The suite watchdog's closing line, 'suite stalled in section X (reason); evidence preserved before kill at ...', reaches the launcher's output on every stall. On 2026-09-12 a twelve-run probe of the suite-progress fixture's chatty scenario (bin/metasystem proof-run launch with --section-cap-ms 400 over scripts/agents/suite-progress-fixtures.sh __printing_forever) lost that line two times in ten on a quiet box: the launcher output carried the watchdog's 'suite process-group cleanup was incomplete: kill process group: killed signal refused ... (dead)' line and the launcher's 'suite progress structure is incomplete' line, but not the verdict, and the launcher exited 1; cadence runs 12 and 13 of deep-battery-under-ten-minutes went red on it. The watchdog announces the section and reason before any kill-capable action since this morning's landing, which keeps the reason on record; this goal finds why the closing line is lost (the watchdog dying inside its own cleanup, the launcher closing the pipe early, or the execution-guard sweep) and proves the fix with a test that drives the same race. DONE: the cause is named in the design note, the closing line survives twenty consecutive bare launches of the chatty scenario, and a Go test pins it.
- Origin: main
- Next step: Reproduce with a bare launch loop of the chatty scenario (the seat's probe script is in its scratchpad; the fixture's launch_fixture call is the template); read internal/proofrun/watchdog.go stopStalledSuite from the SignalSuiteGroup call to its return, and internal/proofrun/launcher.go from touchDone to copies.Wait; instrument the watchdog's exit status in the launcher output first.
- Concluded: Cause: the launcher called exec.Cmd.Wait on the watchdog before the goroutines copying its stdout and stderr pipes had drained, and Wait closes the pipes at exit, so the verdict, the last line the watchdog writes, was lost one launch in five. Fix a53476ba: the watchdog streams have their own wait group drained before Wait; the launcher prints how the watchdog ended; the watchdog announces section and reason before acting (1dd00469). Proof: twenty bare launches of the chatty scenario with the closing line present; TestLaunchSuiteKeepsTheWatchdogsLastLine fails on the old order at round 20 of 25 and passes on the fix; go test ./internal/proofrun green. Recorded in plans/deep-battery-design.md.
- OpenedAt: 2026-09-12T08:11:40Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-12T08:11:40Z 1DSWQ3Y7G4Y3SHH6WNHPTWB8QK-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=suite-watchdog-loses-its-closing-stall-line,deep-battery-under-ten-minutes
- 2026-09-12T08:19:08Z DDX1DPZ19G4GFRTV9GJ86699Y2-m1-c6925449 done actor=human:Wido targets=suite-watchdog-loses-its-closing-stall-line
Integrity: sha256=bd10c63e45f383f77247f1e3205b31ed893860a368db9dc4b310c175df070794
