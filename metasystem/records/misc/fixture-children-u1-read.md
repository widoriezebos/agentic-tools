# Read: fixture-children unit 1 (identity: refs, keys, probes)

Reviewer did not write this change. Diff computed from the worktree at
`/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/e67ca063-900a-46ec-9bc8-9fbdcbc3765e/scratchpad/g18/wt-fcu1`,
nine files, 278 insertions and 21 deletions, 299 changed lines against the 300 cap.

## Material findings

### U1-1 Darwin reports a refused environment read as a proven empty environment

`metasystem/internal/identity/identity_darwin.go:91-97` (the `Probe` block) with the
new environment loop in `procArgsAndExecutable` at `identity_darwin.go:150-161`.

On Darwin the kernel returns the environment region of `kern.procargs2` only for the
calling process and for targets that are not restricted binaries. A SIP platform
binary such as `/bin/sh` hands back argv and the exec path but no environment. The
code cannot tell that apart from a process that genuinely has no environment, so it
sets `EnvironKnown = true` with `Environ` empty.

Measured on this host (darwin/arm64, uid 501), each child started with the full
parent environment plus `METASYSTEM_FIXTURE_OWNER=pid=1;micro=2|T|a1b2c3d4`:

    /bin/sh (platform)     state=alive environKnown=true len(environ)=0  TAG-FOUND=false exe="/bin/sh"
    go test binary         state=alive environKnown=true len(environ)=65 TAG-FOUND=true
    self                   environKnown=true len(environ)=59
    parent (go tool)       environKnown=true len(environ)=59

The failure it causes: the field's own contract in `identity.go:40-41` is "valid only
when EnvironKnown", and the package doctrine at the top of `identity.go` is that a
failed read is Unknown and Unknown never authorizes anything. This inverts it. A
denied read is published as fact. Everything the design stacks on the field then
reads "this process carries no tag" where the truth is "we were not allowed to look":
unit 2's `FixtureSurvivors` selects nothing, the `fixture-survivor?` class for
"signalable but environment unreadable" never fires because the flag says readable,
and the census, health and launcher consumers print a clean report over a live
survivor. A missed reap that reports success is worse than a refusal.

The existing argv field gets this right by construction: `ArgvKnown` is false when the
read fails. The new flags do not, because the read does not fail.

Smallest fix: return a fourth value from `procArgsAndExecutable` saying whether an
environment region followed argv (when the kernel strips it, the trailing region is
empty and the apple strings that normally follow the environment are gone too, so the
discriminator is local and cheap), and in `Probe` set `EnvironKnown` only when that
region was present. Leave `Environ` nil and `EnvironKnown` false otherwise. About six
lines, all in `identity_darwin.go`.

Note for the seat: this fix pushes the diff past the 300-line cap, which is at 299
now. The design's own rule is that a unit passing 300 splits at its witness boundary.

### U1-2 The design's tag carrier does not survive on Darwin, and unit 1 is where that shows

Design section 3.1: "The tag `METASYSTEM_FIXTURE_OWNER=<EncodeKey(key)>` goes in the
environment of every process a fixture launches; environment survives exec, `Setpgid`,
`Setsid` and reparenting". Section 3.7 converts fixtures that are, every one of them,
`/bin/sh`: the leashed reads, the `sh -c 'exec sh script'` grandchild, the hanging git
wrapper, the fake host.

The measurement in U1-1 says the tag on a `/bin/sh` child is unreadable from any other
process on this Mac. It is readable on a Go binary child, and Linux procfs has no such
restriction, so the mechanism works on Linux and for Go children everywhere and fails
on Darwin for exactly the fixtures this goal exists to reap.

This is not unit 1's to fix and the code here is not wrong about it. It is reported
because unit 1 is the first place it is observable, and because units 2, 3, 5 and 6
are all built on the assumption. Discovering it in unit 3 costs three units of work.
The seat needs a Darwin answer first: a carrier that is readable cross-process (argv,
which `AliveTaggedRef` already uses and which `HasExactToken` already matches, or the
ownership record file the census already plans to read at
`<go-tmp>/<TestName><random>/fixture-owner`), or an accepted Darwin degradation
written into the design with the `fixture-survivor?` class carrying it.

## The two seat questions

### 1. The witness

`TestOwnerRefEncodesNativeShape` is a real behaviour test, not a compile check. I
verified it in a throwaway copy of the module at
`/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/e67ca063-900a-46ec-9bc8-9fbdcbc3765e/scratchpad/g18/mut`
(go.mod, go.sum, `internal/identity`, `internal/testenv`; baseline green). Each
mutation keeps every symbol defined and breaks one behaviour. Five of five behaviour
mutations are caught:

| Mutation | Result |
| --- | --- |
| M1 round the start time: `EncodeRef` emits `micro/1e6*1e6` | FAILS. `ref_test.go:23: EncodeRef() = "pid=41;micro=100000000"; want "pid=41;micro=100000123"` |
| M2 round a whole-second ref up instead of rejecting it (fill `StartedAtUnixMicro` from `StartedAtSec` before the native-exact check) | FAILS. `ref_test.go:42: whole-second ref encoded as an exact owner reference` |
| M3 accept a two-field key (`ParseKey` pads a missing nonce) | FAILS. `ref_test.go:45: two-field fixture key parsed successfully` |
| M4 drop the pid field from the ref encoding | FAILS. `ref_test.go:23: EncodeRef() = "micro=100000123"` |
| M5 drop the nonce field from the key encoding | FAILS. `ref_test.go:39: ParseKey() ... identity: malformed fixture key` |

So the three mutations the seat named (rounded start time, two-field key accepted,
field dropped from the encoding) are all caught, and the witness holds. The builder's
evidence was worthless; the test it describes is sound.

Three mutations survive, marking what the witness does not pin down. None of them is
a defect in the code as written, which I checked directly by probe:

| Surviving mutation | What it means |
| --- | --- |
| M6 `ParseRef` stops filling `StartedAtSec` | The round trip is field-by-field, not whole-struct. `StartedAtSec` is not asserted after a parse. |
| M7 `ParseRef` drops the final `NativeExact` check | The design's own sentence, "ParseRef rejects a value whose `Ref.NativeExact()` is false", has no test. The behaviour is right: `ParseRef("pid=41;ticks=7001;boot=boot-a")` on this Darwin host returns `identity: process reference is not native-exact`. One line in the witness would pin it. |
| M8 `EncodeRef` drops the seconds-against-microseconds consistency guard | Untested. Behaviour is right: an inconsistent ref returns `identity: Darwin process reference has inconsistent start times`. |

Direct probes of the parser, all correct: a foreign-platform ref, an extra field, a
bare pid, a legacy `sec=` ref, an empty string, a four-field key and a non-hex nonce
are all rejected; an uppercase nonce and a test name containing `;` and spaces round
trip.

### 2. The unrun checks

I ran the brief's proof obligations on the host. Results, all from this worktree with
go1.27.1 darwin/arm64:

| Obligation | Result |
| --- | --- |
| `gofmt -l` on all nine touched files | clean |
| `go build ./...` | exit 0 |
| `go vet ./internal/identity` | clean |
| `go test -race -count=1 ./internal/identity` | ok, 1.4s |
| `go test -race -count=1 ./internal/proofrun` (the watchdog's own package, whole package) | ok, 128.4s |
| `go vet ./internal/proofrun`, `go vet ./internal/census`, `go test -race ./internal/census` | clean, ok 3.4s |
| Witness 1 | green, and proved by mutation above |
| Real process probe | run: self, parent, and four child kinds, results in U1-1 |

The builder's report of pre-existing failures in proofrun does not reproduce. The full
package is green under `-race`.

Still unproven, for the seat to run:

- `bash scripts/agents/go-gate.sh --fast`. Not run here. It is the whole-repo gate and
  the memory rule about not running a suite next to other work applies; it is also the
  brief's own "run last" item, which belongs after the U1-1 fix anyway.
- Everything Linux. `readEnviron`, the `kernelExecutablePath` call added to the Linux
  `Probe`, and the Linux half of witness 1 (`pid=...;ticks=...;boot=...`) never
  execute on Darwin: `EncodeRef` refuses a non-native ref before reaching its Linux
  branch, so that branch is dead code on this host. The Linux leg of witness 1 is
  proven only by running the identity package in the VM. Run
  `go test -race -count=1 ./internal/identity` there.
- The Darwin measurement in U1-1 should be repeated on Linux before the seat decides
  U1-2. Linux procfs lets a same-uid process read `/proc/<pid>/environ`, so I expect
  the tag to be visible there, but it is an expectation, not a measurement.

## Non-material notes

- N1. The watchdog lift is faithful. `metasystem/internal/proofrun/watchdog.go:284-303`
  keeps the same signal, the same target (the closure discards the pid
  `SignalExact` passes and sends to `target`, so the negative pgid still reaches
  `kill`), the same order (prove identity, then send), and the same refusal text: the
  old message interpolated `identity.Liveness`, whose `String()` returns exactly
  "dead" and "unknown", which is what the two new branches hardcode. All three
  ladder call sites at :268, :271 and :275 are unchanged. The one difference is
  `errors.Is(err, syscall.ESRCH)` where the old code compared with `!=`, which is
  strictly more tolerant of a wrapped ESRCH from an injected sender and identical for
  the bare `syscall.Errno` production returns.
- N2. `SignalExact(prober, ref, sig, sender ...SignalFunc)` deviates from the design's
  three-argument `identity.SignalExact(prober, ref, sig)`. The injectable sender is
  needed (section 6 requires witnesses that assert nothing was sent), but a variadic
  turns a compile-time arity error into the runtime branch `len(sender) > 1`, which
  nothing can reach and nothing tests. An explicit sender parameter or a small
  interface would be cleaner. No behaviour difference.
- N3. `SignalExact` has no direct test. Its four branches are covered only through the
  watchdog ladder, which is green. Worth one table test when unit 5 gives it more
  callers.
- N4. Seat question 5, the census edits. The `Environ` and `Exe` fields on
  `census.Process` are the design's "the fixture process table gains the same fields",
  so they are forced. Nothing the census reports changes: `InventoryItem`, the
  verdict's process projection, carries no environment or exe field, and no code
  marshals `[]Process` outward, so `proc census` output is byte-identical. The only
  writer is the new line at `internal/census/production.go:69` and the only future
  reader is unit 2. No counter dropped, no gate weakened, census package green.
- N5. The `json:"environ,omitempty"` tag makes every process's full environment
  serializable the moment anything marshals the table. Environments carry API keys.
  Nothing marshals it today. Worth a line in the design before unit 6 adds a verb that
  prints table rows.
- N6. `Probe` is now heavier for every caller: two extra reads per process on Linux
  (`/proc/<pid>/environ`, `/proc/<pid>/exe`) and a second buffer walk on Darwin. The
  hot supervision path is unaffected because `AliveRef` prefers `ReadStart` and
  `KernelProber` implements it, but a full census enumeration now pays it per process
  and holds every process's environment in memory. Allowed under R-115, which forbids
  the opposite trade, but the seat should know.
- N7. On Darwin `Exact.Environ` is the environment plus the kernel's trailing apple
  strings (`executable_path=`, `executable_boothash=`, `th_port=`,
  `security_config=0x0` observed). All contain `=`, so a keyed lookup is unaffected;
  a consumer that reconstructs an environment from the slice would carry them.
- N8. `ParseRef` accepts non-canonical integers: `pid=+41;micro=+100000123` parses to
  the same ref that `EncodeRef` renders as `pid=41;micro=100000123`. Harmless while
  units 2 and 6 compare parsed keys, as the design says they do; it would matter to
  any consumer that compares encoded tag strings.
- N9. The Darwin environment loop drops an empty entry (`if cut > 0`) and silently
  drops a final unterminated entry. Neither is reachable with a real environment.
- N10. Boundary is clean. Nothing from units 2 to 8 appears: no scans, no custodian,
  no run-owner export, no `testutil.Fixture`, no verb, no health or launcher consumer.
  `ErrGone`, `ErrUninspectable` and `SignalFunc` are all consumed by the lifted call
  site. The replacement of `p.ReadArgv(pid)` with a direct `procArgsAndExecutable`
  call in the Darwin `Probe` changes nothing: `KernelProber` is an empty struct and
  `ReadArgv` is a method, not an injection seam, and it still delegates to the same
  function.

## What I could not check

- The Linux side of everything: witness 1's Linux leg, `readEnviron`, the Linux
  `Probe` additions, and whether the environment tag is cross-process readable there.
  No Linux host in this session.
- `scripts/agents/go-gate.sh --fast`, deliberately not run.
- Whether the seconds-based `StartedAtSec` that `ParseRef` reconstructs on Darwin is
  read by any later consumer. No consumer exists yet.
- The `/bin/sh` result is from this Mac's OS build. The kernel rule that produces it
  is a property of XNU's procargs handling for restricted binaries, but I read it off
  behaviour, not source.

## Tool calls used

20 of 25: one skill load, one file read, seventeen bash calls, one write.
