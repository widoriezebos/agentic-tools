Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-04

# Goal

Carry slice 2b of the tiering machinery (the material stop and the
close) into a fresh tier-bound chain and raise the goal package's test
coverage over the landing gate's floor. The previous chain
str-p2-build-2d closed on a clean review, but the landing's commit gate
refused its diff: internal/goal measured 79.6% against the ratchet
floor 80.0% (metasystem/scripts/agents/coverage-ratchet.json); every
other package is above floor. A closed chain takes no follow-up, so
this chain starts from main and receives 2d's finished, reviewed diff
as a patch. Two steps, in order.

# Step one: apply the carried diff (mechanical, no judgement)

The certified diff of chain str-p2-build-2d, round 2:
metasystem/artifacts/agents/str-p2-build-2d/rounds/2/diff.patch (46
files; it applies cleanly with `git apply --check` on main at
cfb1b3f7). From your worktree's metasystem directory: `git apply
--index <that path>`, then `git reset -q` so nothing stays staged.
Confirm `go build ./... && go vet ./...` are clean. Change nothing in
the applied files except as step two requires (tests). If the patch
does not apply, stop: that is a gap, report the rejected hunks.

# Step two: tests only, in internal/goal

Raise metasystem/internal/goal to 80.5% or better with tests in that
package. No production change; a production change is a gap.

The seat measured the carried tree with `go tool cover -func`:
internal/goal is 5462 of 6861 statements (79.6%); 80.5% needs about
62 more covered statements. The functions of the two goal files this
slice changed (file.go, verbs.go) that are below 100% are listed,
lowest first, in
metasystem/artifacts/agents/worktrees/str-p2-build-2d/goal-uncovered.txt.
This slice's own additions are the 0% rows `AcceptedRiskDecision`,
`AcceptedRiskDecisionOpID` (verbs.go) and `SortIds` (file.go), plus
`DischargeReviewObligation` at 71.4% and `DeferFindings` at 73.9%:
cover those first through their package-level entry points with a
goal file on disk under a temp root (severity_tiered_rigor_test.go in
the carried diff and file_test.go show the shape), then take the
refusal branches of the other listed functions until the count is
reached. `SetBudget` at 0% is a thin wrapper; one call covers it.

The whole goal package takes 17 minutes on this host; do not run it in
full. Measure with `go test -cover -run '<your new tests>'
./internal/goal/` and `go tool cover -func` on that profile to see the
statements your tests cover; the seat runs the full package after
your return and lands only at or above the floor.

# Gate

`cd metasystem && gofmt -l internal/goal` (empty) `&& go vet
./internal/goal/`; `go test -count=1 -run '<your new tests>'
./internal/goal/` green. Stage nothing, no commit wrapper, no plans or
records. diffBoundary: every file that differs from main (the carried
46 plus your tests). List your new test names and the functions each
one covers in your return.

# Constraints

Wall-clock budget: 40 minutes; return by minute 35 whatever the
state. Version-2 implementer JSON.

# Gap Rule

stop and report a gap; never fill it silently. A rejected hunk or a
test that needs a production change is the gap to report.
