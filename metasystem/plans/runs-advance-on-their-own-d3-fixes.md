# Brief: runs-advance-on-their-own D3-fixes (fix unit after the stop)

Working Mode: Implement
FIRST: `git merge --no-ff goal/runs-advance-on-their-own-d4` is in progress in this worktree with conflicts in cmd/metasystem/steward_seat.go and internal/steward/runner.go (D3 committed 003bc0125 here; D4 7ba8b96b1 on its branch: the review/publication driver). Resolve keeping both sides whole (D3's DriveWork and the policy at the shared step entry; D4's driveUnitReviews), `git add`, `git -c core.hooksPath=/dev/null commit --no-edit`, and run TestDriverPublicStewardCollection, TestDriverPublicContinuationAuthority and TestDriverPublicReviewPublication by name. Then the fixes below.
D3 is committed and stopped with two material findings; fix them. The critic's probe: /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/f77e7ebb-e1ab-4137-9935-206f37035290/scratchpad/d3r2/metasystem/cmd/metasystem/zz_probe_r2_test.go (TestProbeStarvation).
1. cmd/metasystem/steward_seat.go:240 always retries the oldest ready run even when it refuses every tick (e.g. UNIT_NAMED_INPUT_CHANGED after its brief was edited), so younger ready runs never start; and at :194 one damaged entry in any worktree makes DriveWork return before collecting anything. When a ready run refuses, try the next oldest in the same tick (still at most one start per tick), and show the refused run with its reason in status; a bad worktree or entry is reported and skipped, never ends collection. Test: the probe's scenario: the younger run starts within one tick. Mutation: oldest only -> red.
2. cmd/metasystem/intent_work.go:1418-1420 sets the next command of an observing refusal from `work review` to the same command; on the size-acceptance path (intent_unit_review.go:72-83) AcceptBuildSize already cleared Outcome and Stop, so the rerun skips the size branch and ReviewSubject refuses UNIT_REVIEW_NOT_READY. On that path the next command is `work build run:RUN` (what actually continues it); keep the same-command remedy for the retry and unknown-read paths that replay. Test: a person accepts a size under the helm, follows the printed next command, and the build continues. Mutation: same command -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

3. (from D4's read, same liveness class as 1) cmd/metasystem/intent_unit_driver.go:41-65 driveUnitReviews returns at the first unit whose act is neither satisfied nor a judgement, so a unit held for a person (proof-red, a unit-build stop) is re-reviewed every tick and every later unit never reaches automatic review or publication. Continue past acts whose result needs a person. Test: a held unit and a later clean unit: the later one publishes in the same tick. Mutation: return at the first held act -> red.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
