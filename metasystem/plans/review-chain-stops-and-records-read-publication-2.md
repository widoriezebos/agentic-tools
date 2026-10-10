# Brief: review-chain-stops-and-records, unit read-publication-2

Working Mode: Implement
read-publication's first part (carry, range handling, rebase reporting) is committed on this branch; its handoff, /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/handoff-review-chain-stops-and-records-read-publication.md, names the remainder, which is this unit and only this: plans/designs/review-chain-stops-and-records.md Decision 5 paragraph 4, plus the item moved here from tree-boundaries (design page 03:50).

Build: (1) manual and commit-form read publication goes through the tree reservation like the unit form (tree/unit/run order, the fresh tree comparison under the tree lock, locks released during the push); (2) `work commit G --work U` exposes the existing branch commit owner for hand changes with the unit trailer and the same subject and gate path (the correction commit's own static and cheap gate, restored in round-result-fixes, applies), callers need no undocumented trailer; (3) a person's `goal done G --reason TEXT --by NAME` prints and records the impact of closing the remaining review chains with that reason before the conclusion takes effect, then closes those chains after the goal act succeeds, never as clean reads, idempotent and discoverable from the concluded goal if interrupted; an agent's conclusion still cannot bypass transferred work or existing completion obligations.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test (from the design): `work commit G --work U` records the trailer and gates the commit; a person's `goal done` prints and records its impact before conclusion and chain closure, preserving closure reasons, and an interrupted closure completes on repeat; manual publication with the tree changed after the read is not clean. Mutations: close chains as clean reads; skip the gate on work commit.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
