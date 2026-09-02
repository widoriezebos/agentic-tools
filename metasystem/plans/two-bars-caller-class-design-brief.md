Working Mode: design
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal two-bars-for-changes)
Date: 2026-09-02

# Goal

Author a short design for the caller-class slice of goal
two-bars-for-changes (read metasystem/plans/goals/two-bars-for-changes.md
first; the slice is the paragraph beginning "NEW SLICE, THE REFUSE BIT
BINDS ON THE CALLER'S CLASS"). Wido's word, verbatim from ruling R-46-m1b
in metasystem/memory/rulings.md: the "two-bars caller-class slice" is
m1b's first claim, so that "the commit gate branches on the caller's
class so a worker-classified session can no longer commit on the human
branch". The audit that found the hole is
metasystem/records/misc/design-gate-audit-2026-09-02.md, section C1.

# Workspace

The delegate worktree the dispatcher created for this job. Read anything;
write exactly one NEW file, two-bars-caller-class-design.md, in the
metasystem plans directory.

# What the design must settle

1. THE BRANCH. metasystem/scripts/agents/commit.sh lines 8-17 ask
   `lease require-holder` and branch on whether the answer carries a
   claimEpoch: with one, the wrapper re-executes itself on the agent
   path (line 14); without one, on the human path (line 17), and lines
   23-30 then set agent_commit=0 so the landing refusal at line 319 never
   fires. metasystem/internal/lease/verbs.go RequireHolderAt (lines
   359-369) returns Holder:false with NO error and no epoch for a caller
   classified DELEGATE, ADAPTER-SUPERVISOR, or SUPERVISION; gateHolder
   (lines 480-482) passes those classes outright. Specify the branch on
   the reported CLASS: HUMAN keeps the sovereign human path unchanged;
   MAIN with a claimEpoch takes the agent path; every other class
   (DELEGATE, ADAPTER-SUPERVISOR, SUPERVISION, STEWARD, UNTRUSTED)
   refuses to commit at all with a typed message naming the class and
   the lawful path ("run metasystem up from the session with the
   steward armed, or commit from a person's terminal"). Decide WHERE the
   refusal lives: in the wrapper's bash, or in the engine as a commit-
   authority answer (a flag on require-holder, or one new lease verb the
   wrapper calls), and say why. Wido's standard is that behavior is
   enforced in Go; the wrapper may only relay a typed engine verdict.

2. RUN-HELD'S CALLERS. `lease run-held` shares gateHolder and therefore
   also passes DELEGATE. Enumerate every caller of `lease run-held` and
   `lease require-holder` under metasystem/scripts/agents and
   metasystem/cmd (grep them; cite file and line) and state, per caller,
   whether a DELEGATE-classified caller is lawful there (a delegate
   worker running a mission verb inside its own worktree may be). The
   design must not break a lawful delegate call: a changed contract runs
   its callers (ruling R-18 in metasystem/memory/rulings.md). If the
   commit-authority answer must differ from the run-held answer, say so
   and keep them distinct.

3. THE HUMAN PROOF STAYS. metasystem/internal/lease/classify.go decides
   HUMAN by "no recognised ancestor and a controlling terminal" (read
   the comment block before ClassifyAt and the terminal check near the
   end of ClassifyAt). State explicitly that the human path stays
   ungated by design, and name the residual this slice does not close (a
   person launching an unrecognised agent binary from a terminal) as
   out of scope, so the critic does not re-litigate it.

4. THE TRAILER'S LINEAGE. commit.sh line 363 stamps the Machine trailer
   as `<nickname>+${METASYSTEM_OWNER_LINEAGE:-human}`; agent landings
   from a seat whose environment lacks the variable therefore say
   "+human" (fifteen m0b landings today do). Decide whether the wrapper
   derives the suffix from the classification instead (MAIN -> its
   mainId; HUMAN -> human), so the trailer cannot claim a person for an
   agent landing. Same seam, cheap; say yes or no with the reason.

5. FIXTURES AND TESTS. metasystem/scripts/agents/static-reproof-
   fixtures.sh stages callers against commit.sh. Specify the fixture
   cases by name: HUMAN commits with no landing gate; MAIN with an epoch
   is gated and still refuses the promoted codes in
   metasystem/scripts/agents/landing-promotion.json; DELEGATE is
   refused with the exact message; the existing "claim epoch changed
   before the final mutation" refusal is unchanged. Name the Go unit
   tests for any new engine surface (package internal/lease) and the
   coverage floor rule they must respect (metasystem/docs/project-rules.md,
   Local Invariants).

6. NON-GOALS. This slice does not change
   metasystem/scripts/agents/landing-promotion.json, the never-direct-
   fix floor in metasystem/internal/landing/observe.go, or the
   register-carriage allowlist; those are the goal's other legs. State
   that plainly so the build stays inside the box.

Ground every claim in file-and-line evidence from the worktree per
metasystem/docs/design/design-principles.md; where the design cannot see
an implementer-private seam, say so rather than guess (three chains this
week stalled on exactly that, ruling R-39-m0). Self-grade per the house
rule: confidence, weakest claim, reject condition.

# Constraints

Wall-clock budget: 30 minutes. A small design: the branch rule, the
engine surface, the caller table, the fixture list, the non-goals. No
essay. Do not edit anything but the design file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file
named under Workspace.

# Gap Rule

stop and report a gap; never fill it silently.
