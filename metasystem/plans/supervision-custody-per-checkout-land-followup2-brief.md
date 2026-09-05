Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal supervision-custody-per-checkout-land)
Date: 2026-09-05

# Follow-up: the foreign-tag veto stays unconditional

The second critic's register is in chain scp-build1-cc2 (findings SCC-51 to SCC-54; read its return for the full text). One is material and accepted:

- SCC-51 (high): in `internal/supervise/arming.go`, `ShutdownAt`, the SCC-43 correction gates the whole guard, including the pre-existing veto of a lock whose owner tag lies outside the checkout's prefix, on the owner being provably alive. In the suite's foreign-owner scenario the recorded tag is not an argument of the held process, liveness reads Dead, every check is skipped, and shutdown sweeps a lock armed for another repository; the scenario asserts a refusal that names "another repository". Fix: the foreign-tag veto runs unconditionally, before liveness, exactly as the base behaviour did; only the recorded-checkout-path guard is limited to a live owner. Add the dead-foreign-owner case to the invariant test.

SCC-52 to SCC-54 are noted; no change in this round. Seat-side the supervision suite must be green in operator-layout and foreign-owner again; the census-lifecycle and stop-hook-monitor scenarios are other goals' reds. `gofmt -l`, `go vet`, staticcheck on `./internal/registry/... ./internal/supervise/... ./internal/up/... ./cmd/metasystem/`; `go test` on those four; `bash -n` on both scripts. Return within forty-five minutes. Every path in your return is relative to the repository root (starting with `metasystem/`).
