# Round 2 read of fixture-children unit 2: closure of U2-1 only

Worktree: /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/e67ca063-900a-46ec-9bc8-9fbdcbc3765e/scratchpad/g18/wt-fcu2
Throwaway copy used for every mutation: .../g18/tw-round2. The worktree was not edited except this file.

## U2-1: CLOSED

The exe-under-go-tmp route is present at `fixture_survivors.go:132-137` and is exactly the
amendment's third condition, not broader. Predicate, quoted:

    for _, observation := range unreadable {
            _, underGoTmp := goTmpRoot(observation.exact.Exe)
            if observation.exact.ExeKnown && underGoTmp || sharesFixtureScope(observation, certain) {
                    result = append(result, observation.survivor(FixtureSurvivorUnreadable))
            }
    }

`goTmpRoot` (lines 162-172) walks the exe's ancestors and returns true only when some ancestor
directory is named exactly `go-tmp`; it is the same anchor the ownership-record route already
used, now lifted out of `fixtureOwnershipRecord` (which calls it at line 176). It is not "any
temporary directory": `$TMPDIR` and Go's own `go-buildNNN` trees do not match. The amendment,
`metasystem/plans/fixture-children-cannot-outlive-their-test-design.md:570-572`, reads
"unreadable in both slots and in a group or session led by a dead-owned tagged process or with
an exe under `go-tmp`" - the implemented clause is that sentence. The gate still requires both
slots unreadable and signalable (line 124), so the route widens only which of those
observations get reported, never what enters the set.

Mutation run (mine, in tw-round2): removing only the two added lines, leaving
`if sharesFixtureScope(observation, certain)`, made
`TestFixtureScanClassifiesGoTmpUnreadableProcess` fail with
`scan = [...Pid:701...], <nil>; want the unrelated go-tmp process in the unreadable class` -
pid 701 only, 702 omitted, exactly the builder's claim. The new test gives 702 pgid 702 / sid
802 against 701's pgid 701 / sid 801 (`fixture_survivors_test.go:104-106`), so the group and
session route cannot explain the pass; only the exe predicate can. Restored file:
`go vet` clean, `gofmt -l` empty, `go test -race -count=1 ./internal/identity` ok 1.649s.

## New material findings

None.

## Other checks in this round

3. **Still never kills.** The only signal in either file is `unix.Kill(int(pid), 0)` at line 60,
   a liveness/permission test that delivers nothing. `grep -n 'Kill|Signal|SIGKILL|SIGTERM|Terminate|Reap'`
   over both files returns that one line. Nothing consumes the result to act; both scans only
   build and sort `[]FixtureSurvivor`. N-1 from round 1 still stands as the structural
   improvement (the API cannot stop a consumer looping over both classes), unchanged by this fix.
4. **Neither safety rule was disturbed**, proved by re-running all three round-1 mutations against
   the fixed file in tw-round2:
   - dropping `case Alive` -> `live-owner scan = [...two survivors...], <nil>; want no results and an error`, FAIL.
   - dropping `case Unknown` -> `unknown-owner scan = []identity.FixtureSurvivor(nil), <nil>; want no results and an error`, FAIL.
   - key scan matching on `EncodeRef(candidate.Owner)` -> both children returned for key A,
     `want child A only`, FAIL. The full-key comparison at lines 65-72 is still load-bearing.
   The fix touched only the unreadable-emit block and the `goTmpRoot` extraction; the guards at
   lines 80-85 and the matcher at 69-72 are byte-identical to round 1.
5. **Changed lines: 327** (`fixture_survivors.go` 192 + `fixture_survivors_test.go` 135, both new
   files, no existing file touched). Under the seat's raised 350. Not a finding.

## Non-material notes new to this round

- **N-8.** The anchor is any ancestor named `go-tmp`; design 3.5 (line 226) and every real
  specimen in `metasystem/records/misc/fixture-children-live-specimens-2026-09-16.md` are
  `metasystem-build-cache/go-tmp/<Test>/...`. Round 1's N-4 rated this small because the record
  route needed a parseable `fixture-owner` file as a second gate; the new route has no second
  gate, so the anchor's looseness is now load-bearing on its own. Still small - `go-tmp` is not a
  name anything else uses, and the class never acts - but requiring `filepath.Base(filepath.Dir(cacheRoot)) == "metasystem-build-cache"`
  is a one-line tightening if unit 5 sees noise.
- **N-9.** A process whose ownership record parses to a *different* key falls through line 118
  (`ok && matches(key)` false) into the go-tmp route and is reported as `fixture-survivor?`,
  even though we positively know which other fixture owns it. It needs both slots unreadable as
  well, and nothing acts on the class, so this is report noise, not a defect; the amendment's
  clause is unqualified by key.
- **N-10.** The amendment's fourth clause, "a parsed tag whose owner probes Unknown", has no
  expression in unit 2: a tagged process whose key does not match is dropped at line 115. Both
  entry points are scoped to a key or to one owner ref, so an Unknown *other* owner is outside
  the question they ask; this belongs to the census (`TaggedSurvivors`, 3.5) in unit 6. Flagged
  so it is not lost, not charged to this unit.

## What I could not check

- **Linux.** Darwin seat again. The canonical-encoding argument and the procfs both-slot reads
  are still reasoned, not run; the go-tmp route itself is platform-neutral string work.
- **Real kernel.** `survivorPids`, `fixtureSurvivorProber` and `fixtureSurvivorScope` are stubbed
  in every test, including the new one, so no real process has ever been classified by this code.
  Unit 5 or `health` on the seat is the first execution against live pids.
- **`bash scripts/agents/go-gate.sh --fast`** not run (call budget). Build, vet, gofmt and the
  whole package under `-race` are green in the throwaway copy of the worktree tree.
- **Whether the round-1 file was otherwise untouched** could not be diffed: both files are new
  and untracked, so there is no committed round-1 revision to diff against. I compared the
  quoted line ranges from read-fcu2.md against the current file by hand instead.

## Tool calls used

7 of the 12 allowed.
