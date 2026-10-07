# Brief: lane-reads-its-policies, unit U5

Working Mode: Implement
Goal lane-reads-its-policies. The spec is the accepted design plans/designs/lane-reads-its-policies.md: Decision 5, the five-answers table, the U5 scenario and mutations under "Public-verb tests and the mutations they must reject", its acceptance items (round 2), the readers and obligations rows, and "Decided by m1e for Wido". Read the whole page; it is binding. Earlier units of this goal are committed on this branch; use them, never duplicate them. Build only U5.

What U5 builds, in short: the direct exception proof, fresh incident coverage, replacement hand-in metadata, held-entry questions, the push guard.

Sites: the design cites them (its critique rounds verified them against main and the 1b branch); this tree is main after goals 1a, 1b and those integrated since, so re-read each before you change it and say in the return where one moved. Every human act is proven at the enrolled terminal with Proof.Helm == nil (lanePerson); a helm-admitted non-machinery caller replaying a printed command is refused.
# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (impacted tests only; the full gate runs per batch on the goal branch)
Build, vet, the packages you changed with -count=1 -timeout 30m, the cmd tests you added or changed by name, their mutations, then `go run ./cmd/devgate static`. If you change a message or skill text, also run `go test -count=1 -timeout 30m -run 'TestAudit|TestInstruction' ./cmd/metasystem/`. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
