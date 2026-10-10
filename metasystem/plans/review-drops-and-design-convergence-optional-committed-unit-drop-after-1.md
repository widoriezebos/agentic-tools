# Brief: review-drops-and-design-convergence optional-committed-unit-drop (first part), correction 1

Working Mode: Implement
The unit's first part is uncommitted in this worktree. One Opus read found two material defects in how a `dropped:` disposition is routed; fix exactly these.

1. cmd/metasystem/intent_review_binding.go:116-121 deleted the general refusal of `dropped:`; the only guard left is in reviewStoppedUnit, which returns nil for a round whose stop decision is continue or close (intent_unit_stop.go:64-68), and the validator accepts `dropped:` (internal/validate/critiqueclosed.go:274), so a drop on a continuing unit closes the review with nothing reverted; the unchanged test TestIntentContinuingUnitRefusesDropAndSplitBeforeRetention/dropped now fails. After the reviewStoppedUnit call (intent_review_binding.go:130-134) restore the refusal for any remaining `dropped:` decision; that test passes again.
2. cmd/metasystem/intent_unit_review.go:226-232 calls reviewStoppedUnit whenever --dispositions is given and the subject has a commit, before closeWorkReview saves the examination (intent_review_binding.go:120-126), and a round's stop decision is set only by CollectExamination from that save (internal/launch/unit_review.go:225-233), so a first close with --dispositions is refused "the unit has no collected stop decision; nothing was started" with no next step. Take the early route only when subject.Drop != nil, or when the examination is saved and the latest round's stop decision is stop. Test: a round-2 close with --dispositions and no drop proceeds (mutation: the old condition, red).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
