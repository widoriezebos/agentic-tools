# Brief: review-chain-stops-and-records tree-boundaries (first part), correction 1

Working Mode: Implement
tree-boundaries' first part (the tree reservation, waiting refusals, fresh boundary checks; internal/launch/tree_reservation.go) is uncommitted in this worktree. One Opus read found two material defects in treeQuiescent; fix exactly these, staying within 250 production lines.

F1. tree_reservation.go:97-104, :133-140, :144-150: any old Cancelled launch id in any round counts as a run cancellation, and a competing writer's gate then writes the owner's run.json as cancelled and removes the reservation, so a person's later retry or revise is refused UNIT_CANCELLED (unit_run.go:214), although the design says continue keeps ownership. Release only on an explicit run cancellation recorded by the person's stop act; never infer it from old child states at another writer's gate, and never let a gate write the owner's run record. Test: a person stops a hung step, an agent's `work build --work V` waits, the person's retry proceeds as the owner (mutation: infer from an old cancelled child, red).
F2. :129-143: an owner whose round ended (build-failed, proof-red, environment; or the command died before any child launched) has no live child and no cancelled child, `work stop j1:L` on an ended launch never cancels (cmd/metasystem/intent_process.go:1108-1112), and no verb cancels a run, so the tree is never released and `work wait run:A` loops. Add the person's explicit run-cancel on the existing stop path, `work stop run:RUN`: cancel live children with exact custody, record the run cancelled, release the reservation; the waiting message names that act when the owner has no live child. Test: an ended owner, the printed act followed by a person releases the tree and V proceeds; an agent cannot cancel another owner's run (mutation: no run-cancel, the wait loops, red).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
