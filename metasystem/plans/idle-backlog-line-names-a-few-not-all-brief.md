Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal idle-backlog-line-names-a-few-not-all, tier 3, hazard MECHANICAL)
Date: 2026-09-06

# Goal

The turn verdict's idle-with-backlog block (enforceIdleBacklog in
metasystem/internal/goal/turnverdict.go, the display line built near
the end of that function) joins EVERY claimable goal id into one line.
On 2026-09-06 the Stop message on m1c carried more than a hundred ids,
which nobody can act on and which drowns the OPEN WORK line above it.
The line exists to say that claimable work waits and to point at it.

When you are done, the line names the count and at most five goals,
the ones pinned to this machine first and then queue order, and points
the reader at the goal listing verb for the rest; the block-once
behaviour, the refusal count, the block source and the digest that
detects an unchanged backlog are untouched.

# The design

1. metasystem/internal/goal/project.go: ClaimableBudgetedWork gains a
   `Pinned map[string]string` (goal id to the machine nickname it is
   pinned to, only for claimable goals with a non-empty Pinned field).
   readClaimableBudgetedWork fills it from projection.Tree.Live in the
   same loop that appends to Claimable; readLegacyClaimableWork leaves
   it nil. The order of Claimable stays what Next produces (queue
   order).
2. turnverdict.go: a small function (for example
   `idleBacklogNames(work ClaimableBudgetedWork, machine string) string`)
   returns the display text: the goals pinned to `machine` first in
   Claimable order, then the remaining claimable goals in Claimable
   order, cut at five names; when more remain, append
   " and N more (metasystem goal list names them all)"; when five or
   fewer, no suffix. The machine is options.SeatActor.Machine; an
   empty machine means no goal is treated as pinned. The line becomes
   `IDLE WITH BACKLOG: <count> claimable goals await a live claim or
   job: <names>; <countText>; stop_hook_active=...; an attended human
   may run ...` with the count as a number; everything after the names
   is byte for byte the current text.
3. idleBacklogDigest is unchanged (it still hashes the full lists), so
   an unchanged backlog still counts refusals and a changed one still
   resets.

# Tests

- One test in metasystem/internal/goal/turnverdict_idle_test.go beside
  the existing idle tests, in their shape: seven claimable goals of
  which two are pinned to the seat's machine and one to another
  machine; the display names the count 7, the two pinned first, then
  three in queue order, then "and 2 more"; the other machine's pinned
  goal is not promoted. A second case with three claimable goals shows
  all three and no suffix.
- Existing tests keep passing: metasystem/internal/goal/turnverdict_test.go
  line 162 and metasystem/scripts/agents/supervision-hook-fixtures.sh
  line 790 only look for the words "IDLE WITH BACKLOG";
  metasystem/internal/report/stopblock_test.go uses its own literal.
  Change none of them.

# Workspace

The job worktree the dispatcher creates for you, branched from main.
May touch: metasystem/internal/goal/project.go
May touch: metasystem/internal/goal/turnverdict.go
May touch: metasystem/internal/goal/turnverdict_idle_test.go
Must not touch: the escalation path (escalateIdleBacklog), the digest, the steward's verdict readers, anything under plans or scripts.

# Constraints

- Never weaken a test. One round, at most 60 minutes of wall clock.
  Hazard MECHANICAL: a display line's shape and one projection field.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `go test -count=1 ./internal/goal` (expected: ok)
- `go test -count=1 ./internal/steward ./internal/report` (expected: ok; they read the block's first words)
- `go vet ./internal/goal` and `gofmt -l ./internal/goal` (expected: clean)
- `git diff --stat` (expected: only the three May-touch files)

# Acceptance Criteria

1. With more than five claimable goals the line names the count, the
   seat's pinned goals first, at most five names, and the "and N more"
   pointer; with five or fewer it names them all with no pointer.
2. Refusal counting, the block source and the escalation at three are
   unchanged; the existing tests pass unmodified.

# Gap Rule

stop and report a gap; never fill it silently.
