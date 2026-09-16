# Build read: fixture-children unit 3b, round 4 (one static-check fix)

Reader: Opus 5, build-read, the same reader as rounds 2 and 3. I did not write this change.

- **Tree:** `g18/wt-3bonly`, which is trunk d44c98d87 (unit 3a) plus the three 3b files. I only read it; nothing was written there.
- **Scratch:** `R4=/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/1e0f004f-23d9-4653-add1-c3a768ed5661/scratchpad/r4`. All runs are in copies under this directory.
- **Budget:** 8 calls, all used.

## Scope

The only change since round 3 is two comment lines added above `var fixtureCustodianOwnerWatch *os.File`, in `internal/testenv/testenv.go:33-35`:

```go
// The owner retains the sole writer until process exit; closing it is the death notification.
//
//lint:ignore U1000 The write end is held but never read so only process exit tells the custodian the owner died.
```

I checked the scope three ways:
- `diff fcu3b-round3-baseline.diff fcu3b-r3.diff` shows only these two added lines, plus the index hashes and hunk offsets that shift with them.
- `git diff HEAD` in `wt-3bonly` for the two tracked files matches `fcu3b-r3.diff`.
- All three 3b files in `wt-3bonly` are byte-identical to the round-3 tree in `wt-fcu3`, which the fix round also edited.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| N-1 | low | no | The lint reason lacks a comma ("never read, so only"). The wording is otherwise plain and accurate | `testenv.go:35` |
| N-2 | low | no | This is outside the delta and already ruled to unit 5: custodian logs beside the registry homes pile up in `/tmp` | `/tmp` held 58 `metasystem-test-registry-*.custodian.log` files from today's runs across the machine. I removed only the four my own probe runs created |

Material findings: none (no R4 findings).

## Guarantee 1: the write end stays open until process exit

**Held.**

- **Static.** The writer is stored in a package-level variable at `testenv.go:137`. Nothing reads it, closes it or clears it; `grep` finds only the declaration and that one store. A package variable is a garbage-collection root for the whole life of the process. The `*os.File` therefore stays reachable after `m.Run()` returns, while the deferred registry cleanup runs, and through `os.Exit` in `TestMain`. Its finalizer cannot run in that time. The lint comment changes nothing the compiler sees.
- **Behaviour.** I put a probe test in copies, through the identity package's `TestMain` (`os.Exit(testenv.Main(m))`).
  - **What the probe does.** It lists the process's descriptors above 2 that are FIFOs opened write-only. Then it forces six `runtime.GC()` cycles with 50ms sleeps and lists them again. It also checks each such descriptor for `FD_CLOEXEC`. Run with `METASYSTEM_FIXTURE_CUSTODIAN_START=1 go test -race -count=1 -v -run ^TestZZOwnerWatchWriterSurvivesCollection$ ./internal/identity/`.
  - **3b as written** (`$R4/copy`): `writer pipes before=[6] after=[6]`, PASS. After the binary exited, its custodian logged `owner=pid=67480;... action=complete`.
  - **Control** (`$R4/copy2`): the writer is held only in a local variable (`ownerWatch, startErr := startFixtureCustodian(...); _ = ownerWatch`). Result: `before=[6] after=[]`, FAIL, `owner watch writer closed during the run`. So the probe does catch a finalizer closing the writer early. It also shows that the store to the package variable is what keeps the writer open, and that the compiler does not optimise that store away.
  - Output: `$R4/run2.out`, `$R4/probe2-copy.out`, `$R4/probe2-copy2.out`.

## Guarantee 2: no child inherits the write end

**Held.** The delta does not touch the start path; `os.Pipe`, `ExtraFiles` = the reader only, and `Setsid` are unchanged. In the running binary above, the probe's `FD_CLOEXEC` check raised no error on writer fd 6.

## Guarantee 3: nothing else in the start path changed

**Held.** The diff between the before and after 3b diffs is the two comment lines and nothing else, as shown under Scope. The D2 gate, the FIFO and `O_RDONLY` checks, the `<registry>.custodian.log` path with `O_NOFOLLOW`, and `Setsid` are byte-for-byte as I read them in round 3.

## The lint line

- **Plain English:** yes (see N-1 for the comma).
- **Describes the code as it is:** yes. The value is held and never read, and nothing but process exit closes that write end, so its closing is how the custodian learns the owner died. The comment line above says the same.
- **No round, finding or review is named.**
- **It does what it is for.** In a copy with only the two lines removed (`$R4/copy3`), staticcheck v0.8.0 reports `internal/testenv/testenv.go:34:5: var fixtureCustodianOwnerWatch is unused (U1000)`, rc=1. With the lines in place it is clean.

## Commands and results (copy of `wt-3bonly/metasystem`, checked byte-identical for the three 3b files)

- `gofmt -l` on the three 3b files: no output.
- `go vet ./internal/testenv/... ./internal/identity/...`: rc=0.
- `go run honnef.co/go/tools/cmd/staticcheck@v0.8.0 ./internal/testenv/... ./internal/identity/...`: rc=0, no output.
- `go test -race -count=1 ./internal/testenv/... ./internal/identity/...` (probe file removed): testenv `ok 10.605s`, identity `ok 6.791s`, rc=0.
- **Size.** Counted from `fcu3b-r3.diff`, which includes the new witness file:
  - testenv.go +95
  - fixture_custodian_witness_test.go +202
  - testmain_test.go +1/-1

  That is **299** changed lines, at or under the 300 cap.
- **Census.** At 16:46:55, two reparented `identity.test` custodians were still inside their quiet window. At 16:47:02 and 16:47:21 there was no `identity.test`, no custodian and no tagged process. The four custodian logs my probe runs made, all ending in `action=complete`, were removed.

## What I could not check

- **The other fast-gate steps.** I did not run `scripts/agents/go-gate.sh --fast` itself, so its refusal register, SessionStart exit audit and whole-module build are not rechecked; I rely on the seat's `gate-fast-3bonly.out` for those. The step that failed before, staticcheck, I ran myself.
- **Linux.** Finalizer timing and `/dev/fd` there were not checked. The static reachability argument does not depend on the platform.
- **The deep groups** `section/adoption-fixtures` and `section/land-fixtures` were not re-run (seat only).

VERDICT: LAND
