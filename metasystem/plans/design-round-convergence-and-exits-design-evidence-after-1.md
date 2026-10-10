# Brief: design-round-convergence-and-exits design-evidence (first part), correction 1

Working Mode: Implement
design-evidence's first part is uncommitted in this worktree. One Opus read found two material defects; fix exactly these.

1. cmd/metasystem/intent_design_review.go:126-130 and internal/dispatch/finding_register.go:214-218: any chain completed but not closed now fails before the dispositions, retry and close cases, including chains whose round subject was saved before DesignPage existed and returns without wholePageDigest/coverage; the register advance errors before its `round <= foldedRound` unchanged exit; the printed next step reruns the same check and can never succeed. TestSecondDesignRoundRefusedWithoutCritical, TestOneRoundCloseFoldsAccepted, TestDesignCritiqueReplayAndCap and TestDesignCritiqueClosesOnUnchangedDesign are red (green on HEAD). Keep the old collection path for rounds already folded or with no saved subject; legacy chains get a recovery that can succeed; those four tests pass UNCHANGED.
2. intent_design_review.go:499: when CollectExamination succeeds but CritiqueRegisterAdvance fails, collectReview returns failed without findings and the function still writes a permanent empty decisions.md (only if absent) pointing to --dispositions template, so a partial collection reads as nothing to decide. Return before writing the template when result.Outcome is failed or findings are absent. Test: a failed register advance writes no decisions.md; a later valid collection writes the full template. Mutation: write it on failure -> red.
Run the four named tests above unchanged plus your new test.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
