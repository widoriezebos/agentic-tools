# Brief: landing-takes-an-hour, unit U4 (the unit check includes what the unit changed)

Working Mode: Implement
U1 is committed on this branch; U1b/U5 may be uncommitted in the worktree (leave them; stay out of internal/repoproof and the clock). Build unit U4 of plans/designs/landing-takes-an-hour.md (accepted; read "Decided by m1e for Wido" 18:45 and the round tables: R1-3/R1-5 and R2-3 acceptance items) and ONLY U4: goadapter.UnitImpact on git trees and on the working snapshot (untracked and modified files included; non-Go assets such as fixtures and testdata resolved to the packages that own or embed them); `metasystem test impact` printing the selection; the shipped default proof.cheap=metasystem test impact (the unit's own new and changed tests plus the test packages of what it changed, whole for internal/*, cmd/metasystem by impacted test names, never whole cmd); UnitCheck.Base and LANDING_PROOF_BASE on both callers (the unit check and the rebase carry, carry using subject.Parent); process-changes-cover U2a keeps the preview and the deep hold.
Size: at most 250 production lines (the design estimates 245). If it will not fit, build the largest usable first part within 250 and report the rest.
Public-verb test: the design's U4 test: a unit that changes internal/launch runs internal/launch's tests in its own check and goes red there; a unit that changes only a fixture selects its owning package; a rebase carry runs the subject's check with its base. Mutations: select from the committed tree only; drop the base on the carry.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED"; amended 10-08 after goal 2's landing: all 156 integration reds were pre-existing tests of packages the units had changed, none a unit's own test)
`go build ./...` and `go vet` on the packages you changed; then the test package of every package whose production code you changed (the whole package for internal/*, each 1-7 minutes; for cmd/metasystem ONLY the tests whose files name a symbol, verb or fixture you changed, by name, never the whole package); then the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). A red in a changed package's existing test is yours to fix now: fix production where production is wrong, the fixture where the design changed the behaviour (cite the design line), never loosen an assertion. No whole suite: it runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
