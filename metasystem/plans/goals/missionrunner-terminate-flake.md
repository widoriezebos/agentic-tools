# missionrunner-terminate-flake

- State: queued
- Intent: TestTerminateGroupLeaksNoGroupsUnderCompression (internal/missionrunner) failed in the first governed weight-discharge validation (run governed-discharge-20260831-c, 2026-08-31) on a quiet-held shared machine where the package still took 1271s - the wall-clock-patience-under-load family (same class as steward-tick-load-flake, which failed twice in the same gate). Evidence: artifacts/agents/suite-failures/20260831T123307Z-52778/go-engine-gate.log.
- Origin: main
- Next step: Appetite: 2h. Triage whether the 7.24s failure is a patience assumption (condition-based waiting fix, the suite-custody pattern) or a real group-termination leak under compression - the test name touches the kill-through/zombie-reap machinery m2 landed at dab1dbd, so verify against that contract before loosening anything; a leak here would be a real defect, not a flake. Prove under artificial load. TRIAGED AND FIXED (2026-08-31, chain mr-flake-fix2 under standing-validation): the abandonment was the dab1dbd contract behaving as designed - the log shows 'not provably ours; leaving it to the census' immediately before the assertion; the fix is TEST-ONLY (leaked = alive AND unkilled AND outside census custody; abandoned-to-census = lawful fail-closed), production untouched, live-but-lost groups still fail with diagnostics. Proven: targeted double-run green in 11s on the shared box; conformance review persisted (reviewedTree aea4bf87). LANDS after its Fable critique round (pending Wido's budget word on the flat-120 reservation wall) - conclude this goal with that landing.
- OpenedAt: 2026-08-31T12:34:34Z
- Revision: 2

History:
- 2026-08-31T12:34:34Z EHMH5BQET0RG42HVZKV7C48T4Q-m2-bc1be9cb open actor=m2+mac-coordinator targets=missionrunner-terminate-flake
- 2026-08-31T17:28:32Z WW69E2ASKTZZ6D5KMNCPKVN5RK-m2-bc1be9cb edit actor=m2+mac-coordinator targets=missionrunner-terminate-flake
Integrity: sha256=21e97a015a9161023f2e25038df3f8a3aa15baf71193fd54463f2b1e006b7724
