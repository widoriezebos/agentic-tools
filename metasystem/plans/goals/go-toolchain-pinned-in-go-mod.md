# go-toolchain-pinned-in-go-mod

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: every landing with Go changes is refused at the static re-proof until someone pins the toolchain by hand, and a builder in a sandbox reports a red gate it did not cause; novelty 1: one line in go.mod and one env line in the gate; exposure 3: every machine on this host and any machine whose package manager moves Go; accumulation 2: each unpinned upgrade repeats the outage"
- Tier: 3
- Intent: The Go gate's pinned staticcheck (2025.1, the frozen version) cannot read export data from a newer Go than the module declares, and go.mod declares only 'go 1.26.5' with no toolchain line, so GOTOOLCHAIN=auto selects whatever Go the host has when it is newer. On 2026-09-06 at 13:03 Homebrew moved this host to Go 1.27.1 and every static re-proof started failing with 'export data version 4 is greater than maximum supported version 2'; a worktree gate ten minutes earlier had passed. The module pins its toolchain (toolchain go1.26.5 in go.mod) so auto selection stays on the declared version on every host, and the gate names the selected toolchain in its output so a mismatch is visible at once. Landed under GOTOOLCHAIN=go1.26.5 by hand: 2026-09-06, the engine re-arm chain.
- Origin: main
- Next step: MECHANICAL, tier-1 lane: add the toolchain line to go.mod (the exact version the module already declares), have go-gate.sh print the toolchain it selected next to its go version line, and a fixture (or the gate's own preflight) that refuses with a plain message when the selected toolchain is newer than what the pinned staticcheck supports. Until it lands every seat runs its gate and landing with GOTOOLCHAIN=go1.26.5 in the environment. Any free seat; ten minutes of work. Fact from m1c (2026-09-06): pin go1.26.6, not go1.26.5: 1.26.5 fails govulncheck v1.1.4 on five standard-library findings fixed in 1.26.6 (GO-2026-5026 among them), so a 1.26.5 pin moves the full gate's red from staticcheck to govulncheck; go1.26.6 passes both pins, verified on m1 with GOTOOLCHAIN=go1.26.6. The follow-on that moves the pins and the floor to 1.27 is goal gate-static-tools-predate-go-1-27.
- OpenedAt: 2026-09-06T11:16:32Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-06T11:16:32Z 00SNQ8M5N45012DKB5G6KEPNQ1-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=go-toolchain-pinned-in-go-mod
- 2026-09-06T11:19:59Z H4ZGPVMECCKDPPNQY7ETY5B22P-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=go-toolchain-pinned-in-go-mod
Integrity: sha256=37d8fb2cf60ff0e63cef2136e7b861c01d78314881c90a95d340d79aec11dc0b
