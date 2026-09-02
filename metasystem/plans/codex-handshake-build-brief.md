Working Mode: build
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal codex-handshake-budget-load-fragile)
Date: 2026-09-02

# Goal

Build metasystem/plans/codex-handshake-design.md (the landed revision,
certified by Sol; registers metasystem/records/misc/codex-handshake-critique-r1.md
and the later rounds' registers in the same directory). Read the whole
design before writing a line; sections 2 and 3 decided everything, section
4 is the file boundary, section 5 the proof. Zero judgment calls: a gap is
reported, not filled. Section 7 carries two facts you must not rediscover.

# Workspace

The delegate worktree the dispatcher created for this job.

# The change

1. Part 1 (section 2): metasystem/internal/adapter/codex.go
   `BuildCodexCommand` gains the `plugins={}` override on both verbs at the
   position D1.2 fixes; the pins in D1.6 in
   metasystem/internal/adapter/runtime_test.go and
   metasystem/internal/adapter/codexcommand_test.go.
2. Part 2 (section 3), in the order the design's section 4 lists: the new
   capability and its selection (adapters' probes, select.go, build.go,
   record.go, the fake probe); the three deadline writers (ownership.go,
   custody.go, the new handshake_progress.go with its verb); the exit
   verdict in adjudicate.go; the custodian enforcement and progress note in
   runtime-common.sh and codex.sh; `await_handshake` and the internal verb in
   dispatch.sh; fake.sh behaviours; the fixture scenarios; the three docs.
3. Every test and fixture named in section 5, by its exact name.

# Gate

`go build ./...`; `go vet ./...`; `gofmt -l .` empty; `bash -n` on every
touched script; `go test ./... -count=1` green; the four handshake fixture
scenarios (`no-signal`, `slow-session`, `exit-before-session`,
`hang-gone-dispatcher`) pass and every existing scenario in
metasystem/scripts/agents/dispatch-fixtures.sh still passes. Paste the final
lines of each run in the return. Every fixture wait carries a named scaled
ceiling; no benchmarks (R-31).

# Constraints

Wall-clock cap as dispatched. Touch nothing outside the files section 4
names. Commit in the worktree as you go so a cap kill leaves the partial
work landable. Version-2 implementer JSON; diffBoundary lists every file
changed or created.

# Gap Rule

stop and report a gap; never fill it silently.
