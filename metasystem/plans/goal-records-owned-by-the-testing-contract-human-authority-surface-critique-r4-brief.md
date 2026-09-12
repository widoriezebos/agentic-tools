Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal goal-records-owned-by-the-testing-contract)
Date: 2026-09-10

# Review brief: the rebase round of the human-authority surface chain (chain ha-surface1-20260910, reviewing round ha-surface1-20260910-r4)

FINDING IDS: chain-unique, continue at HAS-11, never F-n. You are the
fourth critic on this chain. Report `round` as 1 in your return: it is
this job's own round. One focused round.

Why: round three was reviewed clean (ha-surfacecrit3x-20260910, three
notes). Its landing was refused before the receipt because main moved
under the chain: 2f764c609 and 0f886ca2a of 2026-09-10 changed
cmd/metasystem/goalsync_mutations_test.go and
cmd/metasystem/process_verbs_test.go. Round four rebased the chain onto
main, resolving those two files by keeping both sides, and changed
nothing else.

Scope: the diff of round four against main at the rebase base, compared
with the diff of round three against its base. Attack: that the two
diffs are the same change apart from the rebase (same hunks, same
files, same deletions of the two system-login files, same testing.json
demotion); that the two resolved test files carry both main's fixture
changes and the chain's tests with nothing dropped and nothing
duplicated; that no third file changed; that the packages named in the
fold brief pass on the rebased tree. Do not re-review rounds one to
three: three critics did.

# Mandate

1. The rebase is exact: the chain's change, re-based, and nothing else.
2. The two resolved files keep both sides.

If nothing material remains, say so; that closes the chain and it
lands.

# Constraints

Wall-clock budget: 15 minutes. Return per the code-critic schema with
the reviewedTree from the review record beside the computed diff (the
conformance validator refuses from a sandbox; say so).

# Gap Rule

Stop adding at a gap that needs a decision no page has made; keep what
is written; report the gap with the resolution you propose. Never
delete written work.
