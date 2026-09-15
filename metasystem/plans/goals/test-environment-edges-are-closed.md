# test-environment-edges-are-closed

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: each gap is a rare edge of the shared Go test environment, none changes a result today; novelty 1: small follow-ups to go-tests-never-inherit-a-candidate-engine; exposure 2: every Go test package now goes through testenv.Main; accumulation 1: they do not compound"
- Tier: 1
- Intent: Low findings left by the independent reads of go-tests-never-inherit-a-candidate-engine (2026-09-15), none material. (1) internal/testenv/protection_test.go's build-constraint check misses some GOOS names (hurd, nacl, zos) and GOARCH names (amd64p32, armbe, arm64be, mips64p32, mips64p32le, ppc, riscv, s390, sparc, sparc64), and counts a //go:build comment anywhere in the file, which can only cause a false refusal. (2) A registry home that a joined helper child outlives stays until the next Main scans the same root; under the TMPDIR fallback only a run with the same TMPDIR cleans it. (3) A SIGKILL between MkdirTemp and the publishing rename in testenv.go leaves a dot-named staging directory that the scan never matches. DONE: the check derives the known GOOS and GOARCH names from the Go toolchain (go tool dist list) instead of a hand list and reads only a real build constraint line; a stale staging directory and an orphaned joined home are removed by the next Main once no live lock holds them; a test proves each.
- Origin: human
- Next step: Free, tier 1. Build after go-tests-never-inherit-a-candidate-engine lands.
- OpenedAt: 2026-09-15T06:23:47Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-15T06:23:47Z GKX4CJXWVF6MA2F09R54BKWGNQ-m1e-c6925449 open actor=human:Wido targets=test-environment-edges-are-closed
Integrity: sha256=2117ef9903a2d6c56899c15f3e71c3d13453473c7a7053760ac1c9db1c25e176
