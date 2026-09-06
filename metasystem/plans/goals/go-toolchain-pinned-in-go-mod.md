# go-toolchain-pinned-in-go-mod

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: every landing with Go changes is refused at the static re-proof until someone pins the toolchain by hand, and a builder in a sandbox reports a red gate it did not cause; novelty 1: one line in go.mod and one env line in the gate; exposure 3: every machine on this host and any machine whose package manager moves Go; accumulation 2: each unpinned upgrade repeats the outage"
- Tier: 3
- Intent: The Go gate's pinned staticcheck (2025.1, the frozen version) cannot read export data from a newer Go than the module declares, and go.mod declares only 'go 1.26.5' with no toolchain line, so GOTOOLCHAIN=auto selects whatever Go the host has when it is newer. On 2026-09-06 at 13:03 Homebrew moved this host to Go 1.27.1 and every static re-proof started failing with 'export data version 4 is greater than maximum supported version 2'; a worktree gate ten minutes earlier had passed. The module pins its toolchain (toolchain go1.26.5 in go.mod) so auto selection stays on the declared version on every host, and the gate names the selected toolchain in its output so a mismatch is visible at once. Landed under GOTOOLCHAIN=go1.26.5 by hand: 2026-09-06, the engine re-arm chain.
- Origin: main
- Next step: MECHANICAL, tier-1 lane: add the toolchain line to go.mod (the exact version the module already declares), have go-gate.sh print the toolchain it selected next to its go version line, and a fixture (or the gate's own preflight) that refuses with a plain message when the selected toolchain is newer than what the pinned staticcheck supports. Until it lands every seat runs its gate and landing with GOTOOLCHAIN=go1.26.5 in the environment. Any free seat; ten minutes of work.
- OpenedAt: 2026-09-06T11:16:32Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-06T11:16:32Z 00SNQ8M5N45012DKB5G6KEPNQ1-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=go-toolchain-pinned-in-go-mod
Integrity: sha256=6151a3379736c0db16cebb9eca4c246912391c7668a079b42bb7038a92703f86
