Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal goal-records-owned-by-the-testing-contract)
Date: 2026-09-10

# Fold round four: rebase onto today's main, nothing else

Follow-up round on chain ha-surface1-20260910. Round three was reviewed
clean (ha-surfacecrit3x-20260910, three notes). Its landing was refused
before the receipt: main moved under the chain. Two landings of
2026-09-10 (2f764c609, a breach-stopped claim no longer wedges its
machine; 0f886ca2a, seat fixtures under real agent ancestry) changed
cmd/metasystem/goalsync_mutations_test.go and
cmd/metasystem/process_verbs_test.go, so the chain's diff applied onto
main no longer yields the blobs the critics reviewed for those two files.

## Mandate

1. Rebase the chain's worktree onto main at its current tip (fetch
   origin; the tip at dispatch is 5d203ad3b or later). Resolve the
   conflicts in the two test files above keeping both sides: main's
   fixture changes and this chain's changes (the proc detach test in
   process_verbs_test.go; the human-authority changes in
   goalsync_mutations_test.go). Take main's side for anything the chain
   does not touch. If any other file conflicts, resolve it the same way
   and name it in your return.
2. Change nothing else: no new code, no new tests, no contract change.
   The resulting diff against main must be the round-three change and
   only that, re-based.
3. Prove: go build, vet, gofmt; go test on cmd/metasystem,
   internal/humanauthority, internal/steward, internal/gaterun,
   internal/goal (the two cmd tests that fail on main today, the
   process-classifier repair test and the frozen public-v1 corpus test,
   are not this chain's).

## Constraints

Wall-clock budget: 20 minutes. Return per the implementer schema and
report the round as your own. Stop at a gap that needs a decision no
page has made; report it with the resolution you propose. Never delete
written work.
