# Brief: machinery-housekeeping, unit U2 (test-keyed flake facts)

Working Mode: Implement
U1 is committed on this branch. Spec: the accepted design /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/machinery-housekeeping.md (read it there): Decision 2 "Extend the shipped flake record rather than build another register", the U2 rows, round-1 change 3 (goal 2 lands first; goal 2's runner.KnownFlake on its branch calls landingFlakeJudge, keep that caller's contract), and R2-M2 (U2's sighting records attempt ids and log paths only; output bytes and digests belong to U3). Goal 2 is not on main yet: keep landingFlakeJudge's signature and results as they are on main so goal 2's caller merges unchanged.

Build: the `flake` class on TrunkRedEntry with identity `flake:` + sha256 of [test unit, exact test name] and the two strings kept; RecordFlake creating or joining all tests of one failed unit in one publication with one operation id (replay does not add a sighting); one FlakeFacts reader joining legacy package-keyed and allowanced entries with the new per-test entries, deduplicated by sighting; the validators, replay decoder, closure predicate and the admission and incident readers the class needs on first use. No new register.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test: the design's U2 test (a failed unit with two failing tests records two test-keyed entries in one publication; replay adds no sighting; `incident list` shows them; a legacy package entry reads through FlakeFacts with its history). Mutation: key by package only; count a replay as a sighting.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
