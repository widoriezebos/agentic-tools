Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-13

# Goal

Round 7 of chain gawr-build1: fix four of the five material findings of
the rostered read of the carry (job gawr-carry1-read1). The fifth,
GAWR-C1-05, needs a design decision and is NOT in this round; leave that
behaviour as it is. Specification unchanged:
metasystem/plans/goal-abandoned-with-a-reason-design.md (revision 4).
Build on the worktree as round 6 left it; do not commit.

# The four findings and what each needs

GAWR-C1-03 (high). A human cannot abandon a claimed goal that sits in
trunk's landing slot: abandon clears Claimed, Obligation and Parked
(internal/goal/abandon.go (the file this chain adds)) but leaves the Landing record, and
the tree refuses a Landing record on a goal that is not claimed
(metasystem/internal/goal/file.go). A breach-stopped goal in the landing
slot can then be neither released nor abandoned, which is the wedge
abandon exists for. Clear the landing binding with the claim, the way
trunk's clearClaimBinding does in metasystem/internal/goal/verbs.go, and
test abandon of a goal that is open, claimed and land-ready.

GAWR-C1-04 (high). Abandoning a goal that a parked dependent waits on
fails for `--carried` and for `--waive`: abandon rewrites the dependent's
BlockedBy but not its `Parked blocker=` marker, and the tree requires that
marker to stay inside BlockedBy. Move the marker as trunk's split does in
metasystem/internal/goal/split.go, and lift the park where the blocker is
gone, as returnBlockerParks does for done in
metasystem/internal/goal/verbs.go. Test both forms and `--also`.

GAWR-C1-01 (high). The certified scenario abandoned-with-a-reason in
metasystem/scripts/agents/goal-cli-fixtures.sh cannot run on trunk: it
opens goals with neither --origin nor --blocks, which ruling R-93-m1e now
refuses (a seat opens only the defect that blocks its claimed goal), and
--origin human alone then needs human ratification. Rebuild the
scenario's setup under trunk's rules so it proves the same abandon
behaviour end to end, and run it.

GAWR-C1-02 (medium). The carry's list summary prints `abandoned=N`
between the done count and the tip, and trunk's archive-and-prune
scenario still greps ' done=3 tip='. Decide which side changes, keep both
behaviours proven, and run both scenarios.

Also GAWR-C1-06 (not material): the round added `git init` to the legacy
mutation subtest of metasystem/cmd/metasystem/metrics_verbs_test.go to
work around the sandbox. Remove it if the test passes without it here;
say so if it does not.

# What the round must satisfy

- `go test ./internal/goal/ -count=1` and
  `go test ./cmd/metasystem/ -run 'Goal|Abandon|Reopen|Carry' -count=1` pass.
- The goal-cli bed's scenarios abandoned-with-a-reason and
  archive-and-prune pass. Run them through the bed if your sandbox allows
  it; if a child Git process is denied, say so as a gap and give the
  focused Go evidence instead.
- The fast gate passes.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json)
with evidence items `{command, observed, level}` replayable from the
worktree's repository root, one per command above. whatWasDone names each
finding and the change that answers it.

Every path in your return starts with `metasystem/`.

# Constraints

Wall clock: 75 minutes. A partial round returns with its tests green for
what exists and names what is left.

# Gap Rule

stop and report a gap; never fill it silently.
