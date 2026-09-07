Working Mode: design
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal seat-mutual-awareness)
Date: 2026-09-07

# Revision 6 of the seat-mutual-awareness design: one fold, SMA-C-25

Goal seat-mutual-awareness (tier 3). Contract:
metasystem/plans/goals/seat-mutual-awareness.md. The design on main is
revision 5, metasystem/plans/seat-mutual-awareness-design.md (commit
3f782639). The fifth review,
metasystem/records/misc/seat-mutual-awareness-critique-r5.md, left one
material finding, SMA-C-25 reopened once more, and nothing else.
Revision 6 folds exactly that. Ruling A stands. Do not touch what
converged.

# The fold

Section 8's rollout confirmation (design lines around 728-760) admits
three forms that the traced code shows are written at the MINT stage,
before the new steward runner is launched and verified: the re-arm
component line, the aggregate re-arm fact, and the arming-log line
(metasystem/internal/up/up.go around lines 562-573 writes the fact and
the log at the minted stage; lines 586-596 can return the re-armed
component followed by a steward-runner failure;
metasystem/internal/steward/runner.go around lines 614-645 sets the
minted stage and the re-armed status before opening, preparing and
launching the runner). The existing rearm-launch-fails scenario in
metasystem/scripts/agents/supervision-fixtures.sh (around lines
1049-1075) proves those lines coexist with a failed up. Section 8 also
treats the presence record's engine field as a confirmation although
that record names no enrolled generation.

1. Admit exactly ONE class of confirmation: proof of the LIVE runner
   at the new generation. Concretely: the steward-runner line of
   metasystem health with its enrollment provenance (alive only when
   the running tick's own evidence carries the enrolled generation),
   and, if you keep an up-based form, only a successful up invocation
   whose re-arm fact is PAIRED with a successful steward-runner result
   for the same generation (name the exact record and field pair; a
   mint-only component, aggregate or log line is rejected by name).
2. The presence record's engine field is not a confirmation of
   readiness for slices 1 and 2; say what it is (the presence writer's
   own build) and where it may be read (slices 3 and 4, as an
   observation, never as the gate).
3. The fixture SMA-F-REBUILT-BYTES-NOT-REARMED gains the third state:
   up ran, the identity was minted, the runner launch FAILED (reuse
   the rearm-launch-fails injection); the mint-stage lines are
   present, and the gate is NOT satisfied. Keep the two states it has.
4. Dispositions table for round 5 at the end (C-25 accepted, the
   section that answers it). Self-grade updated. Title bumped to
   revision 6.

# Constraints

Wall-clock budget: 30 minutes. DESIGN-BEARING reach; the deliverable is
the one file, edited in place. Cite only paths that exist under
metasystem/. Gap rule: if no existing record proves the live runner
generation and one must be added, say so with the smallest record
written out, and stop.
