Working Mode: build
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal breach-clock-and-budget-honesty)
Date: 2026-09-02

# Goal

Build metasystem/plans/breach-clock-and-budget-honesty-design.md (the landed
revision, certified by Sol; registers metasystem/records/misc/breach-design-critique-r1.md
and the later rounds' registers in the same directory). Read the whole
design before writing a line. The three Fix sections decided everything and
name every seam by file and line; the proof plan names every test; the
disposition tables and self-grade are context, not instructions. Zero
judgment calls: a gap is reported, not filled. The standard is Wido's,
verbatim: "hard deterministic machinery. This is Go territory enforcing your
behaviour" — no refusal is weakened, no guarantee narrowed to make a test
pass.

# Workspace

The delegate worktree the dispatcher created for this job.

# The change, in this order

1. Fix 1 (the raise-proof breach anchor): the claim binding's episode keys
   (record, render, parse, validation) in metasystem/internal/goal/file.go and
   the grammar; `rebindClaimKeepEpisode` and its one call site in
   metasystem/internal/goal/verbs.go; the projection's episode origin and the
   discharge-proof eligibility rule in metasystem/internal/dispatch/budget.go
   exactly as the design's rule states it (live-obligation filter as shipped;
   the episode obligation key when no obligation is live); the
   CLOCK_REGRESSED wording.
2. Fix 2 (the stored token means what it says): the constructor stores m and
   h verbatim and refuses every d token with the design's exact wording in
   metasystem/internal/goalbudget/budget.go; the formatter retires; the
   legacy reader keeps eight-hour days for stored d; the stop-evidence
   producer copies the stored token; every day-token inventory row converted
   or added exactly as the proof plan classifies it; the documentation row.
3. Fix 3 (a breach parks the goal, never the machine): the state model,
   `ResolveStopAuthority` and the command seam in
   metasystem/cmd/metasystem/goalsync_mutations.go, the cancellation-duty
   route in metasystem/internal/dispatch/stop.go and the steward tick, the
   one-claim rule and every consumer of the claimed set, the parse
   invariants and the hand-edit mapper in metasystem/internal/goal/reconcilemap.go
   (the design's own words for each).
4. Every test and fixture the proof plan names, by its exact name, at the
   seam the plan puts it.

# Gate

`go build ./...`; `go vet ./...`; `gofmt -l .` empty; `bash -n` on every
touched script; `go test ./... -count=1` green; every scenario in
metasystem/scripts/agents/goal-cli-fixtures.sh and
metasystem/scripts/agents/dispatch-fixtures.sh passes, including the rows
the inventory converts or adds. Paste the final lines of each run in the
return. No benchmarks (R-31).

# Constraints

Wall-clock cap as dispatched. Touch nothing the design does not name. Commit
in the worktree as you go so a cap kill leaves the partial work landable.
Version-2 implementer JSON; diffBoundary lists every file changed or created.

# Gap Rule

stop and report a gap; never fill it silently.
