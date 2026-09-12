Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal stop-message-truth)
Date: 2026-09-10

# Goal

stop-message-truth: the Stop message reflects the actual state of the
seat. Its record (metasystem/plans/goals/stop-message-truth.md) traces
the defects; this round builds the mechanical part the record names
plus two judgement defects seen on 2026-09-09/10. The printed shape is
not this goal's (it landed as d533caf17 and 536c74bd4): this goal is the
JUDGEMENT of what is open work.

# What to build

1. A terminal run is never hung. internal/report/scan.go sets
   RunFact.Hung from record.HungSince with no status check, so a run
   that once passed its stale window keeps the flag after it ends;
   decideRuns in internal/goal/turnverdict.go warns on Hung with no
   Acked check, so acknowledging does not silence it. Fix: Hung is
   false for any terminal status; a hung warning respects Acked; run
   prune's 14-day rule applies to hung-flagged terminal records like
   any other. Proof: on 2026-09-09 this seat printed 27 "looks hung"
   lines for runs that finished green in August; after this change the
   same records print nothing.

2. A held claim is not idle. The IDLE WITH BACKLOG verdict (turnverdict.go,
   the countText and detail near lines 448-500 and 590) counts a
   refusal toward the steward takeover whenever no delegate job of this
   checkout is running, even while the seat holds a claim and a landing
   receipt or proof attempt is live (2026-09-10: three refusals counted
   during a 50-minute landing receipt; the steward would have claimed
   at the third). Rule: the verdict is IDLE WITH BACKLOG only when the
   machine holds no claim AND no job runs AND no proof attempt of this
   checkout is live (the proof-run attempt records under the agents
   artifacts directory: an attempt with no terminal block and a start
   within its deadline) AND no
   landing lock is held. With a claim held and nothing in flight, the
   verdict says "CLAIM HELD: <goal id>; nothing in flight" and does not
   count a refusal; with a proof attempt live it says "PROOF ATTEMPT
   <id> running since <time>" under STILL WORKING. The three-strike
   counter resets when the backlog or the seat's claim changes, as
   today.

3. The brain summary keeps within the bound. On a brain seat the
   summary lines lead the display and are never trimmed (536c74bd4);
   their width is uncapped (every claimed goal id, ask and draft
   joined). Cap each brain line at the count plus the first three
   names and "and N more", so the whole display stays within the
   4,000-rune bound on a busy brain seat; the full text stays in the
   verdict file. Tests: a brain seat with forty claims renders within
   the bound and the file carries all forty.

Expected touched paths: internal/report/scan.go, internal/goal/turnverdict.go,
internal/goal/verdictrender.go (the brain cap only), their tests, and
scripts/agents/goal-cli-fixtures.sh only if a scenario asserts one of
these lines. Nothing in scripts/agents/supervision-hook.sh: the health
line's wrong root is another goal's (one-screen, parked behind m1c's
hook chain).

# Proof

internal/report, internal/goal, cmd/metasystem packages; gofmt, vet;
bash -n on any touched fixture. The orchestrator runs the goal-cli bed
and the hook suite on the seat. Report the round as your own.

# Constraints

Wall-clock budget: 40 minutes. Version-2 implementer return with the
complete diffBoundary. Stop at a gap that needs a decision no page has
made; report it with the resolution you propose. Never delete written
work.
