# Read of U5a-2 (call identity, cursor schema 2, classes, digest, job index)

Diff read: 228 insertions, 70 deletions over six files in metasystem/internal/spend,
all inside the declared boundary (no verb, no health line, no receipt field, no zone,
kind or cause work). Proof re-run here: `gofmt -l internal/spend` clean,
`go build ./...` clean, `go vet ./internal/spend` clean,
`go test -race -count=1 ./internal/spend` ok (4.95s).

## Material findings

### RV-1 — every call now carries `reasoningTokens` and `cachedInputTokens`, so a partly priced model loses its money and gains `Unpriced`

File: metasystem/internal/spend/reader_claude.go:32-38 (`missionClasses`), consumed at
reader_claude.go:166-170; the pricer is metasystem/internal/spend/measure.go:311-337.

Before this unit the seat path priced `transcriptTokens(request.usage)`, whose map holds
`cachedInputTokens` only when `cache_read_input_tokens` is present in the entry and
`reasoningTokens` only when `thinking_tokens` is present (reader_claude.go:585-598).
`missionClasses` now always returns all four keys:

```go
return map[string]float64{
	"inputTokens": classes.Input + classes.CacheCreation, "cachedInputTokens": classes.CacheRead,
	"outputTokens": classes.Output, "reasoningTokens": classes.Reasoning,
}
```

`price` refuses the whole call when any key in the map has no configured rate:

```go
for field, value := range classes {
	class := classNames[field]
	rate, ok := settings.Prices[config.SpendPriceKey{Runtime: runtime, Model: model, Class: class}]
	if !ok {
		return 0, 0, 1, foreign
	}
```

Prices are per-class keys `spend.price.<runtime>.<model>.<input|cached|output|reasoning>`,
each set independently (internal/config/spend.go:92-110, validate.go:688-712 check only the
class name and the value). A table that prices input, cached and output but not reasoning —
legal today, and the normal case since Claude transcripts carry no `thinking_tokens` —
priced every seat call before this unit and prices none of them after: `Row.Money` drops to
0 and `Row.Unpriced` rises by one per call. That is money the ledger reports today ceasing
to work (R-115-m1e), and no test can see it because this repository configures no prices at
all, so the bed is unpriced either way.

Smallest fix: carry presence in `rawTokenClasses` (two bools, set from `usage[...]` presence
in `rawTranscriptTokens`) and have `missionClasses` emit `cachedInputTokens` /
`reasoningTokens` only when the entry carried them, matching `transcriptTokens` exactly.
Add a price fixture with input/cached/output rates only and assert `Money > 0`,
`Unpriced == 0`.

### RV-2 — an unmeasured first entry claims the identity and silently eats a real call's input, cache and reasoning tokens

File: metasystem/internal/spend/reader_claude.go:448-465 (`applyTranscriptLine` merge,
`retainedOutput`).

```go
request.ID = key
if retained, exists := cache.Requests[key]; !exists {
	cache.Requests[key] = request
} else if output, ok := transcriptNumber(request.Usage["output_tokens"]); ok && output > retainedOutput(retained) {
	usage := make(map[string]any, len(retained.Usage))
	for name, value := range retained.Usage { usage[name] = value }
	usage["output_tokens"] = request.Usage["output_tokens"]
	retained.Usage = usage
	cache.Requests[key] = retained
}
```

Entries with no `message.usage` object are keyed and stored like any other (the bed's
`missing-usage` line, testdata/bed-20260902/transcripts/seat-session.jsonl:3, proves this
path reaches `cache.Requests` and then the gap list). So when the first entry for a
`message.id` has no usage (or model `<synthetic>`), it wins the key, and a later entry for
the same id — a real, fully measured call — contributes only its `output_tokens`. The stored
usage becomes a map that never existed in the transcript (`{"output_tokens": N}` when
`retained.Usage` was nil); `rawTranscriptTokens` then fails on it
(`transcriptTokens` requires `input_tokens`, reader_claude.go:563-569), so the call is
reported as one unmeasured request and its input, cache-creation, cache-read and reasoning
tokens vanish. Today's last-wins keyed by `requestId` counts that call in full, so this is a
silent undercount introduced by the unit — the exact failure class the goal exists to fix.
The `<synthetic>` variant is worse: the merged entry stays unmeasured by the new
`Model == "<synthetic>"` rule (reader_claude.go:500-508) no matter how much output the real
sibling had.

Smallest fix: let a measurable entry take the key from an unmeasured one — before the
max-output branch, `if retained.Usage == nil || retained.Model == "<synthetic>" { cache.Requests[key] = request; return false }`
— and add a fixture with a usage-less entry followed by a full one sharing a `message.id`,
asserting the full entry's total is counted once.

### RV-3 — the schema-1 rebuild test passes even if the rebuild is skipped

File: metasystem/internal/spend/measure_test.go:523-529, inside
`TestCursorV2MatchesAFullParseAndRebuildsV1`.

```go
cache.SchemaVersion = 1
if err := writeSpendCache(transcriptCursorPath(root, path), cache); err != nil { t.Fatal(err) }
if ledger = assertMatchesFullParse("schema-1 cursor"); ledger.Seat.LifetimeTokens != 22 {
```

`cache` here is the just-validated schema-2 cursor; only the version integer is flipped, so
its payload is exactly what a schema-2 read expects. An implementation that accepted version
1 would treat the file as unchanged, reuse it, and produce the identical ledger — both the
warm/cold byte comparison in `assertMatchesFullParse` and the `!= 22` assertion pass. The
test therefore proves nothing about the rebuild the brief names ("a schema-1 cache is
rebuilt") or about witness 7's "a schema-1 file forces a reparse". A real schema-1 file
differs in meaning: it carries `delegateDigest` and no `jobDigest`, and its `requests` map is
keyed by `requestId` with last-wins usage, so reinterpreting one would report the pre-unit
per-call numbers (the bed's 118425925 instead of 336187).

Smallest fix: write the v1 cache as v1 bytes — a JSON object with `"schemaVersion": 1`,
`"delegateDigest": <any>`, no `jobDigest`, and one `requests` entry whose usage is
deliberately wrong (say `output_tokens: 9999`) — then assert the ledger still matches the
full parse and the total is unchanged. That fails the moment a v1 cache is reinterpreted
rather than rebuilt.

## Non-material notes

- NM-1 — the digest encoding is not the one section 2.4 writes down. `jobDigest`
  (reader.go:86-95) hashes `json.Marshal([]string{path, id, sessionID, resumedSessionID,
  role, runtime, startedAt})` per record; the design specifies
  `path|jobId|sessionId|resumedSessionId|role|runtime|startedAt` with `-` for an absent value
  and lines joined by newline. Injectivity is equivalent (JSON quoting is unambiguous, the
  brackets delimit records) and nothing outside this package reads the value, so behaviour is
  right; but if a later unit reproduces the digest (for example `spend window`'s own
  cursors) it must copy this code, not the design's sentence. Either fix the design line or
  the encoding.
- NM-2 — schema 2 does not yet carry the fields section 2.4 lists for it (file-level `kind`,
  `parentSession`, `delegateKind`, `kindLine`, `lastCause`, `lastDetail`, `starters`, and
  per request `messageId`, `cause`, `detail`, the five raw classes). Correct for this unit —
  kinds are U5a-3 and causes U5a-4 — and the five classes are re-derived deterministically
  from the stored raw `usage` at read time, so warm equals cold. Forward hazard for the seat:
  U5a-3 and U5a-4 must bump `transcriptCursorSchemaVersion` again, or a cursor written by
  U5a-2 will be read as "no kind, no cause" and warm will stop equalling cold.
- NM-3 — `readerJobs.bySession` and `readerJob.record` have no consumer in shipped code;
  only the new `TestJobDigestCoversOnlyOwnershipFields` reads `bySession`. Harmless (U5a-3
  reads them), but note `bySession` appends in directory order and is not ordered by
  `startedAt` then `jobId`, which the ownership rule of 2.2 will need.
- NM-4 — the reported id of an entry with neither `message.id` nor `requestId` changed.
  `request.ID = key` (reader_claude.go:451) stores the absolute transcript path plus line, so
  `UnmeasuredEntry.ID` is now `/Users/.../seat.jsonl:12` where `readSeat` used to print
  `seat.jsonl:12` (reader_claude.go:145-148); that basename fallback is now dead code. The
  design does say the identity is `<file>:<line>`, and `UnmeasuredEntry.File` already carries
  the unrelativizable home path, so this is cosmetic.
- NM-5 — the bed baselines move by orders of magnitude: `DayScope.Tokens`
  173523756 → 55434018, `Seat.DayTokens` 118425925 → 336187. This is the design's own rule
  (2.1: a repeated key in one file "keeps the first entry's fields and raises
  `output_tokens` to the maximum seen") applied to a fixture whose two `seat-request`
  snapshots carry inconsistent input and cache values (1/2/3/4 then
  29343/1299328/116761073/336181), so the implementation is conformant and the arithmetic
  checks out. The seat should confirm the new numbers are the intended baseline:
  plans/token-spend-fence-design.md:187 still quotes the old health line, and
  internal/steward/spend_fence_test.go:54-81 still asserts 173523756/118425925 — that test
  builds its own `Ledger` literal so it stays green, but the two documents now disagree with
  the bed.
- NM-6 — `<synthetic>` handling is new behaviour, not moved behaviour
  (reader_claude.go:500-508 sets `detail = "synthetic model"`). For a synthetic entry with a
  well-formed usage object, today's code measures it as a zero-token request
  (`UnattributedRequests++`); now it becomes a gap (`UnmeasuredRequests++`). Section 2.1
  mandates it ("stay unmeasured requests, as today") and witness 8 requires it, so it is
  conformant, but the two seat counters shift for that shape.
- NM-7 — `jobDigest(jobs)` is recomputed, with a fresh sort and sha256 over every job record,
  once per transcript file (reader_claude.go:284-286). The removed
  `delegateSessionDigest` was called the same way, so no regression; hoisting it into
  `readSeat` would be free.

## Checked and found sound

- Identity order is `message.id`, then `requestId`, then `<path>:<line>`
  (reader_claude.go:441-451: `textOr(message["id"], textOr(raw["requestId"], ""))`, with the
  `path:line` fallback when both are empty). Precedence of `message.id` over `requestId` is
  pinned by `TestCallIdentityReproducesThePrototype` (the two `same-file` entries carry
  different `requestId`s and collapse); the `requestId` path is pinned by the bed
  (`TestSeatTranscriptDedupesByRequestId`, two lines with `requestId=seat-request` and no
  message id collapsing to 336187). No wrong-key merge found: the fallback key contains the
  absolute path, so line-keyed entries cannot collide across files.
- The same-file rule keeps the first entry's fields and raises `output_tokens` to the maximum
  (verified by hand against the identity test: first-model 12+3+8+5 = 28), and it holds
  across appends because the merge runs against the cursor's existing `Requests`.
- The cross-file rule keeps the earliest-stamped entry whole (`earlierStamped`,
  reader_claude.go:183-191, applied at reader_claude.go:120-124); the identity test's
  `cross-file` pair resolves to the 09:00 entry's fields (17 tokens), not the 12:00 one.
  A candidate with a parsable stamp beats a retained one with an unparsable stamp; equal
  stamps keep the first file visited, and file order comes from sorted directory reads.
- Warm equals cold is really tested: `assertMatchesFullParse` compares the written ledger
  bytes of an incremental read against a read with the cursor deleted, at every step of
  `TestCursorV2MatchesAFullParseAndRebuildsV1`, and measure_test.go:702's no-transcript-bytes
  warm read stays green.
- Total of the identity fixture is hand-computable and pinned: 28 + 17 + 3 + 5 = 53 with one
  gap, so that test is failure-sensitive rather than shape-only.
- `jobDigest` covers exactly path, jobId, sessionId, resumedSessionId, role, runtime,
  startedAt and excludes status and endedAt, and
  `TestJobDigestCoversOnlyOwnershipFields` changes each of the seven in turn and asserts the
  digest moves, plus status and asserts it does not. Failure-sensitive.
- The job index holds for a session several jobs touch: `add` appends to `records` and to
  `bySession[sessionID]` (no overwrite), and `referencedSessions` unions `sessionId` and
  `resumedSessionId`, which is exactly the old `delegateSessions` set, so the
  delegate-exclusion filter (reader_claude.go:95) is behaviour-preserved. An unreadable
  record is skipped before `add` (measure.go:166-175) exactly as before, still reported as an
  `unreadable` gap, and its disappearance from the digest forces a rebuild rather than a
  stale read — conservative in the safe direction.
- No dropped counter elsewhere: `Files`, `AgedFiles`, `UnreadableFiles`,
  `SkippedForeignFiles`, `CacheWriteFailures`, `UnmeasuredRequests`, `UnattributedRequests`,
  `LifetimeTokens`, `DayTokens` are all still filled on the same paths; the usage-error gap
  moved from `readSeat` into `transcriptRequestFromCache`'s `detail` and still lands as a
  gap with the same error text through the pre-existing `request.detail != ""` branch;
  `spendCacheSchemaVersion` stays 1 for the job cache and only the transcript cursor moves to
  2, with no writer left at the old constant.
- Boundary: all six files are in internal/spend; measure.go's only change is the index swap;
  no verb, health line, receipt field, zone, kind or cause work appears.

## What I could not check

- Whether a real Claude transcript ever emits a usage-less or `<synthetic>` assistant entry
  ahead of a measured entry that shares its `message.id` (RV-2's trigger). The bed has a
  usage-less entry but not a colliding pair; I judged the defect from the code path, not from
  observed transcript bytes.
- The live blast radius of RV-1: no prices are configured in this repository, so I could not
  observe money changing; the finding rests on `price`'s all-or-nothing rate lookup and on
  per-class config keys being independent.
- `bash scripts/agents/go-gate.sh --fast` — not run here (tool budget); the package's own
  gofmt, build, vet and `-race` tests were run and are green.
- Whether the seat intends the new bed baselines (NM-5) and whether any gate outside this
  package quotes the old ones beyond the two places grepped.

## Tool calls used

24 of 25.
