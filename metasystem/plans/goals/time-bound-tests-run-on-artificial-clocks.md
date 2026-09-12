# time-bound-tests-run-on-artificial-clocks

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: a test that fails under load blocks every seat and every slower machine; novelty 2: injected clocks and probers exist in parts of the tree but most process tests still wait on wall time; exposure 3: every battery on every machine; accumulation 2: each new process test written the old way adds another"
- Tier: 2
- Intent: Every test that waits on wall time or observes a real process state (a kill, a reap, an exec, a pipe drain, a census verdict age, a watchdog silence window or cap, a TERM or KILL grace) drives an injected clock and an injected process prober, so no scheduler and no box load can change its verdict; the real-process tests that remain are end-to-end wiring proofs, few, each named in the design page, with bounds an order of magnitude above the expected time. Wido, 2026-09-12 (R-104-m1e): I want all time-bound tests to use artificial clocks; tests failing under load means they will also fail on slower machines, and that is simply unacceptable. DONE means: an inventory of every such test in internal and cmd with its disposition (converted, kept as a named wiring proof, or deleted as redundant) on the design page; the converted tests pass with the race detector beside sixteen yes burners on the eighteen-core box three times in a row and once on the 4-vCPU VM; no cadence or delivery red of the following week is a timing red; the eight defects of 2026-09-12 (probe argv, burner leak and reap, watchdog verdict, ENOMEM count, terminate-group reap, detached-worktree names, stale census retry) are each covered by a deterministic test. RUNTIME INDEPENDENCE: the mechanism is in the Go engine and the fixture beds, never in one runtime.
- Origin: human
- Next step: Inventory first: grep every _test.go in internal and cmd for exec.Command, time.Sleep, time.Now, Process.Kill, syscall.Kill, groupAlive and the fixture beds for sleep and deadline loops; classify each; design the clock and prober seams package by package (missionrunner, proofrun, supervise, identity, landing, lease, janitor, census first); convert in slices, each landed by human commit with the loaded sweep as its proof.
- OpenedAt: 2026-09-12T12:13:17Z
- Revision: 2
- Pinned: m1e
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-12T12:13:17Z KZEPT63WG4WET47CYY3TW2NTE2-m1-c6925449 open actor=human:Wido targets=time-bound-tests-run-on-artificial-clocks
- 2026-09-12T12:13:22Z 2VB94VFDYJ5CWQF9E9KWSS3Q4D-m1-c6925449 set-pin actor=human:Wido targets=time-bound-tests-run-on-artificial-clocks
Integrity: sha256=b116db8a97319bd430c35b92aafcefbd54094b2285350c35e849d65e86245643
