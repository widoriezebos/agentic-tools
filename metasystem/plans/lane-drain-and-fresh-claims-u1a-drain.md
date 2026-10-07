# Brief: lane-drain-and-fresh-claims, unit U1a drain

Working Mode: Implement
Goal lane-drain-and-fresh-claims. The spec is the accepted design plans/designs/lane-drain-and-fresh-claims.md, Decision 1 (read it whole; binding) with its U1a test and the round-1, -2 and -3 dispositions. Build only U1a; U1b (helm integration) and U3 are other units.

Sites, read on main: the person-only pause clear laneResumable cmd/metasystem/intent_landing.go:684-712 (keep it; an agent's landing start clears neither a pause nor a drain); the keeper constructor and its Observe cmd/metasystem/landing_agent.go:221/227 (AdvanceDrain runs from Observe after the home lock is released, as SyncStopQuestion does); lock order internal/landing/lane/agent.go:171 (install lock before home lock, never the reverse); the hand-in fence site cmd/metasystem/landing_plain.go:150 and HandIn under the queue lock internal/landing/plain/queue.go:112; pause state internal/landing/lane/pause.go:35; status internal/landing/plain/status.go:114; wake internal/landing/plain/wake.go:39; regeneration internal/landing/plain/regeneration.go:34/61 and resolve.go:75 (a drain waits out a running regeneration); proof start prove.go:324.

What U1a builds, in short: the drain record (atomic replace, read without a lock by fence rechecks), `landing drain` (a person's act), the HandIn fence (a refused agent hand-in names `metasystem work land G` once landing status shows admission open; following it after a person's start succeeds), `landing start` by a person clears pause and drain, an agent's start clears neither, AdvanceDrain over today's queue, proof and regeneration records (draining -> held when the queue is finished), status "draining, N waiting"; unknown membership is status data, never an Observe error; an AdvanceDrain write error does not hold the keeper.

The five questions are answered in Decision 1; implement those answers and say in the return where each lives in code.
# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (impacted tests only; the full gate runs per batch on the goal branch)
Build, vet, the packages you changed with -count=1 -timeout 30m, the cmd tests you added or changed by name, their mutations, then `go run ./cmd/devgate static`. If you change a message or skill text, also run `go test -count=1 -timeout 30m -run 'TestAudit|TestInstruction' ./cmd/metasystem/`. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
