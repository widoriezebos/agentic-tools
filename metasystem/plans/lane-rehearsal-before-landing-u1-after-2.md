# Unit U1 correction 2 (last): the candidate engine runs every lane step; one red test; no stray records

Working Mode: Implement. Correction 2 of unit U1 of lane-rehearsal-before-landing after its Opus read (RETURN, 4 material). Worktree uncommitted; change only what this brief names; leave fixes uncommitted. This is the unit's last correction: what is not fixed here becomes a named follow-up unit in the design, not a third round.

## F-2 (reproduced): `go test ./cmd/metasystem -run '^TestIntentPublicCoverage$'` is red
`landing rehearse` has no example and is missing from the public-pairs list (intent_coverage_test.go ~35, ~85). Add one example to the command and the pair to the list. Run the WHOLE changed package's existing tests by name family: every `TestIntent*`, `TestAudit*`, `TestLanding*` must be green.

## F-3 (design D1 threat): wake, batch selection and `landing set` run in the invoking binary, not the candidate engine
Today `landing_rehearse.go` ~25-66 runs `landing set` in-process and `green.go` ~138-152 builds a hand-copied `lane.AgentKeeper` with `plain.WakeReasons`/`plain.SelectBatch` compiled into whichever binary ran `landing rehearse`; only prove, push, status and work land go through `s.Engine`. Fix: every lane step of the scenario goes through the candidate engine binary `s.Engine` as a child process: `landing set`, and the keeper's tick through the engine's own verb (`metasystem landing run` if it performs one keeper tick/wake-select-start, else the verb the real keeper uses; read cmd/metasystem/landing_agent.go ~260-310 to find the real entry) so the rehearsal exercises the real wiring and the in-process copy is deleted. Then `CheckIdentity` stays the single identity check (of s.Engine). If no verb performs the tick, keep the copy but refuse a pass unless `os.Executable()` carries the candidate's build stamp (same check as CheckIdentity) and record the gap in the handoff as follow-up unit U1c.

## F-4: stray records
Delete from the change: `records/rehearsals/5c55bbef*.json`, `records/rehearsals/006a4c79*.json`, `records/rehearsals/54133fff*.json`, `records/lane-rehearsal-u1-setup-deadline-before-fix.json`. A rehearsal run from a test registry or of a private clone must not write under this repository's `records/` at all: the record path is under the scratch store unless the run is the real `landing rehearse` of this checkout's candidate; add the guard and a test.

## F-1: size (about 700 production lines against a 250 limit)
Decision by the steward (recorded): the unit is not split after the fact; F-3's deletion of the keeper copy is the subtraction. Report the production line count after this correction; the design gets a note that U1 landed at that size with the read done on the whole.

## Also (non-material notes, do if cheap): templates read from `s.Module/plans/rehearsal/green` (the pinned candidate), not the working tree; the setup retry removes the first scratch root or records it.

## Checks
`go vet ./...`; `go test -count=1 -timeout 30m ./internal/landing/rehearsal/... ./internal/landing/plain ./internal/layering`; `go test -count=1 -timeout 30m ./cmd/metasystem -run 'TestIntent|TestAudit|TestLanding'`; `go run ./cmd/devgate static` (ignore only the design-ledger goal complaint); one real green rehearsal run and one canary-mutation run with their breakdowns. Never edit this repository's testing.json; never open metasystem.conf.local; do not commit, push or rebase this checkout. Report: diff --stat, every exit, both wall times, the production line count.
