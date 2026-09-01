# missionrunner-terminate-flake

- State: queued
- Intent: TestTerminateGroupLeaksNoGroupsUnderCompression (internal/missionrunner) failed in the first governed weight-discharge validation (run governed-discharge-20260831-c, 2026-08-31) on a quiet-held shared machine where the package still took 1271s - the wall-clock-patience-under-load family (same class as steward-tick-load-flake, which failed twice in the same gate). Evidence: artifacts/agents/suite-failures/20260831T123307Z-52778/go-engine-gate.log.
- Origin: main
- Next step: m0 NATIVE REPRODUCTION (2026-09-01, Debian guest): red 2-of-3 idle at HEAD - not load-dependent here. The refusal texts are identity-proof failures: 'host process group N is not provably ours; leaving it to the census' and 'provably foreign after TERM; skipping the kill of a recycled group' - the wind-down's ownership comparison fails on groups the test itself spawned. LIKELY THE SAME ROOT as vm-epoch-identity-drift (the +-1s start-time epoch instability on clock-disciplined VMs breaks exact identity comparison); whoever takes either goal should read both. This is the only red in m0's full suite (60 packages ok) and predates all of m0's landings, proven by stash-revert.
- OpenedAt: 2026-08-31T12:34:34Z
- Revision: 3

History:
- 2026-08-31T12:34:34Z EHMH5BQET0RG42HVZKV7C48T4Q-m2-bc1be9cb open actor=m2+mac-coordinator targets=missionrunner-terminate-flake
- 2026-08-31T17:28:32Z WW69E2ASKTZZ6D5KMNCPKVN5RK-m2-bc1be9cb edit actor=m2+mac-coordinator targets=missionrunner-terminate-flake
- 2026-09-01T10:05:14Z G4A22GKQ4TPKYZHVQHWJ76ZGBB-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=missionrunner-terminate-flake
Integrity: sha256=5b677ebe16bc6cf453ed7a79f75630e3671e58e7484097c69f12986250822066
