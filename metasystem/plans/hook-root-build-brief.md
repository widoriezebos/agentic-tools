Working Mode: build
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal supervision-hook-wrong-root)
Date: 2026-09-02

# Goal

Build metasystem/plans/supervision-hook-root-design.md (revision 4,
certified by Sol after four rounds; registers
metasystem/records/misc/hook-root-critique-r1.md,
metasystem/records/misc/hook-root-critique-r2.md,
metasystem/records/misc/hook-root-critique-r3.md). Read the whole design
before writing a line; Decisions 1 through 4 decided everything. Zero
judgment calls: a gap is reported, not filled.

# Workspace

The delegate worktree the dispatcher created for this job.

# The change

1. metasystem/scripts/agents/supervision-hook.sh — Decision 1's replacement
   block (root resolution through the engine's executable-anchored answer,
   the linked-worktree rule with git steering variables ignored, the
   installation pairing under a METASYSTEM_BIN override) at the placement
   Decision 1 names; Decision 2's failure shape; every `$repo` consumer row of
   Decision 4 (the engine resolves at the world installation after the
   mapping; the flat layout stays byte-identical).
2. Any engine verb Decision 1 requires that does not exist yet (read the
   design; if it names a verb, build it under metasystem/cmd/metasystem with
   its Go tests).
3. metasystem/scripts/agents/supervision-fixtures.sh — Decision 3's
   scenarios in full (`nested-root` and the worktree and fallback cases),
   each block-asserting case with its own named session id, worlds identified
   by what the hook reads and writes, engines staged by cp.

# Gate

`bash -n` on both scripts; `go build ./...`; `go vet ./...`; `gofmt -l .`
empty; the new fixture scenarios pass and every existing scenario in
metasystem/scripts/agents/supervision-fixtures.sh still passes; `go test
./... -count=1` green. Paste the final lines of each run in the return. Every
fixture wait carries a named ceiling; no benchmarks (R-31).

# Constraints

Wall-clock cap as dispatched. Touch nothing outside the files the design
names. Version-2 implementer JSON; diffBoundary lists every file changed or
created.

# Gap Rule

stop and report a gap; never fill it silently.
