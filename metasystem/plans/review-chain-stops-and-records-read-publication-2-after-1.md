# Brief: review-chain-stops-and-records read-publication-2, correction 1

Working Mode: Implement
read-publication part 2 is uncommitted in this worktree. One Opus read found three material defects; fix exactly these.

1. cmd/metasystem/intent_work_commit.go:79-83: the candidate passed to the cheap check is the scratch worktree's top, so landingProofCommand reads `parent:metasystem.conf` at the repository root, but here it is metasystem/metasystem.conf (rev-parse --show-prefix gives metasystem/); every `work commit` is refused and its "repeat" remedy cannot succeed; the fixture had metasystem.conf at the root. Join the source checkout's --show-prefix onto the candidate path (as the scratch-worktree opener and round_result.go do) for the installation and the cheap check's working directory. Add a fixture whose installation sits in a subdirectory (mutation: the top path, red).
2. cmd/metasystem/intent_unit_review.go:872-889: the saved HEAD under artifacts/agents/read-publication/<sha(goal,commit)> is never deleted, so after the branch moves (for example the next unit's `work commit`, which the verb suggests) every later run is refused, including a repeat after a successful publication, with a remedy to restore the committed result (resetting a person's commit). Delete the file once the read is published or publication is refused for a change; a repeat of an already-published read returns unchanged before the tree check; a refusal for a real change names a new version of the work. Test: start the read in run A, move the tip with `work commit` of the next unit, publish in run B; repeat after publication is unchanged.
3. intent_work_commit.go:43-45: when CheckCommitAccess fails the result says repeat the same command; reuse the manual-submission refusal branch (intent_manual_submit.go:144-153: heldElsewhere, or `goal claim --take-over` for a person). Test: goal held by another seat: the printed next step is the holder or the person's take-over, not a repeat.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
