# Brief: review-chain-stops-and-records tree-boundaries-fixes (fix unit after the stop)

Working Mode: Implement
tree-boundaries (first part) is committed and stopped with one material finding; fix it and apply the read's size cuts (the part is about 350 production lines against the 250 cap; bring it down without losing behaviour).

M1. internal/launch/tree_reservation.go:207-216 with read_sequence.go:53-64: a step's launch id is saved (state starting) before its launch record exists and is among the reservation's children; if the command dies in that window, CancelRun marks the run cancelled (:195-200) and Manager.Cancel("ghost") fails "no such file" (Store.Update needs a record), so `work stop run:A` and the wait fail on every try and the tree is stuck for good. Under the run lock, treat a child id with no launch record as never started in both CancelRun and treeChildrenEnded (safe: launches start only under the run lock). Test (the critic's scenario TestCriticGhostChildStrandsTree): the command dies between saving the id and the launch record; the person's `work stop run:A` succeeds once, the tree releases, V proceeds (mutation: Cancel the ghost, red).
Cuts (behaviour unchanged): one child-id collector shared by reserveTree and CancelRun (:108-117, :183-191); drop the first GateTree in submitManualWork (intent_manual_submit.go:89; the later worktree check covers it); inline gateNamedTree and GateTree; fold the two UNIT_CANCELLED checks in Advance (unit_run.go:214, :231). Keep Phase, Subject and Round (the design names them).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
