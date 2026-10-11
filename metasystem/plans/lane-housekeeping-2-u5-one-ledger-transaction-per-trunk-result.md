# lane-housekeeping-2 unit 5: one ledger transaction per trunk result

Working Mode: Implement. Worktree = this checkout, off origin/main. At most 250 production lines. Leave everything uncommitted; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json by hand. Real shapes (read-only, never edit): the landing install /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/ (results.jsonl, stops.jsonl, queue.jsonl, fixes/, proofs/*.log), origin/main's recent "trunk-red clear" and "goal slice-start" commits. Run every test family that drives the changed verb, the changed internal packages whole, `go run ./cmd/devgate static`, `./internal/layering`, `./cmd/metasystem -run TestAudit`. Report: diff --stat, production line count, every exit, each mutation. Read memory/rulings.md first (R-104 injected clocks, R-138 no shell-outs, R-142 no HAL9000).
Design: plans/lane-housekeeping-2-design-brief.md unit 5 (from finding 107).

## Measured defect
Clearing eight incidents after one green trunk proof wrote eight "trunk-red clear …" commits to main (04:23 on 2026-10-11), and registering them wrote one record commit per proof; every commit is a main move the lane has to classify.

## Build
A trunk result (red: register its incidents; green: clear the register's open incidents it covers) writes ONE ledger commit ("trunk-red record <attempt>: N incidents" / "trunk-red clear <attempt>: N incidents") through the goal verbs' publish path (internal/goal/trunkred.go: the register's record/clear functions and the landing incident recorder in cmd/metasystem/intent_landing_incident.go / plain.RecordMain seam), carrying every incident in one transaction; the incident records and `incident list` are unchanged in content. The commit message names the attempt and the count.

## Tests
Fixture: a green trunk result that clears 8 incidents produces exactly one commit on the fixture origin (mutation: per-incident commits → 8 → test fails); a red trunk result with 3 failed tests registers in one commit; the real shape of today's trunk-red register (plans/goals/trunk-red.json after 7481259dd on origin/main) round-trips.
