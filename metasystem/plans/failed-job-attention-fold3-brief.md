Working Mode: design
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal failed-job-attention)
Date: 2026-09-02

# Goal

Revision 3 of metasystem/plans/failed-job-attention-design.md (revision 2,
landed; edit it in place, bump the revision line): fold the four material
findings of metasystem/records/misc/failed-job-attention-critique-r2.md
by id. Two of the four are regressions the previous fold introduced, so
this revision removes those folds rather than patching them. The goal
record's resume recipe (metasystem/plans/goals/failed-job-attention.md,
Next step) is the direction; the design's own rule stands: the SIMPLEST
design honoring the existing episode pattern wins. Every closure is a
design change verified against the tree, never a softened claim; where
revision 2 promised what the tree cannot deliver, replace the promise with
a mechanism and name the replacement. Wido's order on this goal, verbatim:
"Designing is not enough, I need this to be fixed before you do anything
else... proven with tests"; the design phase's remaining distance is this
one honest fold.

# Workspace

The delegate worktree the dispatcher created for this job. Edit exactly one
existing file, the design; nothing else.

# Direction per finding

- FJA-R2-BIRTH-ABA-REMAINS: documenting the reused-identity suppression is
  not a closure. Either prevent it (the dedup key cannot collide on a
  lawful identifier reuse: read metasystem/internal/dispatch/record.go
  around line 60 for the immutable fields and decide what the key hashes
  today, without the birth token), or declare an explicit BlockedBy on
  goal job-record-birth-token (metasystem/plans/goals/job-record-birth-token.md)
  so the build cannot start before the token exists, and state the
  pre-contract upgrade rule so an absent token never creates a second
  episode for an old record. Choose one and say why.
- FJA-R2-TRANSITION-PHANTOM: drop the write-ahead journal everywhere.
  Derive every narrated transition on the next tick from committed state
  (episode present or absent, acknowledged or not), so nothing can narrate
  a raise or clear that never committed; rewrite the durable-write order
  in the section around lines 368-388 and fixture 12 accordingly, and
  cover the crash between state write and digest append by derivation,
  not by a second durable record.
- FJA-R2-PENDING-SNAPSHOT-RACE: one lock rule for queued notifications: the
  snapshot of the pending queue and its delivery happen under the same
  arbitration lock the tick holds (metasystem/internal/steward/tick.go
  around 110-114, runner.go around 131, steward_verbs.go around 270,
  notify.go around 64-93), or delivery re-checks file existence and
  episode state per item under that lock before sending; state the bound
  that then actually holds (exactly one stale delivery, or zero) and the
  fixture that proves it with two overlapping passes.
- FJA-R2-CHANNEL-MIGRATION-UNOWNED: name the owner and the end condition of
  the migration window between this design's private episodes and the
  alert channel's alert-prefixed episodes
  (metasystem/plans/alert-channel-design.md): who retires a private
  episode when the channel takes over the same standing record, and what
  bounds the duplicate; if the owner is the channel design, say what it
  must add and put that sentence in the goal-facing residual list.

Fold record: extend section 10 (or add section 11) mapping each FJA-R2 id
to its fold. Self-grade per the house rule.

# Constraints

Wall-clock budget: 40 minutes. Design only; edit nothing but the design
file. R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file.

# Gap Rule

stop and report a gap; never fill it silently.
