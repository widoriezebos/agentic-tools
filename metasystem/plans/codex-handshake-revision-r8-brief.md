Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal codex-handshake-budget-load-fragile)
Date: 2026-09-02

# Goal

Revision 8 of metasystem/plans/codex-handshake-design.md (revision 7
landed, in your worktree). Sol's Part 1 build (job ch-build-1c) delivered
D1.1 to D1.6 and stopped, correctly, at ONE specification gap in Part 2:
D2.3 makes the record builders refuse every `handshakeBound` but `launch`
or `progress`, so an omitted bound refuses too, yet section 4 names only two
tests in metasystem/internal/dispatch/decisions_test.go (:364, :447) as
callers to update and does not admit metasystem/internal/dispatch/provenance_test.go
at all, whose `BuildFollowRecord` call at line 51 omits the field and must
keep succeeding. Fold the gap, then the consistency pass over section 4,
section 5 and the round table. Edit in place; diffBoundary is that one file.
Keep it under twelve minutes; verify against the tree lines named here, no
wider reading.

# The fold

1. Section 4, the build.go row: name every successful `BuildRecord` and
   `BuildFollowRecord` call in metasystem/internal/dispatch/decisions_test.go
   as a caller that passes a bound, not just the two tests — the calls in
   `TestMissionProvenanceTuple` (lines 483 to 646 of the current file) pass
   `HandshakeBound: "launch"`, the two tests already named keep `"progress"`
   and their read-back assertion. The refusing calls (the ones that assert
   an error for another reason) may omit it or pass it; say which, and say
   why the outcome is the same.
2. Section 4, a new row: metasystem/internal/dispatch/provenance_test.go
   line 51, `BuildFollowRecord` gains `HandshakeBound: "launch"`; nothing
   else in that file changes.
3. Section 4, the cmd/metasystem row: state whether
   metasystem/cmd/metasystem/dispatch_callsite_test.go (it lists `job
   build-record` at line 56) pins the verb's argv or flag set such that the
   new `--handshake-bound` flag needs a row there; if it does, add it to the
   boundary with the exact change; if not, say so in one sentence.
4. Section 5: the pins above by exact test name; no new test is needed for a
   caller that merely passes the field, say so.
5. Header to revision 8 naming this round; the disposition table gains the
   row "ch-build-1c gap: D2.3 mandatory bound versus out-of-boundary
   callers".

Rule of record, unchanged: the bound is mandatory on every new record; a
record WITHOUT the field, created before the upgrade, reads as `launch`
(D2.3's last sentence). Do not weaken the refusal to make a test pass.
R-31: no benchmarks.

# Constraints

Wall-clock budget: 12 minutes. Design only; edit nothing but the design
file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the design file.

# Gap Rule

stop and report a gap; never fill it silently.
