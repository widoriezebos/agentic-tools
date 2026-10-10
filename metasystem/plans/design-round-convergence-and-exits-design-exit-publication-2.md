# Brief: design-round-convergence-and-exits, unit design-exit-publication-2 (the person forms)

Working Mode: Implement
design-exit-publication's first part is committed (bb8a1a977). Build its declared remainder in plans/designs/design-round-convergence-and-exits.md and ONLY that: Decision 6's person forms, `design review --scope FILE` and `design review --ruling TEXT --reason TEXT --by NAME`, with person authentication (direct person proof) before any advisory read, the ruled head, and the prior impact printed before each effect; an agent's scope or ruling is refused naming the person's command; and the already-written concrete fold close (cmd/metasystem/intent_design_acceptance.go:56 returns nil when material findings remain, so that route falls through to the old close with no exit): that route publishes its exit like the clean route.
Size: at most 250 production lines. If it will not fit, build the person forms first and report the rest.
Public-verb test: the design's person part of TestDesignReviewPublishesAcceptance: a person's scope and ruling take effect with the impact shown first; an agent's are refused; a concrete-fold close publishes its exit. Mutations: read advisory state before authenticating; let an agent rule.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
