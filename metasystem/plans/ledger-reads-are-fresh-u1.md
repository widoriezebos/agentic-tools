# Brief: ledger-reads-are-fresh, unit U1 fresh ledger decisions

Working Mode: Implement
Goal ledger-reads-are-fresh. The spec is the accepted design plans/designs/ledger-reads-are-fresh.md: Decisions 1, 2 and 3, the five-questions table, the U1 tests 1-3 under "Public-verb test and mutations", both round-2 acceptance items, and "Decided by m1e for Wido" (the session-start hook keeps its offline read; only the explicit commands read fresh). Read the whole page; it is binding.

Sites: the design cites them (critique rounds verified them on main 2381325ce); this tree is main after goals 1a and 1b, so re-read each before you change it and say in the return where one moved. Key sites: the restamp callback cmd/metasystem/up.go:113-165 and its three callers (hook_entry.go:356 stays offline; up.go:239 and intent_process.go:682 read fresh); defaultFreshProjectionTimeout internal/goal/project.go:46 (one bounded 4 s attempt, no retry); up's outcome internal/up/up.go:160 and its machine readers internal/missionrunner/launch.go:228 (requireArmed :544), internal/testrun/rearm.go:428, cmd/metasystem/session_isolate.go:82 (unaffected: Outcome stays armed, exit 0; adoption pending is a typed field; session intent partial, intent.go:787); claim and next (intent_planning.go:1084, intent_goals.go:27, goal.go:295); bounded execution internal/boundedexec/boundedexec.go:38.

Not in this unit: the post-preparation reread and per-claim pending preservation (goal person-claims).
# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (impacted tests only; the full gate runs per batch on the goal branch)
Build, vet, the packages you changed with -count=1 -timeout 30m, the cmd tests you added or changed by name, their mutations, then `go run ./cmd/devgate static`. If you change a message or skill text, also run `go test -count=1 -timeout 30m -run 'TestAudit|TestInstruction' ./cmd/metasystem/`. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
