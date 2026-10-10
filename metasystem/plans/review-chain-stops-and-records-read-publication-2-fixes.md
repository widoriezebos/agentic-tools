# Brief: review-chain-stops-and-records read-publication-2-fixes (fix unit after the stop)

Working Mode: Implement
read-publication-2 is committed and stopped with one material finding; fix it.

cmd/metasystem/intent_unit_review.go:929, :903, :918: when a run is refused because the tree changed after the read, the saved publication head is deleted; the next run saves the current (changed) tip as its head and publishes the old read on top of work nobody read (the critic's probe: commit `hand`, start its read, hand-commit `hand2` overwriting hand.txt; first repeat refused, second publishes "read hand" over "units hand2"). TestReadPublication... (intent_read_publication_2_test.go:177-181) locks the wrong behaviour in. Fix: the baseline is the read's own reviewed commit, never a later saved tip: a later commit may publish the read only when it changes none of the paths the reviewed unit changed since the reviewed commit (the next unit's own commit publishes; an overwrite of reviewed bytes stays refused on every repeat); on a refusal for a real change, retire that read (recorded, not deleted) so a repeat starts a new read of the new version, and the printed next step is that new read. Fix the test to expect: run B with the next unit's commit publishes; an overwrite of reviewed bytes is refused on the first and every later repeat (mutation: the deleted-head baseline, red).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
