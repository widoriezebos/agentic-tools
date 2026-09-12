# time-bound-tests-run-on-artificial-clocks

- State: approved
- Priority: 1
- Sequence: 3
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: a test that fails under load blocks every seat and every slower machine; novelty 2: injected clocks and probers exist in parts of the tree but most process tests still wait on wall time; exposure 3: every battery on every machine; accumulation 2: each new process test written the old way adds another"
- Tier: 2
- Intent: Every test that waits on wall time or observes a real process state (a kill, a reap, an exec, a pipe drain, a census verdict age, a watchdog silence window or cap, a TERM or KILL grace) drives an injected clock and an injected process prober, so no scheduler and no box load can change its verdict; the real-process tests that remain are end-to-end wiring proofs, few, each named in the design page, with bounds an order of magnitude above the expected time. Wido, 2026-09-12 (R-104-m1e): I want all time-bound tests to use artificial clocks; tests failing under load means they will also fail on slower machines, and that is simply unacceptable. DONE means: an inventory of every such test in internal and cmd with its disposition (converted, kept as a named wiring proof, or deleted as redundant) on the design page; the converted tests pass with the race detector beside sixteen yes burners on the eighteen-core box three times in a row and once on the 4-vCPU VM; no cadence or delivery red of the following week is a timing red; the eight defects of 2026-09-12 (probe argv, burner leak and reap, watchdog verdict, ENOMEM count, terminate-group reap, detached-worktree names, stale census retry) are each covered by a deterministic test. RUNTIME INDEPENDENCE: the mechanism is in the Go engine and the fixture beds, never in one runtime.
- Origin: human
- Next step: Inventory first: grep every _test.go in internal and cmd for exec.Command, time.Sleep, time.Now, Process.Kill, syscall.Kill, groupAlive and the fixture beds for sleep and deadline loops; classify each; design the clock and prober seams package by package (missionrunner, proofrun, supervise, identity, landing, lease, janitor, census first); convert in slices, each landed by human commit with the loaded sweep as its proof.
- OpenedAt: 2026-09-12T12:13:17Z
- Revision: 4
- Pinned: m1e
- Budget: elapsedLimit=5d attemptLimit=20 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-12T12:23:55Z revision=4 opid=R31WHWRCXMKCXK8910Z18MNG2J-m1-c6925449 authority=proven digest=98e98e1029200df56a656c0432fd1672c015103ba7f5e3f57dcd7efc1605bdfc

History:
- 2026-09-12T12:13:17Z KZEPT63WG4WET47CYY3TW2NTE2-m1-c6925449 open actor=human:Wido targets=time-bound-tests-run-on-artificial-clocks
- 2026-09-12T12:13:22Z 2VB94VFDYJ5CWQF9E9KWSS3Q4D-m1-c6925449 set-pin actor=human:Wido targets=time-bound-tests-run-on-artificial-clocks
- 2026-09-12T12:13:28Z 70TQ878Z89JN76Y6SBG4HNYH6A-m1-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,budget-extends-by-consumption-and-breach-parks,capped-round-continues-instead-of-restarting,coordinator-context-stays-under-budget,coordinator-wakes-on-events-not-polls,critique-always,critique-closes-on-folded-proof,delegate-rounds-reuse-a-warm-gate,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,human-goal-verbs-forgiving,hung-proof-attempts-end-at-their-deadline,job-record-birth-token,land-ready-work-lands-without-a-claim-slot,landing-receipt-survives-records-drift,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-attempts-settle-to-minutes-run,proof-groups-detect-hangs-by-progress-not-the-clock,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,stop-hook-never-forces-an-empty-turn,suite-custody,time-bound-tests-run-on-artificial-clocks,token-spend-fence,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=time-bound-tests-run-on-artificial-clocks from=unranked to=1:3 requested-sequence=3
- 2026-09-12T12:23:55Z R31WHWRCXMKCXK8910Z18MNG2J-m1-c6925449 approve actor=human:Wido targets=time-bound-tests-run-on-artificial-clocks
Integrity: sha256=6dfef194e11a05d317b96a8a685d6a56bc2636b592b6b3afc82dade0e90ec8d6
