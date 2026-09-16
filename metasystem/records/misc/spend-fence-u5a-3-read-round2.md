# Read round 2: spend-fence U5a-3 (closures of RW-1 and RW-2)

Scope of this round: the two closures, plus the three cross-checks the seat asked for
(ledger figures and counters, R-115-m1e, unit boundary). Nothing else was re-reviewed.

Method: the recomputed diff (`git diff -M HEAD --numstat`), design section 2.2 and the
section 6 U5a-3 row, and four mutations run in a throwaway copy of the tree at
/private/tmp/claude-501/.../scratchpad/mut (the worktree itself was not edited). Baseline
of the four attribution tests: ok. Full `go test -race -count=1 ./internal/spend/...
./internal/config/...`: ok. `gofmt -l internal/spend internal/config`: clean.
`go vet ./internal/spend ./internal/config` and `go build ./...`: clean.

## The two findings

### RW-1 — CLOSED

reader_test.go:73-78 now asserts the whole row set by value:
`len(got) != 2`, then `got[0] != ModelAttribution{Model:"claude-opus-4-8", Input:4,
CacheCreation:1, CacheRead:2, Output:5, Reasoning:3, Total:15}` and
`got[1] != ModelAttribution{Model:"claude-sonnet-4-20250514", Input:9, Output:12,
Total:21}`. Struct comparison, so every one of the five classes and `Total` is exact, not
shape-only, and both rows are named.

Wrong-rather-than-absent, measured by mutation, not by reading:

- one class silently dropped (`row.CacheRead += classes.CacheRead` removed from
  `ModelAttribution.add`): FAIL, "model classes were not grouped and sorted:
  [{Model:claude-opus-4-8 Input:4 CacheCreation:1 CacheRead:0 Output:5 Reasoning:3
  Total:15} ...]". So a dropped class fails even though `Total` still reads 15.
- the emission deleted (`for _, row := range models { result.ByModel = append(...) }`):
  FAIL, "model classes were not grouped and sorted: []" — the builder's claim verbatim.
- a wrong model key needs no mutation: the fixture writes the raw string
  `Claude Opus 4.8` and the assertion demands `claude-opus-4-8`, so
  `config.CanonicalModel` is load-bearing for the test to pass. A model present only on
  the engine side is covered by that same row (opus appears only in engine.jsonl), and
  the two rows cross-check the kind rows: 4+1+2+5+3 = 15 = engine, 9+12 = 21 =
  main 14 + delegate 7.
- order: asserted exactly (opus at index 0, sonnet at 1). Detection of a removed
  `sort.Slice` is probabilistic, not certain — see N2-2.

### RW-2 — CLOSED on its headline clause; one sub-clause still open (non-material)

The closure is test-only. `resumed.json`'s `startedAt` moved to 22:15 and a fourth call
`resumed-only` at 22:20 now sits between that start and the real 22:30 owner start
(reader_test.go:87-89, 99). Mutation: adding
`jobs.bySession[job.resumedSessionID] = append(...)` to `readerJobs.add` in reader.go —
the plausible "fix for resumed sessions" the round-1 finding named — gives FAIL,
"shared calls were duplicated or assigned to the wrong start: previous=map[engine:2]
current=map[engine:6]". Exactly the claimed 4/4 to 2/6 flip, and the same mutation leaves
`TestKindsFollowPathsAndJobRecords` green, so this witness is the only thing catching it.

The seat's premise that this is a behaviour change is FALSE, and that matters for the
questions asked. Nothing outside reader_test.go moved between rounds:

- `readerJobs.add` (reader.go:66-80, the `bySession` index keyed on `sessionId` only) is
  landed code at HEAD and does not appear in the diff at all.
- attribution.go is byte-identical to round 1: every line number round 1 cited (43, 46,
  56, 76, 79, 84-91, 92-94, 109-136, 126-132, 138-145) still lands on the same code.
- reader_claude.go is identical too: round 1 cited the `!legacyOwned` guards at 106, 117,
  121, 126, 130, 133, 142, 150 and they are still at 106, 117, 121, 126, 130, 133, 142,
  150.
- deletions in the diff are 20, the same count as round 1, so no landed line was newly
  touched; all 8 added lines are inside reader_test.go.

So "ownership keys on the real session start rather than the resumed id" is what round 1
already reviewed, not a new behaviour. Checked against design 2.2 anyway:

- 2.2 states it in those words: "`resumedSessionId` never owns", "a record owns the file
  named by its `sessionId`", records ordered by `startedAt` with ties by `jobId`, each
  call to the latest start not after it, calls before the earliest to the earliest, and
  "Every engine call takes its owner's `startedAt` as its window stamp". attribution.go
  109-133 and line 77 (`kind, stamp = "engine", owner.start`) implement exactly that.
- ordinary, non-resumed case: one record per `sessionId`, so `owners` has one element,
  `chosen` is that element for every call, and no call can move to another job. A call
  can only move if two records share a `sessionId`, which is the resumed-follow-up shape
  2.2 describes and the new fixture covers.
- resumed once: parent `a`/`b` own `shared`; the child's record names `shared` only in
  `resumedSessionId`, so it is absent from `bySession["shared"]` and owns nothing there,
  while it still keeps the file out of `main` through `referencedSessions`. Verified by
  the mutation above, which is the only way to make it own.
- resumed twice: two children each carrying `resumedSessionId: shared` add nothing to
  `bySession["shared"]` (the loop that would have added them is the mutation, absent),
  so the split stays 4/4 whatever the children's starts are; a true resumed child whose
  `resumedSessionId` equals its own `sessionId` (design 2.2, hazard.go:499) owns only its
  own file. No ordering of two children can pull a parent call away.

Still open from RW-2's tail, and non-material: witness 2's "the sum equals a cold total"
and "no call twice" are only indirectly covered. The two windows sum to 8 = all four
calls once each, which is a real no-double-count check for distinct `message.id`s, but the
23:00 read is warm with no cold 23:00 baseline beside it, and no fixture repeats one id
across two files. Warm-equals-cold is U5a-2's ending witness and passed there; U5a-3's
ending obligation ("by-kind and by-model") is now proven. Not worth lines the cap does
not have — see RW2-3.

## New material finding

### RW2-3 — the unit is 8 changed lines over a cap the design calls hard

Design section 6: "Changed lines are `git diff --stat -M` on the landing, renames
detected, tests included. Each cap is hard; a builder who cannot fit stops and reports
rather than widening." The U5a-3 row's cap is 300.

`git diff -M HEAD --numstat` over the six code files:

    10   2  metasystem/internal/config/spend.go
     3   2  metasystem/internal/config/spend_test.go
   145   0  metasystem/internal/spend/attribution.go
     4   2  metasystem/internal/spend/measure.go
    32  13  metasystem/internal/spend/reader_claude.go
    94   1  metasystem/internal/spend/reader_test.go

288 insertions + 20 deletions = 308 changed lines. Round 1 was 280 + 20 = 300, exactly at
the cap, which is itself evidence the builder sized the unit by insertions+deletions. The
two closures added 8 insertions and no deletions, so the unit crossed a cap whose own text
says to stop and report instead of widening. Under an insertions-only reading it is 288
and fits; the design's phrase is `git diff --stat`, whose summary is both numbers, so I
read it as 308.

Failure it causes: not a code defect — a breach of the unit budget that the design makes
the builder's stop condition, decided silently. It is not fixable by writing more code:
either the seat records the widening to 308 (the closures are two proof fixes it asked
for), or 8 lines come out of the unit. My reading of the trade: the assertions bought
mutation-proven coverage of the unit's two ending deliverables, which is what the cap
exists to protect; the honest close is a recorded widening, not a trim that re-opens RW-1.

Note for the count: this excludes read-u5a3.md (176 insertions, staged at the worktree
root). Round 1's count excluded it too. If review records count toward the landing, the
number is 484 and the breach is much larger — see N2-3.

## Cross-checks the seat asked for

- Figures, counters, gates (R-115-m1e): nothing moved. Both closures live in
  reader_test.go; no production file changed between rounds (evidence above), so every
  ledger figure, every legacy seat counter and R-115-m1e stand exactly where round 1's N1
  left them. All five seat counters are still bumped (reader_claude.go:118, 127, 131, 133,
  155, 180, 189). No test was deleted, renamed or loosened this round: one exact
  assertion was added and one fixture gained a call. The fixture change to engine.jsonl
  (cache_creation 1, cache_read 2, thinking 3) raises the test's engine expectation from 9
  to 15 but touches no ledger figure — engine.jsonl is job-owned, so it stays out of
  `Seat.Files`/`LifetimeTokens`, and that test still asserts Files 1 and LifetimeTokens 3.
  Full race suite green for both packages.
- Boundary: the six changed code files are metasystem/internal/spend (attribution.go,
  measure.go, reader_claude.go, reader_test.go) and metasystem/internal/config (spend.go,
  spend_test.go) for the zone setting, plus the tests beside them. Inside the unit's
  boundary. No other package, no script, no plan, no record under metasystem/ was touched.

## Non-material notes

- N2-1: MUT C (resumed indexing) failed only `TestSharedSessionCallsHaveOneOwnerEach`;
  every other test in the package stayed green. The new witness is the single guard on
  that misattribution class, which is what witness 2 is for, but it means a future
  fixture edit there silently removes the only coverage.
- N2-2: order detection is probabilistic. Removing
  `sort.Slice(result.ByModel, ...)` and running `TestKindsFollowPathsAndJobRecords` six
  times gave pass, fail, fail, pass, pass, pass — Go randomizes map iteration, so with two
  rows a lost sort is caught about half the time (ByKind with three kinds would be ~5/6).
  The assertion is exact and the regression does surface, just intermittently. Hardening
  (a third model, or comparing against a sorted copy) costs lines the cap does not have.
- N2-3: read-u5a3.md is staged at the worktree root and therefore inside `git diff HEAD`.
  The u5a-1 and u5a-2 read records live at metasystem/records/misc/spend-fence-u5a-N-read
  [-round2].md. Before landing, move both round files there (or unstage them) so the
  landing carries no root-level markdown and the cap arithmetic is unambiguous.
- N2-4: every round-1 non-material note (N1-N13) is untouched by this round and still
  reads true; N14's cap warning is now the material RW2-3.

## What I could not check

- Whether the seat reads section 6's "witness 7 (kinds)" as satisfied. measure_test.go is
  still untouched: kinds and models enter the warm-vs-cold ledger comparison because the
  ledger grew, but no fixture there has a job record, so a kind or model regression that
  needs job records would not be caught warm-vs-cold. Recorded in round 1's "could not
  check" as well; I did not promote it, because U5a-3's ending obligation (by-kind and
  by-model) is now mutation-proven and warm-equals-cold is U5a-2's ending witness.
- `bash scripts/agents/go-gate.sh --fast`. I ran gofmt, vet, build and the two package
  suites with -race instead.
- Live behaviour. No `metasystem health` and no real seat measured.
- Whether the design's "changed lines" is insertions+deletions or insertions only. I state
  both numbers in RW2-3; only the seat can settle the reading.

Tool calls used: 11.
