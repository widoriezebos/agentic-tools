# Round 2 of U5a-1: verification of the four round-1 findings

Scope: only RU-1..RU-4 and whether the fixes changed what ships. Worktree now 301 insertions / 59 deletions of code.
Baseline on the worktree: `go test -race -count=1 ./internal/spend` ok (1.97s), `gofmt -l internal/spend` clean, `go vet` clean.

Method for the "does the test catch a regression" questions: the module was copied to a scratch directory and
each fix was reverted there one at a time, then the guarding test was run. The worktree itself was never modified.

## RU-1 — CLOSED

All three fatal paths set it and `readSeat` returns before the prune.
`reader_claude.go`: git toplevel `result.fatal = true` after `recordUnreadable(repoRoot, err)`; home directory
`result.fatal = true` after the `~` gap; the walk callback's root branch
`if path == projects && !os.IsNotExist(walkErr) { result.fatal = true; return fs.SkipAll }`.
`readSeat` appends the gaps, then `if discovered.fatal { return nil, seat, unmeasured, nil }`, which is above
`seat.CacheWriteFailures += pruneTranscriptCursors(...)`. `NotExist` on the projects root still falls through to the
prune, which is the pre-change behaviour and is what `TestDeletedTranscriptCursorIsPruned` asserts in its second half.

The test does fail without the fix. `TestDeletedTranscriptCursorIsPruned` now writes a cursor with a first Measure,
removes `<root>/.git` (nothing above a t.TempDir has a `.git`, so `gitToplevel` fails), measures again, and asserts the
cursor survives. With the three-line `discovered.fatal` return deleted:

    measure_test.go:653: fatal transcript discovery pruned a live cursor: stat .../cache/f5004c8c...json: no such file or directory
    FAIL

Not covered by a test, verified by reading only: the home-directory path and the walk-root path. Both set the same
flag through the same return; the home path is unreachable in practice.

## RU-2 — CLOSED

`TestSeamKeepsEveryVisibleGap` asserts real counters, not shape. It checks `len(discovered.gaps) == 1` and
`discovered.gaps[0].Provenance == "seat unreadable"` and `discovered.counters.UnreadableFiles == 1` from a direct
`discoverClaudeTranscripts` call over a slug directory chmod'ed to 0, then from a full `Measure`:
`Seat.UnreadableFiles == 2`, `AgedFiles == 1`, `SkippedForeignFiles == 1`, `UnmeasuredRequests == 2`,
`CacheWriteFailures != 0`. Every one is a number, and the aged/malformed cases come from the real bed fixture.

Isolated mutation: with the unlistable-directory `recordUnreadable` in the walk callback replaced by `_ = walkErr`
(fix present, everything else clean), the test fails; unmutated it passes.

    --- clean:    ok   github.com/.../internal/spend 0.284s
    --- mutated:  --- FAIL: TestSeamKeepsEveryVisibleGap ... UnreadableFiles:1 (want 2), discovered.gaps empty

## RU-3 — CLOSED

`readSeat` now loops `for index := range readerRegistry`, taking `registered := &readerRegistry[index]` and skipping
on `!registered.inScope || registered.capability != readerCapabilityPerCall`. The prune stayed after the loop, and the
empty-registry panic is gone.

Label and measured scope agree for the registry as shipped: one reader, `claude`, per-call, in scope, label "claude".
The test proves the loop, not just the label: it appends a second reader with `inScope=false` and asserts
`Seat.Files == 2` and label "claude", then flips `inScope=true` and asserts `Seat.Files == 4` and label "claude+other".
Reverting the loop to index 0 only (`for index := 0; index < 1; index++`) fails it:

    reader_test.go:53: in-scope reader was not measured: seat={... Files:2 ...} label="claude+other"

One latent disagreement, not material here: `readerScopeLabel` filters on `inScope` alone while the loop also requires
`capability == readerCapabilityPerCall`. A future reader registered in scope with `readerCapabilityNone` would widen the
printed label without widening the reading. No such reader exists, `readerCapabilityNone` has no registration, and the
label still has no production caller; whoever lands the first non-per-call reader has to settle it.

## RU-4 — CLOSED as an over-counting fix; the builder's "confirmed a real overlap" is not reproducible

Nothing goes uncounted that the seat ledger counted before. Before this change the slug directory was listed with a
single non-recursive `os.ReadDir`, so no file under `<session>/subagents/` was ever discovered. The new
`delegates[fileSession] || delegates[file.parentSession]` skip therefore only re-hides files that the shipped ledger
never counted. Subagent files whose parent has no job record are counted, which is new coverage, not a loss.
The skip is guarded: removing `|| delegates[file.parentSession]` fails the test with
`seat={... LifetimeTokens:10 Files:2 ...}` where the fix gives `LifetimeTokens:3, Files:1`.

The overlap claim itself I could not confirm, and the test does not prove it. Across all of `~/.claude/projects`,
20 parent sessions have a `subagents/` directory, and zero of them match a `sessionId` in any job record under
`~/LocalStorage/GitHub/*/metasystem/artifacts/agents/jobs` (all repos on this machine, not just this checkout). So
there is no live instance of the skip firing, and no way here to compare a job record's `usage` against the tokens in
its `subagents/` directory. The test asserts the property by construction: it writes a job record with
`"usage":{"inputTokens":7}` for session `parent`, which is exactly the 3+4 tokens of the subagent file it then expects
to disappear from seat spend. That tests the skip, not the overlap.

Residual, for U5a-3's ownership rule rather than this unit: if a job record's `usage` does not in fact include the
tokens of the subagents that job launched, those tokens are invisible in both scopes. That is the pre-change baseline,
not something this unit introduces, so it is not a finding against this diff, but it should be settled where the
ownership rule lands rather than left implied by a fixture.

## New material findings

None.

## New non-material notes

- Discovery now takes every `*.jsonl` at any depth under a matching slug directory, not only `subagents/`. On this
  machine there are zero such files (the only other subdirectory kind is `memory/`, which holds no `.jsonl`), so the
  widening is invisible today, but it is unbounded by shape rather than by the `subagents/` convention the design names.
- `TestSeamKeepsEveryVisibleGap` depends on `chmod 0` and `chmod 0555` actually denying access, so it cannot pass as
  root. The gate runs as a normal user; a root container would see it fail.
- The test mutates the package-level `readerRegistry`. Safe as written (no `t.Parallel`, restored by defer and by an
  explicit reassignment), but it is order-fragile if a future test in this package goes parallel.
- `price(request.runtime, ...)` replaces the literal `price("claude", ...)`. With one registered reader named "claude"
  the priced rows are identical; the substitution only becomes observable when a second reader lands.

## What the fixes change about what ships

One real change, and it is the unit's intent rather than a defect: subagent transcripts are now seat spend. Under this
repository's slug directories there are 59 such files carrying 3,331,900 output tokens, and none of their parent
sessions matches a job record, so none of them is excluded by the new skip. The seat number the fence gates on will
step up by that whole amount on the first measure after this lands. Worth knowing before it is read as a regression.

## What I could not check

- Whether a job record's `usage` covers its subagents' tokens: no job record on this machine names a session that has
  a `subagents/` directory, so there is nothing to compare.
- The home-directory and walk-root fatal paths by execution; read only.
- `bash scripts/agents/go-gate.sh --fast`: not run.

## Tool calls used

13.
