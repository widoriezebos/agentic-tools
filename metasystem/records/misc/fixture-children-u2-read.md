# Read of fixture-children unit 2 (identity: the two scans)

Worktree: /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/e67ca063-900a-46ec-9bc8-9fbdcbc3765e/scratchpad/g18/wt-fcu2
Diff: two new files, 300 insertions, `metasystem/internal/identity/fixture_survivors.go` (183) and
`metasystem/internal/identity/fixture_survivors_test.go` (117). No existing file is touched.

Reviewer ran the mutations in a throwaway copy of the package at
/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/e67ca063-900a-46ec-9bc8-9fbdcbc3765e/scratchpad/g18/tw.
The worktree was not edited.

## Material findings

### U2-1 An unreadable process with its exe under go-tmp is silently dropped

File: /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/e67ca063-900a-46ec-9bc8-9fbdcbc3765e/scratchpad/g18/wt-fcu2/metasystem/internal/identity/fixture_survivors.go
Lines: 124-126 (the gate that collects unreadable observations) and 132-136 with
`sharesFixtureScope` at 148-159.

The revision-4 amendment, section "Reading it", scopes the `fixture-survivor?` class to
"unreadable in both slots and in a group or session led by a dead-owned tagged process **or
with an exe under `go-tmp`**, or a parsed tag whose owner probes Unknown". The implementation
keeps only the group/session route: an unreadable observation is emitted only when
`sharesFixtureScope` finds a certain survivor with the same pgid or sid. The exe-under-go-tmp
route is not implemented, so an unreadable-in-both-slots process whose exe is under a `go-tmp`
tree but whose group and session differ from every certain survivor is dropped with no report
at all. That is also the brief's rule 3: "a process whose command line cannot be read is
reported in its own class, not silently dropped".

Evidence (probe test written into the throwaway copy only): a table with a dead owner, one
tagged child at pid 701 in pgid 900/sid 901, and pid 702 unreadable in both slots with
`Exe = <tmp>/go-tmp/TestOther99/bin/metasystem` in pgid 500/sid 501.
`FixtureSurvivorsOfDeadOwner` returned `err=<nil> results=1`, only `pid=701
class=fixture-survivor`. Pid 702 appeared in no class. With 702 moved into pgid 900 (the
builder's own `TestFixtureScanClassifiesScopedUnreadableProcess`) it is reported correctly, so
the group route works and only the go-tmp route is missing.

Effect: a leaked fixture process that reparented into another session and whose argv and
environment are both unreadable is invisible to `health` and to the census built on these
scans in units 6 and 7. Nothing is wrongly killed; the leak is simply not named, which is the
failure mode the page exists to close.

Smallest fix (about four lines, inside the ceiling): the go-tmp predicate already exists
inside `fixtureOwnershipRecord` (lines 163-170). Lift the ancestor walk into
`func underGoTmp(executable string) bool`, have `fixtureOwnershipRecord` call it, and change
line 124 to keep the observation when `sharesFixtureScope(...)` is true **or**
`exact.ExeKnown && underGoTmp(exact.Exe)`. Extend
`TestFixtureScanClassifiesScopedUnreadableProcess` with a second unreadable specimen in a
different group whose exe is under go-tmp.

Related: the diff is exactly at the 300-line ceiling, and the builder's return reported
nothing left out. The brief requires that a unit that cannot fit stops and reports what is
left; this behaviour is missing without being named as deferred.

## Checks that passed

1. **The safety rule holds, including the unknown owner.** `FixtureSurvivorsOfDeadOwner`
   (lines 75-90) errors with `nil` results on `Alive` and on `Unknown`, and reaches the scan
   only on `Dead`. `AliveRef` (identity.go:191-221) maps a probe of Unknown to Unknown, a
   `CompareInvalid` comparison to Unknown, and a pid reused by a different process to Dead, so
   "unknown" cannot arrive as "dead" through the helper either.
   - Mutation A, deleting `case Alive`, produced
     `live-owner scan = [...two survivors...], <nil>; want no results and an error` and the
     test failed. The builder's first mutation reproduces.
   - Mutation B, deleting `case Unknown` (the builder did not test this), produced
     `unknown-owner scan = []identity.FixtureSurvivor(nil), <nil>; want no results and an
     error` and the test failed. The unknown case is covered by
     `fixture_survivors_test.go:70-73`, which the builder's report did not mention.
2. **The key rule holds.** `FixtureSurvivors` (lines 64-73) encodes the whole key with
   `EncodeKey` and compares the full encoded string `<owner>|<test>|<nonce>`; owner-ref-alone
   selection exists only in the dead-owner scan, where the design intends it.
   - Mutation C, matching on `EncodeRef(candidate.Owner)` in the key scan, returned both
     children for key A and the test failed with `want child A only`. The builder's second
     mutation reproduces.
3. **Both carrier slots are read.** `FixtureTag` (lines 32-48) iterates `exact.Argv` first,
   then `exact.Environ`, each guarded by its own `ArgvKnown` / `EnvironKnown` flag, matching a
   whole word with the `METASYSTEM_FIXTURE_OWNER=` prefix and continuing past a word whose key
   fails to parse. The amendment's order (argv, then environment) is respected. The slots are
   exercised for real by `TestCleanupIsScopedToTheFixtureKey`: child 701 carries the tag in
   argv with `EnvironKnown` false, which is the Darwin `/bin/sh` case, and child 702 carries it
   in the environment with `ArgvKnown` false. Both were matched in the mutation A output. A
   scan that read only the environment would fail this test.
4. **Unreadable processes are a class, never a kill target.** The class is
   `FixtureSurvivorUnreadable = "fixture-survivor?"`, requires both slots unreadable and the
   process signalable, and no code path in either scan signals anything. The one gap is U2-1.
5. **No direct Ref equality anywhere in these files.** Identity is compared in exactly three
   places and all three are canonical: `EncodeKey` string equality (line 71), `EncodeRef`
   string equality (line 88), and `SameIdentity` (line 150), which routes through `Compare`.
   The builder's critique claim is correct and load-bearing on Linux: a probed `Exact.Ref()`
   there carries StartedAtUnixMicro as well as StartTicks and BootID, while the owner parsed
   back out of a tag word carries only ticks and boot, so `==` on the structs would never
   match and the dead-owner scan would silently return nothing. `EncodeRef` emits the native
   shape only, which normalises both sides.
6. **Boundary and R-115-m1e hold.** The diff is two new files under `internal/identity`. No
   custodian, no run owner, no `testutil.Fixture`, no verb, no health or launcher consumer. No
   existing file is modified, so no existing identity behaviour, counter or gate changes. The
   new file's `golang.org/x/sys/unix` import adds no platform constraint; `survivors.go`,
   `tagstate.go` and others in the package already import it untagged.
7. **Gates green in the worktree.** `gofmt -l metasystem/internal/identity` empty;
   `go build ./...` rc 0; `go vet ./internal/identity` clean;
   `go test -race -count=1 ./internal/identity` ok, 1.649s, the whole package, not just the new
   tests.

## Non-material notes

- **N-1 (worth acting on before unit 5).** Design 3.1 says both scans return the exact set
  "and separately the signalable-but-unreadable set". The implementation returns one slice
  sorted by pid with certain and unreadable entries intermixed and only the `Class` field
  separating them (lines 128-138). Nothing in the type stops a consumer writing
  `for _, s := range survivors { SignalExact(...) }`, which is the one thing the page forbids.
  A `func (s FixtureSurvivor) Reapable() bool` or a two-slice return would make
  "indeterminacy never acts" a property of the API instead of consumer discipline.
- **N-2.** `FixtureSurvivorUnowned` (line 20) is declared and never produced by either scan.
  It is the census class from 3.5 and presumably lands with unit 6; today it is a dead
  constant.
- **N-3.** `FixtureSurvivors` takes no prober and reads the unexported package var
  `fixtureSurvivorProber` (lines 50, 69). This matches the signature the design fixed in 3.1,
  so it is not a deviation, but unit 6's process-file mode
  (`METASYSTEM_CENSUS_PROCESS_FILE`, driven from `internal/census`) has no seam to inject a
  table-backed prober into the key scan. Unit 6 will need an exported variant.
- **N-4.** `fixtureOwnershipRecord` (lines 163-170) anchors on any ancestor directory named
  exactly `go-tmp`, where 3.5 says `metasystem-build-cache/go-tmp`. A false positive also needs
  a parseable `fixture-owner` file whose key matches, so the risk is small.
- **N-5.** Line 78 returns the same message, "fixture owner is not exactly inspectable", for a
  nil prober and for a ref that will not encode. Two different callers' mistakes read alike.
- **N-6.** The safety switch at lines 80-85 has no `default`, so any `Liveness` value other
  than Alive and Unknown falls through into the scan. The three-way type makes this correct
  today; `if AliveRef(prober, owner) != Dead { return error }` would keep it correct if a
  fourth state is ever added.
- **N-7.** `fixtureSurvivorScope` (lines 57-62) issues `Getpgid`, `Getsid` and `Kill(pid, 0)`
  for every live native-exact process on the machine on every scan, before any match test.
  Signal 0 delivers nothing, so this is cost only, not behaviour.

## What I could not check

- **Linux.** The seat is darwin/arm64. The canonical-encoding fix in check 5 and the both-slot
  procfs reads are reasoned from the code, not run. On Darwin the fixture table's refs make raw
  struct equality succeed, so the mutation that would prove the fix necessary only fails on
  Linux; the test is platform-adaptive (`fixture_survivors_test.go:20-26` fills StartTicks and
  BootID on Linux) and should catch it on the VM.
- **`bash scripts/agents/go-gate.sh --fast`** was not run here, against the call budget. Build,
  vet, gofmt and the full package under race are green in the worktree.
- **Real-kernel behaviour.** Every test stubs `survivorPids`, `fixtureSurvivorProber` and
  `fixtureSurvivorScope`. Neither scan has been run against live processes, so `AllPids`
  enumeration cost and the real pgid/sid/signalable values are unproven. Nothing in unit 2's
  boundary can prove them; unit 5 or the seat's own `health` will.
- **Unit 1's Darwin `EnvironKnown = environ != nil`** was measured by the unit 1 builder and is
  not re-measured here. If a restricted `/bin/sh` ever returned a non-nil empty slice, the
  unreadable gate at line 124 would stop firing.
- **Ordering of the tag inside a real bed's argv** (the prologue of the amendment) is unit 5,
  10 and 11 work and cannot be exercised from here.

## Tool calls used

16 of the 25 allowed.
