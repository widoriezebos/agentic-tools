# Brief: review-chain-stops-and-records, unit build-outcomes-a (finish the holds)

Working Mode: Implement
build-outcomes split at the size cap (your earlier run stopped at 255 production lines; read its handoff at plans/review-chain-stops-and-records-build-outcomes-handoff.md in the m1e checkout /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem, and the original brief plans/review-chain-stops-and-records-build-outcomes.md there). This unit is part 1 of that split: the build result holds and their public remedies. The uncommitted work in this worktree is its start; keep it.

Finish, production paths only: the gap stop and the oversized hold each raise their question through the unit-stop ask (one executable act each: the person's `work review ... --reason TEXT --by NAME` for size, the gap revision for a gap), recovery after a restart reads the retained hold, an agent cannot accept the size, the impact is printed and recorded before it takes effect, and the pending proof resumes (no new build). Move everything about failure attribution (baseline comparison, own attribution, flake repeat accounting, launch-id reservation reconciliation, moved-tree retry) OUT of this unit: if the current uncommitted code started on it, keep only what the holds need and leave a clear seam; part 2 builds it. Repair the fixtures and message wording the holds change.
Size: this unit at most 250 production lines in total (count the existing uncommitted production lines). If it cannot finish within that, stop and report.
Public-verb test: `work build` returning an empty diff produces a gap stop with no proof or read launched, and its question names the gap revision; an oversized diff holds, an agent's size acceptance is refused, the person's act prints and records its impact before resuming the pending proof (no new build). Mutations: move the gap check after proof; let an agent accept the size.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (impacted tests only; the full gate runs per batch on the goal branch)
Build, vet, the packages you changed with -count=1 -timeout 30m, the cmd tests you added or changed by name, their mutations, then the broad cmd selection for the area you touched (at least `go test -count=1 -timeout 60m -run 'TestLanding|TestWork|TestIntent|TestGoal|TestReview|TestUnit|TestClaim|TestSession|TestHelm|TestPolicy|TestQuestion|TestKeeper|TestDispatch|TestEvery|TestAudit|TestInstruction' ./cmd/metasystem/`: every test there must pass, not only yours; a red you did not cause still blocks done), then `go run ./cmd/devgate static`. Report done only when all of these exit 0. If you change a message or skill text, also run `go test -count=1 -timeout 30m -run 'TestAudit|TestInstruction' ./cmd/metasystem/`. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
