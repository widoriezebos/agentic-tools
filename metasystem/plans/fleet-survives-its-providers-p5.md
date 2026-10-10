# Brief: fleet-survives-its-providers, unit P5 (bounded abnormal revival and current-load build admission)

Working Mode: Implement
This worktree is the P4 branch of goal 3 (P1, P4, P4-2 committed); P2 (provider marks) lives on the goal branch and is merged later. Build "P5 — Bounded abnormal revival and current-load build admission" of plans/designs/fleet-survives-its-providers.md and ONLY that, including the 2026-10-08 acceptance items: the round-3 folds naming P5, and P5's printed person remedy is `machine revive SEAT`, shown as unavailable until fleet-provider-and-session-recovery R2 lands (print it so; never print a verb that does not exist).
Part A (~110 lines): in internal/steward/revive.go (:109, :169, :218 per the design) a stable seat retains at most two automatic abnormal restart attempts per rolling hour, stopping earlier on the same death class twice or no retained work progress after revival; count before the irreversible launch; preserve history across steward re-arm; stop with the existing stop/ask owners. Provider-limit holds and P4 handoffs consume no abnormal attempt; an unknown launch outcome cannot create another allowance. P2's provider marks are not on this branch: take the hold's identity through the existing outage reader and leave the P2 consumption point marked for the merge.
Part B (~140 lines): `host.builds=auto|N|person` and a positive declared `host.load-max` through the existing configuration/policy owners (internal/config/policy.go:59, internal/config/defaults.go:66), checked in Manager.Start (internal/launch/launch.go:105, :238) immediately before creating the Starting record, against current load from P1's hostcapacity reader; a refusal writes no launch record (runs-advance D1/D2 depend on this: internal/launch/hostcapacity.go:78 would count it as Starting); `person` asks; a person's start is never refused.
Size: at most 250 production lines. If it will not fit, build Part A first and report the rest.
Public-verb test (TestFleetRevivalAndBuildPublicAdmission, the design's): through steward start/status and `work build`: two abnormal restarts per hour then a stop with the printed remedy; repeated class or no progress stops earlier; a provider hold consumes nothing; over host.load-max a build is refused with no launch record and a person's build starts. Mutations: count after launch; write a Starting record on refusal.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
