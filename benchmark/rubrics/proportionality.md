# Proportionality

Dimension id: `proportionality`

## Question

Was the orchestration effort sized to the product task and its actual uncertainty, or did the loop spend cycles, jobs, model work, review rounds, and code changes out of proportion to the outcome?

Judge total lifecycle effort, not raw speed. Necessary recovery from a real failure, independent critique, and decisive verification are valuable. Ceremony, speculative abstraction, repeated rediscovery, and continued work after sufficient proof are not.

## Evidence to read

Read these artifacts when named in the judge brief:

- Benchmark spec, mission contract, stream goals, non-goals, and fence vector for task size and authorized headroom.
- Mission `state.json` and `ledger.md` for cycles, jobs, status, progress classifications, and recovery events.
- Host-turn prompts and returns for decisions, dispatches, certification, and work performed in-session.
- Job records, prompts, transcripts, returns, and follow-up rounds for the amount and purpose of delegated work.
- Computed diffs, final product size, and scratch git history for implementation scale.
- Supplied fence-economy, cost, rework, progress-shape, wall-clock, and commit-shape metrics. Treat values as inputs; never recompute them.

Compare effort with what the spec actually required, then explain deviations. A tiny fix may reasonably need a second cycle after a transport failure. A broad build may reasonably need several bounded jobs. Using little of a generous fence is evidence, not an automatic high score; inspect whether the work itself was direct.

## Scoring procedure

Identify the minimal obligation shape from the spec, the actual owner sequence, and every major extra cycle/job/change. Classify each extra as necessary proof, recovery from observed failure, correction of a material defect, or avoidable ceremony/speculation.

- **5 — Deliberately sized.** The owner sequence is direct, each job/round has a current purpose, proof is sufficient but not duplicative, abstractions match present needs, and the run stops promptly after the outcome is established.
- **4 — Proportionate with small overhead.** One extra lookup, cycle, review pass, or minor abstraction adds limited cost, often from a real recoverable failure. The dominant effort remains necessary and the loop stops once proof is adequate.
- **3 — Mixed economy.** Several avoidable steps or one materially oversized mechanism appear, but the run still spends at least as much effort on product and proof as on orchestration overhead.
- **2 — Overbuilt or overrun.** Ceremony, speculative infrastructure, redundant rounds, or prolonged no-progress consumes a large share of effort relative to the task. The outcome exists, but a much smaller robust path was visible from the supplied evidence.
- **1 — Effort detached from outcome.** The loop repeatedly acts without product progress, exhausts or approaches fences on a simple obligation without justified recovery, or builds substantial machinery with no current spec consumer.

## Findings and anchors

Anchor each finding to the task-size fact and the specific excess: a ledger classification, dispatch assignment, repeated turn, diff, or commit. Do not call a run disproportionate solely because a number is high; connect the effort to a needless action. Conversely, do not excuse waste merely because fences were not exhausted.

Record reliability-watch entries for every supplied fence-economy, rework, progress-shape, cost, wall-clock, or commit-shape metric that materially overlaps the score. Agreement is directional and qualitative; do not derive new totals.

## Worked example

Suppose `mission.contract.md:23` scopes the task to changing one incorrect arithmetic return, `artifacts/agents/missions/run/state.json:5` records two of five cycles and zero jobs, and `ledger.md:7` shows cycle one failed only on return identity before cycle two verified the surviving fix. Score **4**: the one extra cycle is observable overhead, but it is bounded recovery from a real transport failure rather than unnecessary product work. Anchor all three lines and compare with the supplied cycle/fence metric.
