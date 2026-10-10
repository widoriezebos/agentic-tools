# Brief: design-round-convergence-and-exits, unit design-item-proof

Working Mode: Implement
Branch goal/design-round-convergence-and-exits-dip, stacked on goal 2's branch (ee8ba2f5c); it merges into goal/design-round-convergence-and-exits (design-evidence is in its read there; stay out of internal/readsubject/ and the design-critic schema it owns). Build unit design-item-proof of plans/designs/design-round-convergence-and-exits.md (accepted; read it in full, including its folds) and ONLY that: Decision 6's item shape, digest and readers; strengthen the existing deferred-design obligation consumption through build, code review and completion before new exits write items (areas: internal/goal/file.go, internal/goal/verbs.go, internal/designgate/, internal/project/, internal/landing/, the code-read schema/template, cmd/metasystem/intent_design_gate.go, intent_work.go, intent_delivery.go).
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test: the design's design-item-proof test; mutation: the design's.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
