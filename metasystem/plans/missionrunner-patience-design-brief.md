Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal missionrunner-terminate-flake)
Date: 2026-09-02

# Goal

Author the design document missionrunner-patience-design.md, a NEW file you
create in the metasystem plans directory: port the landed patience doctrine
onto the missionrunner terminate-group test family so load can never turn
slowness into failure.

# Evidence

- The goal record: metasystem/plans/goals/missionrunner-terminate-flake.md
  (in your worktree) — TestTerminateGroupLeaksNoGroupsUnderCompression
  failed under load in the first governed discharge (the package took
  1271 seconds on a quiet-held shared machine); the sibling
  TestTerminateGroupKillsThroughATermImmuneOwnedGroup fails at HEAD on
  this guest; the family flaps between tests with the same
  identity-proof-refusal shape.
- The doctrine: metasystem/records/patience/patience-attempts.md (in your
  worktree) — condition-based waits with progress-resetting failsafes,
  never fixed windows; and the two landed ports as precedent: the steward
  tick test (this seat's landing 65c36111: close-the-channel plus cleanup
  handshake plus progress-based deadline) and the R-35 law that failing
  when slow is a defect.
- The law: R-35-m3 — caps that convert load-slowness into failure are
  defects to fix with progress-based patience.

# Design questions

1. Read the failing tests in internal/missionrunner (and the flapping
   siblings the goal names): which waits are wall-clock assumptions, which
   assertions race teardown, and what is the PROGRESS SIGNAL each wait
   should key on (process table state? group membership counts? emitted
   events?).
2. The port: per test, the condition-based wait with its
   progress-resetting failsafe and its cleanup handshake on every exit
   path — mechanical enough that the implementer makes no judgment calls.
3. The identity-proof-refusal shape the goal names: is the family's flap a
   pure patience defect, or is there a real identity-comparison bug under
   load (the vm-epoch class) that patience would MASK? Answer from the
   test and code text; if masking is a risk, the design must separate the
   two and say what stays red.
4. The proof profile: repetitions under load on the 4-CPU guest, matching
   the steward-tick precedent.

Self-grade with reject condition.

# Constraints

Wall-clock budget: 25 minutes. Design document only; one new file in the
metasystem plans directory.

# Expected Return

Version-2 implementer JSON; diffBoundary lists exactly the one new design
file you created.

# Gap Rule

stop and report a gap; never fill it silently.
