Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-04

# Goal

Carry slice 2b of the tiering machinery (the material stop and the
close) into a fresh tier-bound chain and raise its test coverage over
the landing gate's floors. The old chain str-p2-build-2c cannot take a
follow-up under the law landed with part three (its root record
predates the goal tier), so this chain starts from main and receives
2b's finished, twice-reviewed diff as a patch. Two steps, in order.

# Step one: apply the carried diff (mechanical, no judgement)

The dispatching seat exported 2b's working tree, already rebased onto
main at parts one and three and fully gated there (Go gate green, goal
package included, staticcheck silent), as one binary patch:
metasystem/artifacts/agents/worktrees/str-p2-build-2c/2b-merged.patch
(it applies cleanly with `git apply --check` on main at 6ec29e8b).
From your worktree's metasystem directory: `git apply --index
<that path>`, then `git reset -q` so nothing stays staged. Confirm
`go build ./... && go vet ./...` are clean. Change nothing in the
applied files except as step two requires (tests). If the patch does
not apply, stop: that is a gap, report the rejected hunks.

# Step two: tests only

The seat measured the landing gate's coverage ratchet
(metasystem/scripts/agents/coverage-ratchet.json) on that tree:
internal/dispatch 75.3% against its floor 75.9%, internal/validate
79.4% against 79.9%. The landing refuses below the floor, so raise
both above it with tests in those two packages. No production change;
a production change is a gap.

Uncovered functions this slice added, measured with `go tool cover
-func`: metasystem/internal/dispatch/finding_register.go
`CritiqueOpenFindingIDs`, `CritiqueRegisterDecisionFinding`,
`criticFindingText`, `CritiqueRegisterAcceptRisk` at 0% (driven only
from cmd/metasystem tests, which do not count for this package),
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
plans or records. diffBoundary: every file that differs from main
(the carried 45 plus your tests). Paste the coverage-delta lines in
your return and list your new test names.

# Constraints

Wall-clock budget: 40 minutes; return by minute 35 with the numbers
whatever they are. Version-2 implementer JSON.

# Gap Rule

stop and report a gap; never fill it silently. A rejected hunk or a
test that needs a production change is the gap to report.
