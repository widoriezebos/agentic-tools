Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, follow-up round under goal gate-static-tools-predate-go-1-27)
Date: 2026-09-06

# Correction round 2 for chain gate-go127-build1

Round 1 stopped on two gaps in the orchestrator's brief, both real and
both now closed by the orchestrator's own runs on this Mac (Go 1.27.1,
no GOTOOLCHAIN in the environment): govulncheck v1.1.4 scans the module
successfully ("No vulnerabilities found"), so its pin does not move;
and the lines the brief called a banner (go-gate.sh around lines 147
and 152) feed the toolchain identity hash and print nothing. The
decisions of metasystem/plans/gate-static-tools-go-1-27-build-brief.md
are amended as follows; everything else in it stands.

D2 (amended). Only staticcheck moves: pin it to Staticcheck 2026.2
(module honnef.co/go/tools v0.8.0), the first release that reads Go
1.27 export data while keeping Go 1.26 compatibility, in the exact
`go run ...@<tag>` form the gate uses today, and keep the pinned-version
comment true (name the release and the module version). govulncheck
stays at v1.1.4; its comment gains one sentence saying it was proven
under Go 1.27.1 on 2026-09-06.

D3 (amended). Before the static stages, the gate prints two
human-visible lines: `go gate: effective Go: <output of go version>`
and `go gate: GOTOOLCHAIN: <output of go env GOTOOLCHAIN>`. The hash
inputs stay as they are.

D1 stands: go.mod's `go` line becomes 1.27, no `toolchain` line.

# Gate

From the metasystem directory, with NO GOTOOLCHAIN in the environment:
`scripts/agents/go-gate.sh --fast` passes and prints the two new lines;
`gofmt -l .` prints nothing; `go build ./...` passes. Report each with
its evidence level.

# Constraints

Wall-clock budget: 20 minutes. MECHANICAL reach. Declare the boundary
as every file that differs from main. Gap rule: stop and report a gap
with your proposed contract written out; never fill it silently.
