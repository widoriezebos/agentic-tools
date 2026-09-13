Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-13

# Goal

Round 11 of chain gawr-build1, the last build round before this chain
lands. Two things, both named by the closing read of round 10
(gawr-carry1-read1-r3). Build on the worktree as it stands: it carries
rounds 6 to 10 and sits on the tip, so the design page
metasystem/plans/goal-abandoned-with-a-reason-design.md is revision 6.
Do not commit.

# 1. GAWR-C1-11: counselor ownership after abandonment (revision 6)

Build what revision 6 decides in its section 5 rows 28 and 29 and the
paragraph that follows them. In short: both counselor ownership readers
in metasystem/internal/landing/observe.go must also read the abandoned
map of the supplied accepted ledger, keeping every check they make today
(the operation-id match, exact equality with the carried-landings line,
the human-carried chain filter, the finding-to-commit comparison and the
accepted-risk line validation). Live and done matching stays unchanged.
An archived waiver still writes its counselor line through the existing
append in metasystem/cmd/metasystem/goalsync_mutations.go, owned by the
original goal; no successor and no later landing's goal inherits it.
These rows read evidence and grant no landing authority to an abandoned
goal. Build the fixture revision 6 names, in the file it names.

The reader's evidence: observe.go:232 and :259 iterate only the live and
done maps; the check runs for every carried candidate through
ValidateCarriedCandidatePaths in metasystem/internal/landing/carried.go.
A probe passed both lines with the goal live and refused both with the
goal abandoned ("carried counselor line ... has no equal carried row" and
"accepted-risk counselor line has no equal human-carried accepted-risk
row").

# 2. GAWR-C1-12: two fixtures assert less than the page's rows

TestAbandonKeepsReviewDebtReachable and TestAbandonedCarriedReviewCanBeWaived
in metasystem/internal/goal/carry_lifecycle_test.go do not assert what
the page's section 13 rows spell out. Extend them so they prove, as the
rows say: an abandon of a goal that carries a frozen stop fence, with the
fence lines byte-identical afterwards; a new carry refused with
carry-debt-unpaid while the debt is open; the next carry permitted after
discharge; a discharge attempted with the wrong chain refused; and the
ordinary obligation left open by a human-carried waiver. The behaviour is
already correct, so these are assertions, not new behaviour.

# What the round must satisfy

- `go test ./internal/landing/ -count=1`, `go test ./internal/goal/ -count=1`
  and `go test ./cmd/metasystem/ -run 'Goal|Abandon|Reopen|Carry' -count=1`
  pass. If your sandbox cannot run a whole package, say so as a gap with
  the focused selections green; the seat runs them outside it.
- The fast gate passes.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json)
with `{command, observed, level}` evidence replayable from the worktree's
repository root, one per command above. whatWasDone names both findings
and the code and fixtures that answer them. Every path in your return
starts with `metasystem/`.

# Constraints

Wall clock: 60 minutes. A partial round returns with its tests green for
what exists and names what is left.

# Gap Rule

stop and report a gap; never fill it silently.
