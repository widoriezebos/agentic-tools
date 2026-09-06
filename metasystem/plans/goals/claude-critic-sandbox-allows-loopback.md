# claude-critic-sandbox-allows-loopback

- State: claimed
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a test a critic cannot run is read instead, the old state; novelty 1: one documented sandbox key; exposure 2: every claude critic reviewing a package with listener tests; accumulation 1: nothing compounds"
- Tier: 2
- Intent: A claude critic's sandboxed shell (goal code-critic-runtime-has-no-shell, landed 07d18614) refuses loopback binding: go test ./internal/adapter fails 13 of 131 tests with 'listen tcp 127.0.0.1:0: bind: operation not permitted' on healthy code, and any package whose tests open a local listener is unreviewable by running it (proof job ccs-proof1, F-1). The installed CLI's sandbox settings schema carries allowLocalBinding, which BuildClaudeSettings never sets. DONE means the critic-set settings set allowLocalBinding true (network egress rule unchanged), a settings test pins it, and a dispatched claude critic runs go test ./internal/adapter green.
- Origin: main
- Next step: MECHANICAL: probe allowLocalBinding with the live-probe recipe (scratchpad ccs-probe2.sh pattern), then brief, build, land, prove through a dispatched critic.
- OpenedAt: 2026-09-06T20:34:21Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T21:18:46Z revision=2 opid=10S4262XEJBA9XC00VBMTQ7WE6-m1-7cd0bd60 authority=proven digest=db94a931ce0a0ad26242c03b8f25dcfce3553b24e1d124ef1df8c804bb443c86
- Claimed: machine=m1c lineage=main-1788680061-17829-64951c at=2026-09-06T22:19:21Z revision=3 accountingRevision=3
- StopCapability: generation=3 revision=3 machine=m1c claimEpoch=1 fenceEpoch=0

History:
- 2026-09-06T20:34:21Z SXJG2H40EBPBN5WB3J14VK5ETA-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=claude-critic-sandbox-allows-loopback
- 2026-09-06T21:18:46Z 10S4262XEJBA9XC00VBMTQ7WE6-m1-7cd0bd60 approve actor=human:Wido targets=adopt-bed-authority-probe-passes-tier,adopt-bed-gate-unreachable-under-load,claude-critic-sandbox-allows-loopback,claude-critic-shell-network-deny,claude-delegate-scratch-cleanup,claude-implementer-read-roots-writable
- 2026-09-06T22:19:21Z AX8KY91Z55FPA2YY795SCE7ZJ0-m1c-7cd0bd60 claim actor=m1c+main-1788680061-17829-64951c targets=claude-critic-sandbox-allows-loopback
Integrity: sha256=b81f110de7f8c2b29443a3533da48705d685e364164d57869af07efdddebb9c3
