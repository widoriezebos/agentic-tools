# Brief: briefs-carry-their-rules, unit B4 (bound correction decisions and recurring classes)

Working Mode: Implement
B2 and B3 are committed on this branch. 0. FIRST: origin/main (goal 2 landed) is merged into this branch (fbeadd256). TestWorkBriefResolvesAndChecksBaseCitations and TestWorkBriefShowsReadersDeletionAndEffort (B2, B3) now fail at goalsync_request_facts_test.go:112 "request root = .../metasystem, want ...": goal 2's integration fix-forward made goal endpoints resolve from the installation root (TestGoalEndpointsUseInstallationRoot on main). Update B2's and B3's test beds to main's endpoint resolution (fixture only; production on main is right; never loosen an assertion), and run both tests green before B4.
Build unit B4 of plans/designs/briefs-carry-their-rules.md (accepted; read "Decided by m1e for Wido" 12:30 and its B4 acceptance item) and ONLY B4: bound correction decisions and the recurring defect classes in a unit's correction brief, derived from recorded reads; the four hand-written defect-class sentences are copied once into a shipped template or constant owned by B4 and are the fallback until one class recurs in the recorded window; B4 deletes the block from plans/lane-policies-and-helm-u1-settings.md (lines 9 onward hold it: delete only the defect-class block, keep the Check section that follows); production never reads a plan file.
Size: at most 250 production lines (the design estimates 245).
Public-verb test: the design's B4 test, plus: with the plan file absent the brief carries the constant's four classes; once a class recurs in two recorded reads, the brief names it from the records. Mutations: read the plan file; drop the fallback.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
