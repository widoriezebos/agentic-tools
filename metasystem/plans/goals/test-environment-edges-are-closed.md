# test-environment-edges-are-closed

- State: approved
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: each gap is a rare edge of the shared Go test environment, none changes a result today; novelty 1: small follow-ups to go-tests-never-inherit-a-candidate-engine; exposure 2: every Go test package now goes through testenv.Main; accumulation 1: they do not compound"
- Tier: 1
- Intent: Low findings left by the independent reads of go-tests-never-inherit-a-candidate-engine (2026-09-15), none material. (1) internal/testenv/protection_test.go's build-constraint check misses some GOOS names (hurd, nacl, zos) and GOARCH names (amd64p32, armbe, arm64be, mips64p32, mips64p32le, ppc, riscv, s390, sparc, sparc64), and counts a //go:build comment anywhere in the file, which can only cause a false refusal. (2) A registry home that a joined helper child outlives stays until the next Main scans the same root; under the TMPDIR fallback only a run with the same TMPDIR cleans it. (3) A SIGKILL between MkdirTemp and the publishing rename in testenv.go leaves a dot-named staging directory that the scan never matches. DONE: the check derives the known GOOS and GOARCH names from the Go toolchain (go tool dist list) instead of a hand list and reads only a real build constraint line; a stale staging directory and an orphaned joined home are removed by the next Main once no live lock holds them; a test proves each.
- Origin: human
- Next step: Free, tier 1. Build after go-tests-never-inherit-a-candidate-engine lands.
- OpenedAt: 2026-09-15T06:23:47Z
- Revision: 2
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-15T06:23:53Z revision=2 opid=AATRE9N7Q7941KP3WHJZZ8AQ0S-m1e-c6925449 authority=proven digest=ef0453e037f7cc31a67cbfd72ec16adde1707ac6525f8155e591750cc7ae0fb3

History:
- 2026-09-15T06:23:47Z GKX4CJXWVF6MA2F09R54BKWGNQ-m1e-c6925449 open actor=human:Wido targets=test-environment-edges-are-closed
- 2026-09-15T06:23:53Z AATRE9N7Q7941KP3WHJZZ8AQ0S-m1e-c6925449 approve actor=human:Wido targets=test-environment-edges-are-closed
Integrity: sha256=d2106fd63dbd8e685c2375a6ddbd2ed00331a3c7ac7c580628793e194591d595
