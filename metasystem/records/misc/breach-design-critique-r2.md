# Sol design-critique of the breach-machinery design — round 2 (2026-09-02)

Job breach-design-crit2c (design-critic, codex gpt-5.6-sol), reviewed commit
46bbdc8c, design revision 2 (plans/breach-clock-and-budget-honesty-design.md),
brief plans/breach-clock-critique-brief.md. Three material findings; the
other seven round-1 closures held (001, 002, 004, 005, 007, 008 closed; 003
mechanically closed but its replacement raised BCD-R2-001; 006 reopened as
BCD-R2-002 and BCD-R2-003). Full return:
artifacts/agents/breach-design-crit2c/rounds/1/return.json.

## BCD-R2-001 (high, material, authority boundary)

The episode-scoped discharge replacement exceeds the recorded goal authority.
The design says a discharge consumed under obligation revision 5 still holds
the moved start after a human set-obligation installs revision 7, calling it
a consequence of the thesis; but the goal record authorizes fixing SetBudget
clock resets and dishonest durations and carries no ruling that a new human
obligation inherits an earlier obligation's discharge. Shipped
internal/dispatch/budget.go:133-149 deliberately requires both the current
claim revision and the current obligation revision. Changing what a later
human set-obligation means is adjacent governance scope; a test cannot supply
the missing authority.

Evidence: goal Intent names two defects, neither about set-obligation; the
design's self-grade concedes "a behavior change on the governance seam that
the goal's three defects did not name".

## BCD-R2-002 (high, material)

The day-token proof plan leaves shipped fixtures asserting the behavior being
removed: scripts/agents/goal-cli-fixtures.sh:387 and :416 require typed 8h to
be stored as 1d; :448 and :457 submit fresh 1d values and expect the later
goal-norm refusal; scripts/agents/dispatch-fixtures.sh:1092 submits 1d and
expects a confirmed claim; internal/goal/project_test.go:151-160 calls the
formatter the design deletes. None is in the proof plan's named updates, so
the gate fails to compile or fails before the intended assertions, and the
implementer must guess which rows are legacy-reader coverage, which switch to
hours, and which become explicit day-refusal coverage.

## BCD-R2-003 (medium, material)

The old-binary display regression is broader than the rollout section states
("journal recovery and its own goal set-budget", "that one goal until
re-set"). cmd/metasystem/goalsync_mutations.go:166-180 sends every supplied
tuple through NewBudget, and the same builder feeds open-claim (:217-224),
resume (:367-413), claim with a supplied budget (:653-667) and set-budget
(:669-675); after a rollback an old binary can normalize a new 216h to 27d on
any of those paths for any goal. Enforcement stays 216 hours; the favorable
display lie recurs on multiple writers. The rollout contract must enumerate
or mechanically prohibit those paths.

## Orchestrator decisions (m0b, 2026-09-02 17:50Z), folded by revision 3

- BCD-R2-001: HOLD today's governance. The episode axis replaces only the
  claim-revision filter; whenever an obligation is live, a proof must still
  carry the live obligation revision exactly as budget.go:133-149 demands
  today, so a human set-obligation keeps its shipped meaning (it starts a
  fresh obligation and supersedes earlier discharges). The raise case still
  closes BCD-R1-003 because a raise clears the obligation and the
  episode-scoped proof then counts; the start does not move at the raise.
  Whether a later set-obligation should inherit a discharge is a question
  for Wido, recorded as open and NOT built.
- BCD-R2-002: complete inventory in the proof plan, each row classified.
- BCD-R2-003: complete rollout table over the five old-binary writers; no
  rollback wall (a separate mechanism, out of this goal's scope), the
  residue stated honestly as display-only and per-write.
