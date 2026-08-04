# Plan: Prove-First Course Correction

- Owner: unclaimed (written 2026-08-04 from the 24-hour review; single session)
- Goal and current status: act on the review's findings: the gate was flaky, design outran proof, commits went red in CI, retained findings piled up untriaged, and no mission has ever run. Status: CC-1, CC-2, CC-4 done; CC-5 resolved by the human; CC-3 is the only item left and nothing blocks it
- In flight right now: nothing
- Decisions made (and who made them): the review's findings accepted by the user 2026-08-04; the proportionality ruling and the container ruling both made by the user the same day, which resolved CC-5 without further design
- Waiting on the human: nothing blocking (D-CC1 through D-CC4 answered 2026-08-04); the Devin self-test is unchanged from the master plan
- Dead ends: two hardened-mode designs (trust roots, then verifier domains) were built and cut when the user ruled that isolation comes from a container or VM. Do not rebuild local enforcement against a same-user agent; it is impossible and the rounds proving it are recorded in the ledger
- Next step: CC-3, the minimal runner and Mission Zero. Answer only the decide-now findings the minimal runner itself needs (the lease and turn-log ones, C11-3, C11-5, C11-6, C11-8); the rest wait for the evidence

This plan amends the SEQUENCING of `plans/agent-orchestration-design.md`; it does not amend that plan's design content, which stays canonical. When these items close, this file is deleted and anything durable moves to its owner per `plans/README.md`.

## Findings this plan answers

From the self-review of the first 24 hours, all five accepted by the user:

1. The validation suite was flaky, so the gate could not be trusted.
2. Three of five pushed commits went red in CI; local green was being treated as sufficient.
3. Design outran implementation (one period ran eight design receipts to two implement), and IL-6 was adopted then immediately strained.
4. Sixty-five retained watch-list findings sat as one undifferentiated pile, which later blocked an implementation brief outright.
5. Proportionality was unanswerable from inside: nothing had run, so no evidence existed about which parts earn their keep.

## Items

| Id | Item | Status |
| --- | --- | --- |
| CC-1 | Fix the flake as a class, not per fixture: scaled budgets, isolated contenders, diagnostics naming elapsed and cap, IL-1 preserved | DONE (`fddf870`), proven by three consecutive green runs, a forced-failure run, and the orchestrator's own green run |
| CC-2 | Stop treating local green as sufficient: verify against the pushed state, and keep CI green as the gate of record | DONE in practice: HEAD is green and pushed; the standing rule is that a red CI run blocks the next dispatch |
| CC-3 | Prove-first: build the minimal runner and run Mission Zero, the smallest real unattended mission (make one failing test pass on a scratch repository), before any further mission-mode design | NOT STARTED, after CC-4. Scope per D-CC1: lease, one host turn from the shipped prompt artifacts (6.2a), runner-side measurement, one ledger entry in the stop-loss grammar, four end states. Fences per D-CC3: cycles 5, jobs 5, concurrency 1, job cap 10 minutes, wall clock 1 hour, cheap delegate models |
| CC-4 | Triage every retained watch-list finding into "decide now" (a designer decision, blocks the build) and "test later" (a failing-test obligation under ORCH-21). The third gap-stop proved the pile mixes both and stalls implementation | DONE 2026-08-04 (`investigator-20260804t123608z-ac09`), triage table in `plans/agent-orchestration-watchlist.md`. A third bucket was needed: 31 decide-now, 6 test-later, 28 already-resolved, the last being the overdue pass 6.2b promised. All ten S4 findings closed as shipped fixtures. Item 20 holds 38 of the 65, which Mission Zero's evidence is meant to collapse rather than a reason to decide them now |
| CC-5 | Answer proportionality with evidence rather than argument | RESOLVED by the user's rulings: the cooperative-agent baseline stands, isolation is the operator's container, and Part 9 shrank to documentation. Mission Zero remains the evidence for the rest |

## Decisions, answered by the human 2026-08-04

- **D-CC1, runner scope: minimal, then Mission Zero.** Build only what completes one honest cycle: the lease, one host turn assembled from the shipped prompt artifacts, the runner-side measurement, one ledger entry, and the four end states. Hooks, guard cadence, reconciliation turns, and the proof bundle wait until Mission Zero shows they are needed. Item 20's remaining scope is decided by that evidence, not in advance.
- **D-CC2, target: a scratch repository with one deliberately failing test**, the mission's goal being to make it pass. The gate is unambiguous, nothing real is at risk, and no peer session can collide with it.
- **D-CC3, spend: tight lifecycle fences**, sized small (a handful of cycles, a few jobs, short per-job timeouts, concurrency one) with cheap models for delegates. A mission that cannot finish inside that envelope is itself the finding, and the fences are the enforcement since no invoice-level control exists.
- **D-CC4, continuity: this session continues.** The handoff discipline stays untested for now; the plans and ledgers remain the record either way.

## Completion

Complete when CC-3 and CC-4 are done and Mission Zero has run once end to end with its evidence recorded. Then this file is deleted, its durable lesson (prove before elaborating) having already landed as IL-6 in the instruction ledger.
