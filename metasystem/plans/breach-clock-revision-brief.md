Working Mode: design
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal breach-clock-and-budget-honesty)
Date: 2026-09-02

# Goal

Revise metasystem/plans/breach-clock-and-budget-honesty-design.md (revision 1,
landed uncertified; edit it in place, bump the revision line, and delete the
HANDOVER NOTE block at its top, which this revision supersedes) so that it
closes Sol's eight material findings in
metasystem/records/misc/breach-design-critique-r1.md and carries the second
specimen recorded on the goal (metasystem/plans/goals/breach-clock-and-budget-honesty.md,
Next step). Every finding is closed by changing the design, or refuted with
the file and line that refutes it. Never close a finding by softening a
requirement, weakening a refusal, or narrowing a guarantee: the goal's
standard is Wido's, verbatim, "hard deterministic machinery. This is Go
territory enforcing your behaviour". Verify every closure against the tree in
your worktree; the critic cited file and line for each finding, and your
closure must cite the same seams or explain why the citation is wrong.

# Workspace

The delegate worktree the dispatcher created for this job. Edit exactly one
existing file, the design; nothing else.

# The second specimen (add to the design's evidence and to Fix 2)

Wido's relayed resume of alert-escalation-channel on 2026-09-01 was typed as
--elapsed-limit 8h and recorded as elapsedLimit=1d, which enforces as 24 clock
hours: the human set an eight-hour fence and got a twenty-four-hour one. Same
root as the 9d-enforced-as-72h finding, in the favorable direction, which is
why it went unnoticed. Fix 2 must make a typed duration enforce exactly as
typed, or refuse at the moment it is set and say so. State which of the two
the design chooses for each input shape and why.

# The eight findings, and what closing each requires

- BCD-R1-001 (high): the parked-with-breach resume transition is unreachable
  through the shipped goal resume command, because
  metasystem/cmd/metasystem/goalsync_mutations.go (around lines 370-384)
  resolves a CLAIMED binding and locks on it before calling Resume, and
  metasystem/internal/dispatch/stop.go (around 53-59) rejects a parked goal
  before that. Specify the command-layer path for a parked-with-breach goal
  end to end: what resolves, what locks, and the test at the command seam,
  not only in the goal package.
- BCD-R1-002 (critical): releasing a stopped claim can strand cancellation.
  The custodian in metasystem/internal/dispatch/stop.go (around 153-188)
  publishes the fence, re-resolves a claimed binding, then creates the batch;
  a release between those steps makes batch creation fail, and
  FindBreachStops (around 288-313) skips parked goals so an open batch is
  never rediscovered; the steward tick (metasystem/internal/steward/tick.go
  around 69-83) depends only on those routes. Redesign so that cancellation
  duty provably survives release: either release is refused until the batch
  is complete, or parked-with-breach goals stay on the automatic route until
  their batch completes. Name the invariant and the test that proves no job
  can keep running behind a fence that nothing will satisfy.
- BCD-R1-003 (high): a raise after a consumed discharge rewinds the clock to
  the original episode, because the rebind clears the obligation while
  metasystem/internal/dispatch/budget.go (around 77-85 and 133-135) requires
  an obligation and accepts only a proof matching the current claim
  revision, so obligationBudgetStart falls back to EpisodeAt. Specify how the
  consumed proof and the post-discharge start survive a raise, and add the
  raise-after-discharge case to the proof plan.
- BCD-R1-004 (high): excluding stopped claims only from quota validation
  leaves two simultaneously claimed goals, and the orientation command
  (metasystem/cmd/metasystem/goal.go around 469-472), the serving projection
  (metasystem/internal/goal/goalverbs.go around 820-823) and the turn
  verdict (metasystem/internal/goal/turnverdict.go around 483-490) pick one
  nondeterministically without checking StopFence. Either forbid a second
  claim while a fenced claim stands (and say how the machine becomes
  workable, which is Fix 3's whole point), or make every projection
  deterministically exclude fenced claims; enumerate every consumer of the
  claimed set and state its rule.
- BCD-R1-005 (high): the duration-era marker is not wired into the
  stop-batch producer. EnsureBreachStop in metasystem/internal/dispatch/stop.go
  (around 134-138) builds StopFiringEvidence without the grammar field, and
  metasystem/internal/goal/stop.go (around 145-155) recomputes the boundary
  and refuses disagreement. Add the producer to the design's consumer trace,
  specify the field on the producer side, and move the test to the dispatch
  producer seam.
- BCD-R1-006 (high): the fail-closed mixed-era claim is false for governed
  run snapshots and stranded journal entries. metasystem/internal/run/run.go
  (around 123 and 374-389) embeds and permissively decodes the budget, and
  conclude.go (around 315-318) enforces the decoded duration; the journal
  replay in metasystem/internal/goal/budget.go (around 81-99) ignores extra
  intent arguments and rebuilds through the legacy constructor. Specify what
  an old binary does with a new-grammar record in each of those places, and
  make the guarantee true: either the marker is enforced there too (refuse
  rather than misread), or the design states plainly which readers can
  misread during rollback and why that is acceptable, with Wido named as the
  one who accepts it.
- BCD-R1-007 (medium): the parked stop-authority invariant breaks the
  ordinary hand-park path and does not implement the promised hand-done
  path. metasystem/internal/goal/reconcilemap.go (around 131-146 and
  229-247) tolerates only the old claimed-to-parked diagnostics and permits
  stop retention or clearing only when a claim is cleared by park;
  reconcilepub_test.go (around 295-316) requires ordinary claimed hand-park
  to stay lawful. Specify the mapper contract that admits ordinary
  hand-park, breach hand-park and breach hand-done, and name all three tests.
- BCD-R1-008 (high): the migration cannot perform its grammar reset on a
  live fenced budget, because SetBudget refuses that state
  (metasystem/internal/goal/verbs.go around 514-515) and Resume starts a
  fresh episode. The live specimen is
  metasystem/plans/goals/alert-escalation-channel.md. Specify a migration
  that touches every live budgeted goal without resetting any episode, or
  state that legacy records keep their semantics untouched and no migration
  runs, and show the specimen's exact before-and-after under the choice.

Self-grade per the house rule: confidence, weakest claim, reject condition.

# Constraints

Wall-clock budget: 45 minutes. Design only; no benchmarks, no builds. Do not
change the three fixes' scope: the goal is these three defects and nothing
adjacent.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file.

# Gap Rule

stop and report a gap; never fill it silently.
