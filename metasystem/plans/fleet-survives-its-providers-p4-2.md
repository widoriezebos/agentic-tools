# Brief: fleet-survives-its-providers, unit P4-2 (the headless handoff at the boundary)

Working Mode: Implement
P4's first part is committed on this branch (d57bdee5f): the steward records a completed unit boundary per seat from real outcomes, holding only on open work. Handoff: /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/handoff-fleet-survives-its-providers-p4.md. Build the rest of P4 in plans/designs/fleet-survives-its-providers.md ("P4 — Minimal completed-unit event and existing session handoff") and ONLY that:
the generated headless session instruction (internal/steward/seat_start.go seatBrief) tells the seat to stop advancing at a completed boundary, capture `session handoff` with its configured note and no-delegates declaration (cmd/metasystem/context_verbs.go), then end its turn once the binding is durable; the steward's handoff owner binds the event to that handoff nonce only for headless sessions (a person's session gets the boundary but no binding, instruction or signal); re-entry reuses the boundary, capture and binding, including a capture persisted before binding; once durable it revalidates the recorded predecessor identity and sends SIGTERM through the existing process signalling path (never a reused pid; unknown identity or a signal failure is visible and holds replacement); failed capture or binding leaves the session alive with the exact error and no signal; the existing exact-death admission check (internal/steward/handoff.go:65) admits the successor. No next-step queue, driver, settings or engine consumers (runs-advance-on-their-own consumes the event).
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 (instruction and binding first, then signalling) and report the rest.
Public-verb test (TestFleetBoundaryPublicHandoff, the design's): a headless seat completes a unit; the steward binds the boundary to its handoff, signals the exact predecessor once the binding is durable, and admits the successor only after observed death; a capture persisted before binding is reused on re-entry; a person's session records the boundary with no signal. Mutations: signal before the binding is durable; bind a person's session.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
