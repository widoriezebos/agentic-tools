# Brief: process-changes-cover-declarations-and-interventions, unit U0, correction 2 (the last allowed)

Working Mode: Implement
The uncommitted U0 build plus correction 1 are in this worktree; keep them. The read of correction 1 found one BREAKING finding (defect class 2, wrong authority):
- internal/processchange/check.go:102-114 reuses previous.Act for the same Operation (the unit's .inputs/<key> directory, stable across runs), then json.Unmarshal(retained, &act) where act.AfterArgv shares its backing array with c.AfterArgv: the unmarshal overwrites the caller's argv with the retained one, so `slices.Equal(act.AfterArgv, c.AfterArgv)` always passes when the new argv is no longer than the old. Status returns applied; intent_process_check.go:75 launches the agent's own (changed) command. Reproduction: ordinary check `make d`; person check `go test ./x` (applied); agent check `rm -rf /tmp/zzz` -> applied, no error, caller argv rewritten.
- Second effect: a person's longer argv gets a fresh array, the guard fails and the person's act is refused ("names a different check or admission") — a person must never be refused here.
Fix: never alias the caller's argv (unmarshal into a fresh value / copy before compare); an agent whose argv differs from the person-applied act is held (not applied); a person's act with a different argv is admitted as a new person act. Add a test with exactly the reproduction above (agent's changed check held, caller argv unchanged) and one for the person's longer argv admitted; mutation: restore the aliasing -> both red.
Size: the unit is at 261 net production lines (cap 250): make the fix without adding net lines where you can, and remove any duplication you find in the unit's own code; report the final count.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
