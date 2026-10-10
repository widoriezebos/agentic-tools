# Brief: review-chain-stops-and-records tree-boundaries-3, correction 1

Working Mode: Implement
tree-boundaries part 3 is uncommitted in this worktree (the unit's last part; its remainder moved to round-result and read-publication, design page 03:50). One Opus read found one material defect; fix exactly this.

cmd/metasystem/intent_unit_review.go:880 (publishRead inside commitReviewChecked): part 3 takes the tree lock in ReviewSubject (internal/launch/unit_review.go:105-126) and holds it, with the named-unit and run locks, through the collected-read publication push; only endpointTip and the unit push are wrapped in review.Wait (:211, :354). Every other tree command gets UNIT_RUN_BUSY for the whole push (treeLocked is TryExclusive, tree_reservation.go:65). Run the publishRead push inside the same wait helper (plumb it into commitReviewChecked from its caller): locks released during the remote push, retaken in tree/unit/run order, and the owner and run bytes reread on retake (CommandWait). The fresh tree comparison still runs under the lock just before the push. Test: while a collected-read publication push is in flight, a competing tree command waits on the reservation instead of failing UNIT_RUN_BUSY, and the publication completes (mutation: push under the locks, red).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
