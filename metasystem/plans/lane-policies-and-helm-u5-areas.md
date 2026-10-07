# Brief: lane-policies-and-helm, unit U5 claim areas

Working Mode: Implement
Goal lane-policies-and-helm. The spec is the accepted design plans/designs/lane-policies-and-helm.md, Decision 5 (read it whole; binding) and its U5 test and round-4 acceptance item under "Public-verb tests". Build only U5.

Sites, read on this tree (3a0dbd6b7): DeclaredUnits internal/launch/admit.go:457 (table parsing :210); ClaimRecord internal/goal/file.go:337, parse :1440, render :1862; claimRequest against the fetched ledger internal/goal/verbs.go:1553; claimQuotaRefusal :1393; goal.Claim refuses humans :1384-1385; 1a's claim/queue adapter claimLaneReader cmd/metasystem/landing_plain.go:38 (no configured lane at :40 reads as an empty queue); at-rest quota internal/goal/validate.go:399 (no host queue; do not add one).

What U5 builds, in short: launch.DeclaredAreas (page-level Areas: and/or an areas column, JSON array for many values, normalized, escapes rejected); claim fields areas, areas-source (design id + digest), areas-known, pinned from the same accepted design unit admission uses, verified at publication; goal.AreasOverlap by literal-prefix rule (no actor input); goal.ClaimAreas over the fetched ledger's claims, 1a's newest queue state and the seat's local main containment, reread per publication retry; refusal names the blocking goal and area with next step `metasystem goal claim` after it lands or is dropped; next-goal selection filters with the same predicate; unknown areas (missing, malformed, unreadable design or queue) publish with a recorded "areas unknown" warning, never a hold; a waiting hand-in keeps its areas (snapshot copied onto the hand-in); the one-slot quota stays.

The five questions are answered in Decision 5's "Five answers"; implement those, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (impacted tests only; the full gate runs per batch on the goal branch)
Build, vet, the packages you changed with -count=1 -timeout 30m, the cmd tests you added or changed by name, their mutations, then `go run ./cmd/devgate static`. If you change a message or skill text, also run `go test -count=1 -timeout 30m -run 'TestAudit|TestInstruction' ./cmd/metasystem/`. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
