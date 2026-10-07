# Brief: lane-reads-its-policies, unit U5

Working Mode: Implement
Goal lane-reads-its-policies. The spec is the accepted design plans/designs/lane-reads-its-policies.md: Decision 5, the five-answers table, the U5 scenario and mutations under "Public-verb tests and the mutations they must reject", its acceptance items (round 2), the readers and obligations rows, and "Decided by m1e for Wido". Read the whole page; it is binding. Earlier units of this goal are committed on this branch; use them, never duplicate them. Build only U5.

What U5 builds, in short: the direct exception proof, fresh incident coverage, replacement hand-in metadata, held-entry questions, the push guard.

Sites: the design cites them (its critique rounds verified them against main and the 1b branch); this tree is main after goals 1a, 1b and those integrated since, so re-read each before you change it and say in the return where one moved. Every human act is proven at the enrolled terminal with Proof.Helm == nil (lanePerson); a helm-admitted non-machinery caller replaying a printed command is refused.

The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.

This worktree holds U1 through U4 and their fixes committed; build on them.
