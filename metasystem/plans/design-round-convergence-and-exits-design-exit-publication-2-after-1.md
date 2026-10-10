# Brief: design-round-convergence-and-exits design-exit-publication-2, correction 1

Working Mode: Implement
Part 2 (the person forms) is uncommitted in this worktree. Its read found two material defects; fix exactly these (and the two cheap notes).

1. cmd/metasystem/intent_design_person.go:65-70 keys the retained act by path, ruling, reason, actor and scope text but not the current page, so repeating the same words after the page changed (e.g. ruling again after an undo) reloads the old act with its stale Expected page; the publish check (:143-153) fails and the printed next step ("resume the retained act") fails every time (reproduced: undo, then the original --ruling args: code 1, "the ruled exit is not committed"). Add the current page's digest to the key. Test: rule, undo, rule again with the same words: confirmed. Mutation: key without the page digest -> red.
2. The test does not catch an advisory read before authentication (the brief's mutation survives: adding readIntentDesignRecord + DesignCritiqueChains + CollectExamination before directPersonProof at intent_design_person.go:21 stays green). In intent_design_person_test.go (~:52-60) hook the critique-record and return readers so the refused-agent leg fails if either is read before the proof. Mutation: read before proof -> red.
Cheap notes: intent_design_person.go:96 must refuse (with the reason) a ruling on a design whose DeclaredUnits errors or is empty, instead of publishing a page the gate always refuses; intent_design_gate.go:135 searches for the last "(ruling " in the line.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
