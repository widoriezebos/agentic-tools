Working Mode: build
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-hook-health-cost)
Date: 2026-09-06

# Goal

Goal stop-hook-health-cost (tier 3, approved by Wido at his terminal on
2026-09-06). Its record, metasystem/plans/goals/stop-hook-health-cost.md,
is the contract. In short: the Stop hook's health preview
(`metasystem health --hook-preview`, PreviewHealthAt in
metasystem/internal/steward/health.go) costs two seconds on the m1
checkout and a third of a second on a fresh one, and the hook pays it
at every turn end. DONE means the preview's cost is a small fraction of
the Stop budget, proven by a measurement in a test rather than by hand,
with the refusal path unchanged.

# What the seat found (read it, then verify it in the tree)

The cost is the spend fence. `checkSpendFence` calls `spend.Measure`
(metasystem/internal/spend/measure.go, from line 120), which on every
call reads every job record under the jobs artifacts directory and
prices each (JobUsageAt in metasystem/internal/mission/fence.go, line
778, which may read the round's usage file too), and then scans Claude
transcripts (metasystem/internal/spend/transcript.go): it lists every
directory under the user's Claude projects directory whose name STARTS
WITH the checkout's slug, reads every transcript file in them whole,
and JSON-decodes every line. Two consequences:

1. The prefix match is too wide. The m1 checkout's slug is a prefix of
   the m1b, m1c and m1d checkouts' slugs and of every delegate worktree
   under them: on this host that is 44 directories and tens of
   megabytes, re-read at every turn end. A fresh checkout with a
   longer slug matches two directories, which is the whole difference
   between two seconds and a third of a second. Lines from other
   checkouts are dropped by `seatCWD` (transcript.go line 227), but only
   after the whole file was read and decoded.
2. Even with the right files, the scan re-reads and re-decodes every
   byte every time, and the seat's own transcript grows with every
   turn, so the cost of every turn end grows with the session.

# The change

1. Per-role timing. `RoleVerdict` gains `DurationMillis int64` with json
   tag `durationMillis,omitempty`, set by `evaluateHealthRoles` around
   each check (and around the spend measurement). It lands in the
   health record and the verdict JSON, not on the health line. This is
   the profile the goal asked for, kept for good.

2. Transcript directory selection. A transcript directory is scanned
   only when its slug names the checkout's toplevel itself or a path
   below it: match `dir == slug` or `dir` starts with `slug + "-"` AND
   the directory is not another checkout's. Because slugs cannot be
   decoded back to paths reliably (a hyphen in a name and a path
   separator look alike), decide membership by content: read the first
   line of each candidate file that carries a `cwd` and apply `seatCWD`
   to it; a file whose first cwd is outside the toplevel is skipped
   without reading further (record it in the seat summary as
   `skippedForeignFiles`). Files of delegate sessions stay excluded as
   today.

3. Incremental transcript measurement with exact semantics. Today a
   file's requests are keyed by requestId and a later line with the
   same id replaces an earlier one; the ledger counts each id once.
   Keep exactly that result while reading only new bytes: a per-file
   cursor cache under the steward's spend artifacts directory (one
   JSON file per transcript, named by a digest of the transcript path)
   holding {path, size, modTime, offset, requests: id -> the priced
   fields the ledger needs, plus the invalid-line entries}. On measure:
   when the cached size is at most the current size, parse from the
   cached offset, merge new requests by id over the cached ones, and
   write the cache; when the file shrank or the cache is unreadable,
   parse from zero. The measured ledger for a given set of files must
   be byte-identical to today's full parse; a test proves it for an
   appended file, a request rewritten by id after the cursor, a
   truncated file, and a cache file that is corrupt.

4. Job record cache. Terminal job records never change: cache their
   priced measurement keyed by (path, size, modTime) in one JSON file
   in the same directory; a non-terminal record is read as today. A
   cache miss or a corrupt cache falls back to reading the record; a
   corrupt cache is rewritten, never trusted.

5. The measurement test the goal asks for, in the spend or steward
   package: a synthetic tree with 700 terminal job records, 40
   transcript files of 5 MB each in directories whose slugs share the
   checkout's prefix, of which 5 belong to the checkout, then
   PreviewHealthAt twice. The second (warm) call must finish under one
   second, asserted; the first (cold) time is printed, not asserted.
   The test skips under `-short`.

6. Nothing else: the health line text, the spend ceilings, the ledger
   publish rule ("only when its content changed"), the hook script and
   the refusal path stay as they are.

# Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l .` (empty);
`go test ./internal/spend/ ./internal/steward/ ./internal/mission/ -count=1` green,
the coverage floors in metasystem/docs/project-rules.md respected;
`bash scripts/agents/health-fixtures.sh` green where the sandbox
allows (say so if it cannot run; the seat reruns it at the landing).

# Constraints

Wall-clock budget: 90 minutes; return before it ends even if something
is red, naming it. Declare the boundary as every file that differs from
main. Never touch plans. Gap rule: stop and report a gap with your
proposed contract written out; the incremental cache's exact-semantics
rule above is the contract, so a place where today's semantics cannot
be kept is a gap to report, not to approximate.

# Expected Return

The implementer return per the role schema: `riskiestPart` first,
`diffBoundary` with every touched path relative to the repository root
(each starts with `metasystem/`), `whatWasDone`, `gaps`, and `evidence`
in the settled `{command, observed, level}` shape: the build line, the
package tests with the cold and warm times the measurement test
printed, and the fixture run.

# Acceptance Criteria

- The health verdict JSON carries a duration per role.
- Transcript files of other checkouts are skipped after one line.
- A repeated measurement over unchanged files reads no transcript
  bytes and no terminal job record (prove it with a counter or a
  fake filesystem in the test).
- The ledger is byte-identical to the full parse in the four cases
  named above.
- The warm hook preview under the synthetic volume completes under one
  second.

# Gap Rule

stop and report a gap; never fill it silently.
