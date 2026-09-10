Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal dispatch-admission-refuses-on-a-fenced-sibling, reading the final round of chain bsws-build1b-20260909)
Date: 2026-09-09

# Review brief: closing independent read of the final tree (chain bsws-build1b-20260909, work round bsws-build1b-20260909-r8)

FINDING IDS: chain-unique, continuing the register: BSW-18, BSW-19, ... never F-n.

Why this review exists: your round-1 read of work round 2 found seven
findings, five material (BSW-01 to BSW-05), all accepted; BSW-06 was folded
as cosmetic; BSW-07 (the delivery health role counting a fenced claim's
age) went to the backlog as its own goal. Round 3 stopped on a boundary
error in the orchestrator's brief (it named the resume mutation in the wrong
file); round 4 is the fold, complete, with no gaps. Trunk then moved by
fifty-four commits touching six of the chain's files, so round 5 rebased the
chain onto current trunk and changed no behaviour; a first closing read of
round 4 died on a rate limit and returned nothing; the closing read of round
5 (bsws-crit3-20260910; its register, breach-stop-wedges-seat-critique-r2.md under
records/misc, is in the round packet with the r1 register, both landing with the chain)
found two material defects and one proof hole, folded in round 6: the resume
pre-check now honours the quota's arc exception (BSW-08); the proof and test
accounting readers uniqueActiveProofGoal in metasystem/cmd/metasystem/proof_run.go
and resolveTestingGoal in metasystem/cmd/metasystem/test.go skip fenced
claims (BSW-09); and the channel report's two-slot cut has its fixture
(BSW-10). The closing read of round 6 (bsws-crit4b-20260910; register
breach-stop-wedges-seat-critique-r3.md under records/misc, in the round
packet) found one material defect, folded in round 7: dispatch admission's
machine-wide claim scan in metasystem/internal/dispatch/admission.go no
longer refuses a dispatch for goal B because a fenced sibling A is held by
the same lineage; dispatch against A itself stays refused (BSW-12). Chain bsws-build1b-20260909 is at goal
breach-stop-wedges-seat's three-read ceiling (R-42-m0), so this read is
charged to goal dispatch-admission-refuses-on-a-fenced-sibling, opened for
exactly that finding; the chain lands whole under breach-stop-wedges-seat
once this read closes it. The read of round 7 (bsws-crit5-20260910, register
breach-stop-wedges-seat-critique-r4.md under records/misc, in the round packet)
found one material item, BSW-15: trunk had created its own
internal/dispatch/admission_test.go, colliding with the chain's new file of
that name. Round 8 moved the chain's three admission fixtures to
admission_fenced_sibling_test.go beside it, rebased onto current trunk, and
changed no production code. This chain reads the FINAL tree in a fresh
session and closes the build chain. Round budget: 1
focused round. A finding is material only if it changes what gets built and
names the artifact it would change.

# What round 4 folded (the contract for this read)

- BSW-01: ServingProjection in metasystem/internal/goal/goalverbs.go walks
  OrderedOpenGoalIDs and skips fenced claims; with only a fenced claim it
  serves nothing. Fixture: servingprojection_test.go, forty calls.
- BSW-02: resumeRequest in metasystem/internal/goal/stop.go, after
  VerifyStopBatchComplete and before touch and bindClaim, refuses when the
  machine holds another live claim, with: "goal resume <id> refused: machine
  <m> already holds live claim <other>; conclude, park or release <other>
  first, then resume <id>". Fixture in breach_stop_quota_test.go: refused
  with the fence intact, then release the other, then resume confirms.
- BSW-03: FencedClaimLines is exported from
  metasystem/internal/goal/turnverdict.go and used by the turn verdict, by
  goal next in metasystem/cmd/metasystem/goal.go (printed before the
  selection line in every branch), and by the channel report in
  metasystem/internal/channel/report.go (under Next up; the report's
  two-slot cut now counts claimed lines only).
- BSW-04: metasystem/internal/steward/openwork.go has a fenced-only branch
  through ClaimableBudgetedWork.OnlyFencedClaim with a true reason line.
- BSW-05: a fixture for the live-beside-fenced branch of the verdict
  display. BSW-06: the nil guards.
- Untouched, by diff: clearClaimBinding, the breach-stop writer, and
  metasystem/internal/steward/delivery.go.

# Mandate

0000. The round-8 move. Confirm from the round-8 diff that trunk's
   admission_test.go is byte-identical to trunk, that the chain's three
   fixtures now live in admission_fenced_sibling_test.go with unchanged
   assertions, that no production file changed in round 8, and that the
   chain's patch applies cleanly to current trunk (git apply --check).

000. The round-7 fold. Confirm from the diff of
   metasystem/internal/dispatch/admission.go that the machine-wide scan skips
   fenced claims only when the dispatch is for another goal, that a dispatch
   for the fenced goal itself is still refused with the live stop reason,
   that nothing in metasystem/scripts/agents/dispatch.sh changed, and that
   the three fixtures (live beside fenced passes; fenced itself refused; a
   second fenced claim changes nothing) go red if the fold is reverted.

00. The round-6 folds. For BSW-08, BSW-09 and BSW-10, confirm from the diff
   that each fold is what the register says and that its fixture goes red
   if reverted; for BSW-08 confirm the arc comparison never matches an empty
   arc; for BSW-09 confirm that a machine holding only a stopped claim now
   refuses in one plain sentence naming the stop id, and that commit.sh's
   and land.sh's default delivery check resolve the live goal when one
   stopped and one live claim are held.

0. The rebase. Round 5 was a merge-forward only. Confirm from the round-5
   diff against current trunk that it contains the round-4 change and
   nothing else, that NextVerdict carries both the chain's Fenced list and
   trunk's Refused list side by side, and that every trunk change to the
   six overlapping files survived untouched.

For each of BSW-01 to BSW-06, confirm from the diff and the file that the
fold is what the register says, and that its fixture would go red if the
fold were reverted; name file and line. Then look for what the folds
themselves introduced:

1. The resume pre-check runs before touch, so a refused resume writes no
   history line and leaves the goal byte-identical. Confirm, and confirm
   the check cannot be reached with f.Claimed nil.
2. ServingProjection serving nothing on a fenced-only machine: what does
   the delegate prompt's "Serving goal" block and dispatch's serving-goal
   section then say, and is dispatch against the fenced goal still refused
   by admission as before? Read metasystem/internal/mission/prompt.go and
   metasystem/internal/dispatch/servinggoal.go and admission.go.
3. The channel report's slot change: with two fenced claims and two live
   claims, what does Next up show, and is that the intent (fenced lines
   first, then at most two Next up lines)?
4. Any remaining reader of "State is claimed and Claimed.Machine equals this
   machine" that feeds a decision rather than a count, in the packages the
   folds touched and their callers. Your round-1 search found four; say
   whether any is left.
5. The quota rule still refuses an ordinary second claim; the arc exception
   (one arc counts once) still holds with a fenced member in the arc.

# Constraints

Read-only. Return per the code-critic schema, every finding with file and
line, a severity, and the artifact it would change. Wall-clock budget: 45
minutes. Plain English in every human-visible field.

# Orchestrator runs

Outside the sandbox on the round-4 tree: focused goal fixtures (23 s), the
cmd test the sandbox cannot run, the steward and channel packages, all
green; the goal-cli bed is running as you start. The landing receipt runs
go-gate --fast and the beds again.
