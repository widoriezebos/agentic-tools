Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal landing-receipt-survives-records-drift)
Date: 2026-09-09

# Follow-up brief: round 2 of chain lrsrd-build1 — the four gaps are answered

You stopped correctly. Round 1 wrote the two receipt canaries, observed
`TestReadTestReceiptSurvivesRegisterAppendAfterReceipt` red on the untouched
tree with the required text, confirmed the refusal canary green, and reported
four gaps instead of guessing. That evidence stands; keep the canaries as
they are. This round builds the rest.

The page is now revision 3.4, landed at 4fabfdb0 (sha256 2a0cc95129f35dbfdac573939f7e78d49691ee24166d04bfe72a114ce0787b4b); your
worktree may still hold revision 3.3, so the four dispositions are restated
here verbatim and they are binding. Where this brief and the page disagree, this brief restates the page and
the page wins.

## The four answers

1. **`DriftEntry`.** It is the drift output line as a value:
   `Kind string` (one of `untracked`, `staged`, `register-not-append`,
   `unstaged`), `Index byte` and `Worktree byte` (the two porcelain
   columns), `Path string` (toplevel-relative, as git printed it).
   `WorktreeDrift` returns `([]DriftEntry, []string, error)`; the second
   result is the tolerated register paths.
2. **The peer legs.** `leg_peer` is a plain clone of the leg's origin
   (`metasystem/scripts/agents/land-fixtures.sh` line 170) and is behind
   origin after the matching-receipt landing. Before each peer commit in the
   two new legs, run `git -C "$leg_peer" pull --ff-only origin main`, then
   commit and push. A non-fast-forward is never forced.
3. **The deadline that does not exist.** `fixture-bed-scenarios.sh` waits
   on each leg without a bound; the page's claim of a per-leg cap was false
   and is corrected. The leg bounds its own appender: start it with a
   recorded PID; it exits by itself after at most 60 seconds; install
   `trap 'kill "$appender_pid" 2>/dev/null' EXIT` in the leg before
   starting it, so a failed leg leaves no process. The shared runner is not
   touched.
4. **The exec rule.** The build brief said package landing never imports
   the standard library's process-execution package; that was the
   orchestrator's misstatement. The page's rule, which stands: every git
   call in `advance.go` (and `drift.go`) goes through the gittree
   operations named in 3e, none through the exec package directly.
   `receipt.go` and `tierone.go` keep the imports they have.

## What this round builds

Implementation map steps 1 through 9 of the page, in order, each step
leaving the fast gate green. The build brief you were dispatched with is in
your round-1 prompt; its essentials, restated so nothing depends on a file
your worktree may not hold:

- The wall: your `diffBoundary` is exactly the files the map's steps 1-9
  name plus the new files it creates (`registers.go`, `drift.go`,
  `advance.go`, their tests). No goal record, no `AGENTS.md`, no other
  script. The receipt's index tree and candidate tree stay exact; only the
  working-tree projection is filtered; the append-only rule keeps its
  behaviour. If a step cannot be done without crossing the wall, stop and
  say which.
- Canaries: the two you wrote in round 1 stay as they are; the fail-before
  evidence you recorded stands. The version-rule cases a-f, the drift rule
  matrix in both modes, and every advance test are written before or with
  the code they prove.
- Proving runs, from the module root: the landing selection the page
  names (`go test ./internal/landing/ -run 'TestCreateTestReceipt|TestReadTestReceipt|TestWorktreeDrift|TestAdvance|TestAppendOnlyRegisters' -count=1 -timeout=3m`),
  then `go test ./internal/gittree/ ./internal/lease/ ./internal/behaviorsurface/ ./internal/refusal/ ./internal/landing/ -count=1 -timeout=3m`,
  then `scripts/agents/go-gate.sh --fast` with the local module cache and
  a writable staticcheck cache, as you did in round 1.
- The shell beds cannot run in your sandbox; write step 9 to the page and
  stop there. Do not weaken anything to make them runnable.
- No round, slice or finding references in source comments. Keep the
  page's exact strings: refusal codes, the drift line grammar, land.sh's
  existing messages, the receipt's field names.
- Step 10, the crossover landing, stays the orchestrator's.

Return per the implementer schema with evidence rows at level `ran` for
the proving runs and the fast gate; `diffBoundary` exactly the map's files
plus the new ones. Gap rule unchanged: stop and report; never fill silently.
Wall-clock budget: 120 minutes.
