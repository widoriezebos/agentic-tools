# Brief: runs-advance-on-their-own, unit D1, correction 1

Working Mode: Implement
The uncommitted D1 build is in this worktree; keep it. The read found:
1. BREAKING: the new refusal code LAUNCH_PROVIDER_CAPACITY (internal/launch/hostcapacity.go:121,124) has no row in internal/refusal/register.go; `go test ./internal/refusal -run '^TestHCL03'` fails ("collected refusal token ... has no row or exclusion"). Resolved by item 2 if the code goes away; otherwise register it.
2. A second admission: the providerCapacity call added to Manager.Start (launch.go:192, hostcapacity.go:115-127) refuses agent builds on provider marks, fails closed when the provider record is unreadable or its lock busy (~500ms) and tells the agent "waiting for recovery", which never fixes a corrupt record (defect class 1). Goal 3 put only load and build-cap admission in Manager.Start; provider marks pause clocks and the steward. REMOVE the provider check from Manager.Start (and its refusal code); D1 keeps work pending only on the existing admissions.
3. Remedies an agent cannot complete (defect class 1): intent_selection.go:398-401 workContinuation gives every pending act "retry ... when capacity is available"; for LAUNCH_BUILD_PERSON waits the remedy is the person's act at the enrolled terminal (the held result's PersonActRemedy). intent_work.go:1778: the `work build run:RUN` route must print `work build run:` for its pending codes, not `work wait run:`.
Each fix gets a test that fails on the old code (mutation). Also report what of the design's managed-suite half (test.go acquire, QueueDurationMS, deadline after acquire) is unbuilt; do not build it in this correction.
Run the changed tests by name plus `go test ./internal/refusal -run '^TestHCL03'`; nothing package-wide.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
