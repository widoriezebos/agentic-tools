# missionrunner-terminate-flake

- State: queued
- Intent: TestTerminateGroupLeaksNoGroupsUnderCompression (internal/missionrunner) failed in the first governed weight-discharge validation (run governed-discharge-20260831-c, 2026-08-31) on a quiet-held shared machine where the package still took 1271s - the wall-clock-patience-under-load family (same class as steward-tick-load-flake, which failed twice in the same gate). Evidence: artifacts/agents/suite-failures/20260831T123307Z-52778/go-engine-gate.log.
- Origin: main
- Next step: Appetite: 2h. Triage whether the 7.24s failure is a patience assumption (condition-based waiting fix, the suite-custody pattern) or a real group-termination leak under compression - the test name touches the kill-through/zombie-reap machinery m2 landed at dab1dbd, so verify against that contract before loosening anything; a leak here would be a real defect, not a flake. Prove under artificial load.
- OpenedAt: 2026-08-31T12:34:34Z
- Revision: 1

History:
- 2026-08-31T12:34:34Z EHMH5BQET0RG42HVZKV7C48T4Q-m2-bc1be9cb open actor=m2+mac-coordinator targets=missionrunner-terminate-flake
Integrity: sha256=e2e7cb4e56b3b410f35cd24fd5be43576ed5f620093500efa1b95161f73cb32a
