# codex-jobs-run-unsandboxed-on-a-trusted-host

- State: approved
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="A sandbox mode flag on a launch path every seat uses (exposure 3); the mode exists in Codex and is a one-value change (novelty 1); a wrong value leaves builds blind as today or lifts the sandbox on this trusted host, which Wido accepted on 2026-10-04 until the fleet runs in a VM (severity 2); nothing accumulates (1)."
- Tier: 2
- Intent: On a host the person declares trusted, every Codex job the machinery launches (build, revise, read) runs with --sandbox danger-full-access, so builders and readers can run the repository's tests (Go cache, process listing, loopback, declared static tools); the default keeps today's read-only/workspace-write mapping.
- Origin: main
- Next step: One unit. A per-host setting launch.codex.sandbox (values workspace-write, the default, and danger-full-access) read by the adapter mapping (internal/adapter/codex.go CodexPermissionSettings and BuildCodexCommand, both the fresh-thread and the resume form) and by the unit launcher (internal/launch/codex.go, which hardcodes -s workspace-write); when it says danger-full-access the sandbox flag is danger-full-access for every Codex launch on this host whatever the job's envelope says, and network_access is not passed. Tests: the mapping for both values, the launcher's argv for both values, the resume form, and that the setting is read from the seat's local settings. Docs: the settings reference names the key and Wido's 2026-10-04 risk acceptance (until the fleet runs in a VM). Measured cause: 9 of the 27 review rounds analysed for 2026-10-03/04 were proof-red rounds because the Codex builder could not run the tests inside its sandbox.
- OpenedAt: 2026-10-04T07:27:31Z
- Revision: 3
- Pinned: m1e
- Budget: elapsedLimit=1d attemptLimit=8 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-04T07:28:22Z revision=2 opid=8QQHYQKZEJC8BRWPQ4R8JZNVS1-m1e-718ba0eb authority=proven digest=664bd03b458504827d4fb522fc8f187bbc0f2b8fc38d4f67b2cff431edbbf707 episode=2

History:
- 2026-10-04T07:27:31Z J6KK8P5ZHNKZ51JV7EH1CKD0C1-m1e-718ba0eb open actor=human:Wido targets=codex-jobs-run-unsandboxed-on-a-trusted-host
- 2026-10-04T07:28:22Z 8QQHYQKZEJC8BRWPQ4R8JZNVS1-m1e-718ba0eb approve actor=human:Wido targets=codex-jobs-run-unsandboxed-on-a-trusted-host
- 2026-10-04T07:28:34Z Q60D5NPFGJA40TSFMA742XH2DZ-m1e-718ba0eb set-pin actor=human:Wido targets=codex-jobs-run-unsandboxed-on-a-trusted-host
Integrity: sha256=966d9d7b1d85dd7ad7912485e17c0d16169bdf8606183287fb10eab808c17045
