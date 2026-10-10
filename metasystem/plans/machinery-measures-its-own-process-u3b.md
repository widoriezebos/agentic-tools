# Brief: machinery-measures-its-own-process, unit U3b (undo and unset; plus U3's missing test)

Working Mode: Implement
U3's first part is committed (b5a2a1ca3): drift stops with the elapsed and suite-minute bands, one stop per goal and unit, admission held while a stop stands. Handoffs: /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/handoff-machinery-measures-its-own-process-u3.md and -u3-after-1.md.
0. U3's stop finding (test only): internal/processmeasure/decide.go:42 (band at :38): a nil check allowance (CheckMinutes is optional, internal/launch/unit_plan.go:36,:47) must show unknown and open no stop; nothing proves it (nil -> zero survives). Add one assertion through `goal status` with a plan without checkMinutes: the band shows unknown and no stop opens. Mutation: nil limit -> zero, red.
1. The rest of Decision 3 (plans/designs/machinery-measures-its-own-process.md): the first act printed for a process-change stop is the exact reverse command with impact and before/after; reversion as a new act through the same writer: `settings set KEY BEFORE --repo CHECKOUT --undo ID`, and `settings unset KEY --repo CHECKOUT --undo ID` only for an act whose before-layer was absent (inheritance restored, never copied into an override; reuses the settings parser/writer and authority checks; never removes a whole file); an undo never deletes or rewrites the original act; an undone change resolves the episode's intervention hold ("resolved by removal") without declaring the cumulative measure in band; new affected executions open a new episode. The default policy makes undo a person's remedy; a person's undo is never refused.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 (the printed reverse and set --undo first) and report the rest.
Public-verb test: an agent's settings change, a drift stop showing the reverse command; following it records a new act with the original intact and resolves the hold; an act whose before-layer was absent prints and runs `settings unset`, and `settings show` then reports the inherited source. Mutations: undo writes an override for an absent before-layer; undo rewrites the original act.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
