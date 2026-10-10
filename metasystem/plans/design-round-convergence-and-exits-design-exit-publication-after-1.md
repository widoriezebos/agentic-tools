# Brief: design-round-convergence-and-exits design-exit-publication (first part), correction 1

Working Mode: Implement
design-exit-publication's first part is uncommitted in this worktree. Its read found three findings; finding 3 (Decision 6's person forms: `design review --scope FILE`, `--ruling TEXT --reason --by` with authentication before advisory reads, the ruled head, impact before each effect; and the already-written concrete fold close, intent_design_acceptance.go:56 returning nil when material > 0) is this unit's remainder and goes to design-exit-publication-2. Fix exactly these two now:

1. internal/goal/design_item.go:47 (set at intent_design_acceptance.go:86): the prepared exit freezes the goal revision and the claiming session, and nothing re-prepares or discards it, so once the goal moves or the session changes every resume is refused "changed or is not held by this session" with no next command (the critic's probe: prepare under a held person policy, bump the goal revision, run the same command twice). Recheck the current source state and authority when the effect is admitted (or re-prepare when the commit is refused), per the design. Test: the probe scenario reaches confirmed. Mutation: compare the prepare-time revision -> red.
2. cmd/metasystem/intent_design_review.go:127 and intent_design_acceptance.go:34-38: after a successful acceptance entry.Exit is never cleared and closed chains are still listed, so a plain `design review FILE` says "design acceptance remains pending" and a later chain with a different root is refused as "belongs to another critique". When the goal already holds the committed exit, the projected page matches it and the chain is closed, report accepted and let a new chain through. Test both. Mutation: keep the stale entry -> red.
Also (not material, cheap): internal/project/publish.go:44 leaves an untracked `<design>.md.publication-lock` beside every published page; move the lock under the state root.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
