Working Mode: design
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal two-bars-for-changes)
Date: 2026-09-02

# Goal

Revise your design metasystem/plans/two-bars-caller-class-design.md
to revision 3 by folding the three ACCEPTED findings of critique round
2 (critic chain two-bars-cc-crit-3, round 2; dispositions with the
orchestrator's evidence in
metasystem/plans/two-bars-caller-class-dispositions-r2.md). Round 3 is
the declared failsafe round: rewrite the affected sections in one pass,
keep every line-and-file grounding, and re-run your reject condition.

# Workspace

The delegate worktree the dispatcher created for this job. Read
anything; write exactly one file, the existing
metasystem/plans/two-bars-caller-class-design.md (edit in place; mark
the header "revision 3" and extend the changelog with the three
finding ids).

# What revision 3 must settle — constraints fixed by the orchestrator

1. THE LANDING SIDE DOOR (TBCC-R2-LAND-SIDE-DOOR). The premise "a
   worker commit never lands" must be mechanically true.
   metasystem/scripts/agents/land.sh:244-252 invokes the wrapper
   without --push and then :265-316 fetch, rebase and push the CURRENT
   branch to origin, so a worker in its worktree can run the landing
   driver and publish the agent branch while the landing bar never
   fired. Fix by refusing land.sh inside a job worktree: at its start,
   the same geometry rule as the worker path's step 1 (toplevel,
   common dir, prefix; a toplevel under <main>/artifacts/agents/worktrees
   is a job worktree) refuses with one sentence naming the lawful path
   ("land refused: this is a job worktree; landings ride land.sh
   --chain from the main checkout"), exit 2, before any step runs.
   Decide whether land.sh calls a small engine verb for the geometry or
   reuses the wrapper's own rev-parse lines (commit.sh:141-142), and
   say why; land.sh leaves the non-goals list. Add a leg to the landing
   fixtures (metasystem/scripts/agents/land-fixtures.sh) proving the
   refusal from a staged worktree and the pass from a main checkout.
2. THE RUNNING PREDICATE (TBCC-R2-NONRUNNING-WORKER). Step 4 of the
   worker rule requires `J.status == "running"` exactly, citing the
   status vocabulary in metasystem/internal/dispatch/record.go:38-52
   (pending-setup and pending are live but not running; custody
   registration is permitted while pending, custody.go:40-42). Extend
   TestCommitAuthorityRefusesDelegateOutsideItsWorktree's table with
   pending, pending-setup, empty and unknown statuses, each refused.
3. THE TRAILER MONOPOLY (TBCC-R2-AMBIGUOUS-MACHINE-LINEAGE). The
   wrapper owns three trailers (Machine, Landing-Provenance,
   Landing-Provenance-Verdict; commit.sh:363-365). Specify: the inner
   half parses the message it is about to commit (the -F file or -m
   text, and the trailer block git would recognise) and refuses with
   `commit refused: the message already carries a wrapper-owned trailer
   (<name>); the wrapper stamps it` and exit 2 when any of the three
   appears, before the token is minted; the check is byte-exact on the
   trailer key and case-insensitive as git's trailer parsing is. Add a
   leg. State the fleet consequence plainly in the design: a landing
   message that hand-types `Machine: <nickname>` refuses from the day
   this lands (42 commits today carry one), and the receipt of the
   landing that ships this slice announces it.

Ground every new claim in file-and-line evidence from the worktree per
metasystem/docs/design/design-principles.md. Self-grade again.

# Constraints

Wall-clock budget: 30 minutes. Edit only the design file. Do not
weaken anything rounds 1 and 2 did not touch.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file.

# Gap Rule

stop and report a gap; never fill it silently.
