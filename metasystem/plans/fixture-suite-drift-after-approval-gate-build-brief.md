Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal fixture-suite-drift-after-approval-gate)
Date: 2026-09-04

# Build brief: five fixture suites red on main after the execution-approval gate

Goal `fixture-suite-drift-after-approval-gate` (tier 1, approved by Wido 2026-09-04, box 1 hour / 3 attempts / 1 active job / no review round). No critic in this chain.

## The defects (found by part one's re-review, records/misc/severity-tiered-rigor-build1-critique-cc2.md, F-12 to F-16)

1. `scripts/agents/channel-fixtures.sh` lines 83 and 128 claim goals with a budget tuple and `--approved-ref`. Since the execution-approval gate landed (c285d5a0), a claim after approval takes no tuple: the fixture must approve the goal first (`goal approve` with a temporary human word and a review-by date, the way seats do it) and then claim bare.
2. `scripts/agents/dispatch-fixtures.sh`: the same tuple-bearing claim (around line 1101 and any other), and the `config tailor` calls that run without `--runtimes` (check every call; three already pass `--runtimes fake`). Its steward-continuation leg times out on census freshness under load: give the leg the same patience the product uses (count attempts, not wall-clock; a wall-clock bound only as a silence failsafe).
3. The channel poll verb has a fixed 15-second context (`cmd/metasystem/channel_verbs.go` lines 59, 120, 200, 303). Make it one configured value (`channel.poll-timeout-sec` in the config, default 15) so the fixture and slow hosts can raise it; the fixture sets it explicitly.
4. The adoption fixture's vendored receipt leg depends on state-root resolution (the leg lives in `scripts/agents/static-reproof-fixtures.sh` or the supervision suite; find the leg that vendors a receipt and fails before its changed line) and must resolve the state root the way the product does.
5. `scripts/agents/supervision-fixtures.sh` scenario `stop-hook-monitor` (around line 1515) asserts the enrollment path; supply the enrollment the product now writes instead of the old path.

6. `scripts/agents/dispatch-fixtures.sh`, scenario `dispatch`, is red on plain main today (m2, 2026-09-04 10:51Z) at the assertion "source-valued roster did not relay only the canonical model downstream" (around line 1370): the job record `model-alias-roster.json` carries `aliasedFrom` and `rosterAliasedFrom` as null while the fixture expects `fake-source`. Decide from the code which side is right (the alias relay moved into the engine's `job resolve-roster` verb with the model-alias landing) and fix that side; do not delete the assertion. The other three scenarios of that suite (mission-runner, adapter-selftest, steward-continuation) pass.

## What to build

Make each of the five scripts green on main on a Mac. Rewrite claims as approve-then-claim; make the poll context configurable; fix the two setup legs. Change product code only for item 3 (one config key, read in one place, declared in `internal/config/validate.go` and documented with the other keys in `docs/orchestration.md`). Do not weaken any assertion: a leg that fails because the product changed is rewritten to assert the product's current lawful behavior, never deleted.

## Verification

Your sandbox cannot run process-owning fixture suites (KI-15). Run `bash -n` on every touched script, `go test ./cmd/... ./internal/channel/...` for the config key, and reproduce each of the five failures by reading the refusal the product gives today (quote the message for each in your return). The orchestrator runs the five suites seat-side and returns any red leg to you as a follow-up.

## Bounds

Touch only the five fixture scripts, `cmd/metasystem/channel_verbs.go`, `internal/config/validate.go` for the one key, and its line in `docs/orchestration.md`. Return within the box.

Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/` (for example `metasystem/internal/channel/report.go`).
