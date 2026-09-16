# Round 2 of U5a-2: verification of the three RV fixes

Diff now 286 insertions / 70 deletions over the same six files in
metasystem/internal/spend (plus read-u5a2.md, the findings file). Baseline proof re-run in
the worktree: `gofmt -l internal/spend` clean, `go vet ./internal/spend` clean,
`go test -count=1 ./internal/spend` ok (4.2s). Nothing outside internal/spend changed, so
the boundary still holds.

Method note: for each fix I reverted it in a throwaway copy of the module
(scratchpad/mut-u5a2, the worktree untouched) and re-ran the package, so "the test would
fail" below is an observed failure, not a reading.

## The three findings

- **RV-1 CLOSED.** `rawTokenClasses` carries `HasCacheRead` / `HasReasoning` and
  `missionClasses()` (reader_claude.go:31-45) emits `cachedInputTokens` /
  `reasoningTokens` only when set, so the map handed to `price` is key-for-key what
  `transcriptTokens` produced before this unit. Equivalence checked at each key:
  `inputTokens` = `Input + CacheCreation` restores `input + creation` exactly (integers,
  no float drift); `CacheRead`/`Reasoning` take the legacy values; the flags are pure key
  presence, and a key that is present but not a non-negative number still errors inside
  `transcriptTokens` (reader_claude.go:601-613, 615-621), so the flag is never reached on
  that path and the entry stays a gap exactly as before. `tokensFromMission`
  (measure.go:304-309) reads absent keys as 0, and `price` (measure.go:311-337) iterates
  only present keys, so for a model priced today money and all four token figures are
  unchanged, and for a model with no price `price` still returns `unpriced = 1` on the
  first missing rate. Failure-sensitive: making both branches unconditional fails
  `TestPartialClaudePriceTablePreservesLegacyMoneyAndTokens` (observed FAIL) — it pins an
  input/cached/output-only table at money 0.000041, priced 1, unpriced 0, tokens 20, and
  then the absent-`cache_read` case at 0.000036 / 15. Bed money still 67.911555.

- **RV-2 CLOSED, both orders.** reader_claude.go:458-470 inserts
  `} else if !measurableTranscriptRequest(retained) && measurableTranscriptRequest(request)`
  ahead of the max-output merge, and `measurableTranscriptRequest`
  (reader_claude.go:474-478) is false for missing/invalid usage, an unparsable timestamp
  and `Model == "<synthetic>"` — the three holders RV-2 named. Unmeasured first then
  measured: the real call takes the key whole and keeps its input, cache-creation,
  cache-read and reasoning tokens. Measured first then unmeasured: the guard is false, so
  control falls to the max-output branch, where a usage-less candidate fails
  `transcriptNumber(request.Usage["output_tokens"])` and the retained measured entry is
  left intact — no tokens lost in either order. Failure-sensitive: deleting the new clause
  fails `TestCallIdentityReproducesThePrototype` (observed FAIL). That test pins both
  orders' payloads — `measured-after-gap` at 40 tokens behind a usage-less sibling, and
  `measured-after-bad-stamp` at 15 behind an invalid-timestamp sibling whose output (6) is
  larger than the real call's (4), so the max-output branch cannot mask the loss — plus
  `UnmeasuredRequests == 1` for the lone `<synthetic>` entry, which still reports as a gap.

- **RV-3 CLOSED.** measure_test.go:550-565 now writes an authentic v1 payload as bytes: a
  JSON object with `"schemaVersion": 1`, `"delegateDigest": "v1"`, no `jobDigest`, and one
  `requests` entry whose usage is deliberately wrong (`input_tokens` 20,
  `output_tokens` 9999 against a transcript that says 2). It then asserts the ledger is 22
  and that the cursor left on disk is valid schema 2 with `retainedOutput == 2`. How I
  convinced myself it bites: I made the reader accept and reuse a v1 file (version guard
  `< 1`, the two `JobDigest == ""` guards neutralised) and the test failed with
  `LifetimeTokens: 10019` — 20 + 9999, the planted usage — so the assertion observes the
  reused bytes rather than a shape. The old flip-the-integer version could not: its payload
  was the just-validated v2 cursor.

## New material findings

None.

## New, not material (recorded, not actioned)

- The RV-2 fix diverges from the design sentence it implements: section 2.1
  (metasystem/plans/spend-fence-reports-tokens-per-model-and-cause-design.md:38-39) says a
  repeated key in the same file "keeps the first entry's fields and raises `output_tokens`
  to the maximum seen", with no exception for an unmeasured holder. The code is the better
  behaviour and is what RV-2 asked for, but the written rule now has an unwritten
  exception; U5a-3 and U5a-4 read this design. Same class as round 1's NM-1 — the seat
  should reconcile the sentence.
- The v1 step calls `Measure` directly instead of `assertMatchesFullParse`, so warm-equals-
  cold is no longer asserted at that step (round 1's vacuous version did assert it). Not a
  real hole: the corrupt-cursor step immediately above (measure_test.go:534-539) drives the
  same full-rebuild path through `assertMatchesFullParse`, and the new step still checks the
  rebuilt cursor is valid schema 2.
- `TestPartialClaudePriceTablePreservesLegacyMoneyAndTokens` calls `missionClasses()` and
  `price()` directly rather than through `readSeat`, so it pins the class map and the money
  but not the `readSeat` wiring; that wiring is one line (reader_claude.go:173-176) and is
  covered by the bed. It also folds the second `rawTranscriptTokens` error into the final
  composite `if` instead of checking it at the call.

## R-115-m1e check: nothing weakened, nothing new shipped

Every pre-existing assertion is still present; no counter, gap or gate was relaxed to make
the fixes pass. The bed baselines the fixes ship are the same ones round 1 read
(`DayScope.Tokens` 55434018, `Seat.DayTokens` 336187, `Seat.LifetimeTokens` 336487, money
67.911555) and the cursor test's step totals are unchanged (42 / 42 / 75 / 22), so neither
fix moved a reported number. Gap counters only gain accuracy: a streaming snapshot that
shares a `message.id` with a measured call stops being counted as a separate gap, which is
the identity rule this unit exists to install, and a lone unmeasured or `<synthetic>` entry
still reports as `UnmeasuredRequests`. The two helpers the fixes add
(`measurableTranscriptRequest`, `retainedOutput`) are package-private and have no other
caller. No verb, health line, receipt field, zone, kind or cause work appeared.

## What I could not check

- `bash scripts/agents/go-gate.sh --fast` — not run (tool budget); gofmt, vet and the
  package tests were run green in the worktree.
- Live money: no prices are configured in this repository, so RV-1's closure rests on the
  class-map equivalence argument above plus the new unit test, not on an observed ledger.
- Whether real Claude transcripts actually emit an unmeasured entry ahead of a measured
  sibling sharing a `message.id` (RV-2's trigger); still judged from the code path.
- Round 1's NM-5 is untouched by the fixes and still open as a note: the design's health
  line and internal/steward/spend_fence_test.go:54-81 still quote the pre-unit bed numbers.
  The seat should confirm the new baselines; I did not re-adjudicate it here.
- I did not re-review the parts of the unit outside these three findings.

## Tool calls used

10 of 15.
