# Brief: machinery-measures-its-own-process, unit U4 (the same detector covers settings/check interventions)

Working Mode: Implement
U1a, U1b, U2, U3, U3b, U5a and U5b are committed on this branch. Build Decision 4 of plans/designs/machinery-measures-its-own-process.md ("The same detector covers settings/check interventions by the steward (U4)") and ONLY that, including the acceptance items folded at 22:55 that name U4: processchange.RepeatedClass at proposal admission under the target process lock, at U2's seams ApplySetting and AdmitCheck (reached by any caller, not a steward-only helper); classes added-check, widened-check, changed-policy, changed-process from retained before/after identities (requester labels never change the class); the window keyed by canonical target checkout and actual actor lineage with the episode resolved from that lineage's current work; an agent proposal must name the resolved goal (positional G for work build, --goal G for settings), a missing or mismatching goal holds before effect and never opens a fresh window; the progress measure is U1's named expected measure moving in the declared direction since the preceding applied intervention (no observation = unknown, not improvement); a repeat stops through U3's stop/question path with stop identity (checkout, lineage, episode, class, proposed act id); a standing drift stop is reused; a person's successful execution of the exact pending act closes its ask. UnitRunner.Revise and budget owners are out (follow-up).
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test: the design's U4 test: an agent widens a check twice in one episode without the measure improving: the second is held as a repeat with the reverse/person remedy; a person's execution of the pending act closes the ask; a different goal opens no fresh window without naming its goal. Mutations: key the window by invocation; let a label change the class.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
