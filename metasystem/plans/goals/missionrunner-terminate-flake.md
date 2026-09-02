# missionrunner-terminate-flake

- State: claimed
- Intent: TestTerminateGroupLeaksNoGroupsUnderCompression (internal/missionrunner) failed in the first governed weight-discharge validation (run governed-discharge-20260831-c, 2026-08-31) on a quiet-held shared machine where the package still took 1271s - the wall-clock-patience-under-load family (same class as steward-tick-load-flake, which failed twice in the same gate). Evidence: artifacts/agents/suite-failures/20260831T123307Z-52778/go-engine-gate.log.
- Origin: main
- Next step: SECOND SIBLING (m0, 2026-09-01): TestTerminateGroupKillsThroughATermImmuneOwnedGroup fails at HEAD on the guest too - the family flaps between tests, same identity-proof refusal shape. Also observed: internal/supervise TestTakeoverRefusalNamesTheRecordedComponent flapped once under full-suite parallelism and passes 3/3 in isolation - likely cross-package process-group interference from this family's kills. Root-cause lead unchanged: vm-epoch-identity-drift.
- OpenedAt: 2026-08-31T12:34:34Z
- Revision: 6
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1
- Claimed: machine=m0b lineage=main-1788250419-3170380-8a1fb3 at=2026-09-02T00:14:12Z revision=6
- StopCapability: generation=6 revision=6 machine=m0b claimEpoch=1 fenceEpoch=0

History:
- 2026-08-31T12:34:34Z EHMH5BQET0RG42HVZKV7C48T4Q-m2-bc1be9cb open actor=m2+mac-coordinator targets=missionrunner-terminate-flake
- 2026-08-31T17:28:32Z WW69E2ASKTZZ6D5KMNCPKVN5RK-m2-bc1be9cb edit actor=m2+mac-coordinator targets=missionrunner-terminate-flake
- 2026-09-01T10:05:14Z G4A22GKQ4TPKYZHVQHWJ76ZGBB-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=missionrunner-terminate-flake
- 2026-09-01T12:38:25Z 1SDECS03JNWQR6CBSDKJ47FQK8-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=missionrunner-terminate-flake
- 2026-09-01T20:27:06Z RF8SR6Z3ZWWK7B9ZCRPNE7A2ZV-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=missionrunner-terminate-flake
- 2026-09-02T00:14:12Z X2Z08TN6C8KVNK1EMQBNRVFQGS-m0b-6638932d claim actor=m0b+main-1788250419-3170380-8a1fb3 targets=missionrunner-terminate-flake
Integrity: sha256=b15d5fa1a9fdf789a12a47d99c57fd0c58cfa1774921379fd4a771ad8accc3b3
