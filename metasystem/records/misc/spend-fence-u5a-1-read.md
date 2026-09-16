# Read of U5a-1 (the reader seam and discovery)

Worktree: /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/e67ca063-900a-46ec-9bc8-9fbdcbc3765e/scratchpad/g16/wt-u5a1
Diff: 241 insertions, 59 deletions over 3 files, `transcript.go` -> `reader_claude.go` detected as a rename. Exactly at the 300-line cap.

Checks I ran here: `gofmt -l internal/spend` clean, `go vet ./internal/spend` clean, `go build ./...` clean, `go test -race -count=1 ./internal/spend` ok (3.4s).

Conformance in one line: the registry, the capability, the scope label, the typed `discoveryResult`/`scanResult`, the rename and recursive discovery with the delegate flag and parent session from the path are all there, and the parsing matches the real on-disk layout (`~/.claude/projects/<slug>/<parent-session>/subagents/<agent>.jsonl`, verified against 21 such directories on this machine: `parts[index-1]` yields the parent session id). Nothing from a later unit was built: no verb, no health line, no receipt field, no cache schema change.

## Material findings

### RU-1 — a failed discovery now deletes every transcript cursor

File: `metasystem/internal/spend/reader_claude.go:109` (with the early returns at `:182` and `:187`, and the walk error at `:197`).

Before this change `readSeat` returned on three fatal discovery paths *before* pruning: an unresolvable git toplevel, an unresolvable home directory, and a `~/.claude/projects` listing error that was not `NotExist` (old `transcript.go`: `recordUnreadable(...); return nil, seat, unmeasured, nil`). Now all three come back as an empty `discoveryResult`, the file loop runs zero times, and control reaches

```go
seat.CacheWriteFailures += pruneTranscriptCursors(repoRoot, visitedCursorPaths)
```

with `visitedCursorPaths` empty. `pruneTranscriptCursors` (`reader_claude.go:321`) removes every `<64 hex>.json` file in the spend cache directory that is not in `visited`, so one transient `git rev-parse` failure silently destroys the whole incremental transcript cursor cache. The next measure re-reads every transcript from byte 0 (multi-MB session files on this machine). The numbers stay right — that is what makes it silent — but the warm-read property the fence depends on is gone until the caches rebuild. This is an error-path behaviour drop introduced by the refactor, not a move: R-115 forbids it.

No test catches it: `TestUnresolvableGitToplevelIsSeatUnreadable` (measure_test.go:241) writes no cursors before it triggers the failure, and `TestDeletedTranscriptCursorIsPruned` (631) only exercises the success path.

Smallest fix: give `discoveryResult` a `fatal bool`, set it at `reader_claude.go:182`, `:187` and on the non-`NotExist` walk error at `:197`, and in `readSeat` return `nil, seat, unmeasured, nil` before line 109 when it is set. (`NotExist` on the projects root must keep falling through — pruning there is today's behaviour.)

### RU-2 — witness 6 was not built, and it guards the one path this unit rewrote

File: `metasystem/internal/spend/reader_test.go` (whole file — only `TestDiscoveryFlagsSubagentFilesFromPaths` is present).

The U5a-1 row of section 6 ends the unit on "witnesses 17, 6", and the brief's proof list names "the parts of witnesses 3 and 6 that section 7 places in this unit". Witness 17 is built. Witness 6 (`TestSeamKeepsEveryVisibleGap`: an unlistable slug directory, an aged file, a foreign file, a malformed usage line, a read-only cache directory reaching today's counters and entries) is absent, and the witness-3 fragment present is only the label check — the "its files unread" half is not exercised.

This is not a naming complaint. The unlistable-slug-directory path is exactly the code that changed shape, from `os.ReadDir(projects)` plus a per-slug `os.ReadDir` to one `filepath.WalkDir` with a `walkErr` branch and `fs.SkipDir` (`reader_claude.go:193-205`), and no existing test covers a permission-denied slug directory: the two existing unreadable tests cover a git-toplevel failure (measure_test.go:241) and a directory named `*.jsonl` (263). The rewritten branch ships unproven.

Smallest fix: add `TestSeamKeepsEveryVisibleGap` with at least the unlistable slug directory (chmod 0 on the slug dir, assert one `seat unreadable` entry and `UnreadableFiles == 1`) plus the aged/foreign/malformed cases. If the hard 300-line cap cannot hold it, the unit does not end on witness 6 and the omission has to be recorded and carried into U5a-2, not dropped.

### RU-3 — the registry is read by index, so the scope label can over-claim

File: `metasystem/internal/spend/reader_claude.go:59` — `registered := readerRegistry[0]`.

Nothing iterates `readerRegistry`. A second registration is invisible to `readSeat` no matter what it declares, while `readerScopeLabel` (`reader.go:50`) still joins its name into the printed scope. That inverts the guarantee section 2.1 states for the seam ("adding a reader in scope is a code change that changes the printed label, never a silent widening"): the label would widen while the reading did not, and witness 3's "an out-of-scope reader leaves its files unread" holds only by accident, because *every* reader past the first is unread. It also panics on an empty registry.

No behaviour differs today with one registration, so the orchestrator may reasonably close this as deferred to U5a-3, where the scope filter lands — but then the label and the loop must land together there.

Smallest fix, here, three lines: `for _, registered := range readerRegistry { if !registered.inScope || registered.capability != readerCapabilityPerCall { continue } ... }`, with the prune kept after the loop.

### RU-4 — a delegate's subagent transcripts now enter the seat ledger, and the flag that would stop them is unused

File: `metasystem/internal/spend/reader_claude.go:74` (`if delegates[fileSession]`), with `claudeTranscriptFile` at `:231-246`.

Discovery now computes `file.delegate` and `file.parentSession`, and no caller reads either field. The delegate filter still matches only the file's *own* session name. Before this change, files under `<session>/subagents/` were not discovered at all; after it, a subagent transcript whose parent session is a delegate/job session is counted as seat spend, because only the parent's top-level `.jsonl` is skipped by `delegates[...]`.

If a job record's `usage` already covers the subagents that job launched, the same tokens are now counted twice — once in the job scope, once in the seat scope — and the seat total is the number the fence gates on. I could not settle this: the checkout has no job records under `metasystem/records/`, so I could not compare a job record's usage against the tokens in its `subagents/` directory. Witness 17's own wording ("with no job record") suggests the design knows this case exists and leaves it to the ownership rule in U5a-3; if so, this closes as deferred, but it should close explicitly rather than by omission, because the exposure is live the moment this lands.

Smallest fix if the overlap is real: `if delegates[fileSession] || delegates[file.parentSession] { continue }` at line 74.

## Non-material notes

- `delegateSessionDigest(jobs)` is now computed once per transcript file inside `scanClaudeTranscript` (`reader_claude.go:250`) instead of once per seat read as before. Wasted work in the hot loop; small and bounded.
- Gap wording changed: "cannot list Claude transcript slug %s" became "cannot list Claude transcript path %s" (`:197`), and "cannot stat Claude transcript" became "cannot stat %s transcript" with the reader name (`:81`). Visible in the unmeasured report, equivalent in meaning.
- A directory named `x.jsonl` inside a slug directory is appended as a transcript file before the `entry.IsDir()` check (`:218-221`), then surfaces as an unreadable gap. The same shape existed before the change; it is now reachable at any depth.
- `discoveryResult.counters` is a whole `SeatSummary` of which only `UnreadableFiles` is ever read.
- The new test asserts file counts and path metadata but not that the subagent file's tokens reach `Seat.LifetimeTokens`; one more assertion would catch a "discovered but not measured" regression.
- `filepath.Rel` errors are dropped in both discovery (`relative, _ :=`, `:206`) and `claudeTranscriptFile`.
- `readerScopeLabel` has no production caller yet; its consumers land in U5a-3. In scope for this unit per the brief's "explicit scope".

## What I could not check

- Whether job-record usage overlaps subagent transcripts (RU-4): no job records in this checkout to compare against.
- `bash scripts/agents/go-gate.sh --fast`: not run here.
- Ordering of `Unmeasured` entries is normalised by the sort at `measure.go:262`, so the reordering the refactor introduces (scan gaps per file, then parse gaps) is not observable — checked, no finding.
- Double counting between a parent transcript and its `subagents/` files: checked on this machine's live transcripts and found none (0 `isSidechain` lines in the parent, 0 requestId overlap against 152 subagent requestIds). No finding.

## Tool calls used

13.
