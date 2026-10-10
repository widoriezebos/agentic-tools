# Brief: review-chain-stops-and-records, unit read-publication-3 (the person's conclusion cleanup)

Working Mode: Implement
read-publication parts 1 and 2 and their fixes are committed on this branch. This unit is the last remaining item of Decision 5 paragraph 4 (plans/designs/review-chain-stops-and-records.md), and only that: on a person's `goal done G --reason TEXT --by NAME`, print and record the impact of closing the goal's remaining review chains with that reason BEFORE the conclusion takes effect; after the goal act succeeds, close those chains with the reason, never as clean reads; the cleanup is idempotent and discoverable from the concluded goal if interrupted (a repeat completes it); an agent's conclusion still cannot bypass transferred work or existing completion obligations.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test (from the design): a person's `goal done` prints and records its impact before the conclusion and the chain closure, preserving closure reasons without calling residual reads clean; interrupt after the goal act and before the closure: a repeat completes it; an agent's `goal done` with open transferred work is refused. Mutations: close chains as clean reads; close before the goal act; skip the impact record.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
