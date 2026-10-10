# Brief: design-round-convergence-and-exits design-item-proof, correction 1

Working Mode: Implement
design-item-proof is uncommitted in this worktree. Its read found two material defects; fix exactly these.

1. cmd/metasystem/intent_design_gate.go:134 decides a page is convergence-marked only from its `- Critique:` head line containing `(convergence ...)`, and the head is outside project.DesignBodyDigest (internal/project/record.go:233), so removing the marker makes the gate treat the page as an old accepted page: CheckDesignAcceptance and the AcceptedUnits filter (intent_work.go:878) never run and landing compares "" with "". The critic's probe (a tier-2 goal, design.gate.mode=refuse, an exit and its item, marker removed, body requirement appended, a unit outside the accepted units built: code 0, launched build/proof/read). Decide convergence-marked from the goal's DesignExits (any exit with DesignID == the page's id) as well as the head; a page whose goal records an exit but whose head lacks the marker gets AcceptanceError. Test: the probe scenario is refused. Mutation: head only -> red.
2. cmd/metasystem/intent_delivery.go:2378 runs DesignCompletionProblem at the top of handLandingSubject, also on the --records path (:2047), so a records-only hand-in is refused for an open design item with a code-proof remedy unrelated to records. Apply it only when records is false. Test: a records hand-in with an open item proceeds; a code hand-in is refused. Mutation: check on records -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
