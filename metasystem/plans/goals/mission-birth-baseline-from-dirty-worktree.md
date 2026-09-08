# mission-birth-baseline-from-dirty-worktree

- State: queued
- Priority: 2
- Sequence: 24
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: a stale birth wedges a headless resume with a drift refusal an operator has to decode; novelty 1: the wall preflight already encodes the clean-or-sealed rule for start; exposure 2: every mission birth on a live checkout; accumulation 1: one refusal, one fixture"
- Tier: 2
- Intent: mission state-init accepts a baseline computed from the WORKTREE projection (gittree Snapshot seeds from HEAD then add -A), so a mission born on a dirty checkout records dirty files in its baseline; any later change to those files before resume (a landing appending the narrator digest, a pull, an edit) shows as drift and the resume refuses. Found 2026-09-06 preparing the first headless run on m1: the born baseline fb2ebfae83 included three long-dirty files and stayed valid only while nothing moved. Decide: birth refuses a dirty worktree unless the contract seals it (the wall preflight already knows this rule for start), or birth and resume are one act so the window is zero. Build behind a fixture: state-init on a dirty scratch checkout refuses naming the dirty paths; state-init on a clean one succeeds; the sealed-baseline path still admits.
- Origin: main
- Next step: Read internal/mission/snapshotscope.go VerifyBaselineIsLive and internal/missionrunner/wall.go admittedBaseline; the fix is one refusal in state-init plus a fixture in the mission fixtures. Sol builds behind the fixture, Fable reviews, land. Blocks a clean headless retry only in that the operator must birth immediately before resume until it lands.
- OpenedAt: 2026-09-06T21:22:42Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T21:22:42Z FR2Z2TEY18R672RBAXX54WJ0R7-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=mission-birth-baseline-from-dirty-worktree
- 2026-09-08T15:58:34Z B54H0WP2NYTTB9MN761J5RNM97-m1-7cd0bd60 set-priority actor=human:Wido targets=mission-birth-baseline-from-dirty-worktree reason=priority-order subject=mission-birth-baseline-from-dirty-worktree from=unranked to=2:24 requested-sequence=24
Integrity: sha256=3d33218a01ebb89ee6a17b31a263ad385b9fd96a9a20905552069719d4e38abe
