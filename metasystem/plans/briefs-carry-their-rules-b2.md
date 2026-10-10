# Brief: briefs-carry-their-rules, unit B2 (path normalization and base-blob line validation)

Working Mode: Implement
Branch goal/briefs-carry-their-rules from origin/main. Build unit B2 of plans/designs/briefs-carry-their-rules.md (accepted; read it in full, including "Decided by m1e for Wido" and the round tables) and ONLY B2: path normalization and base-blob line validation of a brief's code citations (internal/dispatch/brief.go, brief_frozen.go, the work and design admission callers); `git cat-file -e` needs exit-status handling that tells absent from error (brief.go:218 today turns any error into absent); a missing or moved citation is reported with the exact path and line, never silently accepted.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test: the design's B2 test; mutation: the design's, plus "treat a git error as absent".


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
