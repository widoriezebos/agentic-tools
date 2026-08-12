# Linux Portability

Investigation ledger and implementation brief, 2026-08-11. Scope: what it takes to build and run the `metasystem` Go engine on Linux, and whether one Linux build can be trusted on other Linux distributions.

This file is written to be read on its own. An agent picking up the port should need nothing from the conversation that produced it. Evidence for every claim is marked **ran it**, **read it**, or **inferred it**, per `docs/collaboration.md`. "Inferred it" means standard, documented platform behaviour that was reasoned about but not executed against this codebase — treat those as the parts most worth verifying first.

## Verdict

**One Linux build does work on every Linux distribution, provided cgo stays off.** Built with `CGO_ENABLED=0`, a Go program is a single statically linked executable with no C library dependency, and runs unchanged on Debian, Ubuntu, Red Hat, Alpine, distroless and `scratch`. This module qualifies today. What is missing is that nothing *pins* the flag, so the property is accidental rather than enforced.

**The engine does not compile for Linux at all.** Process identity is implemented only for macOS. That is the real work, and it is smaller than it first appears: **two new files and one changed build tag.**

**The hard-looking design question turned out to be a non-issue.** Linux reports process start time at roughly 10 millisecond resolution where macOS gives microseconds — but every decision path in this codebase truncates to whole seconds before comparing. Detail and evidence in "The resolution question" below. No escalation is needed; one comment needs updating and one diagnostic field changes precision.

## Background: what makes a Go binary portable across Linux

The deciding flag is `CGO_ENABLED` (inferred it — standard Go behaviour).

With `CGO_ENABLED=0` the runtime issues system calls directly. Output is one static file depending on nothing in the host image beyond a compatible kernel and processor architecture. The distribution stops mattering.

With `CGO_ENABLED=1` — the default whenever a C compiler is present on the build machine — the binary links dynamically against the system C library, and two failures follow:

- **C library version skew.** A binary built against a newer GNU C library fails to start on an older one, reporting a missing `GLIBC_2.xx` symbol. Compatibility runs forward only, so the build host must be no newer than the oldest deployment target.
- **musl systems.** Alpine ships no GNU C library, so a dynamically linked binary does not start at all.

**This module is cgo-free today** (ran it): no file contains `import "C"`, and `go.mod` requires only `github.com/pelletier/go-toml/v2` v2.4.3 and `golang.org/x/sys` v0.47.0, both pure Go. `golang.org/x/sys` wraps system calls; it is not a C binding.

**One latent trap.** Four files import the standard `net` package — `internal/adapter/adapter_fake.go`, `internal/adapter/adapter_selftest.go` and their two tests (ran it). With cgo enabled, `net` can select the C library's name resolver and quietly produce a dynamically linked binary even though the project has no cgo of its own (inferred it). Pinning the flag removes the possibility entirely.

**Neither build path pins it** (read it): `scripts/adopt.sh:240` and `scripts/agents/go-gate.sh:47` both call `go build` with the environment default.

## Current state: the build failure

```
$ CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./...
# github.com/widoriezebos/agentic-tools/metasystem/internal/identity
internal/identity/custodian.go:25:24: undefined: KernelProber
```

Building each package separately shows **13 packages fail** (ran it), every one reporting this same error: `internal/identity` sits below all of them, so a single missing type stops the module. `internal/identity/custodian.go:25` calls `(KernelProber{}).Probe(pid)` from a file with no build tag, while the type exists only in a Darwin-gated file.

Three files carry `//go:build darwin` (ran it):

- `internal/identity/identity_darwin.go`
- `internal/identity/enumerate_darwin.go`
- `internal/census/ancestor_production.go`

Plus one Darwin-gated test, `internal/identity/enumerate_darwin_test.go`.

## The port surface

### 1. `internal/identity/identity_linux.go` — new file

Implements `KernelProber` and its `Probe` method. The interface it must satisfy is in `internal/identity/identity.go:61`:

```go
Probe(pid int64) (Exact, Liveness, error)
```

`Liveness` is deliberately three-way (`identity.go:38`): `Alive`, `Dead`, `Unknown`. The package doc is explicit that **only a successful read proving absence is `Dead`; a failed read is `Unknown`, and `Unknown` never authorizes anything**. Preserving that distinction is the single most important correctness property in this port.

The Darwin behaviour to mirror, read from `identity_darwin.go:28-61`:

| Condition | Return |
|---|---|
| `pid < 1` | `Exact{}, Unknown, error` |
| Kernel says no such process (`ESRCH`/`ENOENT`) | `Exact{}, Dead, nil` |
| Any other read error | `Exact{}, Unknown, error` |
| Empty result from a successful call | `Exact{}, Dead, nil` |
| Result too short to parse | `Exact{}, Unknown, error` |
| Implausible start time | `Exact{}, Unknown, error` |
| Success | `Exact{Pid, StartedAt, Argv}, Alive, nil` |

Argv is **best-effort**: `identity_darwin.go:53-59` notes that a process whose argv cannot be read is still alive, and that the kill decision separately demands a readable argv (REG-6). So an argv read failure must not downgrade liveness.

**Linux implementation** (inferred it):

- Read `/proc/<pid>/stat`. Open failing with `ENOENT` is the definitive negative → `Dead`. `EACCES`/`EPERM` or any other error → `Unknown`.
- Start time is **field 22** (`starttime`), in clock ticks since boot.
- **Parsing trap, do not skip this.** Field 2 is the executable name in parentheses and *may itself contain spaces and closing parentheses*. Naive whitespace splitting corrupts every later field. Parse by locating the **last** `)` in the line; the text after it begins at field 3. Therefore, splitting that remainder on whitespace: `remainder[0]` is field 3, `remainder[1]` is field 4 (`ppid`), and `remainder[19]` is field 22 (`starttime`).
- Convert to wall clock: read `btime` (boot time, epoch seconds) from `/proc/stat`, then `StartedAt = time.Unix(btime, 0) + starttime_ticks * (time.Second / USER_HZ)`.
- `USER_HZ` is **100** on all mainstream Linux architectures. It is the userspace ABI constant and is independent of the kernel's internal `CONFIG_HZ`. There is no cgo-free `sysconf` in the standard library, so define it as a named constant with a comment rather than adding a dependency.
- Argv comes from `/proc/<pid>/cmdline`: NUL-separated, usually with a trailing NUL. Split on NUL and drop the trailing empty element. An empty file (kernel threads, zombies) is an argv read failure, not a liveness failure.

Two semantics worth deciding explicitly and recording in the code:

- **Zombies.** A zombie process still has a `/proc/<pid>` entry, so the above reports it `Alive`. Darwin's `kern.proc.pid` likewise returns zombie processes, and the current code does not inspect process state — so *not* filtering zombies is the behaviour-preserving choice. Match Darwin; do not add a state check without a reason.
- **`hidepid` mount option.** If `/proc` is mounted `hidepid=2`, another user's process is invisible and its `stat` open fails `ENOENT` — indistinguishable from a dead process, which would wrongly read as `Dead` rather than `Unknown`. This is the one place the three-way guarantee can silently break on Linux. Standard container and desktop mounts do not use it; note the limitation in the file and consider detecting it once at startup.

### 2. `internal/identity/enumerate_linux.go` — new file

Three exported functions, with these callers (ran it):

| Function | Called from |
|---|---|
| `AllPids() ([]int64, error)` | `internal/missionrunner/proc.go:79`, `internal/lease/sweep.go:127`, `internal/census/production.go:26` |
| `ProcessCwd(pid int64) (string, bool)` | `internal/census/production.go:61` |
| `ParentPid(pid int64) (int64, bool)` | `internal/validate` (test doubles only) |

Semantics to mirror, read from `enumerate_darwin.go`:

- **`AllPids`** returns every process id on the machine. On Linux: read the `/proc` directory and keep entries whose names are entirely numeric. This includes kernel threads, matching Darwin's `kern.proc.all`, which also returns everything. Kernel threads are filtered downstream by `nativeProcTree.Info`, which rejects an empty command (`ancestor_production.go:29`), so no extra filtering belongs here.
- **`ProcessCwd`** returns the working directory, with `ok=false` when it cannot be read — the census treats that as a denial, not as absence (`enumerate_darwin.go:48-49`). On Linux: `os.Readlink("/proc/<pid>/cwd")`. Reading another user's process fails with `EACCES` (inferred it), which correctly yields `ok=false`.
- **`ParentPid`** returns the parent pid, with `ok=false` when the pid cannot be read **or when there is no distinct live ancestor** — specifically `ppid <= 0` or `ppid == pid` (`enumerate_darwin.go:96,118-120`). The caller stops its ancestry walk on false. On Linux: field 4 of `/proc/<pid>/stat`, parsed with the same last-parenthesis rule. Note pid 1 has `ppid == 0`, which correctly returns false.

### 3. `internal/census/ancestor_production.go` — change the build tag only

This file is gated `//go:build darwin` but **contains nothing Darwin-specific** (read it). Its dependencies are:

- `identity.KernelProber{}.Probe` — exists on Linux once step 1 lands
- `identity.ParentPid` — exists on Linux once step 2 lands
- `unix.Getpgid` — available on Linux
- `kernelProbe(pid)` at line 69 — defined in `internal/census/run.go:577`, which has **no build tag** (ran it), so it is already portable

So the fix is to widen or delete the tag, then confirm it compiles. **No `ancestor_linux.go` is needed.** This failure is currently invisible to the compiler because `internal/identity` fails first; `cmd/metasystem/census.go:195` calls `FindAncestorProduction` from portable code, so it will surface as soon as identity builds.

If the tag is deleted rather than widened, consider renaming the file, since `_production` reads oddly next to a `production.go` that means something else in this package. That is cosmetic — do not let it expand scope.

## The resolution question, and why it is not a blocker

`identity.go:8-12` says the package reads "exact kernel start times (microseconds on darwin)" to shrink the residual that the whole-second shell helpers leave. `identity_darwin.go:17` calls microsecond resolution "the exactness the Go ruling exists for". That framing suggests the port has a contract problem, because Linux `/proc` reports start time in 10 millisecond ticks.

**It does not.** Every consumer truncates to whole seconds before comparing (ran it — grep of all non-test `StartedAt` uses):

- `identity.AliveRef` (`identity.go:81`) compares `exact.StartedAt.Unix() != ref.StartedAtSec`. `Ref.StartedAtSec` is seconds by definition (`identity.go:22`).
- `identity.Custodian` (`custodian.go`) compares the fixture's `pidStartedAt`, also seconds.
- `cmd/metasystem/identity.go:24` prints `.Unix()`. `internal/missionrunner/proc.go:26` returns `.Unix()`. `cmd/metasystem/supervise_arming.go:406` records `.Unix()`.

Linux's 10 millisecond resolution is **100 times finer than the one-second resolution every decision actually uses**. The only sub-second consumer anywhere is diagnostic output: `cmd/metasystem/identity.go:42-44` emits a microsecond-formatted timestamp and a `startedAtUnixMicro` field. On Linux those values will be quantized to 10 millisecond steps. Nothing compares them.

**Recommendation:** implement against `/proc`, accept the resolution, and update the two comments (`identity.go:10`, `identity_darwin.go:17`) to state the per-platform resolution honestly. No human ruling required.

One caveat to record while implementing: the absolute anchor `btime` is itself a whole-second value, so Linux start times carry a constant offset error against true wall clock (inferred it). It is *constant per machine*, and — see below — every reader in this system goes through the same code, so comparisons stay self-consistent. It would only matter if something outside this binary computed a start time independently.

### There is only one clock, and it is this binary

Worth stating because it removes an obvious worry. The shell layer does **not** compute start times itself: it shells out to the Go binary, `metasystem identity started-at --pid <pid>` (ran it — `scripts/agents/arm-supervision.sh:214,376`, `scripts/agents/mission-fixtures.sh:264-265,497`, `scripts/agents/second-session.sh:39`, `scripts/agents/flight-recorder-fixtures.sh:137`, `scripts/validate-metasystem.sh:1846,2814,3286`, `scripts/agents/delegate-caps-fixtures.sh:204`).

`ps -o lstart=` appears only in capability probes (`scripts/agents/supervision-fixtures.sh:209-210`) and inside a fake `ps` used by fixtures. So there is no second, independently-computed start time that the Go value must agree with. Whatever `KernelProber` returns *is* the system's definition.

## Test strategy

`internal/identity/identity_test.go` has **no build tag** (ran it), so it will exercise the Linux implementation automatically once it compiles. It already sanity-checks that a probed start time is in the recent past (line 26).

`internal/identity/enumerate_darwin_test.go` **is** Darwin-gated, which means the Linux enumeration code would ship untested. Add an untagged `enumerate_test.go` asserting the cross-platform contract, so both platforms are held to one standard:

- `AllPids()` contains `os.Getpid()`
- `ParentPid(self)` equals `os.Getppid()` and reports `ok`
- `ProcessCwd(self)` matches the process working directory
- `Probe(self)` returns `Alive` with a start time in the recent past
- `Probe` of a pid that cannot exist returns `Dead` with a nil error — the three-way guarantee, which is the property most worth locking down

Keep the Darwin-only ABI assertions (struct sizes, field offsets) where they are; they are genuinely platform-specific and still valuable.

## Two environment traps

**The development filesystem hides case bugs.** The macOS filesystem in use here is case-insensitive (ran it — created `CaseTest.txt`, then found it as `casetest.txt`). A Linux virtual machine that mounts the source tree from the host inherits that behaviour, while a real Linux filesystem is case-sensitive. An import or filename with the wrong case will build in the virtual machine and fail on a genuine Linux host. **Defence:** for any build that matters, copy the tree onto the Linux machine's own disk and build there, not from the mounted host directory.

**Processor architecture is a wider gap than distribution.** Development is on 64-bit ARM (`go version go1.26.5 darwin/arm64`, ran it); most Linux deployment targets and hosted runners are 64-bit Intel. While the module stays cgo-free this costs nothing, since `GOOS=linux GOARCH=amd64 go build` cross-compiles from macOS with no extra toolchain. If cgo is ever introduced, cross-compilation needs a full C cross-toolchain and this becomes expensive. That is a further reason to pin `CGO_ENABLED=0` now.

**Distribution choice barely matters.** Debian, Ubuntu and Red Hat differ in packaging, not in the kernel interfaces this code touches. Alpine is the exception and only because of the C library, which `CGO_ENABLED=0` removes.

## Prior decisions this port must respect

Two rulings already govern this ground (read it — `plans/receipts.log`, `plans/known-issues.md`):

- **2026-08-11T10:00Z** the repo-root continuous integration workflow was deleted, reasoning that "the Go engine is darwin-only (identity reads sysctl/proc_info), so ubuntu-latest could never build bin/metasystem and every run since the port failed." This ledger reaches the same conclusion independently, from the compiler.
- **KI-10, ruled by the human 2026-08-05:** the local suite is the gate, and free-tier runner capacity is not something this project designs around.

Therefore the Linux check below goes in the **local gate**, not a hosted runner: a cross-compile needs no runner, no capacity and no fixtures, so it restores the lost signal without reopening a settled question. Reinstating the deleted workflow is a separate decision that only makes sense once the engine builds on Linux at all.

For context, the shell layer has already had Linux attention — an earlier receipt records "Fix Linux watcher portability … Ubuntu full harness passes" (read it). The Go engine is the part that never followed.

## Recommended sequence

1. **Pin `CGO_ENABLED=0`** in `scripts/adopt.sh:240` and `scripts/agents/go-gate.sh:47`. Independent of everything else, safe to do first, and it makes the portability guarantee enforced rather than inherited.
2. **Write `internal/identity/identity_linux.go`** per section 1. Mirror the three-way table exactly.
3. **Write `internal/identity/enumerate_linux.go`** per section 2.
4. **Widen or remove the build tag on `internal/census/ancestor_production.go`** per section 3, then rebuild — this is where the second wave, if any, appears.
5. **Add the untagged `enumerate_test.go`** per the test strategy, and update the resolution comments at `identity.go:10` and `identity_darwin.go:17`.
6. **Add `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./...` to `scripts/agents/go-gate.sh`.** Seconds to run, no runner needed, respects KI-10.
7. **Run the full suite on a real Linux host** (built from a copied tree, not a host mount — see the case-sensitivity trap). Cross-compiling proves it builds; only execution proves the `/proc` reads are right.

## Verification commands

```bash
# Must succeed with no output — the gate for steps 2 to 4.
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./...

# Per-package, to see every failure instead of stopping at the first.
for p in $(go list ./...); do
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build "$p" || echo "FAILED: $p"
done

# Confirm the binary really is static.
file bin/metasystem          # expect: statically linked
ldd  bin/metasystem          # expect: not a dynamic executable

# Behavioural check on a real Linux host, from a copied tree.
go test ./internal/identity/... ./internal/census/...
```

## Assumptions, gaps and risk

- **Not run:** no Linux build has succeeded, so no test has executed on Linux. Every `/proc` mapping above is standard-platform knowledge, not measured behaviour in this codebase. Verify the field-22 parse against a known process before trusting it.
- **Not established:** the intended Linux deployment target — distribution, version, architecture, containers or not. Steps 1 to 6 do not depend on it. `CGO_ENABLED=0` makes the "oldest supported C library" question moot.
- **Masked work:** the `internal/census` situation is predicted from reading the source, not observed, because compilation stops earlier. Step 4 is where reality gets a vote; there may be further Darwin assumptions behind it.
- **Residual risk after the build goes green:** the dangerous failures are behavioural, not compile-time — `/proc` permission differences, processes exiting mid-scan, pid reuse, and the `hidepid` case that can turn `Unknown` into a false `Dead`. These are precisely what the supervision code depends on being right, which is why step 7 is not optional.
- **Untouched:** no code was changed by this investigation. It is a read-only ledger.

## Proposed receipt

Not written, per the review-only rule in `AGENTS.md`. For whoever lands the first change:

```
scripts/receipt.sh add --type other --outcome shipped --verify skipped \
  --note "investigation: plans/linux-portability.md records the Linux port surface and an implementation brief. CGO_ENABLED=0 makes one build portable across all Linux and the module qualifies (no cgo, pure-Go deps), but neither adopt.sh:240 nor go-gate.sh:47 pins the flag. Module does not build for linux/amd64: 13 packages fail on one cause, KernelProber undefined. Port is two new files (identity_linux.go, enumerate_linux.go) plus widening the //go:build darwin tag on internal/census/ancestor_production.go, which has no direct darwin dependency. The microsecond-resolution worry is a non-issue: every consumer truncates to .Unix(), and the shell delegates start-time reads to this binary, so there is one clock. No Linux build or test was run"
```

## Promotion

This file is task-local evidence. When the port ships, the durable lessons — the `CGO_ENABLED=0` rule, the per-platform start-time resolution, and the `/proc` last-parenthesis parsing trap — belong with their canonical owners via `wow.md`, and this ledger should be deleted. Anything settled as a do-not-retry dead end moves to `plans/known-issues.md`.
