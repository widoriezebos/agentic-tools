# claude-critic-sandbox-allows-loopback

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a test a critic cannot run is read instead, the old state; novelty 1: one documented sandbox key; exposure 2: every claude critic reviewing a package with listener tests; accumulation 1: nothing compounds"
- Tier: 2
- Intent: A claude critic's sandboxed shell (goal code-critic-runtime-has-no-shell, landed 07d18614) refuses loopback binding: go test ./internal/adapter fails 13 of 131 tests with 'listen tcp 127.0.0.1:0: bind: operation not permitted' on healthy code, and any package whose tests open a local listener is unreviewable by running it (proof job ccs-proof1, F-1). The installed CLI's sandbox settings schema carries allowLocalBinding, which BuildClaudeSettings never sets. DONE means the critic-set settings set allowLocalBinding true (network egress rule unchanged), a settings test pins it, and a dispatched claude critic runs go test ./internal/adapter green.
- Origin: main
- Next step: MECHANICAL: probe allowLocalBinding with the live-probe recipe (scratchpad ccs-probe2.sh pattern), then brief, build, land, prove through a dispatched critic.
- OpenedAt: 2026-09-06T20:34:21Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T20:34:21Z SXJG2H40EBPBN5WB3J14VK5ETA-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=claude-critic-sandbox-allows-loopback
Integrity: sha256=5df7442d9e175ab1823c5657ce2562bd9dbd6ff7f40d70bba713beae8765dd76
