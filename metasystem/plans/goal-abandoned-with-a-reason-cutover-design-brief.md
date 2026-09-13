Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-13

# Goal

You are the design author for goal goal-abandoned-with-a-reason. Its
landing-side slice is carried onto trunk and green except for one seam the
builder refused to invent. Decide it, write the decision into
metasystem/plans/goal-abandoned-with-a-reason-design.md (revision 6 in your
worktree), raise the revision, and name the fixture that proves it.

# The gap, as the builder wrote it

"The receipt-cutover scenario remains open. Trunk requires its
deliberately pinned older engine to judge the landing, but that engine
reports the certified slice's current landing-promotion record as
malformed. The brief provides no compatibility recipe deciding whether the
older engine, a newer observation judge, or a versioned promotion record
owns this seam, so I did not invent one."

What the scenario is for: in
metasystem/scripts/agents/land-fixtures.sh the receipt-cutover leg pins a
pre-cutover engine and has it judge a landing, so that a seat whose engine
is older than the candidate still lands lawfully. Its comment records the
precedent: the claim grammar grew episode keys on 2026-09-11 and the leg
was built so an older reader meets a claim record in the grammar it
writes. The landing-promotion record and its reader are
metasystem/internal/landing/promotion.go with
metasystem/scripts/agents/landing-promotion.json.

# What the page must carry when you are done

- A decision naming which side owns cross-engine compatibility for the
  landing-promotion record: the older engine tolerating a record it does
  not fully understand, the newer judge writing a record an older reader
  accepts, or a versioned record with a stated compatibility window. Say
  what an engine older than the record's version must do, and what it may
  refuse.
- What the seat sees when the two disagree: which exit, which message,
  and what the human does next.
- Whether the receipt-cutover leg keeps its pinned engine as it is, or its
  pin moves, and why. Do not weaken what the leg proves: a landing judged
  by an engine older than the candidate.
- One named fixture (an existing file, or a new file named in prose
  without the metasystem/ prefix).
- Keep every other decision of revision 6. Plain English, short sentences.
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
