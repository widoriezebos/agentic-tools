Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-13

# Goal

You are the design author for goal goal-abandoned-with-a-reason. Decide
one question its build cannot answer, write the decision into the design
page metasystem/plans/goal-abandoned-with-a-reason-design.md (revision 4,
a tracked file in your worktree), and say in the page which fixture
proves it. The seat has decided nothing here; the decision is yours.

The page was written before trunk had the carry machinery, so section 4
deliberately leaves open review obligations on an abandoned record and
has no refusal for them. The rostered read of the carry found what that
now costs.

# The finding, as the reader wrote it

GAWR-C1-05 (high). Abandon hides trunk's carry debt. Trunk's carry machinery reads only live and done goals: the debt gate, the open-carry cap, the carry counts, and the lookup that closes a carried landing. Abandon moves the goal into the abandoned archive, so an open human-carried review obligation and any open carry word silently leave the gate that blocks further carries. The carried record for that word can then never be written, and the obligation cannot be discharged, because discharge-review-obligation reads live goals only (verbs.go:1749-1751). Trunk's done refuses both open review obligations and open carry words before a goal leaves the live set (verbs.go:1907-1914); the certified abandon, designed before carries existed, refuses neither. The remedy is a design choice (see gaps).

Evidence the reader cited: metasystem/internal/goal/verbs.go:3794-3816 (carryWords over Live and Done), :3773-3791 (CarryWordAt over Live and Done), :4108-4143 (CarryDebtAt obligations over Live only), :4042-4066 (CountCarries over Live and Done). abandon.go:263-417 has no carry check. In the probe, before abandon the gate reported 'carry-debt-unpaid' with Debt:1 and Open:1. After the confirmed abandon: no debt, zero open words, and CarryWordAt returned 'goal g is absent', while the obligation stayed open on the abandoned record.

# What the page must carry when you are done

- A decision for the open carry word and for the open review obligation
  on abandon, in the section each belongs to. The options the reader
  names are a refusal like trunk's `done` (which refuses both before a
  goal leaves the live set), or a rule that keeps the debt reachable
  after the goal is archived; choose, and say why in one or two
  sentences. If you choose a refusal, say what the human is told and what
  they do next (discharge, waive, carry), so a wedged goal still has a way
  out, which is what abandon exists for.
- What happens to a goal abandoned while it is a carry successor, and to
  a carried record whose successor is abandoned.
- One named fixture per decision, in the page's fixture section, with the
  file it lives in (an existing file, or a new file named in prose
  without the metasystem/ prefix).
- Keep every other decision of revision 4. Plain English, short
  sentences. Raise the revision number and say what changed.

# Workspace

Your job worktree, branch agent/<job>. Modify exactly one file, the
design page above. Touch nothing else. Do not commit. Do not write code.

# Constraints

- Every path you cite with the metasystem/ prefix must exist in the
  worktree. No globs. Do not run bin/metasystem test run, test plan or
  test verify.
- Wall clock: 45 minutes. Stop and report if the page is not finished.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json):
jobId, round, runtime, sessionId, model, evidence, gaps, mode, riskiestPart,
diffBoundary, whatWasDone. evidence carries exactly two `{command, observed,
level}` items, replayable from the worktree's repository root: (1)
`git -C metasystem status --short` observing the one modified file; (2)
`( cd metasystem/plans && wc -l goal-abandoned-with-a-reason-design.md )`
observing the line count. whatWasDone states the decision and names its
fixture. riskiestPart names what you are least sure of.

Every path in your return starts with `metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
