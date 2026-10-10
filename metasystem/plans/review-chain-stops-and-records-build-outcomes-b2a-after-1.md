# Brief: review-chain-stops-and-records build-outcomes-b2a, correction 1

Working Mode: Implement
b2a is uncommitted in this worktree. One Opus read found four material defects; fix exactly these.

1. cmd/metasystem/intent_work.go:474-495: the flake judge receives the proof command's folder and builds main's path relative to the installation, so its git read fails in real use (`git show origin/main:../../...` "outside repository"; `origin/main:testing.json` does not exist) and every red naming failed tests becomes "cannot be attributed". Pass the repository root (git rev-parse --show-toplevel) and the repository-relative path; make the cmd test's fake git match the full path, not a suffix (intent_unit_attribution_test.go HasSuffix cases). Mutation: the folder-relative path, red.
2. intent_work.go:491-495 uses only Known; landing also requires !Affected (internal/landing/plain/replay.go:48). In unit_attribution.go:124-127 a known flake failing twice gets cause flake with no base comparison, so a change that breaks a registered flaky test is never own. Require !judgements[u].Affected for the repeat; if the repeat is still red, run the base comparison instead of stopping at flake. Test: an affected registered flaky test red twice is compared with the base and is own when the base is green (mutation: Known only, red).
3. unit_attribution.go:180-185 `git worktree add --detach <round>/base-N` is never removed and registers a worktree in the shared repository. Export the base tree into a scratch folder without registering a worktree (e.g. git archive or read-tree into a temporary index), run there, retain the exit, remove the folder by the path the code created (never a variable path from input). Test: after an attribution, no new entry in `git worktree list` and no base-N folder remains.
4. unit_attribution.go:239-247: RecordMain runs before step.Cause is saved; a lost process between them records the main sighting twice on resume (internal/goal/trunkred.go:697), which can turn a pending flake into a trunk red. Save a "main recorded" marker before the call and skip on resume (or make the recorder skip an attempt it has seen). Test: resume after RecordMain, one sighting.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
