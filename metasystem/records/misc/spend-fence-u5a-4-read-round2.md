# Round 2 verification of spend-fence unit U5a-4

Worktree: /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/e67ca063-900a-46ec-9bc8-9fbdcbc3765e/scratchpad/g16/wt-u5a4
Base 7dbe0195c. Scope of this round: verify the two closures, the line figure, and R-115-m1e. Nothing else re-read.

All mutations were run in a throwaway rsync copy at
/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/e67ca063-900a-46ec-9bc8-9fbdcbc3765e/scratchpad/u5a4-mut.
The worktree was never written to. Its own hygiene re-checked after the fixes: `gofmt -l internal/spend/` clean,
`go vet ./internal/spend` clean, `go test -race -count=1 ./internal/spend` ok (2.558s).

## Closures

### RY-1 — CLOSED. The count is windowed and once per delegate path.

Placement is right. `metasystem/internal/spend/attribution.go:145-148` now sits *after* the window test at
line 142 (`if stamp.Before(window.From) || !stamp.Before(window.To) { continue }`), inside the per-call branch:

```go
if file.delegate && file.kindMissing && !kindMissingCounted[file.path] {
    result.KindMissing++
    kindMissingCounted[file.path] = true
}
```

The synthetic per-file entry (`call.request.id == ""`) reaches `continue` at line 127 and no longer increments,
so it is no longer once per file regardless of window. `kindMissingCounted` (declared line 109, keyed by
`file.path`) makes it once per path, not once per call; the real reader always sets a non-empty path
(`reader_claude.go:294`, `transcriptFile{path: path, ...}`), so the key cannot collide across files in production.
For a delegate file the `else if owner :=` branch at line 138 is never taken, so `stamp` at the window test is
the call's own request timestamp — the window test that gates the count is the delegate call's own.

Mutation A, run by me: I restored the lifetime form — put `if file.delegate && file.kindMissing { result.KindMissing++ }`
back inside the `call.request.id == ""` branch above the starter loop and removed the windowed block.
Observed: `--- FAIL: TestKindMissingIgnoresDelegateFilesOutsideWindow (0.00s) reader_test.go:153:
out-of-window file counted as kind-missing 1`. Exactly the builder's claim, 1 instead of 0.
`TestByCauseRowsCarryTurnsAndCalls` still passed under mutation A, as expected — its kind-missing file is in window.

Half the fix is unwitnessed; see NEW-1 below. The code is correct; the test is not what proves it.

### RY-10 — CLOSED. The repaired fixture reaches classification and separates queued from human.

`reader_test.go:103-106` now sends a real user record through the gate:

```go
applyTranscriptLine(&cache, []byte(`{"type":"user","promptSource":"queued","message":{"content":"queued"}}`), "x", 2, ".", nil)
...
assertSpend(t, len(cache.Starters) == 2 && cache.Starters[1].Cause == causeHuman && cache.Starters[1].Detail == "queued" && ...)
```

It reaches classification: `type == "user"` passes the gate at `reader_claude.go:478`, and the proof is empirical
rather than by reading — mutation C below changes the outcome from inside `classifyTurnStarter`, which a line
rejected at the gate could not do.

It distinguishes queued from human: `origin` is absent so `o == ""`, and the text `queued` prefix-matches no rule,
so with the `ps == "queued"` rule gone the line falls through to rule 8 and comes back `causeHuman` / `unclassified`
— a different detail, not a pass-for-both. Mutation C, run by me: deleted `, {ps == "queued", causeHuman, "queued"}`
from the rule table at `reader_claude.go:578`. Observed: `--- FAIL: TestCauseFollowsTheTurnStarterNotAQueuedDelivery
(0.00s) reader_test.go:104: queued delivery or unstarted call was classified incorrectly`. The builder's claim holds.

The original RY-10 subject — the `queued_command` attachment line — is still only weakly constrained, but it is
constrained: `len(cache.Starters) == 2` fails if the attachment line ever becomes a starter. That is the behaviour
at issue, so I am not reopening it.

## New finding

### NEW-1 (material: no) — the once-per-path guard is the load-bearing half of RY-1's fix and no test covers it

Mutation B, run by me: removed `&& !kindMissingCounted[file.path]` so the count fires once per call.
Observed: the whole package stays green — `ok github.com/widoriezebos/agentic-tools/metasystem/internal/spend 1.328s`
on `go test -count=1 ./internal/spend`. No test gives a kind-missing delegate file two in-window calls, so nothing
distinguishes once-per-path from once-per-call. In `TestByCauseRowsCarryTurnsAndCalls` the `missing` file has exactly
one in-window call (i == 7); in `TestKindMissingIgnoresDelegateFilesOutsideWindow` the one call is out of window.

Not material by the test: the landed code is correct (I read the guard and the key), no defect ships, no existing
test or gate was weakened, and the builder never claimed this half was witnessed. Worth saying anyway because in
production a delegate transcript has many calls, so this guard is the difference between `kind-missing=1` and
`kind-missing=<call count>` for the same file — the expensive half, proved only by reading. One extra in-window call
on `missing` in `TestByCauseRowsCarryTurnsAndCalls`, with `KindMissing == 1` unchanged, would close it.

## Line figure (question 3)

`git diff --stat -M HEAD -- metasystem` from the worktree root: **301 insertions, 13 deletions, 314 changed lines**
over six files, all under `metasystem/internal/spend`:
attribution.go 79/1, cache.go 16/2, measure_test.go 2/2, reader.go 4/2, reader_claude.go 129/6, reader_test.go 71/0.
The fix round added 14 insertions over the 287/13 (300) I recorded last round. 314 is inside the 330 the seat raised
this round to, so I am reporting it, not calling it a finding.

## R-115-m1e (question 4)

No. No figure the ledger reports today moved: `KindMissing` is new in this unit (`attribution.go:70`, inside the
embedded `causeAttribution`), so nothing that shipped before U5a-4 changed value. No counter was dropped — `ByKind`,
`ByModel`, `OutOfScope`, `Unknown`, `ByCause`, `KindMissing`, `CauseUnclassified` are all still assembled and emitted
(`attribution.go:63-75`, 160-172). The fix narrows `KindMissing` from lifetime to windowed, which is the definition
design 2.5 asks for and the point of RY-1, and it makes no gap invisible: a delegate launched today without a `Kind:`
line has in-window calls and is still counted. The one narrowing worth stating: a delegate file that spent nothing
in window, or whose every call has an unparseable timestamp (skipped at line 130), is no longer counted. That is the
windowed definition, not a hidden gap.

## What I could not check

- The delta of the fix round itself. I have no snapshot of the pre-fix tree, so I verified the current tree's
  behaviour against the two claims rather than diffing what the fix round touched. `reader_claude.go` carries 129
  insertions and I did not establish which of those, if any, are new this round.
- `bash scripts/agents/go-gate.sh --fast`. Same gap as last round: gofmt, vet and the raced package suite only.
- Everything outside the two closures, the line figure and R-115-m1e. Last round's non-material notes RY-2 through
  RY-9 were not re-read and are unchanged as far as this round can tell.

## Tool calls used

7 of 15, plus this write.
