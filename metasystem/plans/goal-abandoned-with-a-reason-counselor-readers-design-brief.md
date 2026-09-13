Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-13

# Goal

You are the design author for goal goal-abandoned-with-a-reason. Revision
5 of metasystem/plans/goal-abandoned-with-a-reason-design.md (landed
073d3078, in your worktree) decided that review debt survives abandonment
and listed, in section 5, the carry and review-debt readers that must
retain archived evidence. The closing read of the build found one reader
family the page does not name, and its own flows now break on it. Decide
it, write the decision into the page, raise the revision and say which
fixture proves it.

# The finding, as the reader wrote it

GAWR-C1-11 (medium). After abandonment, the landing observer refuses the
counselor register lines that revision 5's own flows produce, whenever
those lines ride a carried landing. Its two ownership readers match each
appended counselor line to a carried row or a human-carried accepted-risk
row on live and done goals only. The build made carried rows and archived
waivers first-class on abandoned records, but these readers still skip the
abandoned map. Two lines are affected: the carried-landings line written
when a carried record lands, if its goal is abandoned before that line is
committed; and the accepted-risk line that `goal accept-risk` writes on an
abandoned record, which by definition exists only after abandonment. A
carried landing whose candidate contains either line is refused with
record-not-owned; an ordinary held-goal landing can still carry them.

Evidence the reader cited: metasystem/internal/landing/observe.go:232 and
:259 iterate only the live and done maps; the check runs for every carried
candidate (observe.go:177-184, through ValidateCarriedCandidatePaths in
metasystem/internal/landing/carried.go:241); the command edge appends the
archived waiver's register line
(metasystem/cmd/metasystem/goalsync_mutations.go:812-815). A probe that
re-ran trunk's ownership cases passed both lines with the goal live and
refused both with the goal abandoned.

The reader's own note: extending those readers to abandoned goals is the
evident reading of the page's retained-evidence rule, but the page does
not say so. It also did not trace how often a carried landing's candidate
actually contains such a line, which depends on how
metasystem/scripts/agents/commit.sh stages the counselor register for a
carried landing.

# What the page must carry when you are done

- A decision on the landing observer's two counselor ownership readers
  for abandoned records, in the section where section 5's reader table
  lives, with the same care that table already takes: say what each reader
  must retain and what must not change for live goals.
- Whether the counselor register line written for an archived waiver is
  written at all, and if so who owns it, since the goal is no longer live.
- One named fixture, with the file it lives in (an existing file, or a new
  file named in prose without the metasystem/ prefix).
- Keep every other decision of revision 5. Plain English, short sentences.
  Raise the revision number and say what changed.

# Workspace

Your job worktree, branch agent/<job>. Modify exactly one file, the design
page. Touch nothing else. Do not commit. Do not write code.

# Constraints

- Every path you cite with the metasystem/ prefix must exist in the
  worktree. No globs. Do not run bin/metasystem test run, test plan or
  test verify.
- Wall clock: 40 minutes.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json)
with exactly two evidence items: `git -C metasystem status --short`, and
`( cd metasystem/plans && wc -l goal-abandoned-with-a-reason-design.md )`.
whatWasDone states the decision and names its fixture. Every path in your
return starts with `metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
