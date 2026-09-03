Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-03

# Goal

FINISH part one of the tiering machinery. The previous chain
(str-build1) built most of it and was ended by the 120-minute
reservation cap before its return; its tree is preserved on the branch
`preserve/str-build1-r4` (already fetched in this repository; 51 files,
+1390/-399 against main at 12ce45c8). Take that work, complete it, gate
it, return. The contract is unchanged:
metasystem/plans/severity-tiered-rigor-build1-brief.md (the change list
and the four binding test obligations),
metasystem/plans/severity-tiered-rigor-build1-gap-brief.md (the two
configuration keys), metasystem/plans/severity-tiered-rigor-build1-gap2-brief.md
(the classify-sweep contract) and
metasystem/plans/severity-tiered-rigor-build1-gap3-brief.md (the claim
launch path). Design: metasystem/plans/severity-tiered-rigor-design.md
revision 3. Note revision 4 (metasystem/plans/severity-tiered-rigor-p2-design.md,
another seat's part two) will derive the tier from four risk answers;
part one keeps the bare `--tier` and the tier field as revision 3 says,
so part two can build on it. Do not implement revision 4.

# Workspace

The delegate worktree the dispatcher created for this job. First
command, from the repository root:

    git cherry-pick --no-commit preserve/str-build1-r4

This is a three-way merge of the preserved work (base 12ce45c8) onto
main. Main gained one code landing under the same files since that
base: 2c3776b8 "claude-fable-5 now means the latest Fable 5.x at
resolution" (25 files under cmd/, internal/ and scripts/). Expect
conflicts in metasystem/cmd/metasystem/dispatch_verbs.go,
metasystem/internal/dispatch/build.go, cap_test.go and
provenance_test.go: keep BOTH the alias resolution from main and the
goalTier plumbing from the preserved work; resolve, `git add`, and
continue without committing (the dispatcher's conformance reads the
working tree). Never check the branch out over main: main carries
newer records. A conflict you cannot resolve by keeping both sides is
a gap: stop and report it.

# What is known unfinished

`go vet ./...` in that tree fails on callers not yet updated to the
new signatures, at least metasystem/internal/channel/channel_test.go:587
(`goal.RecordedNormApproval` now returns six values) and one caller of
`goalbudget.New` that passes four values where the five-member tuple
wants five. Fix every caller across the module; then run the gate
below and fix what it finds until green. The four binding test
obligations must each have their test.

# Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l . (empty)`;
`go test ./internal/goal/... ./internal/goalbudget/... ./internal/config/... ./internal/dispatch/... ./internal/channel/... -count=1 -timeout 25m`
green; `bash scripts/agents/goal-cli-fixtures.sh` green. Do not run
scripts/agents/path-class-fixtures.sh (ripgrep is absent on this
host).

# Constraints

Wall-clock budget: 90 minutes. Budget your time: the goal package's
tests take about 25 minutes on this Mac; run them ONCE at the end, and
run narrower packages while iterating. Return BEFORE the budget ends
even if a test is still red: a red test named in the return is worth
more than a timeout. DESIGN-BEARING reach. Declare the boundary as
every file that differs from main, with the metasystem/ prefix.

# Expected Return

Per the implementer schema: the boundary; for each of the four
obligations the test that proves it; the two configuration keys; every
gate command with its observed result; anything still red, named.

# Gap Rule

stop and report a gap with your proposed contract written out; never
fill it silently.
