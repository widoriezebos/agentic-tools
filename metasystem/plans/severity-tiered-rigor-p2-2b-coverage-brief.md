Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-04

# Goal

Tests only, before the closing review of slice 2b (chain
str-p2-build-2c, your round-3 tree, which the seat has since rebased
onto main at parts one and three; the merge kept both sides and
builds green). The dispatching seat measured the landing gate's
coverage ratchet on that merged tree
(metasystem/scripts/agents/coverage-ratchet.json): internal/dispatch
75.3% against its floor 75.9%, internal/validate 79.4% against 79.9%.
The landing refuses below the floor, so this round raises both above
it with tests in those two packages. No production change; a
production change is a gap.

# What is uncovered

In-package coverage of functions this slice added, measured by the
seat with `go tool cover -func` on your tree:
metasystem/internal/dispatch/finding_register.go `CritiqueOpenFindingIDs`,
`CritiqueRegisterDecisionFinding`, `criticFindingText`,
`CritiqueRegisterAcceptRisk` at 0% (they are driven only from
cmd/metasystem tests, which do not count for this package),
`findingRegisterRound` 55.6%; metasystem/internal/dispatch/critique.go
`CritiqueExhaustionAdvance` 50%, `exhaustions` 53.3%;
metasystem/internal/validate/critiqueclosed.go `CritiqueClosedWithRegister`
0%, metasystem/internal/validate/conformance.go `resolveFacts` 58.1%.
Cover the 0% functions first, through their package-level entry
points with a register on disk (the existing finding_register_test.go
fixtures show the shape), then the refusal branches of the others.

# Gate

`cd metasystem && gofmt -l internal/dispatch internal/validate` (empty)
`&& go vet ./internal/dispatch/ ./internal/validate/`;
`bash scripts/agents/coverage-delta.sh ./internal/dispatch ./internal/validate`
prints both packages at or above their floors; `go test -count=1
./internal/dispatch/ ./internal/validate/` green (name any sandbox-only
red exactly, the seat reruns). Stage nothing, no commit wrapper, no
plans or records. diffBoundary: every file that differs from main.
Paste the coverage-delta lines in your return.

# Constraints

Wall-clock budget: 30 minutes; return by minute 25 with the numbers
whatever they are. Version-2 implementer JSON with the test names.

# Gap Rule

stop and report a gap; never fill it silently. A test that needs a
production change to be written is the gap to report.
