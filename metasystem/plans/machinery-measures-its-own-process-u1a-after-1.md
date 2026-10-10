# Brief: machinery-measures-its-own-process U1a, correction 1

Working Mode: Implement
U1a is uncommitted in this worktree. One Opus read found two material defects; both let the measured agent choose or skip its own denominator. Fix exactly these.

F1. cmd/metasystem/intent_unit_estimate.go:38-41, :64 with intent_design_gate.go:126-134: the estimate is read from the working-tree page and its digest is the digest of that same file, so an agent edits the Estimates row (Status left accepted) and its own number is frozen; after a hold for a missing row the agent adds a row and reruns. Decided by m1e for Wido (design page, 23:30): read the page from the git object of the accepted page on origin/main and retain that object's digest; when the working-tree page differs from it at first admission, an agent's build is held as "estimate unavailable" (never a person's). Public-verb test: page committed with row 17, working tree with row 90: agent build held, person build proceeds with no estimate (mutation: read the working tree, red).
F2. cmd/metasystem/intent_work.go:582-583 runIntentBuildPlan calls AdvanceNamed without AdmitEstimate, and the strict decoder (internal/launch/unit_plan.go:517, :527, :537) accepts an "estimate" object from a caller-written plan file whose Validate checks shape only: `work build --plan FILE` skips the hold and sets any number. Reject an estimate field in a caller-supplied plan (keep Estimate out of the intake decode); set AdmitEstimate inside unitRunner() so every admission path runs it, --plan included. Public-verb test: an agent's --plan build with no row is held, and a plan carrying a forged estimate is refused (mutation: accept the field, red).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
