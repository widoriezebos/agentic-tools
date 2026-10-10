# Brief: design-round-convergence-and-exits, unit design-evidence-2 (the stop's fix, then the rest of Decision 1)

Working Mode: Implement
design-evidence's first part is committed (4dfba9c86) and stopped. Handoff: /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/handoff-drce-handoff-design-evidence.md.
0. The stop's finding (fix first): cmd/metasystem/intent_design_review.go:178-195, the branch for a page edited since the examination, builds a permanent decisions template from the raw return before read.json exists (the register advance runs only in collectReview or at follow-up launch), so the template omits the collector's derived findings (unanswered cells, missing inventory rows) and `--dispositions` (validate.CritiqueClosed) refuses "return JSON is unjoinable: .../read.json" on every rerun. When DesignEvidenceRequired is true, run CritiqueRegisterAdvance before this branch writes the template; build the template from the derived findings; return failed with no template if the advance fails. Test: a return with one unanswered coverage cell, then the page edited: the template holds both findings and closes via --dispositions. Mutation: the raw-return template -> red.
1. The rest of Decision 1 in plans/designs/design-round-convergence-and-exits.md: the heading move and rename mapping with its ambiguity check, so a moved or renamed section keeps its identity (internal/readsubject/design.go:544 hashes the raw heading line today) and an ambiguous mapping is unknown, never a guess; the design test's "retain section identity" step.
Size: at most 250 production lines. If it will not fit, build item 0 and the largest usable part of item 1 and report the rest.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
