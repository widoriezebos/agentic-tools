# Brief: process-changes-cover-declarations-and-interventions, unit U3, correction 1

Working Mode: Implement
The uncommitted U3 build is in this worktree; keep it. The read found:
F1 BREAKING: when origin's goal branch is ahead of the local branch (adopt state, commit.go:197-199, baseTip = remoteTip), installCommitOnto with patchOnly uses checkoutBase = state.baseTip and PatchCheckout runs `git read-tree -m -u <remote> <index> <newTip>` with an index built on the old local tip: every file origin changed is staged back to its old content (reproduced: `M  p`); the next work commit publishes that undo. Fix by subtraction: refuse the inverse in the adopt state with a plain remedy (bring the local branch current first), or base it on HEAD. Test the adopt state; mutation red.
F2: CommitFrozenPatchWithInputs (commit.go:733-748) replaces the caller's BeforeCommit, so the rebase gate and proof.cheap never run for a revert, and `--patch FILE` is only checked by validateCommitPaths(Unit), so any code path can be committed. Refuse any patch path other than the declaration file (metasystem.conf), and keep the commit-time checks for the revert. Test: a patch touching a code file is refused; mutation red.
F3: TestDeclarationInversePublicRevert/corrupt-baseline failed once ("restored declaration reference unavailable", intent_inverse_test.go:337): find the nondeterminism (map order, parallel fixtures sharing a path, time) and remove it; run the subtest 50 times with -count=50.
Size: U3 is at 250 net: make the fixes by subtraction; report the final count.
Run the changed tests by name; nothing package-wide.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
