# Brief: design-round-convergence-and-exits design-evidence-2, correction 1

Working Mode: Implement
design-evidence-2 is uncommitted in this worktree (item 0 is right). Its read found two material defects in item 1; fix exactly these.

1. internal/readsubject/design_sections.go:70-81: a heading gone from the new page with no `Section mapping` line drops its identity and a new heading gets a fresh one, so the chain can close with a renamed finding's section history reset; the design's test row says an unmapped heading yields unknown and cannot close (cutover mutation "root reset by rename"). When a prior heading is missing and unmapped while an unmatched new heading exists, return unknown. Flip the `unmapped` scenario of TestDesignReviewRetainsSectionIdentity (cmd/metasystem/intent_design_evidence_test.go) to expect unknown and no close. Mutation: fresh identity -> red.
2. internal/dispatch/examination_read.go:110 derives round N+1 identity from rounds/<N>/decisions.md, which cmd/metasystem/intent_design_review.go:247 retainRoundDecisions overwrites on a later `--dispositions` for round N (continueDesignChain :202-245 lets it through; the To check applies only with a mapping); every later `design review` then fails "immutable section evidence changed" with a remedy that never mentions the decisions file. Read the decisions frozen with the follow-up request (checked against DecisionsSHA256), and refuse to retain decisions for round N once round N+1 exists, naming why. Test: a second --dispositions for round N after N+1 exists is refused and the next design review still collects. Mutation: read the overwritable file -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
