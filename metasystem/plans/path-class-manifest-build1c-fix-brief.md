Working Mode: build
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Follow-up on chain path-class-build1c: your gap was correct. The
cumulative patch was exported as a working-tree diff and the two list
deletions were staged in the source worktree, so the patch omitted
them. The reviewed tree deletes both files; complete the re-issue.

# The change

1. Delete metasystem/scripts/agents/register-carriage-paths.txt and
   metasystem/scripts/agents/instruction-bearing-paths.txt (git rm). No
   other product byte changes.
2. Run the gate.
3. Declare the boundary as the twenty paths of round 1 plus these two
   deletions, all with the metasystem/ prefix.

# Gate

`go build ./...` clean; `go vet` on internal/pathclass, internal/landing, internal/validate, internal/stateroot, cmd/metasystem; `gofmt -l` empty on those; `go test ./internal/pathclass/ ./internal/landing/ ./internal/validate/ ./internal/stateroot/ ./cmd/metasystem/ -count=1` green (the one process-visibility test the sandbox cannot run is replayed by the orchestrator, KI-15); `bash scripts/agents/path-class-fixtures.sh` green, including its deleted-reader search, which must find no reader of either list.

# Constraints

Wall-clock budget: 25 minutes. DESIGN-BEARING reach. R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the twenty-two paths.

# Gap Rule

stop and report a gap; never fill it silently.
