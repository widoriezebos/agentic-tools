# Build read: fixture-children unit 3b, round 5 (is `fixtureCustodianOwnerWatch` dead?)

Reader: Opus 5, build-read, the same reader as rounds 2 to 4. I did not write this change.

- **Tree:** `g18/wt-3bonly`, which I only read.
- **Budget:** 4 calls, all used. Nothing was built for this round.

## What changed since round 4

`fcu3b-r3.diff` and `fcu3b-r4.diff` differ only in the reason text on line 245, plus that file's index hash. The tree carries the new line at `internal/testenv/testenv.go:35`:

```go
//lint:ignore U1000 Nothing reads this; it keeps the write end reachable, so the file's finalizer cannot close it and tell the custodian the owner died while it still runs.
var fixtureCustodianOwnerWatch *os.File
```

The seat's `gate-fast-3bonly-r4.out` ends `fast mode passed (gofmt, vet, staticcheck, ...)` and `gate rc=0`.

## Short answer

The variable is not dead. This is case (b): the thing that reads it is the Go garbage collector, which treats a package variable as a root. The new reason line names that mechanism correctly.

There is also a fix at the source that needs no suppression and is at least as strong: keep the write end as a raw descriptor, duplicated with `F_DUPFD_CLOEXEC`, instead of an `*os.File`. With no `*os.File`, there is no finalizer to guard against. I judge that shape better under the "fix at the source" rule. The current shape is still correct, stays within its brief, and ships no defect, so this is not a material finding. Whether the rule forbids a suppression when a clean shape exists is the seat's decision. If it does, the shape below is the fix, and it would need a short re-read because it touches the pipe setup.

## 1. Is it dead?

No. My round-4 control settles this for how the variable behaves.

- **Test.** The probe listed write-only pipe descriptors above 2, then forced six `runtime.GC()` cycles during `m.Run`.
  - 3b as written: `before=[6] after=[6]`.
  - The same tree with the writer held only in a local (`ownerWatch, startErr := startFixtureCustodian(...); _ = ownerWatch`): `before=[6] after=[]`, the test failed.

  The only thing the store at `testenv.go:137` does is keep the `*os.File` reachable. Without it, the file's finalizer closes the write end while the test binary is still running. Reachability is a language and runtime rule, not a Darwin quirk.
- **Why the early close matters.** In `fixture_custodian.go:56-74`, while the pipe is open, a probe that returns `Unknown` makes the custodian wait (`if watchEOF != nil { continue }`). Once the pipe has hit EOF, `Unknown` starts `proveOwnerDead`, which gives up after its bound with `could not prove owner dead`. A write end closed early therefore lets a custodian give up and exit while its owner is still alive, whenever the owner probe is `Unknown` for longer than the bound. That leaves the owner's later children unguarded. The variable holds guarantee 1, and guarantee 1 is what prevents this.
- **What the control does not settle.** No 3b test fails if the variable is removed. The real-process witnesses probe the owner as `Dead`, never `Unknown`, so an early EOF only makes the custodian poll. This proof gap is the same for every shape below (non-material note 2).

## 2. Does anything read it?

Nothing in code. I searched the whole `wt-3bonly` tree, all file types, `.git` excluded:

- **Code:** the name appears only at the declaration (`testenv.go:36`) and the single store (`:137`). The only other hit is prose quoting the old code in `records/misc/fixture-children-u3-read.md:140`.
- **Other packages:** the variable is unexported, and `internal/testenv` has no `go:linkname`, so no other package can reach it.
- **Reflection:** the only `reflect` use in `internal/testenv` is `reflect.DeepEqual` over directory lists in `testenv_test.go:127`.
- **Build tags:** no `//go:build` constraint in `internal/testenv`. The `//go:build` hits in `protection_test.go` are string fixtures.
- **Later units:** the design (`plans/fixture-children-cannot-outlive-their-test-design.md:155-156`) says only that the pipe's "only write end is in the owner", opened close-on-exec. No unit reads, closes or hands off the writer.

The only reader is the garbage collector's root scan, which staticcheck's U1000 cannot see. The new reason names it ("keeps the write end reachable, so the file's finalizer cannot close it"). Under m1e's own test, (b) holds, and the reason now names the reader.

## 3. Is there a fix without a suppression that is just as strong?

Three candidates. None was built.

- **A. `runtime.KeepAlive` in `Main`: weaker, not recommended.**
  - Placement is tricky. The registry-cleanup defer is registered before the custodian starts, and deferred calls run last-registered-first. To cover the cleanup, the keep-alive must be a closure deferred *before* the cleanup defer, over a variable declared before `prepare`. The plain `defer runtime.KeepAlive(w)` evaluates `w` too early.
  - Even placed correctly, it ends when `Main` returns. Every `TestMain` then runs `os.Exit(testenv.Main(m))`, and with `-race` `os.Exit` runs its exit hooks. A garbage collection plus the finalizer in that gap can still close the write end before the process exits. That window is narrow, but guarantee 1 says "to process exit", so this shape holds it less strongly than the package variable does. It would also need a comment explaining the defer order.
- **B. A raw descriptor with no finalizer: equal or stronger, and better under the rule.**
  - After the custodian's post-start probe succeeds (where `keepWriter = true` is today), duplicate the write end with `unix.FcntlInt(writer.Fd(), unix.F_DUPFD_CLOEXEC, 0)`. Then let the existing defer close both `*os.File` ends unconditionally, and drop `keepWriter`, the return value and the package variable. One plain comment would say the duplicate is never closed, because the kernel closes it at process exit and that close is the custodian's death notice.
  - **Guarantee 1:** structural. An `int` has no finalizer and nothing in the process closes it, so it stays open up to the exit system call (normal exit, `os.Exit` or SIGKILL), with no dependence on reachability.
  - **Guarantee 2:** `F_DUPFD_CLOEXEC` sets close-on-exec atomically. It is defined for Darwin (`0x43`) and Linux (`0x406`) in the module's `golang.org/x/sys` v0.47.0, so no fork window opens. The `*os.File` ends stay close-on-exec as today.
  - **Guarantee 3:** the D2 gate, the FIFO and `O_RDONLY` checks, the `<registry>.custodian.log` path with `O_NOFOLLOW`, `ExtraFiles` and `Setsid` do not change.
  - **Cost:** by my count, removing the variable block and the `keepWriter` logic takes out about as many lines as the duplication adds, so 3b stays around 296 to 299 lines, under the 300 cap. This is an estimate; I did not build it.
  - **Two small points for whoever builds it.** `Fd()` switches the shared open file to blocking mode, which does not matter because nothing writes to it; `SyscallConn().Control` avoids that if preferred. If the duplication fails after the custodian has started, kill the custodian as the probe-failure path does now.
- **C. Other ideas I rejected.**
  - Removing the finalizer is not possible: it is set on `os.File`'s internal `*file`, which callers cannot reach.
  - Closing the writer explicitly at the end of `Main` would announce the owner's death while the owner still runs, which the fix brief forbids.
  - Creating the pipe with `syscall.ForkLock`, `unix.Pipe` and `CloseOnExec` would copy `os.Pipe`'s internals and cost more lines than B for the same result.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| N-1 | low | no | A shape without a suppression exists (B) and is equal or stronger on guarantees 1 to 3 | See section 3. The current code is correct and within the fix brief, which allowed `//lint:ignore U1000` with a reason |
| N-2 | low | no | No test fails if the write end is closed early, whatever the shape | Section 1: the witnesses probe the owner as `Dead`, never `Unknown` |

Material findings: none, so no R5 findings.

## What I could not check

- Shape B was not built or run, per the brief. Its size and its behaviour under the round-4 probe are estimates.
- I did not re-run the fast gate on the reworded line. I rely on the seat's `gate-fast-3bonly-r4.out`, and a comment-only change cannot alter the round-4 staticcheck, vet or race results.

VERDICT: LAND
