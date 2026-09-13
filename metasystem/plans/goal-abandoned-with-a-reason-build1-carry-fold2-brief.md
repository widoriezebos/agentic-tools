Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-13

# Goal

Round 8 of chain gawr-build1: the read of round 7 (gawr-carry1-read1-r2)
resolved GAWR-C1-02 and GAWR-C1-03 and left two material findings. Fix
both, and the cosmetic GAWR-C1-10 with them. GAWR-C1-05 stays out of this
round: it is a design decision. Specification unchanged:
metasystem/plans/goal-abandoned-with-a-reason-design.md (revision 4).
Build on the worktree as round 7 left it; do not commit.

# GAWR-C1-01 (high), the certified scenario still cannot run

The scenario abandoned-with-a-reason in
metasystem/scripts/agents/goal-cli-fixtures.sh now splits with
`--by Wido --fixture-human-authority`, but `goal split` does not define
that flag: metasystem/cmd/metasystem/goalsync_mutations.go registers it
only for approve, set-budget, accept-risk, open, edit, grant, revoke,
park and reopen, and runGoalSplit parses with parseSyncFlags("split"),
then calls the human-authority proof directly. The scenario stops with
"flag provided but not defined: -fixture-human-authority" (exit 2) before
any abandon behaviour runs; without the flag it stops with
SPLIT_RATIFY_REFUSED ... TERMINAL_NOT_REACHED.

Give the scenario a lawful setup that runs in the fixture bed and still
proves the certified abandon behaviour end to end through the real
binary: for example no split at all, or a main-origin parent split by the
lease holder. Do not add a fixture-only flag to a human verb. Then RUN
the scenario to the end and fix whatever further trunk rules it meets;
the reader notes its later steps have never run.

# GAWR-C1-04 (medium), the park repair still breaks two lawful abandons

The repair in internal/goal/abandon.go (the file this chain adds) returns
early unless the park's marker names a goal in the abandoned set, and
assigns the successor whenever one is given; a waived dependent's edges
are removed rather than re-pointed. Two lawful abandons go wrong:

1. The dependent's marker names a blocker that is already done and the
   waiver removes its last unfinished blocker: the dependent stays parked
   with every blocker done, until an unrelated done triggers trunk's
   sweep or a human unparks it.
2. An abandon that names `--carried` and also waives a parked dependent
   moves the marker to the successor although the waiver removed that
   edge, and the whole abandon is rejected with "Parked blocker=successor
   is not in BlockedBy".

Key the repair on the rewritten blocker list, as the reader says: move a
successor marker only onto an edge that was re-pointed, and lift the park
whenever no unfinished blocker remains, whichever blocker the marker
names. Trunk's rule to follow is in metasystem/internal/goal/verbs.go
(returnBlockerParks). Test both cases above, beside the three the fold
already covers.

# GAWR-C1-10 (not material), the park reason text

When the repair moves a marker, the park's reason text still names the old
blocker, so a record can read "blocked by b-one; returns when it is done"
while the marker names b-two. Update the text with the marker.

# What the round must satisfy

- `go test ./internal/goal/ -count=1` and
  `go test ./cmd/metasystem/ -run 'Goal|Abandon|Reopen|Carry' -count=1` pass.
- The goal-cli scenarios abandoned-with-a-reason and archive-and-prune
  pass through the real binary. If a child Git process is denied in your
  sandbox, say so as a gap and give the focused Go evidence instead; the
  seat runs the bed outside the sandbox.
- The fast gate passes.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json)
with `{command, observed, level}` evidence replayable from the worktree's
repository root, one per command above. whatWasDone names each finding and
the change that answers it. Every path in your return starts with
`metasystem/`.

# Constraints

Wall clock: 75 minutes. A partial round returns with its tests green for
what exists and names what is left.

# Gap Rule

stop and report a gap; never fill it silently.
