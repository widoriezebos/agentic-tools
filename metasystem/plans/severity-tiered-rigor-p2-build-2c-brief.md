Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-03

# Goal

FINISH slice 2b of the tiering machinery (the material stop and the
close). The previous chain (str-p2-build-2b) built it in its third
round and was ended by the 45-minute reservation cap while `go test
./...` was running, before its return; its tree is preserved on the
branch `preserve/str-p2-build-2b-r3` in this repository (41 files,
+2009/-507 against main at 6f4bc4dd; vet, build and staticcheck were
green in that tree). Take that work, complete it, gate it, return. The
contract is unchanged: metasystem/plans/severity-tiered-rigor-p2-build-brief-2b.md
(items 1 to 7, the fixture obligations, the diffBoundary),
metasystem/plans/severity-tiered-rigor-p2-build-2b-gap-brief.md (gaps
03, 04, 05) and metasystem/plans/severity-tiered-rigor-p2-build-2b-gap2-brief.md
(out-of-scope write, discharge selection, the register line, and the
grain of the gap rule). Design: metasystem/plans/severity-tiered-rigor-p2-design.md
revisions 4.1 and 4.2 over metasystem/plans/severity-tiered-rigor-design.md
revision 3 part two.

# Workspace

The delegate worktree the dispatcher created for this job. First
command, from the repository root:

    git cherry-pick --no-commit preserve/str-p2-build-2b-r3

Main has gained only plans and records since 6f4bc4dd, so expect no
conflicts; if one appears, keep both sides, `git add`, and continue
without committing (the dispatcher's conformance reads the working
tree). Never check the branch out over main.

# What is known

The preserved tree's own gate run by the dispatching seat after the
cap: `scripts/agents/go-gate.sh --fast` green. The result of
`go test -count=1 -timeout 30m ./...` and of the three fixture
scripts is recorded below under "Gate findings"; fix every red line
there first, then run the gate until green. Every fixture named in
the three briefs must exist as a named test and pass; list the new
test names in the return. Record every mechanical choice the
preserved tree made that you keep or change under `decisions` in the
return (one line each: the choice, the alternative, the reason).

# Gate findings

Run by the dispatching seat on the preserved tree, 2026-09-03 23:30
to 00:05 local:

1. MUST FIX. `go test ./internal/validate` red, six tests, one cause:
   TestMissionAuthorizationIssuance, TestMissionMergeIssuancePreconditions,
   TestMissionMergeRefusesEmptyDiff, TestMissionAuthorizationGuardrailLane,
   TestWardenExhaustionDiscipline (authorization_test.go:85, :215,
   :245, :315, :410) and TestConformanceReviewAndCritiqueMerge
   (conformance_test.go:210) all fail with `conformance failure:
   code-critic chain critic: canonical finding register is missing or
   malformed`. The union in mergeCritique (item 6 of the 2b brief)
   now refuses a critic chain whose register was never written. The
   design's rule is that the register is canonical when it exists;
   decide from the tree what a critic chain WITHOUT a register means
   at merge (the pre-2b fixtures of those tests write findings without
   a register) and record the decision; either the union treats a
   missing register as the pre-2b path for chains dispatched before
   the register existed, or the six fixtures gain a register. Both
   are mechanical; a refusal that breaks every existing test fixture
   is not the design's intent.
2. PRE-EXISTING on main at 6f4bc4dd, do not fix, out of scope: in
   `go test ./internal/goal`,
   TestAuthenticatedChannelApprovalRequiresTheTokenOnce/resume_consumes_the_answer
   (channel_authority_test.go:189) fails with `goal claim carries no
   tuple or --approved-ref`; scripts/agents/dispatch-fixtures.sh
   scenario `dispatch` fails with the same text; scripts/agents/goal-cli-fixtures.sh
   scenarios structured-budget, scope-bounds and archive-and-prune
   fail with `APPROVAL_REQUIRED: open --claim is retired`. All four
   are fallout of landing c285d5a0 (another seat's), reproduced on
   main by the dispatching seat. Name them in the return as
   pre-existing; every OTHER test and scenario must be green.
3. ENVIRONMENTAL: dispatch-fixtures.sh scenario `steward-continuation`
   failed once with `census verdict is stale (age=13s window=10s)`
   while the host was loaded by the goal package's tests; rerun it
   alone; report if it fails again.
4. Green on the preserved tree: gofmt, vet, build, staticcheck,
   return-schema-fixtures.sh, every other package's tests, the
   dispatch-fixtures scenarios mission-runner and adapter-selftest,
   the goal-cli scenarios migration-recovery and labels-and-filtering.

# Gate

`cd metasystem && gofmt -l . (empty) && go vet ./... && go build ./...`;
`GOFLAGS=-buildvcs=false go run honnef.co/go/tools/cmd/staticcheck@2025.1 ./...`
silent; `go test -count=1 -timeout 30m $(go list ./... | grep -v /internal/goal$)`
green except the named pre-existing failures; for internal/goal run
only `go test -count=1 -run 'STR|Accept|Obligation|Discharge' ./internal/goal/`
(the whole package takes 27 minutes on this host, longer than this
round; the dispatching seat ran it on the preserved tree and runs it
again on your tree after the return);
metasystem/scripts/agents/return-schema-fixtures.sh,
dispatch-fixtures.sh and goal-cli-fixtures.sh each green in your
sandbox (report the exact refusal if the sandbox cannot run one). No
benchmarks (R-31), no sleeps (R-35). Leave the work in your working
tree, stage nothing, do not run the commit wrapper. diffBoundary as
the 2b brief. Paste the final gate lines in your return.

# Constraints

Wall-clock budget: 45 minutes. Return by minute 40 whatever the
state: a version-2 implementer JSON with `whatWasDone`, the gate
lines, the test names, and `gaps` empty unless a law-changing gap
stops you (the grain rule of the gap2 brief). A round that ends at
the cap without a return is charged and proves nothing; the return
is the deliverable.

# Gap Rule

stop and report a gap; never fill it silently. The grain of the rule
is fixed in metasystem/plans/severity-tiered-rigor-p2-build-2b-gap2-brief.md:
mechanical choices are recorded under `decisions` and built.
