Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-14

# Goal

Round 6 of chain gawr-build3, the last before it lands. The closing read
(gawr-build3-read1-r2) resolved C3-01, C3-02 and C3-03 and found one
material defect, GAWR-C3-08, in the same re-check. Fix it, and the
message nuance C3-10 with it. Specification:
metasystem/plans/goal-abandoned-with-a-reason-design.md, revision 7.
Build on the worktree as round 5 left it; do not commit.

# GAWR-C3-08 (medium), as the reader wrote it

"The push-time re-check `landing held` finds its range by walking first
parents from the pushed tip one git call at a time until it meets the
base. When the fetched base is not an ancestor of the tip, the walk does
not stop at the range; it runs back through history to the newest
first-parent merge, or to the root commit if there is none.
`commit.sh --push` meets this shape whenever origin moves after the commit
is made, which is common on main. On this repository the refusal takes
about 21 seconds, scales with history depth rather than the size of the
push, and names an unrelated old merge commit. The page's sentence
'<base> is not a first-parent ancestor of <tip>' never appears. The
decision (range-not-linear, exit 2, nothing pushed) is right; the cost and
the reason given are not."

Its evidence: heldRange in internal/landing/held.go (the file this chain
adds) spawns one `git rev-list --parents -n 1` per step and checks for a
merge before a root. A read-only probe on the real checkout, with a base
that had moved past the tip, took 20.9 seconds, walked 1,410 commits, and
refused naming a merge from 2026-09-10. On a merge-free history the walk
would cover all 6,855 commits.

# What the round must satisfy

- The re-check decides the range without walking history beyond it: ask
  Git for the range once (an ancestry test, or a bounded range listing),
  and when the base is not a first-parent ancestor of the tip, refuse with
  the page's own sentence naming the base and the tip, not an unrelated
  commit. Keep the decision unchanged: range-not-linear, exit 2, nothing
  pushed, and a real merge inside the range still refuses as a merge.
- The cost scales with the size of the push, not with the depth of
  history. Prove it: a test that fails on the old walk, and evidence of
  the time on a moved base.
- C3-10 (not material, fix it here): once one commit in a range is
  goal-free, the verdict's outcome stays goal-free for the rest of the
  walk, so a range whose later commits pass prints "goal-free ledger"
  instead of "ok N commit(s) above <base>". Report the outcome the range
  actually earned.
- `go test ./internal/landing/ -count=1` and
  `go test ./cmd/metasystem/ -run 'Land|Landing|Carry|Held' -count=1`
  pass, the land bed's 27 scenarios and the static re-proof bed stay
  green, and the fast gate passes.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json)
with `{command, observed, level}` evidence replayable from the worktree's
repository root, one per command above, plus `git -C metasystem status --short`.
whatWasDone names both findings and the change that answers each. Every
path in your return starts with `metasystem/`.

# Constraints

Wall clock: 60 minutes. A partial round returns with its tests green for
what exists and names what is left.

# Gap Rule

stop and report a gap; never fill it silently.
