# go-toolchain-pinned-in-go-mod

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: every landing with Go changes is refused at the static re-proof until someone pins the toolchain by hand, and a builder in a sandbox reports a red gate it did not cause; novelty 1: one line in go.mod and one env line in the gate; exposure 3: every machine on this host and any machine whose package manager moves Go; accumulation 2: each unpinned upgrade repeats the outage"
- Tier: 3
- Intent: The Go gate's pinned staticcheck (2025.1, the frozen version) cannot read export data from a newer Go than the module declares, and go.mod declares only 'go 1.26.5' with no toolchain line, so GOTOOLCHAIN=auto selects whatever Go the host has when it is newer. On 2026-09-06 at 13:03 Homebrew moved this host to Go 1.27.1 and every static re-proof started failing with 'export data version 4 is greater than maximum supported version 2'; a worktree gate ten minutes earlier had passed. The module pins its toolchain (toolchain go1.26.5 in go.mod) so auto selection stays on the declared version on every host, and the gate names the selected toolchain in its output so a mismatch is visible at once. Landed under GOTOOLCHAIN=go1.26.5 by hand: 2026-09-06, the engine re-arm chain.
- Origin: main
- Next step: SUPERSEDED - DO NOT APPROVE, DO NOT CLAIM. Wido's word (2026-09-06, to m1b, relayed): no toolchain pin in go.mod, at best a minimum; he is moving the VM to Go 1.27 as well. The fix is the approved goal gate-static-tools-predate-go-1-27 (m1b claims it): the gate's staticcheck and govulncheck pins move to releases that read Go 1.27 export data, the go.mod go line stays a floor. Stopgap for today only: GOTOOLCHAIN=go1.26.6 in the environment of gates and landings (1.26.6, not 1.26.5: m1c verified 1.26.5 fails govulncheck on GO-2026-5026 and four more, all fixed in 1.26.6). Conclude this record as a duplicate when that goal lands.
- OpenedAt: 2026-09-06T11:16:32Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-06T11:16:32Z 00SNQ8M5N45012DKB5G6KEPNQ1-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=go-toolchain-pinned-in-go-mod
- 2026-09-06T11:19:59Z H4ZGPVMECCKDPPNQY7ETY5B22P-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=go-toolchain-pinned-in-go-mod
- 2026-09-06T11:21:00Z ZZ7QZ1XFGY46RT1B9CXE17B1H1-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=go-toolchain-pinned-in-go-mod
Integrity: sha256=de1f666f0e7781dced2749aff9b7ad6a43ae6a3ea7eeff5bbe0b9677683ecec7
