# Read: spend-fence U5a-3 (ownership, scope filter, zone day, by kind and model)

Reviewed: the computed diff (`git diff -M HEAD`, 280 insertions / 20 deletions over
six files = 300 changed lines, exactly at the hard cap), the brief, and design
sections 2.2, 2.5, the U5a-3 row of section 6, witnesses 1, 2, 3, 11 and the kinds
part of witness 7, plus the landed code the diff leans on (reader.go, the cursor
path in reader_claude.go, config/validate.go).

Ran: `gofmt -l internal/spend internal/config` (clean),
`go test -race -count=1 ./internal/spend/... ./internal/config/...` (both ok).
go 1.27.1.

Behaviour conforms everywhere I probed. Both material findings are proof gaps in
witnesses the brief names, not wrong numbers.

## Material findings

### RW-1 — the by-model deliverable has no test; deleting it leaves the suite green

File: metasystem/internal/spend/attribution.go:84-91 and 138-145
(`models[model]`, `ModelAttribution.add`); the only by-model assertion in the
tree is metasystem/internal/spend/reader_test.go:128, which asserts the row set is
EMPTY (`len(ledger.Attribution.ByModel) != 0`).

The U5a-3 row of section 6 ends the unit on "by-kind and by-model", and 2.5
specifies by model as five classes and a total. No test asserts a single model
row: not `Input`/`CacheCreation`/`CacheRead`/`Output`/`Reasoning`, not `Total`,
not the `config.CanonicalModel` grouping, not the sort. Witness 7
(measure_test.go:466-482) compares the whole serialized ledger warm against cold,
so it now covers attribution bytes, but it would compare two equally wrong
values, and its fixture has one model and no job records.

Failure it causes: the unit cannot be certified as delivering by-model. Remove
the `models` map, `add`, and the ByModel append entirely and every test in the
repository still passes. A real defect in the same lines (a class dropped, `Total`
counted twice, two models folded into one canonical key) ships silently and
surfaces only at U5b's per-model ceilings, where it will look like a ceiling bug.

Smallest fix: one assertion in `TestKindsFollowPathsAndJobRecords` on the fixture's
model row, naming the five classes and the total (the fixture totals are already
known there: 14 + 7 + 9 for the three kinds against a single canonical model), so
a dropped class or a doubled total fails.

### RW-2 — witness 2's resumed-session clause cannot fail

File: metasystem/internal/spend/reader_test.go:82-106, in particular the fixture at
line 99 (`resumed.json`, `sessionId":"other"`, `resumedSessionId":"shared"`,
`startedAt":"2026-09-02T22:00:00Z"`).

Witness 2 requires "a `resumedSessionId` alone owns nothing", and the brief lists
witness 2 as "the resumed-session shape". The fixture cannot show it. The three
calls are at 21:45, 22:30 and 22:45; the competing starts are a=21:30 and
b=22:30. Under `owner()` (attribution.go:109-136) the call at 21:45 takes the
earliest record (a), and both later calls take b, whether or not `resumed`
(22:00) is a candidate. I checked the behaviour by reading instead: `add()` indexes
`bySession` from `sessionID` only (reader.go:73-75), so `resumedSessionId` really
does not own. The rule is right and the witness does not prove it.

Failure it causes: the misattribution class the witness exists to catch stays
uncovered. If a later change indexed `bySession` by `resumedSessionID` too (a
plausible "fix" for resumed sessions), a fresh-context child would seize the
parent's calls from its own `startedAt` onward, restamping and relabelling them as
`engine` under the child's role, and this suite would stay green.

Also missing from the same witness: "the sum equals a cold total" and "no call
twice" are asserted nowhere (the second `Measure` at line 103 is a warm read, never
compared with a cold one).

Smallest fix: move `resumed.json`'s `startedAt` to `2026-09-02T22:15:00Z` and keep
the 22:30 and 22:45 calls; if `resumedSessionId` ever owned, the 22:30 call would
shift to the resumed record's start and the expected `engine` split would break.
A cold-total comparison costs more lines; note the cap below.

## Non-material notes

- N1 (checked, no action): the legacy stream is untouched. Every legacy counter and
  every legacy side effect in the rewritten block is guarded by `!legacyOwned`
  (reader_claude.go:106, 117, 121, 126, 130, 133, 142, 150), and `requests` is fed
  only for non-legacy files, so today's ledger stream, its crossings and its
  counters keep their values. R-115-m1e holds. The witnesses that guard this
  (witness 6, witness 7, measure_test.go's no-bytes-on-a-warm-read) pass.
- N2 (checked, no action): the suspicious-looking cursor sharing is safe. A
  legacy-owned file is scanned with `referencedSessions` cleared
  (reader_claude.go:113-115), and that map is applied while the cursor cache is
  built (`applyTranscriptLine`, reader_claude.go:460), so the cache contents differ
  between the two modes. It cannot leak into the legacy stream because
  `legacyOwned` depends only on job records, every job-record change changes
  `jobDigest` (reader.go:83-92), and a digest mismatch resets and reparses the
  cursor (reader_claude.go:330-345). Within one digest generation a file's mode is
  fixed.
- N3 (checked, no action): `result.foreign && !legacyOwned` (reader_claude.go:126)
  looks like it lets another repository's tokens into attribution. It cannot:
  whenever `foreign` is true, `readTranscriptCursor` returns nil calls (lines
  334-340 and 371-375, and `transcriptCursorSnapshot` line 508-510). Only
  `seat.SkippedForeignFiles` is not bumped, which is correct, because that file was
  not part of the seat stream before either.
- N4: new cost and one new invisible gap. Job-owned transcripts were skipped before
  the `os.Stat` and are now fully parsed, get cursor files written, and are marked
  visited so `pruneTranscriptCursors` (reader_claude.go:158) keeps them. Cost:
  a full parse of every job-owned transcript on the first pass after any
  job-record change. Gap: a cursor write failure for such a file is swallowed
  (line 117), so SF-6's "every visible gap is counted" does not extend to the new
  reads. Both are deliberate and both are needed to keep the legacy counters
  identical, so this is a note for the seat, not a fix.
- N5: witness 3 is implemented short of its text. reader_test.go:128 asserts the
  floor as `DayScope.Tokens`, but not the codex record's `day` crossing and not that
  it still sits in `Rows`, which the witness names. The brief's shorter phrasing
  ("with the floor kept") is met. One extra clause on the crossing would close it.
  The same test's `unknown=1` clause is real but its "gives zero tokens" half is
  vacuous: the home directory is empty, and a capability-`none` reader is skipped in
  discovery anyway (reader_claude.go:80).
- N6: witness 11 asserts both sides as required (`ledger.Day`, `Seat.DayTokens` 4
  old; `Window.Day`, `main` 2 new) but not the endpoints themselves. The window is
  half-open by reading: `stamp.Before(window.From)` excludes, `!stamp.Before(window.To)`
  excludes (attribution.go:56 and 79), `To` is `from.AddDate(0,0,1)` in the named
  zone (line 46). A call exactly at `from` or exactly at `to` is untested; witness 11
  names only 21:59 and 22:01, so this conforms.
- N7: `TestDiscoveryFlagsSubagentFilesFromPaths` (witness 17, U5a-1's ending
  witness) no longer exists as a name: its body was renamed to witness 1
  (reader_test.go:17) and extended. The assertions survive, and section 8's DONE
  commands name no test, so no gate breaks. The design lists 1 and 17 as separate
  witnesses and metasystem/records/misc/spend-fence-u5a-1-read.md cites the old
  name; either split them again or record the merge.
- N8: `ByKind` emits no row for a kind with no calls (attribution.go:92-94), so a
  quiet day yields fewer than three rows. Nothing in 2.2 or 2.5 requires a zero
  row; U5b's health split must not assume all three exist.
- N9: `location, _ := time.LoadLocation(zone)` (attribution.go:43) drops the error,
  and `now.In(nil)` panics. Unreachable today because `ReadSpendSettings` validates
  the zone (config/spend.go:81 and 129-131), and `validate.go:684-686` routes
  `spend.zone` through `checkFixed` because it sits in `spendFixedDefaults`. One
  line (`if err != nil { location = time.UTC }`) would make a future caller safe.
- N10: ownership boundaries are right. `chosen := &owners[0]` before the scan
  (attribution.go:126-132) gives a call before every start to the earliest record,
  and `!stamp.Before(start)` gives a call exactly on a start to that record; ties
  on `startedAt` break by `jobId` (line 120-125), as 2.2 says. A call in a file no
  record owns is `main`. A record without `startedAt` falls out of both counters
  (line 56) and stays unmeasured as today.
- N11: the continuation rule is proven and failure-sensitive. attribution.go:76
  keeps a `steward-continuation`-owned file as `main` and leaves its own timestamp
  as the window stamp; reader_test.go:69 expects main 14, delegate 7, engine 9, so
  counting the continuation as engine gives 20/3 and as delegate gives 18, either
  of which fails.
- N12: a call whose `message.id` appears in two files is counted once
  (reader_claude.go:139-141), but its kind follows whichever file won, and on an
  exact timestamp tie `earlierStamped` keeps the first walked. Deterministic for a
  given file set and within the design's file-level ownership, but worth watching
  when U5a-4 attaches causes to the same seam.
- N13: attribution.go stays inside the unit. It carries window, kinds, models,
  `outOfScope` and `unknown` only; no causes, no `kindMissing`, no landings, no
  verbs, no health, no receipt field. `measure.go` bumps the ledger to schema 2 as
  2.5 requires, and no consumer gates on schema 1 for this ledger (the other
  `schemaVersion` hits are the goal baseline, mission state and supervise ledgers);
  the terminal-job cache version (cache.go:14) is untouched, so nothing is
  invalidated.
- N14: the diff is exactly 300 changed lines against a hard 300 cap, so both fixes
  above need the seat's decision on what to trade. RW-1 is one `if` plus one
  `t.Fatalf`; RW-2 is a one-character fixture change (22:00 to 22:15) plus whatever
  the shifted expectation costs.

## What I could not check

- The brief's remaining proof: `go build ./...`, `go vet ./internal/spend`, and
  `bash scripts/agents/go-gate.sh --fast`. I ran gofmt and the two package test
  suites only.
- Failure sensitivity by mutation. I may not edit the implementation, so every
  "this test would still pass" claim above is from reading, except witness 1's
  continuation rule, where the fixture numbers make it mechanical (N11).
- Whether the seat reads "witness 7 grows with U5a-3 kinds" as satisfied by the
  ledger-byte comparison. measure_test.go is untouched; kinds and models now enter
  that comparison because the ledger grew, but no fixture there has a job record,
  so only `TestSharedSessionCallsHaveOneOwnerEach`'s second `Measure` exercises a
  warm read of a legacy-owned file, and never against a cold total.
- Live behaviour. Everything here is reading plus the two test commands; I did not
  run `metasystem health` or measure a real seat.

Tool calls used: 24.
