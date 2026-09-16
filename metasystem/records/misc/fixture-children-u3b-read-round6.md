# Build read: fixture-children unit 3b, round 6 (shape B: a raw duplicate of the write end)

Reader: Opus 5, build-read, the same reader as rounds 2 to 5. I did not write this change.

- **Tree:** `g18/wt-3b-r5` at trunk `e1cc059ca` with only 3b applied. I only read it. Its staged diff against HEAD is byte-identical to `fcu3b-r5.diff`.
- **Copy:** all runs used a copy of `metasystem/` outside every worktree, at `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/1e0f004f-23d9-4653-add1-c3a768ed5661/scratchpad/r6`.
- **Budget:** 5 of 8 calls used.

## Short answer

Shape B does what it claims:

- The duplicated write end stayed open through six forced garbage collections, in both the plain run and the `-race` run.
- In both runs the custodian's read end stayed paired with that duplicate the whole time, so the custodian could not see EOF while the owner ran.
- Two controls show the probe can tell the difference. A duplicate wrapped in a `*os.File` gets closed by the collector. With no duplicate, the custodian has no writer from the start.
- Guarantees 1 to 3 hold.
- The dup-failure path kills the custodian before any deferred close, which is correct.
- The comment is plain and accurate.

There are no material findings.

## 1. Forced-GC probe on a copy

The probe is my round-4 probe with two additions:

- It uses `lsof` to show which pipe each descriptor belongs to, in both the owner and the custodian.
- It starts a fresh child and lists that child's descriptors from 3 up.

The probe runs under `METASYSTEM_FIXTURE_CUSTODIAN_START=1`, with its own skip gate `ZZ_OWNER_WATCH_PROBE=1`, and only in `internal/identity`. It forces `runtime.GC()` six times, 50 ms apart.

| Run | Write-only pipes above fd 2, before and after | Owner pipe, before and after the collections | Custodian fd 3 after | Result |
| --- | --- | --- | --- | --- |
| 3b as written | `[8]` / `[8]` | fd 8 `0x64692ef8ef4c085b -> 0x413b77a347551e91` both times | `0x413b77a347551e91 -> 0x64692ef8ef4c085b`, custodian alive | survives |
| 3b as written, `-race` | `[8]` / `[8]` | fd 8 `0x392b2d561b96c255 -> 0xa23ac388eb1fd022` both times | `0xa23ac388eb1fd022 -> 0x392b2d561b96c255`, custodian alive | survives |
| Control A: duplicate wrapped in an unreferenced `os.NewFile` | `[8]` / `[]` | fd 8 present, then gone | peer empty (no writer left, so EOF) | closed by the finalizer, and the probe fails as it should |
| Control B: duplicate removed | `[]` / `[]` | no write end at all | peer empty from the start | the probe fails as it should |

What this shows:

- **The listed descriptor is the duplicate.** Removing the one `FcntlInt` call removes fd 8 (control B), and the custodian's read end is paired with fd 8 by address. This settles the builder's weak probe, which listed `[1 2 4 8]`. Descriptors 1 and 2 are the `go test` output pipe. My fd 6 is the read end of the pipe that collects each `lsof` run's output; it changes address every run and is not write-only.
- **Why no EOF is possible:** a pipe reaches EOF only when every write end is closed. After the collections, the owner's fd 8 is still the write end paired with the custodian's fd 3 (same addresses as before). The round-4 "writer held in a local" control no longer applies because no `*os.File` is kept. Control A is its replacement: it shows that exactly that kind of holder is what the collector closes.
- **Exit:** after each owner exited, its custodian was gone within 2 to 3 seconds and had logged `fixture-custodian owner=pid=...;micro=... action=complete`. That line alone does not prove the EOF path, since a `Dead` probe also leads there. The kernel closing every descriptor at exit is not in doubt.
- **My probe had one defect.** The fresh-child check failed in every run, controls included, because it flagged `lsof`'s own two internal pipes at fds 4 and 5. Their addresses never match the duplicate's; a duplicate would show the same address, since it is the same pipe. They also appear in control B, where no duplicate exists. So no child inherited the duplicate. That check was simply too broad.

## 2. Guarantees 1 to 3

**1. The write end stays open until process exit.** It holds.

- `FcntlInt` returns a bare `int` that the code throws away (`_`). No Go object refers to it, no finalizer is attached, and nothing in the owner can close it by name.
- Only process exit closes it, SIGKILL included. The probe runs above confirm this.
- **No gap:** the duplicate is made while `writer` is still open. The deferred closure captures `writer`, so it stays reachable through the `Fd()` call. The original is closed only when the function returns, so there is no moment with no write end. Control B shows what that moment would look like.
- **Closes by number:** the tree has no `closefrom` or `CloseRange`. The `os.NewFile(3|4|5)` sites in `internal/usage/context_cost_helper_test.go:29-30` and `internal/steward/identity_test.go:352-354` are, by their names, the child end of helper handshakes. I did not trace each one. A child cannot hold the duplicate (see guarantee 2), and this exposure is the same as the round-4 shape. `janitor/headroom.go:105` wraps a descriptor it opened itself.

**2. No child inherits the duplicate.** It holds.

- `F_DUPFD_CLOEXEC` sets close-on-exec atomically, and the probe's `FD_CLOEXEC` check on fd 8 reported nothing.
- The custodian starts before the duplicate exists. `lsof` on the custodian shows exactly one pipe: fd 3, the read end.
- A fresh child holds no pipe with the duplicate's address (section 1).
- On Linux, `GOOS=linux GOARCH=amd64 go build ./internal/testenv/ ./cmd/metasystem/` passes (rc=0), so the constant exists there too.

**3. Nothing else in the start path moved.** It holds.

- The whole `r4` to `r5` delta is in `internal/testenv/testenv.go`:
  - the variable, the lint line and `keepWriter` are removed;
  - `startFixtureCustodian` returns only `error`, with every `return nil, x` changed to `return x`;
  - the call site in `Main` changed to match;
  - the defer closes both ends unconditionally;
  - the duplicate block and its comment line are new.
- Unchanged: the owner identity probe, environment filtering, the log path and its `O_NOFOLLOW` open, `ExtraFiles`, `Stderr`, `Setsid`, the post-start custodian probe, and the custodian branch in `Main`.
- The fixture-children-only files (`fixture_custodian_witness_test.go`, `testmain_test.go`) are not in the delta.
- **Size:** 296 changed lines. The same count gives 299 for `fcu3b-r4.diff`, which matches round 4, so 296 stands, under the cap.

## 3. The dup-failure path

`testenv.go:187-190` kills the custodian and then returns the error. The order is correct:

1. **Kill first.** `command.Process.Kill()` runs before any deferred close, so the original write end is still open when the custodian is killed. The custodian gets no EOF from this path before the kill. Even if it did, the owner is alive, so it would not reap anything.
2. **Then the deferred closes,** in reverse order of registration: `logFile.Close()`, then `reader.Close()` and `writer.Close()`. No descriptor leaks.
3. **Then `Main`** prints `start fixture custodian: <err>`, returns 2, and still runs the registry cleanup defer.

This matches the post-start probe failure path just above it (`:182-185`). Two small points, neither material:

- The killed custodian is not waited for, so it stays a zombie until the owner exits, exactly as on the probe path.
- The error is returned without extra context, like the other bare returns in this function (pipe, log open, `Start`); `Main`'s prefix names the step.

No test drives this path; I checked it by reading only.

## 4. The comment

`// The duplicate is never closed; the kernel closes it at process exit, which tells the custodian the owner died.`

It is plain English and describes the code as it is: nothing closes the duplicate, and exit does. It carries no round or finding references. One optional wording point: EOF is the custodian's cue to confirm the death, which it still proves with a probe before reaping. "Tells the custodian the owner died" matches the design's own "death notification" wording, so I do not ask for a change.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| N-1 | low | no | As in round 5, I found no test in the tree that checks the write end stays open; only my probe outside the tree catches controls A and B | Section 1. I did not run the tree's own suite under the controls. |
| N-2 | low | no | The dup-failure path is untested and does not wait for the killed custodian | Section 3; same as the probe-failure path |
| N-3 | nit | no | The comment could say EOF prompts the custodian to confirm the death | Section 4; optional |

Material findings: none, so there are no R6 findings.

## Outside 3b (for the seat, not counted)

On trunk, `internal/identity/enumerate_darwin_retry_test.go` (from `18b5effc0`) has no darwin build constraint, even though `sysctlRaw` exists only in `enumerate_darwin.go`. As a result, `GOOS=linux GOARCH=arm64 go vet ./internal/identity/` fails with `undefined: sysctlRaw`. If Linux runs the identity tests, they will not compile. This comes from trunk, not from 3b.

## Cleanup and limits

- **Cleanup:**
  - removed the four `/tmp/metasystem-test-registry-*.custodian.log` files my runs created;
  - their registry directories were already gone;
  - no custodians or `identity.test` processes remain;
  - `testenv.go` in the copy was restored and matches the tree byte for byte;
  - nothing was written to `wt-3b-r5`.
- **Limits:**
  - Darwin only for the runtime probe; on Linux I only checked that the code compiles.
  - I did not re-run the gate; I rely on `gate-fast-3bonly-r5.out` (rc=0) and the seat's race runs.

VERDICT: LAND
