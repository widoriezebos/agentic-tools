# Plan: Prove-First Course Correction

- Owner: unclaimed (written 2026-08-04 from the 24-hour review; single session)
- Goal and current status: act on the review's findings: the gate was flaky, design outran proof, commits went red in CI, retained findings piled up untriaged, and no mission has ever run. Status: CC-1 done; CC-2 done; CC-5 resolved by the human; CC-3 and CC-4 not started
- In flight right now: nothing
- Decisions made (and who made them): the review's findings accepted by the user 2026-08-04; the proportionality ruling and the container ruling both made by the user the same day, which resolved CC-5 without further design
- Waiting on the human: D-CC1 below; the Devin self-test is unchanged from the master plan
- Dead ends: two hardened-mode designs (trust roots, then verifier domains) were built and cut when the user ruled that isolation comes from a container or VM. Do not rebuild local enforcement against a same-user agent; it is impossible and the rounds proving it are recorded in the ledger
- Next step: CC-4's triage, then CC-3's Mission Zero

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
| CC-3 | Prove-first: build the minimal runner and run Mission Zero, the smallest real unattended mission (make one failing test pass on a scratch repository), before any further mission-mode design | NOT STARTED, next after CC-4 |
| CC-4 | Triage every retained watch-list finding into "decide now" (a designer decision, blocks the build) and "test later" (a failing-test obligation under ORCH-21). The third gap-stop proved the pile mixes both and stalls implementation | NOT STARTED |
| CC-5 | Answer proportionality with evidence rather than argument | RESOLVED by the user's rulings: the cooperative-agent baseline stands, isolation is the operator's container, and Part 9 shrank to documentation. Mission Zero remains the evidence for the rest |

## Decision reserved for the human

**D-CC1: how minimal is Mission Zero's runner?** Recommended: the smallest runner that can complete one cycle honestly, which is the lease, one host turn through a shipped prompt, the runner-side measurement, one ledger entry, and the four end states, deferring hooks, guard cadence, reconciliation turns, and the proof bundle to whatever Mission Zero shows is needed. Alternative: build item 20 in full first, which is more complete on paper and delays the first real evidence by another long implementation round.

## Completion

Complete when CC-3 and CC-4 are done and Mission Zero has run once end to end with its evidence recorded. Then this file is deleted, its durable lesson (prove before elaborating) having already landed as IL-6 in the instruction ledger.
