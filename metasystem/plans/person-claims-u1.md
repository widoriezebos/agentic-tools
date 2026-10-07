# Brief: person-claims, unit U1

Working Mode: Implement
Goal person-claims. The spec is the accepted design plans/designs/person-claims.md, Decisions 1, 2 and 3, the five-questions table, the U1 tests under "Public-verb tests and their mutations", the readers and obligations row, and "Decided by m1e for Wido". Read the whole page; it is binding. Build only U1.

What U1 builds, in short: one claim record for a person's reservation through the ordinary claim owner, actor-aware area advice (a verified person warns, never refuses; never goal.Actor.Human from --by), all-match restamp adoption at session start, pending and remedy output, and execution eligibility independent of epoch in the seat ladder, the generic revival classification (classifySharedBacklog / readOpenWorkShared) and work admission; an approved budgeted epoch-0 steward-seat reservation is StepDue.

Sites: the design cites them (its critique rounds verified them on main 2381325ce); this tree is main after goals 1a, 1b and those integrated since, so re-read each before you change it and say in the return where one moved. 1b's ClaimAreas is in internal/goal/areas.go.
# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (impacted tests only; the full gate runs per batch on the goal branch)
Build, vet, the packages you changed with -count=1 -timeout 30m, the cmd tests you added or changed by name, their mutations, then `go run ./cmd/devgate static`. If you change a message or skill text, also run `go test -count=1 -timeout 30m -run 'TestAudit|TestInstruction' ./cmd/metasystem/`. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
