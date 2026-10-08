# Brief: review-chain-stops-and-records, unit unit-stop

Working Mode: Implement
Goal review-chain-stops-and-records (plan goal 2). The spec is the accepted design plans/designs/review-chain-stops-and-records.md: Decisions 1 and 2 (with the sites cited there), the five-questions rows for the unit-stop owners, the unit-stop scenario and mutations under "Public-verb proof and mutations", its round-4 acceptance item (one ask per unresolved finding, keyed (loop, subject, attempt, finding), each with its own command), the readers rows, and "Decided by m1e for Wido". Read the whole page; it is binding. Build only unit-stop; drop effects, the tree reservation and design convergence are not in this goal or are later units.

What unit-stop builds, in short: the read record with finding classes and unique outputs; the two dispositions (fixed / split) and completion coverage; the stop policy from review.stop (1b's ResolvePolicy) applying the plan's unit rule (one build plus at most two corrections; material must fall; a repeated class at the same repository-relative path stops); status shows the stop; a green read closes the unit; an unrequired unit stops with one ask per unresolved finding carrying its executable act (work review --dispositions with split: for a required unit; the person's reasoned work revise, goal accept-risk or goal done --reason for an unrequired one); a person-authored dropped: is answered plainly that drop effects are not built yet and names the working act; unknown input gets one fresh examination, then stopped <cause>; each person override (reasoned work revise and the others the design lists) prints and records its impact statement first (R-143-m1e); mechanism 2's unit-round handoffs as amended in plans/designs/machinery-mechanisms.md.

Sites: the design cites them (critique rounds verified them on main); this tree is main after goals 1a, 1b, the drain and ledger-reads, so re-read each before you change it and say in the return where one moved.
# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (impacted tests only; the full gate runs per batch on the goal branch)
Build, vet, the packages you changed with -count=1 -timeout 30m, the cmd tests you added or changed by name, their mutations, then `go run ./cmd/devgate static`. If you change a message or skill text, also run `go test -count=1 -timeout 30m -run 'TestAudit|TestInstruction' ./cmd/metasystem/`. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
