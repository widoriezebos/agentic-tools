# Brief: review-drops optional-uncommitted-unit-drop, correction 1

Working Mode: Implement
The unit is uncommitted in this worktree. Its read found three material defects; fix exactly these.

1. cmd/metasystem/intent_unit_drop.go:57,87 lets a committed-only drop continue with a dirty owned tree and treats the round's result.patch as pending work, but every build round keeps that file (internal/launch/unit_run.go:501) and here it is already committed: reverse-applying it re-adds the unit, the commit tree equals HEAD and `git commit` fails "nothing to commit" on every rerun (with a correction commit, :237 conflicts; :151 resets and every rerun conflicts again); PatchDigest then sticks, so cleaning the tree no longer helps. Take the patch path only when the round's result is not already committed (the round has no Commit and the patch does not reverse-apply to HEAD); otherwise keep the clean-tree refusal. Test: a committed unit with another unit's uncommitted edit: refused clean-tree, and after the person cleans, the drop succeeds. Mutation: patch path for a committed round -> red.
2. internal/goal/branch/commit.go:543-552 with intent_unit_drop_patch.go:52-53: in the mixed case the pending patch is removed from the owned tree before the inverse commit is installed; an interruption in between leaves the patch gone and no inverse published, and rerunning refuses (installPendingRemoval's tree check; its idempotent shortcut covers only len(Covered)==0; the resumed scratch HEAD is the new drop commit, so :188 refuses "branch moved"). In the mixed case treat an owned tree that already has the patch removed as done, and join the scratch commit on resume. Test: interrupt between removal and install, rerun reaches confirmed with one drop commit. Mutation: no resume join -> red.
3. intent_unit_drop_patch_test.go:256-272 never checks the mixed drop commit's content (the fixture fakes Commit, Checkout and Index); removing the reverse-apply of -remaining.patch at intent_unit_drop.go:237 stays green. Assert the mixed CommitTree equals HEAD's tree minus only the unit's covered changes, with other pending files kept out, using real git for that step. Mutation: drop the reverse-apply -> red.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
