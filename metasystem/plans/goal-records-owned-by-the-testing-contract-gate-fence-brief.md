Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal goal-records-owned-by-the-testing-contract)
Date: 2026-09-10

# Bounded policy correction: the gate-fence section moves to the cadence set

Since 1b12f534 the landing receipt copies the ENROLLED engine (built
at HEAD) into the isolated candidate worktree and runs the selected
sections there. section/gate-fence-fixtures runs the dispatch preflight,
whose engine/script skew check refuses any candidate that changes
engine code ("engine commit <HEAD> is older than checkout commit
<candidate> and engine or agent scripts changed"); a candidate-built
engine cannot be enrolled instead. So since 1b12f534 no chain that
changes the engine can mint a sufficient receipt on this host: three
reviewed chains (hp-terminal-grade-for-stopping-acts,
dispatch-cap-necessity, stop-message-truth) are closed and waiting.

Until the receipt runs its sections with a proof engine built from the
candidate, the section cannot serve as a per-landing proof for engine
changes. It keeps its place in the standing validator's cadence set,
where the enrolled engine and the checkout agree.

## Mandate

1. In metasystem/testing.json remove `section/gate-fence-fixtures`
   from the `runtime-custody` surface's `deep` list. Leave it in
   `cadence`. Nothing else in the contract changes.
2. Prove with the contract's check verb and `go test
   ./internal/testpolicy/` that the contract still validates.
3. No other file changes.

## Proof

The commands in 2 and `git diff --stat` showing only
metasystem/testing.json. Report the round as your own.

## Constraints

Wall-clock budget: 10 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
