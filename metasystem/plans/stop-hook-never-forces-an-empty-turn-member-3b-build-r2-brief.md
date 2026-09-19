# Fold brief: the fake adapter's stop replay names no other runtime (member 3b, round 2)

DIRECT MODE (Wido via m1e). Working Mode: implement. Model gpt-5.6-sol. Date 2026-09-19.
Goal-Unit: stop-hook-never-forces-an-empty-turn/stop-notice-says-delivery-unconfirmed
Declared size: 40 changed lines

| Unit | Lines |
|---|---|
| stop-notice-says-delivery-unconfirmed | 40 |

Your working directory is the module root `metasystem/` of a detached git worktree that holds round 1's uncommitted change. Bash and Go only. Leave every change uncommitted; never run git add, git apply, git stash, git rebase or git commit. Never edit `memory/receipts.log`, anything under `records/` or `plans/`. Change only `scripts/agents/adapters/fake.sh` and `cmd/metasystem/runtime_conformance_test.go`.

## Start state

First command: `git rev-parse HEAD` must print `1e84763e202245843cca98b1a41265d39f3fb51e`. Second: `git status --porcelain | wc -l` must print `7`. Third: `git diff --stat HEAD | tail -1` must print `7 files changed, 434 insertions(+), 12 deletions(-)`. Paste all three under `## Start state`. If any differs, write `MISMATCH` and what you saw as the first line of the return and stop.

## What changes and why

`TestRuntimeFilePlacement` (`cmd/metasystem/runtime_placement_test.go`) is red on round 1's tree:

```text
--- FAIL: TestRuntimeFilePlacement (0.02s)
    runtime_placement_test.go:86: cross-runtime code in per-runtime files:
        fake.sh:455: [fake file] hook_config="$bed/scripts/enforcement/claude-code-hooks.json"
```

`fake.sh` belongs to the fake runtime, so its code may not name another runtime. The `stop-replay` action reads the bed's installed Stop entry from the Claude hook configuration by a fixed path.

## What to build

1. `stop-replay` gains a required option `--hook-config FILE`: the hook configuration whose Stop entry the replay runs, relative to the bed root or absolute. `stop_replay` reads the Stop command from that file the way it does now, and refuses with a usage error when the option is missing or the file is not readable. The usage text names the option. No line of `fake.sh` names another runtime.
2. `runFakeStopReplay` in `cmd/metasystem/runtime_conformance_test.go` passes `--hook-config scripts/enforcement/claude-code-hooks.json` on every call. Nothing else in the test changes: every case, assertion and name stays.
3. Mutation, once: put the fixed Claude path back in `stop_replay` in place of the option, run only `TestRuntimeFilePlacement`, paste its red line verbatim under `## Mutation`, restore, and show that `git diff --stat HEAD` equals the stat before the mutation.

Source comments say what the code does and why in plain English; never a brief, a round, a finding, a seat, a unit or a goal id.

## Bounds

- Before every `go build`, `go test`, `go vet` or gate run: `test -e /tmp/metasystem-testrun-lock`; while it exists run none of them and check again every 60 seconds. One go test process at a time. No single command may wait longer than 240 seconds: run a longer one in the background with its output to a file and poll the file.
- Every go command runs with this environment set: `GOCACHE=/tmp/stopinc-3b-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-3b-staticcheck`. Never unset or strip it.
- The sandbox refuses kern.proc, `ps`, loopback binds, DNS and `nice`. `TestFakeStopReplay` is red in the sandbox only because process inspection is refused (`sysctl kern.proc.all: operation not permitted`, `command line unverifiable`); paste its red and classify it in one line under `## Sandbox reds`. Any other red of `TestFakeStopReplay`, such as a usage error or exit status 2, is yours to fix. Do not run the whole `cmd/metasystem` package, `fake.sh selftest` or `supervision-hook-fixtures.sh`; the seat runs them on the host.
- If the change passes 80 changed lines, stop and return `BUILD: blocked size N` with the planned shape.

## Required checks (from `metasystem/`, each output pasted verbatim into the return)

1. `gofmt -l ./cmd/metasystem` (must print nothing) and `bash -n scripts/agents/adapters/fake.sh`.
2. `go vet ./cmd/metasystem/` and `go vet -tags batchtest ./cmd/metasystem/`.
3. RUN SET, once plain and once with `-tags batchtest`: `go test -count=1 -run 'TestRuntimeFilePlacement|TestPlacementTokenizerCatchesIdentifiers|TestProofAttempt|TestLandFixture|TestEveryPackageUsesSharedMain|TestBedsResolveStopReportsThroughTheEngine|TestCapabilityDeclarationsMatchRegistrations|TestCapabilityFlagsBackedByExecutables' ./cmd/metasystem/`, then `go test -count=1 -run 'TestFakeStopReplay' ./cmd/metasystem/`, then `go test -count=1 -run 'TestDegradedStopFormsMatchTheGoldenContract|TestDegradedStopRendererIsSideEffectFree' ./internal/hooks/`.
4. `go build ./...` and `go build -tags batchtest ./...`.
5. `go test -count=1 -run 'TestTestEnvironmentStandardInventoryMatchesPackageTests' ./internal/testenv/` and `go test -count=1 -run 'TestRepositoryManifestClassifiesEveryTrackedPath' ./internal/pathclass/`.
6. `go run ./cmd/metasystem audit parallel-ratchet --root .`
7. `git diff --numstat HEAD` and `git status --porcelain`.
8. `bash scripts/agents/go-gate.sh --fast` (background it and poll; paste the whole output).
Every check must be green or a classified sandbox red. A red you cannot make green is returned as `BUILD: blocked` with the red pasted, never hidden.

## Diff and return (shell redirection only, absolute paths)

- Diff: from the worktree root (the parent of `metasystem/`) run `git diff HEAD > RETURNDIR/dm-stopinc-3b.diff`. No file is untracked.
- Return: write RETURNFILE with `cat > RETURNFILE <<'EOF'` style redirection. First line exactly `BUILD: done` or `BUILD: blocked REASON`. Sections in this order: `## Start state`, `## Change` (the new usage line and the one test call line), `## Checks` (each command exactly as run, then its output), `## Mutation`, `## Sandbox reds`, `## Commit message` (round 1's subject and body from `RETURNDIR/stopinc-3b-build-return-r1.md` under its `## Commit message`, plus one body sentence saying the replay takes the hook configuration it runs as an option, so the fake adapter names no other runtime; last line `Goal-Unit: stop-hook-never-forces-an-empty-turn/stop-notice-says-delivery-unconfirmed`). Literal output only; leave no angle-bracket placeholder in the return.

RETURNDIR = /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct
RETURNFILE = /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct/stopinc-3b-build-return-r2.md
