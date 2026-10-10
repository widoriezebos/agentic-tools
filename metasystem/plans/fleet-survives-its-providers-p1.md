# Brief: fleet-survives-its-providers, unit P1 (fresh host view; central proof default of one)

Working Mode: Implement
Goal fleet-survives-its-providers (plan goal 3). Spec, and ONLY this text: the accepted design /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/fleet-survives-its-providers.md (read it there; it is not on this branch yet), section "P1 — Fresh host view and centrally defaulted proof serialization", the P1 rows of the Units, Estimates and public-verb tables, the five-questions rows for P1, and acceptance item R3-3's P1 part (update internal/config/defaults.go:98 with metasystem.conf:101). This worktree is goal/fleet-survives-its-providers from main 343cb22cb. Confirm each cited site on this tree before changing it. The provider section of the host view is filled by P2 later: leave it as a clearly named empty section, not a stand-in.

Size: at most 250 production lines (estimate 230). If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test: the design's P1 test, with its mutation.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
