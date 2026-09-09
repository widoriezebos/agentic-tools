Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal stop-refusal-fits-on-one-screen)
Date: 2026-09-09

# Review brief: the Stop refusal fits on one screen (chain one-screen-build1b-20260909)

FINDING IDS: chain-unique, OSR-01, OSR-02, ... never F-n.

Round budget: 1 focused round, then at most one correction and its
re-review (tier 3). R-60-m1's rule: material only if it changes what
gets built and names the artifact.

Threat model: a line that blocks the stop vanishing from the bounded
display (the unwatched-work line, an OPEN-WORK line, the idle line, a
goal line); the verdict line not first, or not exactly one line; a
run-warning class collapsed when it has one member, or a member lost
when it has many (the full-text file must carry every line the old
composeDisplay would have printed, in its old order); a recorded green
continuation (an instruction to the seat) summarised away; the rune
bound trimming the verdict line or the file-naming line instead of the
bottom of the actionable list; the full-text file written outside the
verdict's flock, or written non-atomically, or its write failure
changing ShouldBlock or BlockSource; the session slug in the file name
colliding across sessions; the system-message bound changing the
decision or the reason (it may only shorten the systemMessage); the
fence-aware planField missing a fence that opens with an info string,
treating an indented backtick line as a fence, or never closing at end
of file so a real field after an unclosed fence is lost; a change to
decideRuns's or decideGreens's conditions, to the green cursor, or to
ShouldBlock anywhere (out of scope by the brief, so any such change is
material); the hook fixture asserting the bound on the wrong field
(the reason, not the systemMessage) or passing without the 200 seeded
runs actually reaching the verdict. Out: the wording of summary lines;
the choice of 4,000 runes; taste.

Scope: the computed diff of the implementer job under review. It
touches ten files: report.go under cmd, turnverdict.go and its brain
test under internal/goal, two NEW files beside them (verdictrender.go
and verdictrender_test.go, created by the round), openwork.go,
stopblock.go and their tests under internal/report, and the
supervision hook fixture script under scripts/agents.
Contract: the build brief for this goal in the metasystem plans directory
(stop-refusal-fits-on-one-screen-build-brief.md, written this session and
not yet committed) and the goal record
metasystem/plans/goals/stop-refusal-fits-on-one-screen.md.
The implementer's own return is
metasystem/artifacts/agents/one-screen-build1b-20260909/rounds/1/return.md;
it states plainly which proofs its sandbox could not run (the hook
fixture suite, the combined package tests, the coverage floor); the
orchestrator replays those outside and their result reaches you as a
note in the follow-up if any is red.

# Mandate

1. The bounded display reads, in order: one verdict line; every
   actionable line (each still carrying whatever clearing command the
   old text had); the brain summary; one summary line per run-warning
   class present, with a single-member class printed in full; the
   greens as one line plus up to three recorded continuations in full;
   the line naming the full-text file. Within 4,000 runes, trimmed only
   from the bottom of the actionable list with a notice.
2. The full-text file under artifacts/agents/supervision/stop-verdicts/
   is the exact old display, written atomically inside the flock, one
   per session; a write failure is a diagnostic and a final line, never
   a decision change.
3. report stop-block's --system-message is bounded by the same
   constant, first line kept, notice appended; decision and reason
   untouched.
4. planField reads header fields outside fenced regions only, for every
   reader; a real field beside a fenced example is still read; an
   unclosed fence does not swallow the rest of the file silently (say
   what it does).
5. decideRuns, decideGreens, the cursor, ShouldBlock and BlockSource
   are unchanged in behaviour; the tests prove the shape with 200 seeded
   runs and one actionable line.
6. Nothing outside the ten named files changed.

If nothing material remains, say so; that closes the chain and the fix
lands.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
one-screen-build1b-20260909.

# Gap Rule

Stop adding at a gap that needs a decision no page has made; keep what
is written; report the gap with the resolution you propose. Never
delete written work.
