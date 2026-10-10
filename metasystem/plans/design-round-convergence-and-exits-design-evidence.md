# Brief: design-round-convergence-and-exits, unit design-evidence

Working Mode: Implement
Branch goal/design-round-convergence-and-exits, stacked on goal 2's branch (review-chain-stops-and-records at ee8ba2f5c, before its main merge); it lands after goal 2. Build unit design-evidence of plans/designs/design-round-convergence-and-exits.md (accepted; read it in full, including the R3-M1 fold, R-148-m1e and the channel-message item) and ONLY that unit: Decision 1, the frozen inventory and section identity and the validated whole-page collection, consumed by the existing design review; unknown cannot close (areas: internal/readsubject/, internal/dispatch/examination_read.go, internal/returnschema/, internal/protocol/schemas/design-critic.schema.json, cmd/metasystem/intent_design_review.go, review brief composition).
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test: the design's design-evidence test through `design review`; mutation: the design's.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
