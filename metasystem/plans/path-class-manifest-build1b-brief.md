Working Mode: build
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Re-issue the path-class manifest's first part as a fresh, correctly
declared chain. The work is done and reviewed three times (chain
path-class-build1: build, two corrections, code reviews
metasystem/records/misc/path-class-manifest-code-critique-r1.md and
-r2.md, a final correction folding the last three points). That chain
cannot close because its fourth round declared its boundary without the
project prefix and omitted one changed file, and the chain law never
lets a later round correct an earlier declaration. So: apply the exact
cumulative diff, change nothing else, run the gates, and declare every
changed path correctly.

# Workspace

The delegate worktree the dispatcher created for this job.

# The change

1. Apply the patch at
   metasystem/artifacts/agents/path-class-build1/cumulative.patch on the
   primary checkout (git apply from the repository root; it is the
   complete diff of the reviewed tree against main). Do not edit any
   product byte beyond what the patch contains; if the patch does not
   apply cleanly, stop and report the gap with the rejected hunks.
2. Run the gate below.
3. Declare the boundary as every path the patch changes, each with the
   metasystem/ prefix, deletions included; the two deleted lists
   (scripts/agents/register-carriage-paths.txt and
   instruction-bearing-paths.txt) count as changed paths.

# Gate

`go build ./...` clean; `go vet` on internal/pathclass, internal/landing, internal/validate, internal/stateroot, cmd/metasystem; `gofmt -l` empty on those; `go test ./internal/pathclass/ ./internal/landing/ ./internal/validate/ ./internal/stateroot/ ./cmd/metasystem/ -count=1` green (the one process-visibility test the sandbox cannot run is replayed by the orchestrator, KI-15); `bash scripts/agents/path-class-fixtures.sh` green.

# Constraints

Wall-clock budget: 25 minutes. DESIGN-BEARING reach (the chain's class; the work is reviewed). R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly every path the patch
changes, metasystem/ prefixed.

# Gap Rule

stop and report a gap; never fill it silently.
