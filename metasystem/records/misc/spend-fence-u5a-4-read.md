# Read of spend-fence unit U5a-4 (causes, kinds, by-cause rows)

Worktree: /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/e67ca063-900a-46ec-9bc8-9fbdcbc3765e/scratchpad/g16/wt-u5a4
Base: 7dbe0195c. Diff: 287 insertions, 13 deletions over six files in metasystem/internal/spend (the hard cap is 300).

Checks I ran here: `gofmt -l internal/spend/` clean, `go build ./...` clean, `go vet ./internal/spend` clean,
`go test -race -count=1 ./internal/spend` ok (2.281s). Two of the builder's four mutation claims re-run below.

## Material findings

### RY-1: `attribution.kindMissing` is a lifetime count inside a windowed block

File: metasystem/internal/spend/attribution.go:111-114, with the synthetic per-file entry at
metasystem/internal/spend/reader_claude.go:132.

`buildAttribution` counts a delegate file's missing `Kind:` line outside the window test:

```go
if call.request.id == "" {
    if file.delegate && file.kindMissing {
        result.KindMissing++
    }
    for _, starter := range file.starters {   // this loop, and only this loop, filters on the window
```

Every other counter in the same function is windowed: `OutOfScope` and `Unknown` (lines 94-105) skip a job
record whose `startedAt` is outside `[from, to)`, and `CauseUnclassified` (line 126) sits inside the starter
loop that has already applied the window test. `KindMissing` sits above that loop and fires once for every
delegate transcript that discovery reaches, whatever its age: readSeat adds the synthetic entry at line 132
before the aged check at line 138, so a file whose last byte was written weeks ago still increments it.

Failure this causes: `attribution` is defined as windowed, not lifetime (design 2.5), and U5b-1 prints
`kind-missing=<n>` from it. On this machine the repo's own slug directory holds 71 delegate transcripts
(1208 across all slugs), and the design itself says the pre-S3 baseline delegates have no `Kind:` line. So the
figure reads about 71 and up on a day when none of those files spent a token, it never falls, and the gap it
exists to expose (today's delegates launched without a kind line) is unreadable inside it.

Smallest fix: count the file only when it reaches the window. Keep a `counted map[string]bool` in
`buildAttribution` and increment `result.KindMissing` from the per-call branch, once per `call.file.path`,
when that call has already passed the window test — the same place the calls of that delegate file are counted.

## Non-material notes

### RY-2: `pending` is reported as `missing`

reader_claude.go:396-402. Design 2.4 fixes a three-valued `kindLine`: `present`, `missing`, `pending` until the
first user message arrives. The implementation carries one bool and folds pending into missing:
`cache.KindMissing || cache.DelegateKind == ""`. A delegate transcript with no user entry yet (or one killed
before its brief landed) is therefore counted as a missing kind line rather than held as pending. In practice
a delegate's first line is its brief, so the pending window is momentary; and once RY-1 is fixed such a file
has no in-window call and drops out of the count anyway. Reported because the cursor schema drops a state the
design named, not because it moves a figure today.

### RY-3: the coordinator and peer sub-rules are evaluated in the design's reverse order

reader_claude.go:573-574. Design 2.3 rule 3 lists generic peer first (`origin.kind` peer, or the text starting
`Another Claude session sent a message`, or `<cross-session-message` in the first 400 characters), then the
coordinator sub-rule, and says first match wins; the reference script
(metasystem/records/misc/token-diagnosis-2026-09-15.py:54-55) has the same order. The implementation puts
coordinator first. For a cross-session message that carries both signals — `origin.kind` peer and the text
`The coordinator sent a message` — the design yields `peer` with an empty detail and this code yields `peer`
with detail `coordinator`. The cause is identical either way, so no by-cause row, no counter and no token
figure changes; only the detail string kept in the cursor differs, and that detail is not surfaced by any
reported field in this unit. The implementation's order arguably serves the design's own intent for the
detail better. Flagged so the seat picks one on purpose.

### RY-4: turns for engine files land on a different row than their calls

attribution.go:120-124 keys a turn by the starter's own cause for a non-delegate file and by
`delegate:<kind>` for a delegate file. An engine file is not `delegate`, and ownership is resolved per call,
not on the synthetic per-file entry, so an engine job's brief (`promptSource` sdk, which design rule 5 maps to
`human` with detail `sdk`) adds its turn to the `human` row while every call of that same turn is counted on
`delegate:<kind>`. Delegate files do not have this split because the code overrides their turn cause. So the
`human` row's `turns` and `calls` describe different populations on any seat that runs engine jobs, and
tokens-per-turn computed from that row is wrong. This follows the design's letter (an sdk brief *is* a `human`
starter under 2.3) and the design never says which row a non-main turn belongs to, which is why I am not
calling it material — but the two halves of this implementation answer the same open question differently.
If the seat reads "turns" as "the turns whose calls are on this row", this becomes material. Fix if wanted:
give the synthetic entry the reader's name as its runtime and resolve `jobs.owner(file.session, runtime,
starterStamp)` inside the starter loop, keying an owned starter the way the owned calls are keyed.

### RY-5: unclassified starters inside delegate files are never counted

attribution.go:125-127 gates `CauseUnclassified` on `!file.delegate`. The reason is visible: the design
withdrew the reference script's `delegate_brief` starter rule (py:49), so a delegate brief matches no rule and
falls to rule 8, and without the gate every delegate file would add one to the counter. The cost is that a
genuinely unclassifiable starter *inside* a delegate file — a resumed SendMessage delivery, which the
reference script records as a real shape — is invisible in the counter. The calls are never dropped: they are
counted under `delegate:<kind>` as the design requires. A cleaner shape would be a delegate-brief rule at the
head of the rule table, which the classifier cannot express today because `applyTranscriptLine` does not know
the file is a delegate.

### RY-6: a starter with an unparseable timestamp is silently dropped from turns

attribution.go:116-119. `parseTime` failure skips the starter, so no turn is counted and no unmeasured entry
is written, while its calls still count. A call with a bad timestamp becomes an unmeasured entry today; a
starter with one becomes nothing. Turns are new, so nothing today is displaced, and real user entries carry a
timestamp.

### RY-7: `roleKinds` is a mutable package global that a test writes

attribution.go:29 and reader_test.go:116-118. The U4 extension point ("U4 may append rows") is a package-level
map, and `TestKindDomainIsFourValuesBeforeAndAfterU4Rows` writes two rows into it and deletes them in a defer.
No test in this package calls `t.Parallel` (checked), so `-race` is clean today; the hazard is a future
parallel test or any runtime write. An append-at-init function or a copy in the test would remove it.

### RY-8: `roleKinds` is not in the file the design names

Design 2.3 says `roleKinds` lives in "the new internal/spend/attribute.go"; it is in the landed
attribution.go. The design's file name never existed on this branch — U5a-3 landed attribution.go. Naming
drift only.

### RY-9: three ledger fields are hidden behind an embedded unexported struct

attribution.go:63-75. `ByCause`, `KindMissing` and `CauseUnclassified` are added to `Attribution` through an
embedded `causeAttribution` rather than as direct fields, which reads as a way to fit the 300-line cap. JSON
flattens them, so the ledger schema is exactly what the design asks for, and go1.27 accepts the promoted field
in the `Attribution{...}` literal at line 87 (I checked: `go build ./internal/spend` is green). The cost is
that the ledger's own struct no longer lists its fields, and a keyed `Attribution{ByCause: ...}` literal in
another package now depends on the newer language rule.

### RY-10: the `queued_command` half of witness 4 is proved weakly

reader_test.go:104-106 feeds `{"type":"attachment","attachment":{"type":"queued_command"}}`, which
`applyTranscriptLine` rejects at the `type == "user"` gate before any classification runs, so the test cannot
tell a queued-delivery defect from any other non-user line. The behaviour is nonetheless correct against the
reference: the diagnosis script also handles `queued_command` only under `t == 'attachment'`
(py:168-172) and never passes it to `classify`.

## Answers to the six questions asked

1. Causes. Each rule of 2.3 is present and in the design's order except the coordinator/peer pair (RY-3), and
   the vocabulary is the closed one (attribution.go:11-21). An unclassified starter is `human` with detail
   `unclassified` plus the counter — a guess by the design's own rule 8, faithfully implemented — and a call
   before any starter is `unstarted`. Nothing is dropped: unclassified and unstarted calls are counted in
   their rows with full tokens (verified in `TestByCauseRowsCarryTurnsAndCalls` and by mutation D below).
2. Kind vocabulary. Four values, `design`, `build-read`, `critique`, `other`, no fifth (attribution.go:17-20);
   the pre-U4 map is exactly the design's, with `steward-continuation` absent and every unmapped role falling
   to `other` (attribution.go:29-35). `delegateKindLine` (reader_claude.go:550-558) trims the whole message,
   takes only up to the first newline, trims again, and requires the exact `Kind: <value>` form; it runs once
   per file, on the first user entry, so a later message cannot change it. A missing line counts, subject to
   RY-1 and RY-2.
3. Cache. A cached entry cannot carry a stale cause: `validTranscriptCursor` (cache.go:127-134) now rejects
   any cursor without a `Starters` list or with a request whose `Cause` is empty, so every pre-U5a-4 cursor is
   rebuilt from offset 0; a truncated or shrunk file rebuilds a fresh cache with empty starters
   (reader_claude.go:357-361); a foreign file clears starters as well (line 386). The grown path appends to the
   persisted starter list, so an appended assistant line sees the same latest starter a cold parse sees. Warm
   equals cold is what mutation D below breaks. One gap in the reset: the foreign branch clears Requests,
   Invalid, Tail and Starters but not `DelegateKind`/`KindMissing`, so a cursor on disk can keep a kind derived
   from content that was discarded — unreachable today, because a foreign file is skipped by readSeat before
   its metadata is used.
4. Mutations I re-ran (the two where a vacuous test would be hardest to spot by reading — the cache round trip
   and the rule table; the by-cause call count is asserted literally in the test source, so reading proves it):
   - Starter-cause propagation into cached requests (`if len(cache.Starters) > 0` at reader_claude.go:506
     forced to `if false`): `TestCursorV2MatchesAFullParseAndRebuildsV1` FAILS at measure_test.go:488, as
     claimed. `TestByCauseRowsCarryTurnsAndCalls` still passes under this mutation, which is expected — it
     builds its requests directly.
   - Stop-hook classification reverted to `human` (reader_claude.go:571):
     `TestCauseFollowsTheTurnStarterNotAQueuedDelivery` FAILS at reader_test.go:97 with
     `{1 ... human needs review}`, as claimed.
   Both mutations were applied to a copy-backed file and reverted; `cmp` reports the restored file identical
   and `gofmt -l` is clean, so the worktree is exactly as the builder left it (287/13 over six files).
5. R-115-m1e. Nothing landed by U5a-1 to U5a-3 is displaced. reader.go only embeds `transcriptMetadata` into
   `transcriptFile` and `scanResult`; reader_claude.go adds a user-entry branch that returns the same `false`
   the old `type != "assistant"` check returned, and adds `Cause` to the cached request without touching the
   identity, merge, foreign, tail, unmeasured or aged paths; the duplicate merge keeps the retained entry's
   cause, which matches witness 8's "earliest entry's fields"; cache.go only adds fields and two validity
   checks. No counter is dropped and no existing gap is hidden. One operational consequence worth stating in
   the landing note: the stricter validity check invalidates every existing cursor, so the first measure after
   landing reparses every transcript once (about 1200 files on this machine). The only new figure that is
   wrong is RY-1's.
6. Boundary. Clean. Six files, all under internal/spend, no verb, no router line, no health line, no receipt
   field, no landings, no `plans/` edit, nothing committed.

## What I could not check

- Live transcript shapes. I compared the rule table against the design and its cited reference script only; I
  did not classify a real transcript day and diff the causes against the script's own table.
- `bash scripts/agents/go-gate.sh --fast`, which the brief requires to run last. I ran gofmt, build, vet and
  the raced package suite, not the gate.
- The builder's other two mutation claims (removing `design` from the accepted kinds, disabling call-count
  accumulation). The by-cause call count is asserted literally at reader_test.go:141, so it is proved by
  reading.
- Whether U5b-1 prints `kind-missing` the way RY-1 predicts; U5b is not built.
- Other packages that unmarshal `Attribution`. `go build ./...` proves they compile; I did not run their tests.

## Tool calls used

21 of 25.
