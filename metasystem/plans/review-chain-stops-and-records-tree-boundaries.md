# Brief: review-chain-stops-and-records, unit tree-boundaries

Working Mode: Implement
Goal 2. build-outcomes and all its fix units are committed on this branch. Spec: plans/designs/review-chain-stops-and-records.md Decision 4 "A round owns one worktree and one result", paragraphs 1-3 (the tree reservation, ownership recovery, fresh boundary checks) and the tree-boundaries rows of the Units and public-verb tables; the result tree replay at publication (paragraph 4) is round-result's, not this unit's. Confirm each site on this branch (internal/launch/unit_named.go:316 hash, :419 command lock; unit_run.go:437 proof snapshots, :527 no snapshot after the read).

Build: one durable reservation per canonical worktree in the runner's state directory with a short transition lock (worktree, owner run/round, phase, retained subject, admitted child ids), lasting through build, proof and review judgement across command exits; the owner's own proof, review and publication proceed; another unit, a rebase or an unrelated publication returns waiting with the owner and `work wait run:RUN`; locks in tree, unit, run order, none held while waiting; release only when fresh run/launch custody proves every owned child ended (never inferred from elapsed time); continue keeps ownership for the next correction; a person's recovery quiesces the exact owner through the existing cancellation path. Fresh tree bytes compared before and after proof and after a read; a changed tree is an environment cause, never a clean read; a snapshot from before a command yielded is not fresh on resume.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 (the reservation and its waiting refusals first) and report the rest.
Public-verb test (from the design): start U, let the command return while its child lives, then `work build ... --work V` and `work rebase G` on the same tree both wait for U; U's own review proceeds; change the tree during its read: publication is not clean; cancel U and follow the printed wait/recovery path: V proceeds only after quiescence. Mutations: reuse a stale boundary snapshot; release on inferred process absence; omit the fresh post-read check.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
