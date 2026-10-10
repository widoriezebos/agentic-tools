# Brief: fleet-survives-its-providers, unit P4 (every-seat boundary; headless handoff and end)

Working Mode: Implement
P1 is committed on this branch (P2 builds in parallel in another worktree; P4 does not depend on it and must not touch the outage mark code). Spec, and ONLY this text: /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/fleet-survives-its-providers.md, section "P4 — Minimal completed-unit event and existing session handoff", the P4 rows of the Units, Estimates and public-verb tables, five-questions rows for P4, round-2 change 4 (who ends the headless session and how; the boundary for every seat, the handoff bound only for headless seats) and acceptance item R3-3's P4 part (reuse steward.SeatAtUnitBoundary, internal/steward/rearm_checkout.go:59; name the path that ends a seat session; the steward today signals only runners, runner.go:1249, :1429). Goal 4 (runs-advance-on-their-own) consumes this event: keep it minimal and stable.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test: the design's P4 test (the boundary event for a person seat and for a headless seat; the headless seat's handoff bound and its session ended), with its mutation.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
