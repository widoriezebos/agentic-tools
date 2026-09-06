Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal gate-static-tools-predate-go-1-27)
Date: 2026-09-06

# Goal

Goal gate-static-tools-predate-go-1-27 (approved by Wido on 2026-09-06).
Its record, metasystem/plans/goals/gate-static-tools-predate-go-1-27.md,
is the contract. In short: the gate pins staticcheck 2025.1 and
govulncheck v1.1.4 in metasystem/scripts/agents/go-gate.sh (lines 433
and 479); on a Mac whose brew Go is 1.27.1 staticcheck 2025.1 refuses
every package ("export data version 4 is greater than maximum supported
version 2"), so every landing on the Macs refuses at the static
re-proof. The VM runs Go 1.26.5 (Debian 12), the Macs run brew 1.27.1,
go.mod declares go 1.26.5 and no toolchain line.

# Facts (ran on 2026-09-06, m1b)

- `GOTOOLCHAIN=go1.26.5 scripts/agents/go-gate.sh --fast` passes on the
  Mac (the toolchain downloads on first use); two landings shipped under
  that pin. Without it the same command fails on staticcheck as above.
  Seats m1 and m1c verified that go1.26.5 FAILS govulncheck v1.1.4 on
  GO-2026-5026 and four more advisories fixed in go1.26.6, and that
  go1.26.6 passes both pinned tools. Wido's word (2026-09-06): no
  toolchain pin, at best a minimum; the VM was upgraded to 1.27.1 the same hour.
- The gate already records `go env GOTOOLCHAIN` in its environment
  banner (go-gate.sh lines 147 and 152).

# Decisions (the orchestrator's; the floor is Wido's word)

D1. A minimum, never a pin (Wido's word). Every machine in the fleet now
runs Go 1.27.1 (the Macs by brew, the VM upgraded by Wido on
2026-09-06), so go.mod's `go` line moves to 1.27, the version the fleet
proves on; no `toolchain` line is added and none may exist. A machine
below the minimum gets Go's own auto-download, which is the minimum
doing its job, not a pin.

D2. The static tool pins move to the first releases that read Go 1.27
export data while still accepting 1.26: find them (staticcheck's
release notes name the Go versions each release supports; govulncheck
likewise) and pin exactly those versions in go-gate.sh, keeping the
pinned-version comment true. Prove both pins run green on this Mac
under the installed Go 1.27.1 with no GOTOOLCHAIN in the environment.

D3. The gate says which Go it proved: its environment banner already
prints GOTOOLCHAIN; add the effective `go version` line beside it so a
landing's record names the Go that produced it.

D4. Non-goals: no change to the gate's stage order or its fast mode; no
change to landing scripts; nothing under plans.

D5. Pins: the gate's own fixture, if one asserts the pinned versions or
the banner, is updated; otherwise the proof is the gate run itself,
recorded in the return with the exact versions and the `go version`
output on this Mac.

# Gate

From the metasystem directory, WITHOUT any GOTOOLCHAIN in the
environment: `scripts/agents/go-gate.sh --fast` passes; `go run
golang.org/x/vuln/cmd/govulncheck@<pinned> ./...` passes or reports only
findings the return names; `gofmt -l .` prints nothing. Report the
versions chosen and why, with evidence level.

# Constraints

Wall-clock budget: 30 minutes. MECHANICAL reach: the orchestrator's gate
is the examination. Declare the boundary as every file that differs from
main. Gap rule: stop and report a gap with your proposed contract written
out; never fill it silently.
